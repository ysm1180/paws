package portforward

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/ysm1180/paws/internal/aws"
)

type Session struct {
	InstanceID   string
	InstanceType string
	LocalPort    int
	RemotePort   int
	Endpoint     string
	BastionID    string
	SessionInfo  *aws.SessionInfo
	Process      *exec.Cmd
	cancel       context.CancelFunc
	reconnecting bool
}

type Manager struct {
	client        *aws.Client
	sessions      map[string]*Session
	mu            sync.RWMutex
	onLog         func(instanceType, message string)
	ctx           context.Context
	autoReconnect bool
}

func NewManager(onLog func(instanceType, message string)) *Manager {
	return &Manager{
		sessions:      make(map[string]*Session),
		onLog:         onLog,
		autoReconnect: true,
	}
}

func (m *Manager) SetClient(client *aws.Client) {
	m.client = client
}

func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Manager) Start(ctx context.Context, client *aws.Client, instanceType, instanceID, endpoint string, localPort, remotePort int, bastionID string) error {
	m.mu.Lock()

	if existing, exists := m.sessions[instanceID]; exists {
		if !existing.reconnecting {
			m.mu.Unlock()
			return fmt.Errorf("session already exists for %s", instanceID)
		}
	}
	m.mu.Unlock()

	m.client = client
	m.ctx = ctx

	return m.startSession(ctx, client, instanceType, instanceID, endpoint, localPort, remotePort, bastionID, false)
}

func (m *Manager) startSession(ctx context.Context, client *aws.Client, instanceType, instanceID, endpoint string, localPort, remotePort int, bastionID string, isReconnect bool) error {
	sessionInfo, err := client.StartPortForwardingSession(ctx, bastionID, endpoint, localPort, remotePort)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	if isReconnect {
		m.log(instanceType, fmt.Sprintf("Reconnected: %s", sessionInfo.SessionID))
	} else {
		m.log(instanceType, fmt.Sprintf("Session started: %s", sessionInfo.SessionID))
	}

	sessionConfig := map[string]interface{}{
		"SessionId":        sessionInfo.SessionID,
		"StreamUrl":        sessionInfo.StreamURL,
		"TokenValue":       sessionInfo.TokenValue,
		"ResponseMetadata": map[string]interface{}{},
	}
	configJSON, _ := json.Marshal(sessionConfig)
	targetJSON, _ := json.Marshal(map[string]string{"Target": bastionID})

	pluginPath := getPluginPath()
	args := []string{
		string(configJSON),
		sessionInfo.Region,
		"StartSession",
		"",
		string(targetJSON),
		fmt.Sprintf("https://ssm.%s.amazonaws.com", sessionInfo.Region),
	}

	procCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procCtx, pluginPath, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	session := &Session{
		InstanceID:   instanceID,
		InstanceType: instanceType,
		LocalPort:    localPort,
		RemotePort:   remotePort,
		Endpoint:     endpoint,
		BastionID:    bastionID,
		SessionInfo:  sessionInfo,
		Process:      cmd,
		cancel:       cancel,
		reconnecting: false,
	}

	m.mu.Lock()
	m.sessions[instanceID] = session
	m.mu.Unlock()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			m.log(instanceType, fmt.Sprintf("[plugin] %s", scanner.Text()))
		}
	}()

	go m.monitorSession(session)

	go func() {
		cmd.Wait()
		m.handleSessionEnd(session)
	}()

	m.log(instanceType, fmt.Sprintf("Port forwarding: localhost:%d → %s:%d", localPort, endpoint, remotePort))
	return nil
}

func (m *Manager) monitorSession(session *Session) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		m.mu.RLock()
		currentSession, exists := m.sessions[session.InstanceID]
		m.mu.RUnlock()

		if !exists || currentSession.SessionInfo.SessionID != session.SessionInfo.SessionID {
			return
		}

		if !m.isPortOpen(session.LocalPort) {
			m.log(session.InstanceType, fmt.Sprintf("Port %d is not responding, checking session...", session.LocalPort))

			if session.Process != nil && session.Process.ProcessState != nil && session.Process.ProcessState.Exited() {
				m.handleSessionEnd(session)
				return
			}
		}
	}
}

func (m *Manager) isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (m *Manager) handleSessionEnd(session *Session) {
	m.mu.Lock()
	currentSession, exists := m.sessions[session.InstanceID]
	if !exists || currentSession.SessionInfo.SessionID != session.SessionInfo.SessionID {
		m.mu.Unlock()
		return
	}

	if currentSession.reconnecting {
		m.mu.Unlock()
		return
	}

	currentSession.reconnecting = true
	m.mu.Unlock()

	m.log(session.InstanceType, fmt.Sprintf("Session ended for %s", session.InstanceID))

	if !m.autoReconnect {
		m.mu.Lock()
		delete(m.sessions, session.InstanceID)
		m.mu.Unlock()
		return
	}

	m.log(session.InstanceType, fmt.Sprintf("Auto-reconnecting %s in 3 seconds...", session.InstanceID))
	time.Sleep(3 * time.Second)

	m.mu.RLock()
	stillExists := m.sessions[session.InstanceID] != nil
	m.mu.RUnlock()

	if !stillExists {
		return
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		m.mu.RLock()
		_, exists := m.sessions[session.InstanceID]
		m.mu.RUnlock()

		if !exists {
			return
		}

		err := m.startSession(m.ctx, m.client, session.InstanceType, session.InstanceID, session.Endpoint, session.LocalPort, session.RemotePort, session.BastionID, true)
		if err == nil {
			m.log(session.InstanceType, fmt.Sprintf("Successfully reconnected %s", session.InstanceID))
			return
		}

		m.log(session.InstanceType, fmt.Sprintf("Reconnect attempt %d/%d failed: %s", i+1, maxRetries, err))

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 5 * time.Second)
		}
	}

	m.log(session.InstanceType, fmt.Sprintf("Failed to reconnect %s after %d attempts", session.InstanceID, maxRetries))
	m.mu.Lock()
	delete(m.sessions, session.InstanceID)
	m.mu.Unlock()
}

func (m *Manager) Stop(instanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[instanceID]
	if !exists {
		return fmt.Errorf("no session for %s", instanceID)
	}

	session.reconnecting = false
	session.cancel()
	if session.Process != nil && session.Process.Process != nil {
		session.Process.Process.Kill()
	}
	delete(m.sessions, instanceID)

	m.log(session.InstanceType, fmt.Sprintf("Port forwarding stopped for %s", instanceID))
	return nil
}

func (m *Manager) IsActive(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.sessions[instanceID]
	return exists
}

func (m *Manager) GetActivePort(instanceID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.sessions[instanceID]; ok {
		return s.LocalPort
	}
	return 0
}

func (m *Manager) log(instanceType, message string) {
	if m.onLog != nil {
		m.onLog(instanceType, message)
	}
}

func getPluginPath() string {
	switch runtime.GOOS {
	case "windows":
		return "C:\\Program Files\\Amazon\\SessionManagerPlugin\\bin\\session-manager-plugin.exe"
	default:
		return "/usr/local/sessionmanagerplugin/bin/session-manager-plugin"
	}
}

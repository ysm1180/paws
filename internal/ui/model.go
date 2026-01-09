package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ysm1180/paws/internal/aws"
	"github.com/ysm1180/paws/internal/config"
	"github.com/ysm1180/paws/internal/portforward"
)

type tabType int

const (
	tabRDS tabType = iota
	tabElastiCache
)

type panel int

const (
	panelInstances panel = iota
	panelDetails
	panelLogs
)

type inputMode int

const (
	inputNone inputMode = iota
	inputFilter
	inputPort
	inputBastion
)

type Model struct {
	ctx           context.Context
	awsClient     *aws.Client
	config        *config.Config
	pfManager     *portforward.Manager
	width         int
	height        int
	currentTab    tabType
	activePanel   panel
	inputMode     inputMode
	rdsInstances  []aws.Instance
	ecInstances   []aws.Instance
	bastions      []aws.Instance
	cursor        int
	bastionIdx    int
	bastionCursor int
	filterInput   textinput.Model
	portInput     textinput.Model
	logViewport   viewport.Model
	logs          []string
	loading       bool
	err           error
}

type instancesLoadedMsg struct {
	rds      []aws.Instance
	ec       []aws.Instance
	bastions []aws.Instance
}

type logMsg struct {
	instanceType string
	message      string
}

type errMsg struct{ error }

func NewModel(ctx context.Context) (*Model, error) {
	client, err := aws.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	filterInput := textinput.New()
	filterInput.Placeholder = "filter..."
	filterInput.CharLimit = 50
	filterInput.Prompt = ""

	portInput := textinput.New()
	portInput.Placeholder = "port"
	portInput.CharLimit = 5
	portInput.Prompt = ""

	logViewport := viewport.New(80, 5)

	m := &Model{
		ctx:         ctx,
		awsClient:   client,
		config:      cfg,
		width:       100,
		height:      30,
		activePanel: panelInstances,
		filterInput: filterInput,
		portInput:   portInput,
		logViewport: logViewport,
		logs:        make([]string, 0),
		loading:     true,
	}

	m.pfManager = portforward.NewManager(func(instanceType, message string) {
		m.addLog(instanceType, message)
	})

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadInstances,
		textinput.Blink,
	)
}

func (m *Model) loadInstances() tea.Msg {
	rds, err := m.awsClient.ListRDSInstances(m.ctx)
	if err != nil {
		return errMsg{err}
	}

	ec, err := m.awsClient.ListElastiCacheInstances(m.ctx)
	if err != nil {
		return errMsg{err}
	}

	bastions, err := m.awsClient.ListEC2Instances(m.ctx)
	if err != nil {
		return errMsg{err}
	}

	return instancesLoadedMsg{rds: rds, ec: ec, bastions: bastions}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logViewport.Width = msg.Width - 4
		m.logViewport.Height = max(3, m.height/6)

	case instancesLoadedMsg:
		m.rdsInstances = msg.rds
		m.ecInstances = msg.ec
		m.bastions = msg.bastions
		m.loading = false
		m.restoreSettings()
		m.addLog("system", fmt.Sprintf("Loaded %d RDS, %d EC, %d EC2", len(msg.rds), len(msg.ec), len(msg.bastions)))

	case logMsg:
		m.addLog(msg.instanceType, msg.message)

	case errMsg:
		m.err = msg.error
		m.loading = false
		m.addLog("error", msg.Error())

	case tea.KeyMsg:
		if m.inputMode == inputBastion {
			return m.handleBastionInput(msg)
		}

		if m.inputMode == inputFilter {
			if key.Matches(msg, Keys.Escape) || key.Matches(msg, Keys.Enter) {
				m.filterInput.Blur()
				m.inputMode = inputNone
				m.cursor = 0
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}

		if m.inputMode == inputPort {
			if key.Matches(msg, Keys.Escape) || key.Matches(msg, Keys.Enter) {
				m.portInput.Blur()
				m.inputMode = inputNone
				return m, nil
			}
			var cmd tea.Cmd
			m.portInput, cmd = m.portInput.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, Keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, Keys.Tab):
			m.activePanel = (m.activePanel + 1) % 3

		case key.Matches(msg, Keys.ShiftTab):
			m.activePanel = (m.activePanel + 2) % 3

		case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
			m.currentTab = tabRDS
			m.cursor = 0
			m.restoreSettings()

		case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
			m.currentTab = tabElastiCache
			m.cursor = 0
			m.restoreSettings()

		case key.Matches(msg, Keys.Up):
			if m.activePanel == panelInstances {
				if m.cursor > 0 {
					m.cursor--
					m.restoreSettings()
				}
			} else if m.activePanel == panelLogs {
				m.logViewport.LineUp(1)
			}

		case key.Matches(msg, Keys.Down):
			if m.activePanel == panelInstances {
				instances := m.filteredInstances()
				if m.cursor < len(instances)-1 {
					m.cursor++
					m.restoreSettings()
				}
			} else if m.activePanel == panelLogs {
				m.logViewport.LineDown(1)
			}

		case key.Matches(msg, Keys.Filter):
			m.inputMode = inputFilter
			m.filterInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, key.NewBinding(key.WithKeys("p"))):
			m.inputMode = inputPort
			m.portInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, key.NewBinding(key.WithKeys("b"))):
			if len(m.bastions) > 0 {
				m.inputMode = inputBastion
				m.bastionCursor = m.bastionIdx
			}

		case key.Matches(msg, Keys.Connect):
			return m, m.startPortForwarding

		case key.Matches(msg, Keys.Disconnect):
			return m, m.stopPortForwarding

		case key.Matches(msg, Keys.Refresh):
			m.loading = true
			return m, m.loadInstances
		}
	}

	return m, nil
}

func (m *Model) handleBastionInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, Keys.Escape):
		m.inputMode = inputNone

	case key.Matches(msg, Keys.Enter):
		m.bastionIdx = m.bastionCursor
		m.saveBastion()
		m.inputMode = inputNone

	case key.Matches(msg, Keys.Up):
		if m.bastionCursor > 0 {
			m.bastionCursor--
		}

	case key.Matches(msg, Keys.Down):
		if m.bastionCursor < len(m.bastions)-1 {
			m.bastionCursor++
		}
	}

	return m, nil
}

func (m *Model) View() string {
	if m.width < 60 || m.height < 12 {
		return "Terminal too small"
	}

	var view string
	if m.width < 100 {
		view = m.renderCompactView()
	} else {
		view = m.renderFullView()
	}

	if m.inputMode == inputBastion {
		return m.overlayBastionPopup(view)
	}

	return view
}

func (m *Model) renderCompactView() string {
	instances := m.filteredInstances()
	listHeight := m.height - 10

	var content strings.Builder

	content.WriteString("\n")
	content.WriteString(PanelTitleStyle.Render(" Paws "))
	content.WriteString("\n")
	content.WriteString(m.renderTabs())
	content.WriteString("\n")

	start := 0
	if m.cursor >= listHeight {
		start = m.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(instances) {
		end = len(instances)
	}

	if len(instances) == 0 {
		if m.loading {
			content.WriteString(StatusLoadingStyle.Render(fmt.Sprintf(" %s Loading...", IconLoading)))
		} else {
			content.WriteString(LabelStyle.Render(" No instances"))
		}
	} else {
		for i := start; i < end; i++ {
			inst := instances[i]
			isActive := m.pfManager.IsActive(inst.ID)
			isSelected := i == m.cursor

			icon := IconDisconnected
			suffix := ""

			if isActive {
				icon = IconConnected
				suffix = fmt.Sprintf(" :%d", m.pfManager.GetActivePort(inst.ID))
			}

			id := truncate(inst.ID, m.width-10)
			line := fmt.Sprintf(" %s %s%s", icon, id, suffix)

			if isSelected {
				content.WriteString(ListItemSelectedStyle.Render(line))
			} else if isActive {
				content.WriteString(ListItemActiveStyle.Render(line))
			} else {
				content.WriteString(ListItemStyle.Render(line))
			}
			content.WriteString("\n")
		}
	}

	inst := m.selectedInstance()
	if inst != nil {
		content.WriteString("\n")
		portStyle := InputStyle
		if m.inputMode == inputPort {
			portStyle = InputFocusedStyle
		}

		bastionName := "none"
		if len(m.bastions) > 0 && m.bastionIdx < len(m.bastions) {
			bastionName = truncate(m.bastions[m.bastionIdx].Name, 15)
		}

		info := fmt.Sprintf(" Port:%s Bastion:%s",
			portStyle.Render(m.portInput.View()),
			ValueHighlightStyle.Render(bastionName))
		content.WriteString(info)
	}

	if m.inputMode == inputFilter || m.filterInput.Value() != "" {
		style := InputStyle
		if m.inputMode == inputFilter {
			style = InputFocusedStyle
		}
		content.WriteString("\n")
		content.WriteString(style.Render("/" + m.filterInput.View()))
	}

	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, content.String(), helpBar)
}

func (m *Model) renderFullView() string {
	header := "\n" + PanelTitleStyle.Render(" Paws ")

	leftWidth := m.width * 40 / 100
	rightWidth := m.width - leftWidth - 2

	contentHeight := m.height - 4

	leftPanel := m.renderInstancesPanel(leftWidth, contentHeight)

	detailHeight := contentHeight * 55 / 100
	logHeight := contentHeight - detailHeight

	rightPanel := lipgloss.JoinVertical(lipgloss.Left,
		m.renderDetailsPanel(rightWidth, detailHeight),
		m.renderLogsPanel(rightWidth, logHeight),
	)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, mainContent, helpBar)
}

func (m *Model) overlayBastionPopup(base string) string {
	popupWidth := min(50, m.width-4)

	var content strings.Builder
	content.WriteString(PopupTitleStyle.Render(" Select Bastion "))
	content.WriteString("\n")

	maxVisible := min(8, m.height-6)
	start := 0
	if m.bastionCursor >= maxVisible {
		start = m.bastionCursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.bastions) {
		end = len(m.bastions)
	}

	for i := start; i < end; i++ {
		b := m.bastions[i]
		prefix := "  "
		style := PopupItemStyle

		if i == m.bastionCursor {
			prefix = IconArrowRight + " "
			style = PopupItemSelectedStyle
		}

		icon := IconServer
		if i == m.bastionIdx {
			icon = IconSuccess
		}

		name := truncate(b.Name, popupWidth-8)
		content.WriteString(style.Render(fmt.Sprintf("%s%s %s", prefix, icon, name)))
		if i < end-1 {
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(PopupHintStyle.Render("↑↓ enter esc"))

	popup := PopupStyle.Width(popupWidth).Render(content.String())

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		popup,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1E1E2E")),
	)
}

func (m *Model) renderInstancesPanel(width, height int) string {
	isFocused := m.activePanel == panelInstances

	tabBar := m.renderTabs()
	instances := m.filteredInstances()

	listHeight := height - 5
	if listHeight < 3 {
		listHeight = 3
	}

	var listContent strings.Builder

	start := 0
	if m.cursor >= listHeight {
		start = m.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(instances) {
		end = len(instances)
	}

	if len(instances) == 0 {
		if m.loading {
			listContent.WriteString(StatusLoadingStyle.Render(fmt.Sprintf(" %s Loading...", IconLoading)))
		} else {
			listContent.WriteString(LabelStyle.Render(" No instances"))
		}
	} else {
		for i := start; i < end; i++ {
			inst := instances[i]
			isActive := m.pfManager.IsActive(inst.ID)
			isSelected := i == m.cursor

			icon := IconDisconnected
			suffix := ""

			if isActive {
				icon = IconConnected
				suffix = fmt.Sprintf(" :%d", m.pfManager.GetActivePort(inst.ID))
			}

			id := truncate(inst.ID, width-8)
			line := fmt.Sprintf(" %s %s%s", icon, id, suffix)

			if isSelected {
				listContent.WriteString(ListItemSelectedStyle.Render(line))
			} else if isActive {
				listContent.WriteString(ListItemActiveStyle.Render(line))
			} else {
				listContent.WriteString(ListItemStyle.Render(line))
			}

			if i < end-1 {
				listContent.WriteString("\n")
			}
		}
	}

	filterBar := ""
	if m.inputMode == inputFilter || m.filterInput.Value() != "" {
		style := InputStyle
		if m.inputMode == inputFilter {
			style = InputFocusedStyle
		}
		filterBar = style.Width(width - 4).Render("/" + m.filterInput.View())
	}

	scrollInfo := ""
	if len(instances) > listHeight {
		scrollInfo = LabelStyle.Render(fmt.Sprintf(" %d/%d", m.cursor+1, len(instances)))
	}

	title := m.panelTitle("Instances", isFocused) + scrollInfo

	content := tabBar + "\n" + listContent.String()
	if filterBar != "" {
		content += "\n" + filterBar
	}

	panelStyle := PanelStyle
	if isFocused {
		panelStyle = PanelFocusedStyle
	}

	panel := panelStyle.Width(width).Height(height - 1).Render(content)

	return title + "\n" + panel
}

func (m *Model) renderDetailsPanel(width, height int) string {
	isFocused := m.activePanel == panelDetails
	title := m.panelTitle("Details", isFocused)

	inst := m.selectedInstance()
	var content strings.Builder

	if inst == nil {
		content.WriteString(LabelStyle.Render(" No instance selected"))
	} else {
		isActive := m.pfManager.IsActive(inst.ID)

		statusIcon := IconDisconnected
		statusStyle := StatusDisconnectedStyle
		if isActive {
			statusIcon = IconConnected
			statusStyle = StatusConnectedStyle
		}

		content.WriteString(fmt.Sprintf(" %s %s\n", statusStyle.Render(statusIcon), ValueStyle.Render(inst.ID)))
		content.WriteString(fmt.Sprintf(" %s %s\n", LabelStyle.Render("Type:"), ValueStyle.Render(inst.Type)))
		content.WriteString(fmt.Sprintf(" %s %s\n", LabelStyle.Render("Host:"), ValueStyle.Render(truncate(inst.Endpoint, width-10))))

		portStyle := InputStyle
		if m.inputMode == inputPort {
			portStyle = InputFocusedStyle
		}
		content.WriteString(fmt.Sprintf(" %s %s\n", LabelStyle.Render("Port:"), portStyle.Render(m.portInput.View())))

		bastionText := LabelStyle.Render("none")
		if len(m.bastions) > 0 && m.bastionIdx < len(m.bastions) {
			bastionText = ValueHighlightStyle.Render(truncate(m.bastions[m.bastionIdx].Name, width-12))
		}
		content.WriteString(fmt.Sprintf(" %s %s %s", LabelStyle.Render("Bastion:"), bastionText, LabelStyle.Render("[b]")))

		if isActive {
			port := m.pfManager.GetActivePort(inst.ID)
			content.WriteString(fmt.Sprintf("\n %s", StatusConnectedStyle.Render(fmt.Sprintf("%s localhost:%d", IconSuccess, port))))
		}
	}

	panelStyle := PanelStyle
	if isFocused {
		panelStyle = PanelFocusedStyle
	}

	panel := panelStyle.Width(width).Height(height - 1).Render(content.String())

	return title + "\n" + panel
}

func (m *Model) renderLogsPanel(width, height int) string {
	isFocused := m.activePanel == panelLogs
	title := m.panelTitle("Logs", isFocused)

	var logContent strings.Builder
	maxLogs := 30
	startIdx := 0
	if len(m.logs) > maxLogs {
		startIdx = len(m.logs) - maxLogs
	}

	for i := startIdx; i < len(m.logs); i++ {
		logContent.WriteString(m.logs[i])
		if i < len(m.logs)-1 {
			logContent.WriteString("\n")
		}
	}

	m.logViewport.Width = width - 2
	m.logViewport.Height = height - 2
	m.logViewport.SetContent(logContent.String())
	if !isFocused {
		m.logViewport.GotoBottom()
	}

	panelStyle := PanelStyle
	if isFocused {
		panelStyle = PanelFocusedStyle
	}

	panel := panelStyle.Width(width).Height(height - 1).Render(m.logViewport.View())

	return title + "\n" + panel
}

func (m *Model) renderTabs() string {
	rdsStyle := TabInactiveStyle
	ecStyle := TabInactiveStyle

	if m.currentTab == tabRDS {
		rdsStyle = TabActiveStyle
	} else {
		ecStyle = TabActiveStyle
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		rdsStyle.Render(fmt.Sprintf("%s RDS(%d)", IconDatabase, len(m.rdsInstances))),
		ecStyle.Render(fmt.Sprintf("%s EC(%d)", IconCache, len(m.ecInstances))),
	)
}

func (m *Model) renderHelpBar() string {
	var items []string

	if m.inputMode == inputBastion {
		items = append(items, m.helpItem("↑↓", "sel"), m.helpItem("⏎", "ok"), m.helpItem("esc", "×"))
	} else {
		items = append(items, m.helpItem("↑↓", "nav"))
		items = append(items, m.helpItem("1/2", "tab"))
		items = append(items, m.helpItem("c/d", "conn"))
		items = append(items, m.helpItem("b", "bastion"))
		items = append(items, m.helpItem("p", "port"))
		items = append(items, m.helpItem("/", "filter"))
		items = append(items, m.helpItem("q", "quit"))
	}

	return HelpBarStyle.Width(m.width).Render(strings.Join(items, " "))
}

func (m *Model) helpItem(k, desc string) string {
	return HelpKeyStyle.Render(k) + HelpDescStyle.Render(desc)
}

func (m *Model) panelTitle(title string, focused bool) string {
	style := PanelTitleStyle
	if focused {
		style = PanelTitleFocusedStyle
	}
	return style.Render(title)
}

func (m *Model) filteredInstances() []aws.Instance {
	var instances []aws.Instance
	if m.currentTab == tabRDS {
		instances = m.rdsInstances
	} else {
		instances = m.ecInstances
	}

	filter := strings.ToLower(m.filterInput.Value())
	if filter == "" {
		return instances
	}

	var filtered []aws.Instance
	for _, inst := range instances {
		if strings.Contains(strings.ToLower(inst.ID), filter) ||
			strings.Contains(strings.ToLower(inst.Endpoint), filter) {
			filtered = append(filtered, inst)
		}
	}
	return filtered
}

func (m *Model) selectedInstance() *aws.Instance {
	instances := m.filteredInstances()
	if m.cursor >= 0 && m.cursor < len(instances) {
		return &instances[m.cursor]
	}
	return nil
}

func (m *Model) currentTabType() string {
	if m.currentTab == tabRDS {
		return "RDS"
	}
	return "ElastiCache"
}

func (m *Model) restoreSettings() {
	inst := m.selectedInstance()
	if inst == nil {
		return
	}

	if savedPort := m.config.GetSavedPort(m.currentTabType(), inst.ID); savedPort > 0 {
		m.portInput.SetValue(strconv.Itoa(savedPort))
	} else {
		m.portInput.SetValue(strconv.Itoa(inst.Port))
	}

	if savedBastion := m.config.GetSavedBastion(inst.ID); savedBastion != "" {
		for i, b := range m.bastions {
			if b.ID == savedBastion {
				m.bastionIdx = i
				break
			}
		}
	}
}

func (m *Model) saveBastion() {
	inst := m.selectedInstance()
	if inst == nil || m.bastionIdx >= len(m.bastions) {
		return
	}
	m.config.SetBastion(inst.ID, m.bastions[m.bastionIdx].ID)
	m.config.Save()
}

func (m *Model) startPortForwarding() tea.Msg {
	inst := m.selectedInstance()
	if inst == nil {
		return logMsg{"error", "No instance selected"}
	}

	if len(m.bastions) == 0 || m.bastionIdx >= len(m.bastions) {
		return logMsg{"error", "No bastion selected"}
	}

	localPort, err := strconv.Atoi(m.portInput.Value())
	if err != nil || localPort < 1024 || localPort > 65535 {
		return logMsg{"error", "Invalid port (1024-65535)"}
	}

	bastion := m.bastions[m.bastionIdx]

	m.config.SetPort(m.currentTabType(), inst.ID, localPort)
	m.config.SetBastion(inst.ID, bastion.ID)
	m.config.Save()

	err = m.pfManager.Start(m.ctx, m.awsClient, m.currentTabType(), inst.ID, inst.Endpoint, localPort, inst.Port, bastion.ID)
	if err != nil {
		return logMsg{"error", err.Error()}
	}

	return logMsg{m.currentTabType(), fmt.Sprintf("%s %s :%d", IconSuccess, inst.ID, localPort)}
}

func (m *Model) stopPortForwarding() tea.Msg {
	inst := m.selectedInstance()
	if inst == nil {
		return logMsg{"error", "No instance selected"}
	}

	if err := m.pfManager.Stop(inst.ID); err != nil {
		return logMsg{"error", err.Error()}
	}

	return logMsg{m.currentTabType(), fmt.Sprintf("%s %s", IconDisconnected, inst.ID)}
}

func (m *Model) addLog(instanceType, message string) {
	timestamp := time.Now().Format("15:04:05")

	timeStr := LogTimeStyle.Render(timestamp)

	var msgStyle lipgloss.Style
	switch instanceType {
	case "error":
		msgStyle = LogErrorStyle
	case "system":
		msgStyle = LogInfoStyle
	default:
		msgStyle = LogSuccessStyle
	}

	m.logs = append(m.logs, fmt.Sprintf("%s %s", timeStr, msgStyle.Render(message)))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

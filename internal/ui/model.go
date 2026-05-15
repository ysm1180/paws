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
	"github.com/ysm1180/paws/internal/transfer"
)

type tabType int

const (
	tabRDS tabType = iota
	tabElastiCache
	tabEC2
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
	ec2Instances  []aws.Instance
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

	// Render-to-input side channel: written by render funcs, read by
	// handleMouseMsg on the next MouseMsg. Not application state.
	tabRegions        []Region
	instancesListRect Region
	instancesListRow0 int // first visible row index (scroll offset)
	logsRect          Region

	screen        screen
	tm            *transfer.Manager
	browseShell   *aws.ShellSession
	transferShell *aws.ShellSession
	browser       *fileBrowserState
}

type instancesLoadedMsg struct {
	rds      []aws.Instance
	ec       []aws.Instance
	ec2      []aws.Instance
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

	m.tm = transfer.NewManager()

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadInstances,
		textinput.Blink,
		tickEvery(),
	)
}

func tickEvery() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
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

	ec2, err := m.awsClient.ListSsmManagedEC2Instances(m.ctx)
	if err != nil {
		return errMsg{err}
	}

	return instancesLoadedMsg{rds: rds, ec: ec, ec2: ec2, bastions: bastions}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logViewport.Width = msg.Width - 4
		m.logViewport.Height = max(3, m.height/6)

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)

	case instancesLoadedMsg:
		m.rdsInstances = msg.rds
		m.ecInstances = msg.ec
		m.ec2Instances = msg.ec2
		m.bastions = msg.bastions
		m.loading = false
		m.restoreSettings()
		m.addLog("system", fmt.Sprintf("Loaded %d RDS, %d EC, %d EC2(SSM), %d EC2(all)", len(msg.rds), len(msg.ec), len(msg.ec2), len(msg.bastions)))

	case logMsg:
		m.addLog(msg.instanceType, msg.message)

	case errMsg:
		m.err = msg.error
		m.loading = false
		m.addLog("error", msg.Error())

	case browserOpenedMsg:
		// All mutations of m.browseShell/m.transferShell happen here, in
		// Update — NEVER inside a tea.Cmd goroutine.
		m.browseShell = msg.browseSh
		m.transferShell = msg.transSh
		m.screen = screenBrowser
		m.browser = newFileBrowserState(msg.instance, msg.cwd, msg.entries)

	case browserOpenFailedMsg:
		if msg.browseSh != nil {
			id := msg.browseSh.SessionID()
			_ = msg.browseSh.Close()
			_ = m.awsClient.TerminateSession(m.ctx, id)
		}
		if msg.transSh != nil {
			id := msg.transSh.SessionID()
			_ = msg.transSh.Close()
			_ = m.awsClient.TerminateSession(m.ctx, id)
		}
		m.addLog("error", msg.err)

	case browserListedMsg:
		if m.browser != nil {
			m.browser.applyListing(msg.cwd, msg.entries, msg.err)
			if msg.err == nil {
				m.config.SetEC2Cwd(m.browser.instance.ID, msg.cwd)
				_ = m.config.Save()
			}
		}

	case transferProgressMsg, transferDoneMsg:
		// Re-render on next tick; nothing to mutate beyond the Job itself.

	case browserClosedMsg:
		m.closeBrowserSessions()
		m.screen = screenList
		m.browser = nil

	case tickMsg:
		return m, tickEvery()

	case tea.KeyMsg:
		if m.screen == screenBrowser {
			return m.handleBrowserKey(msg)
		}
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
			m.setTab(tabRDS)

		case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
			m.setTab(tabElastiCache)

		case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
			m.setTab(tabEC2)

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

		case key.Matches(msg, Keys.Enter):
			if m.currentTab == tabEC2 && m.screen == screenList && m.activePanel == panelInstances {
				if inst := m.selectedInstance(); inst != nil {
					m.addLog("system", fmt.Sprintf("Connecting to %s via SSM (up to 15s)...", inst.ID))
				}
				return m, m.openBrowserCmd()
			}
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

	if m.screen == screenBrowser && m.browser != nil {
		return m.renderFileBrowser()
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

			line := formatInstanceRow(inst, icon, suffix, m.width)

			style := ListItemStyle
			if isSelected {
				style = ListItemSelectedStyle
			} else if isActive {
				style = ListItemActiveStyle
			}
			content.WriteString(style.Width(m.width).Render(line))
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

// renderFullView is a vertical stack at full terminal width. JoinHorizontal
// arithmetic was fragile: when a row in any column overflowed its budget,
// it pushed adjacent columns off-screen.
func (m *Model) renderFullView() string {
	header := strings.Repeat("\n", HeaderTopGap) + PanelTitleStyle.Render(" Paws ")
	if HeaderToBodyGap > 0 {
		header += strings.Repeat("\n", HeaderToBodyGap)
	}

	headerRows := HeaderTopGap + 1 + HeaderToBodyGap
	contentHeight := m.height - headerRows - HelpBarHeight
	if contentHeight < 9 {
		contentHeight = 9
	}

	detailsHeight := 8

	// Cap Instances at what its content actually needs (header + tabs +
	// list rows + optional filter + optional hint), with a floor so the
	// panel feels substantial even when only a few items are loaded.
	// Without the cap the remainder math gave Instances every leftover
	// row; without the floor a short list shrinks Instances to almost
	// nothing.
	instances := m.filteredInstances()
	filterRows := 0
	if m.inputMode == inputFilter || m.filterInput.Value() != "" {
		filterRows = 1
	}
	hintRows := 0
	if m.activePanel == panelInstances {
		hintRows = 1
	}
	const (
		instancesChromeRows = 4 // panelHeader(2) + tabBar(2)
		instancesAbsFloor   = 16
		logsMin             = 8
	)
	// Floor: max(absolute floor, ~55% of body). 55% gives Instances the
	// dominant share without re-introducing the old "fills the screen"
	// problem on tall terminals.
	instancesFloor := contentHeight * 55 / 100
	if instancesFloor < instancesAbsFloor {
		instancesFloor = instancesAbsFloor
	}
	roomForInstances := contentHeight - detailsHeight - logsMin
	if instancesFloor > roomForInstances {
		instancesFloor = roomForInstances
	}
	if instancesFloor < 9 {
		instancesFloor = 9
	}
	needed := instancesChromeRows + len(instances) + filterRows + hintRows
	if needed < instancesFloor {
		needed = instancesFloor
	}
	instancesHeight := needed
	if instancesHeight > roomForInstances {
		instancesHeight = roomForInstances
	}
	logsHeight := contentHeight - detailsHeight - instancesHeight
	if logsHeight < logsMin {
		logsHeight = logsMin
	}

	instancesPanel := m.renderInstancesPanel(m.width, instancesHeight)
	details := m.renderDetailsPanel(m.width, detailsHeight)
	logs := m.renderLogsPanel(m.width, logsHeight)

	m.logsRect = Region{
		X: 0,
		Y: headerRows + instancesHeight + detailsHeight,
		W: m.width,
		H: logsHeight,
	}

	body := lipgloss.JoinVertical(lipgloss.Left, instancesPanel, details, logs)
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, helpBar)
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

func (m *Model) panelHeader(name string, focused bool, width int, scrollInfo string) string {
	titleStyle := PanelTitleStyle
	sepColor := ColorBorder
	if focused {
		titleStyle = PanelTitleFocusedStyle
		sepColor = ColorBorderFocus
	}
	chip := titleStyle.Render(name) + scrollInfo
	sep := lipgloss.NewStyle().Foreground(sepColor).Render(strings.Repeat("─", width))
	return chip + "\n" + sep
}

// fitBlock pads or clips content to exactly width × height cells. Lipgloss
// pads with trailing spaces when Width is set; this also clips overflow rows
// which Lipgloss does not.
func fitBlock(content string, width, height int) string {
	if height < 1 {
		height = 1
	}
	if width < 1 {
		width = 1
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderInstancesPanel(width, height int) string {
	isFocused := m.activePanel == panelInstances

	tabBar, tabRegions := m.renderTabsWithRegions()
	// Absolute Y so handleMouseMsg can compare directly against MouseMsg.Y.
	tabBarAbsY := HeaderTopGap + 1 + HeaderToBodyGap + PanelHeaderRows
	for i := range tabRegions {
		tabRegions[i].Y = tabBarAbsY
	}
	m.tabRegions = tabRegions
	instances := m.filteredInstances()

	tabsHeight := 2
	filterHeight := 0
	if m.inputMode == inputFilter || m.filterInput.Value() != "" {
		filterHeight = 1
	}
	hintHeight := 0
	if isFocused {
		hintHeight = 1
	}
	// listHeight excludes the hint row so the bottom-anchored hint never
	// overlaps the last list entry.
	listHeight := height - 2 - tabsHeight - filterHeight - hintHeight
	if listHeight < 3 {
		listHeight = 3
	}

	rowWidth := width
	if rowWidth < 1 {
		rowWidth = 1
	}

	start := 0
	if m.cursor >= listHeight {
		start = m.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(instances) {
		end = len(instances)
	}

	var listContent strings.Builder
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

			line := formatInstanceRow(inst, icon, suffix, rowWidth)

			style := ListItemStyle
			if isSelected {
				style = ListItemSelectedStyle
			} else if isActive {
				style = ListItemActiveStyle
			}
			listContent.WriteString(style.Width(rowWidth).Render(line))
			if i < end-1 {
				listContent.WriteString("\n")
			}
		}
	}

	filterBar := ""
	if filterHeight > 0 {
		style := InputStyle
		if m.inputMode == inputFilter {
			style = InputFocusedStyle
		}
		filterBar = style.Width(width - 2).Render("/" + m.filterInput.View())
	}

	scrollInfo := ""
	if len(instances) > listHeight {
		scrollInfo = LabelStyle.Render(fmt.Sprintf(" %d/%d", m.cursor+1, len(instances)))
	}

	header := m.panelHeader("Instances", isFocused, width, scrollInfo)

	topBody := tabBar + "\n" + listContent.String()
	if filterBar != "" {
		topBody += "\n" + filterBar
	}
	// Render the non-hint area at its full height first; this pads with
	// blank rows below the list so the hint lands on the panel's last row
	// instead of floating right under the list when it's short.
	topBlock := fitBlock(topBody, width, height-2-hintHeight)
	body := topBlock
	if hintHeight > 0 {
		body += "\n" + renderPanelHint(panelInstances, m.currentTab)
	}

	listStartAbsY := tabBarAbsY + TabsToListGap
	m.instancesListRect = Region{X: 0, Y: listStartAbsY, W: width, H: listHeight}
	m.instancesListRow0 = start

	return header + "\n" + body
}

func (m *Model) renderDetailsPanel(width, height int) string {
	isFocused := m.activePanel == panelDetails

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
		if inst.Name != "" && inst.Name != inst.ID {
			content.WriteString(fmt.Sprintf(" %s %s\n", LabelStyle.Render("Name:"), ValueStyle.Render(truncate(inst.Name, width-10))))
		}
		content.WriteString(fmt.Sprintf(" %s %s\n", LabelStyle.Render("Type:"), ValueStyle.Render(inst.Type)))
		if inst.Endpoint != "" {
			content.WriteString(fmt.Sprintf(" %s %s\n", LabelStyle.Render("Host:"), ValueStyle.Render(truncate(inst.Endpoint, width-10))))
		}

		// EC2 tab uses SSM file browser (Enter); port-forwarding fields
		// don't apply, so hide them rather than show inputs that go nowhere.
		if m.currentTab != tabEC2 {
			portStyle := InlineInputStyle
			if m.inputMode == inputPort {
				portStyle = InlineInputFocusedStyle
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
	}

	header := m.panelHeader("Details", isFocused, width, "")
	hintStr := ""
	if isFocused {
		hintStr = renderPanelHint(panelDetails, m.currentTab)
	}
	hintHeight := 0
	if hintStr != "" {
		hintHeight = 1
	}
	topBlock := fitBlock(content.String(), width, height-2-hintHeight)
	body := topBlock
	if hintHeight > 0 {
		body += "\n" + hintStr
	}

	return header + "\n" + body
}

func (m *Model) renderLogsPanel(width, height int) string {
	isFocused := m.activePanel == panelLogs

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

	bodyHeight := height - 2
	if isFocused {
		// Reserve one row for the hint line so the viewport doesn't
		// scribble over it.
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.logViewport.Width = width
	m.logViewport.Height = bodyHeight
	m.logViewport.SetContent(logContent.String())
	if !isFocused {
		m.logViewport.GotoBottom()
	}

	header := m.panelHeader("Logs", isFocused, width, "")
	viewportBlock := lipgloss.NewStyle().Width(width).Height(bodyHeight).Render(m.logViewport.View())

	body := viewportBlock
	if isFocused {
		body += "\n" + renderPanelHint(panelLogs, m.currentTab)
	}

	// logsRect is owned by renderFullView (it knows the absolute Y).

	return header + "\n" + body
}

// renderTabsWithRegions returns the rendered tab bar and the x-range each
// tab occupies. X is column 0; callers placing the bar at a non-zero column
// must offset X themselves. Height is always 1.
func (m *Model) renderTabsWithRegions() (string, []Region) {
	labels := []string{
		fmt.Sprintf("%s RDS(%d)", IconDatabase, len(m.rdsInstances)),
		fmt.Sprintf("%s EC(%d)", IconCache, len(m.ecInstances)),
		fmt.Sprintf("%s EC2(%d)", IconServer, len(m.ec2Instances)),
	}
	active := []tabType{tabRDS, tabElastiCache, tabEC2}

	regions := make([]Region, 3)
	rendered := make([]string, 3)
	cursor := 0
	for i, label := range labels {
		style := TabInactiveStyle
		if m.currentTab == active[i] {
			style = TabActiveStyle
		}
		s := style.Render(label)
		w := lipgloss.Width(s)
		rendered[i] = s
		regions[i] = Region{X: cursor, Y: 0, W: w, H: 1, Index: i}
		cursor += w
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...), regions
}

// renderTabs drops the regions. Compact view's only caller doesn't support
// mouse clicks; wiring mouse to compact view requires switching this call
// site to renderTabsWithRegions and stashing m.tabRegions.
func (m *Model) renderTabs() string {
	s, _ := m.renderTabsWithRegions()
	return s
}

func (m *Model) renderHelpBar() string {
	var items []string

	if m.inputMode == inputBastion {
		items = append(items, helpPair("↑↓", "sel"), helpPair("⏎", "ok"), helpPair("esc", "×"))
	} else {
		items = append(items,
			helpPair("1/2/3", "tab"),
			helpPair("⇥", "focus"),
			helpPair("r", "reload"),
			helpPair("q", "quit"),
		)
	}

	// Drop items from the right until they fit. Byte-slicing the joined
	// string would cut through ANSI escapes and corrupt the terminal.
	maxLen := m.width - 2 // HelpBarStyle Padding(0,1)
	if maxLen < 0 {
		maxLen = 0
	}
	fits := items
	for len(fits) > 0 && lipgloss.Width(strings.Join(fits, "  ")) > maxLen {
		fits = fits[:len(fits)-1]
	}
	return HelpBarStyle.Width(m.width).Render(strings.Join(fits, "  "))
}

func (m *Model) panelTitle(title string, focused bool) string {
	style := PanelTitleStyle
	if focused {
		style = PanelTitleFocusedStyle
	}
	return style.Render(title)
}

// formatInstanceRow renders one Instances row as "<name>  <id-dim><suffix>"
// when Name differs from ID (EC2: ID is "i-…", Name is the Tag:Name) and as
// just "<id><suffix>" otherwise. When the row is too narrow for both, the id
// is dropped rather than the name truncated to nothing.
func formatInstanceRow(inst aws.Instance, icon, suffix string, rowWidth int) string {
	const rowChromeCells = 3 // " " + 1-cell icon + " "
	const nameToIDGap = 2
	const minNameCells = 8
	avail := rowWidth - rowChromeCells - len(suffix)
	if avail < 1 {
		avail = 1
	}

	primary := inst.ID
	secondary := ""
	if inst.Name != "" && inst.Name != inst.ID {
		primary = inst.Name
		secondary = inst.ID
	}

	if secondary != "" && len(secondary)+nameToIDGap+minNameCells <= avail {
		primaryMax := avail - nameToIDGap - len(secondary)
		primary = truncate(primary, primaryMax)
		return fmt.Sprintf(" %s %s  %s%s", icon, primary, LabelStyle.Render(secondary), suffix)
	}
	primary = truncate(primary, avail)
	return fmt.Sprintf(" %s %s%s", icon, primary, suffix)
}

// setTab also resets focus to the Instances list — Enter's open-browser
// branch is gated on activePanel == panelInstances, so leaving focus on
// Details/Logs after a tab switch would make Enter silently no-op.
func (m *Model) setTab(t tabType) {
	m.currentTab = t
	m.cursor = 0
	m.activePanel = panelInstances
	m.restoreSettings()
}

func (m *Model) filteredInstances() []aws.Instance {
	var instances []aws.Instance
	switch m.currentTab {
	case tabRDS:
		instances = m.rdsInstances
	case tabElastiCache:
		instances = m.ecInstances
	case tabEC2:
		instances = m.ec2Instances
	}

	filter := strings.ToLower(m.filterInput.Value())
	if filter == "" {
		return instances
	}

	var filtered []aws.Instance
	for _, inst := range instances {
		if strings.Contains(strings.ToLower(inst.ID), filter) ||
			strings.Contains(strings.ToLower(inst.Name), filter) ||
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
	switch m.currentTab {
	case tabRDS:
		return "RDS"
	case tabElastiCache:
		return "ElastiCache"
	case tabEC2:
		return "EC2"
	}
	return ""
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

// openBrowserCmd returns a Cmd that opens two SSM sessions and the initial
// directory listing. The Cmd must NOT touch Model state directly — it
// returns everything via browserOpened/FailedMsg so assignments happen
// inside Update on the main goroutine. Bounded by a 15s timeout so a hung
// session-manager-plugin can't freeze the UI; the plugin's stderr is
// appended on failure (most reliable clue for SSM session deaths).
func (m *Model) openBrowserCmd() tea.Cmd {
	inst := m.selectedInstance()
	if inst == nil {
		return func() tea.Msg { return logMsg{"error", "no instance"} }
	}
	target := *inst
	parentCtx := m.ctx
	client := m.awsClient
	cfgCwd := m.config.GetEC2Cwd(target.ID)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Second)
		defer cancel()

		bSh, err := client.StartShellSession(ctx, target.ID)
		if err != nil {
			return browserOpenFailedMsg{err: fmt.Sprintf("browse session: %v", err)}
		}
		tSh, err := client.StartShellSession(ctx, target.ID)
		if err != nil {
			return browserOpenFailedMsg{
				browseSh: bSh,
				err:      withStderr(fmt.Sprintf("transfer session: %v", err), bSh),
			}
		}
		cwd := cfgCwd
		if cwd == "" {
			h, herr := aws.EchoHome(ctx, bSh)
			if herr != nil {
				// Surface this; falling back to "/" would mask the real
				// first failure when a subsequent ls fails the same way.
				return browserOpenFailedMsg{
					browseSh: bSh,
					transSh:  tSh,
					err:      withStderr(fmt.Sprintf("echo $HOME: %v", herr), bSh),
				}
			}
			cwd = h
			if cwd == "" {
				cwd = "/"
			}
		}
		entries, err := aws.ListRemoteDir(ctx, bSh, cwd)
		if err != nil {
			return browserOpenFailedMsg{
				browseSh: bSh,
				transSh:  tSh,
				err:      withStderr(fmt.Sprintf("ls %s: %v", cwd, err), bSh),
			}
		}
		return browserOpenedMsg{
			instance: target,
			browseSh: bSh,
			transSh:  tSh,
			cwd:      cwd,
			entries:  entries,
		}
	}
}

func withStderr(msg string, sh *aws.ShellSession) string {
	if sh == nil {
		return msg
	}
	se := strings.TrimSpace(sh.Stderr())
	if se == "" {
		return msg
	}
	se = strings.ReplaceAll(se, "\n", " | ")
	return fmt.Sprintf("%s [plugin stderr: %s]", msg, se)
}

func (m *Model) closeBrowserSessions() {
	if m.browseShell != nil {
		id := m.browseShell.SessionID()
		_ = m.browseShell.Close()
		_ = m.awsClient.TerminateSession(m.ctx, id)
		m.browseShell = nil
	}
	if m.transferShell != nil {
		id := m.transferShell.SessionID()
		_ = m.transferShell.Close()
		_ = m.awsClient.TerminateSession(m.ctx, id)
		m.transferShell = nil
	}
}

// Shutdown terminates any active SSM sessions paws opened. Safe to call
// multiple times.
func (m *Model) Shutdown() {
	m.closeBrowserSessions()
}

package ui

import tea "github.com/charmbracelet/bubbletea"

func rowFromY(rect Region, firstVisibleRow int, clickY int) (int, bool) {
	if clickY < rect.Y || clickY >= rect.Y+rect.H {
		return 0, false
	}
	return firstVisibleRow + (clickY - rect.Y), true
}

// handleMouseMsg runs on Bubble Tea's main goroutine — mutations must be
// synchronous; never spawn a goroutine that touches the Model from here.
func (m *Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.screen != screenList || m.inputMode == inputBastion {
		return m, nil
	}

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			return m.handleLeftClick(msg.X, msg.Y), nil
		}
	case tea.MouseActionMotion:
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.handleWheel(-1), nil
	case tea.MouseButtonWheelDown:
		return m.handleWheel(+1), nil
	}

	return m, nil
}

func (m *Model) handleLeftClick(x, y int) tea.Model {
	for _, r := range m.tabRegions {
		if r.Contains(x, y) {
			m.setTab(tabType(r.Index))
			return m
		}
	}
	if row, ok := rowFromY(m.instancesListRect, m.instancesListRow0, y); ok {
		instances := m.filteredInstances()
		if row >= 0 && row < len(instances) {
			m.cursor = row
			m.activePanel = panelInstances
			m.restoreSettings()
		}
		return m
	}
	if m.logsRect.Contains(x, y) {
		m.activePanel = panelLogs
		return m
	}
	return m
}

func (m *Model) handleWheel(direction int) tea.Model {
	switch m.activePanel {
	case panelInstances:
		instances := m.filteredInstances()
		next := m.cursor + direction
		if next < 0 {
			next = 0
		}
		if next > len(instances)-1 {
			next = len(instances) - 1
		}
		if next != m.cursor && len(instances) > 0 {
			m.cursor = next
			m.restoreSettings()
		}
	case panelLogs:
		if direction < 0 {
			m.logViewport.LineUp(1)
		} else {
			m.logViewport.LineDown(1)
		}
	}
	return m
}

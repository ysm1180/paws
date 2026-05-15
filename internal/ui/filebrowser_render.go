package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ysm1180/paws/internal/transfer"
)

func (m *Model) renderFileBrowser() string {
	s := m.browser
	header := PanelTitleStyle.Render(fmt.Sprintf(" %s %s ", IconServer, s.instance.Name))
	cwdLine := ValueHighlightStyle.Render(s.cwd)
	if s.mode == browserModePathBar {
		cwdLine = InputFocusedStyle.Render(":" + s.pathInput.View())
	}

	listHeight := m.height - 9
	if listHeight < 4 {
		listHeight = 4
	}

	var list strings.Builder
	start := 0
	if s.cursor >= listHeight {
		start = s.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(s.entries) {
		end = len(s.entries)
	}
	rowWidth := m.width
	if rowWidth < 1 {
		rowWidth = 1
	}

	if s.loadErr != nil {
		list.WriteString(LogErrorStyle.Render(" " + s.loadErr.Error()))
	} else if len(s.entries) == 0 {
		list.WriteString(LabelStyle.Render(" (empty)"))
	} else {
		for i := start; i < end; i++ {
			e := s.entries[i]
			icon := IconFile
			if e.IsDir {
				icon = IconFolder
			} else if e.IsLink {
				icon = IconLink
			}
			size := humanBytes(e.Size)
			line := fmt.Sprintf(" %s %-30s %10s  %s",
				icon,
				truncate(e.Name, 30),
				size,
				e.MTime.Format("2006-01-02 15:04"),
			)
			style := ListItemStyle
			if i == s.cursor {
				style = ListItemSelectedStyle
			}
			list.WriteString(style.Width(rowWidth).Render(line))
			list.WriteString("\n")
		}
	}

	transferStrip := m.renderTransferStrip()

	prompt := ""
	if s.mode == browserModeDownloadPrompt {
		prompt = "\n" + InputFocusedStyle.Render("Save to: "+s.dlPathInput.View())
	}

	status := ""
	if s.statusMsg != "" {
		status = "\n" + LabelStyle.Render(s.statusMsg)
	}

	help := HelpBarStyle.Width(m.width).Render(strings.Join([]string{
		HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render("nav"),
		HelpKeyStyle.Render("⏎") + HelpDescStyle.Render("open/download"),
		HelpKeyStyle.Render("h") + HelpDescStyle.Render("up"),
		HelpKeyStyle.Render(":") + HelpDescStyle.Render("path"),
		HelpKeyStyle.Render("esc") + HelpDescStyle.Render("back/cancel"),
		HelpKeyStyle.Render("q") + HelpDescStyle.Render("quit"),
	}, " "))

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		" "+cwdLine,
		list.String(),
		transferStrip,
		prompt,
		status,
		help,
	)
	return body
}

func (m *Model) renderTransferStrip() string {
	j := m.tm.LastJob(m.browser.instance.ID)
	if j == nil {
		return ""
	}
	switch j.Status {
	case transfer.StatusRunning, transfer.StatusPending:
		barWidth := 30
		filled := j.PercentDone() * barWidth / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		return " " + IconDownload + " " + bar + fmt.Sprintf(" %3d%%  %s/%s  %.1f KB/s  %s",
			j.PercentDone(),
			humanBytes(j.Transferred), humanBytes(j.Expected),
			j.Speed()/1024,
			truncate(j.RemotePath, 40),
		)
	case transfer.StatusDone:
		return " " + StatusConnectedStyle.Render(fmt.Sprintf("%s %s → %s", IconSuccess, j.RemotePath, j.LocalPath))
	case transfer.StatusCancelled:
		return " " + LabelStyle.Render(fmt.Sprintf("cancelled %s", j.RemotePath))
	case transfer.StatusFailed:
		return " " + LogErrorStyle.Render(fmt.Sprintf("failed %s: %v", j.RemotePath, j.Err))
	}
	return ""
}

func humanBytes(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%dB", n)
	case n < k*k:
		return fmt.Sprintf("%.1fKB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1fMB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.2fGB", float64(n)/(k*k*k))
	}
}

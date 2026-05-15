package ui

import "strings"

func helpPair(k, desc string) string {
	return HelpKeyStyle.Render(k) + " " + HelpDescStyle.Render(desc)
}

func renderPanelHint(p panel) string {
	var pairs [][2]string
	switch p {
	case panelInstances:
		pairs = [][2]string{
			{"↑↓", "nav"},
			{"⏎", "enter"},
			{"c", "conn"},
			{"d", "disc"},
			{"/", "filter"},
		}
	case panelDetails:
		pairs = [][2]string{
			{"p", "port"},
			{"b", "bastion"},
		}
	case panelLogs:
		pairs = [][2]string{
			{"↑↓", "scroll"},
		}
	}

	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = helpPair(p[0], p[1])
	}
	return strings.Join(parts, "  ")
}

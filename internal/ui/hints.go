package ui

import "strings"

func helpPair(k, desc string) string {
	return HelpKeyStyle.Render(k) + " " + HelpDescStyle.Render(desc)
}

// renderPanelHint takes currentTab because the Enter binding only opens the
// EC2 file browser; showing "⏎ enter" on RDS/EC misleads users into pressing
// a key that does nothing.
func renderPanelHint(p panel, t tabType) string {
	var pairs [][2]string
	switch p {
	case panelInstances:
		pairs = append(pairs, [2]string{"↑↓", "nav"})
		if t == tabEC2 {
			pairs = append(pairs, [2]string{"⏎", "enter"})
		}
		pairs = append(pairs,
			[2]string{"c", "conn"},
			[2]string{"d", "disc"},
			[2]string{"/", "filter"},
		)
	case panelDetails:
		// EC2 has no port-forwarding fields, so neither key applies.
		if t == tabEC2 {
			return ""
		}
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

package ui

import (
	"strings"
	"testing"
)

func TestRenderPanelHint(t *testing.T) {
	tests := []struct {
		name     string
		panel    panel
		tab      tabType
		mustHave []string
		mustMiss []string
	}{
		{
			name:     "instances on EC2 shows enter",
			panel:    panelInstances,
			tab:      tabEC2,
			mustHave: []string{"↑↓", "nav", "enter", "c", "conn", "d", "disc", "/", "filter"},
			mustMiss: []string{"scroll"},
		},
		{
			name:     "instances on RDS hides enter",
			panel:    panelInstances,
			tab:      tabRDS,
			mustHave: []string{"↑↓", "nav", "c", "conn", "d", "disc", "/", "filter"},
			mustMiss: []string{"enter", "⏎"},
		},
		{
			name:     "instances on ElastiCache hides enter",
			panel:    panelInstances,
			tab:      tabElastiCache,
			mustHave: []string{"↑↓", "nav", "c", "conn", "d", "disc", "/", "filter"},
			mustMiss: []string{"enter", "⏎"},
		},
		{
			name:     "details panel on RDS shows port + bastion",
			panel:    panelDetails,
			tab:      tabRDS,
			mustHave: []string{"p", "port", "b", "bastion"},
			mustMiss: []string{"filter"},
		},
		{
			name:     "details panel on EC2 is empty (no port-forwarding)",
			panel:    panelDetails,
			tab:      tabEC2,
			mustHave: []string{},
			mustMiss: []string{"port", "bastion"},
		},
		{
			name:     "logs panel shows scroll",
			panel:    panelLogs,
			tab:      tabEC2,
			mustHave: []string{"↑↓", "scroll"},
			mustMiss: []string{"filter", "conn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderPanelHint(tt.panel, tt.tab)
			for _, s := range tt.mustHave {
				if !strings.Contains(out, s) {
					t.Errorf("hint for %v missing %q\ngot: %q", tt.panel, s, out)
				}
			}
			for _, s := range tt.mustMiss {
				if strings.Contains(out, s) {
					t.Errorf("hint for %v unexpectedly contains %q\ngot: %q", tt.panel, s, out)
				}
			}
		})
	}
}

package ui

import (
	"strings"
	"testing"
)

func TestRenderPanelHint(t *testing.T) {
	tests := []struct {
		name     string
		panel    panel
		mustHave []string
		mustMiss []string
	}{
		{
			name:     "instances panel shows nav + actions",
			panel:    panelInstances,
			mustHave: []string{"↑↓", "nav", "enter", "c", "conn", "d", "disc", "/", "filter"},
			mustMiss: []string{"scroll"},
		},
		{
			name:     "details panel shows port + bastion",
			panel:    panelDetails,
			mustHave: []string{"p", "port", "b", "bastion"},
			mustMiss: []string{"filter"},
		},
		{
			name:     "logs panel shows scroll",
			panel:    panelLogs,
			mustHave: []string{"↑↓", "scroll"},
			mustMiss: []string{"filter", "conn"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderPanelHint(tt.panel)
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

package ui

import "testing"

func TestRegion_Contains(t *testing.T) {
	tests := []struct {
		name string
		r    Region
		x, y int
		want bool
	}{
		{"inside", Region{X: 2, Y: 3, W: 5, H: 4}, 4, 5, true},
		{"top-left corner", Region{X: 2, Y: 3, W: 5, H: 4}, 2, 3, true},
		{"bottom-right edge (exclusive)", Region{X: 2, Y: 3, W: 5, H: 4}, 7, 7, false},
		{"just inside bottom-right", Region{X: 2, Y: 3, W: 5, H: 4}, 6, 6, true},
		{"left of region", Region{X: 2, Y: 3, W: 5, H: 4}, 1, 5, false},
		{"above region", Region{X: 2, Y: 3, W: 5, H: 4}, 4, 2, false},
		{"zero-size never contains", Region{X: 2, Y: 3, W: 0, H: 0}, 2, 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Contains(tt.x, tt.y); got != tt.want {
				t.Errorf("Contains(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

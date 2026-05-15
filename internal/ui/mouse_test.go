package ui

import (
	"testing"

	"github.com/ysm1180/paws/internal/aws"
)

func TestListRect_RowFromY(t *testing.T) {
	// Pretend the Instances list begins at row 7 and is 10 rows tall, with
	// the first visible row being row index 4 (scroll offset).
	rect := Region{X: 0, Y: 7, W: 80, H: 10}
	rowZero := 4

	tests := []struct {
		clickY  int
		wantRow int
		wantOK  bool
	}{
		{clickY: 7, wantRow: 4, wantOK: true},   // first visible row
		{clickY: 8, wantRow: 5, wantOK: true},
		{clickY: 16, wantRow: 13, wantOK: true}, // last visible row (Y=7+10-1)
		{clickY: 17, wantRow: 0, wantOK: false}, // past the list
		{clickY: 6, wantRow: 0, wantOK: false},  // above the list
	}

	for _, tt := range tests {
		row, ok := rowFromY(rect, rowZero, tt.clickY)
		if ok != tt.wantOK {
			t.Errorf("rowFromY(Y=%d) ok=%v, want %v", tt.clickY, ok, tt.wantOK)
		}
		if ok && row != tt.wantRow {
			t.Errorf("rowFromY(Y=%d) row=%d, want %d", tt.clickY, row, tt.wantRow)
		}
	}
}

func TestRenderTabs_RegionsLineUp(t *testing.T) {
	m := &Model{
		rdsInstances: make([]aws.Instance, 2),
		ecInstances:  make([]aws.Instance, 5),
		ec2Instances: make([]aws.Instance, 11),
	}
	_, regions := m.renderTabsWithRegions()

	if len(regions) != 3 {
		t.Fatalf("want 3 tab regions, got %d", len(regions))
	}
	for i := 1; i < len(regions); i++ {
		if regions[i].X != regions[i-1].X+regions[i-1].W {
			t.Errorf("tab %d starts at X=%d, prev ended at X=%d",
				i, regions[i].X, regions[i-1].X+regions[i-1].W)
		}
	}
	for i, r := range regions {
		if r.Index != i {
			t.Errorf("region %d Index=%d, want %d", i, r.Index, i)
		}
		if r.W <= 0 {
			t.Errorf("region %d W=%d, want >0", i, r.W)
		}
		if r.H != 1 {
			t.Errorf("region %d H=%d, want 1", i, r.H)
		}
	}
}

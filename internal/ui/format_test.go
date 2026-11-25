package ui

import (
	"strings"
	"testing"

	"github.com/ysm1180/paws/internal/aws"
)

func TestFormatInstanceRow_NameAndIDShownTogetherForEC2(t *testing.T) {
	inst := aws.Instance{ID: "i-04ab12cd34", Name: "ec2-prod-app"}
	line := formatInstanceRow(inst, IconDisconnected, "", 80)

	if !strings.Contains(line, "ec2-prod-app") {
		t.Errorf("want name in row, got: %q", line)
	}
	if !strings.Contains(line, "i-04ab12cd34") {
		t.Errorf("want id in row, got: %q", line)
	}
}

func TestFormatInstanceRow_OnlyIDWhenNameEmpty(t *testing.T) {
	inst := aws.Instance{ID: "heart-fiction-prod-db", Name: ""}
	line := formatInstanceRow(inst, IconDisconnected, "", 80)

	if !strings.Contains(line, "heart-fiction-prod-db") {
		t.Errorf("want id in row, got: %q", line)
	}
}

func TestFormatInstanceRow_OnlyIDWhenNameEqualsID(t *testing.T) {
	id := "heart-fiction-prod-db"
	inst := aws.Instance{ID: id, Name: id}
	line := formatInstanceRow(inst, IconDisconnected, "", 80)

	// id appears exactly once (not duplicated as both name and secondary).
	if strings.Count(line, id) != 1 {
		t.Errorf("want id to appear once when name==id, got: %q", line)
	}
}

func TestFormatInstanceRow_NarrowDropsSecondary(t *testing.T) {
	inst := aws.Instance{ID: "i-04ab12cd34", Name: "ec2-prod-app"}
	// Width 18: " ○ ec2-prod-app  " barely fits the name + 2-cell gap; the
	// 11-char id won't fit alongside. formatInstanceRow should drop the id.
	line := formatInstanceRow(inst, IconDisconnected, "", 18)

	if strings.Contains(line, "i-04ab12cd34") {
		t.Errorf("want id dropped at narrow width, got: %q", line)
	}
	if !strings.Contains(line, "ec2-prod-app") {
		t.Errorf("want name still present at narrow width, got: %q", line)
	}
}

func TestFormatInstanceRow_SuffixIncluded(t *testing.T) {
	inst := aws.Instance{ID: "i-04ab12cd34", Name: "ec2-prod-app"}
	line := formatInstanceRow(inst, IconConnected, " :5433", 80)

	if !strings.Contains(line, ":5433") {
		t.Errorf("want port suffix in row, got: %q", line)
	}
}

func TestFormatInstanceRow_RegionShown(t *testing.T) {
	inst := aws.Instance{ID: "heart-fiction-prod-db", Region: "ap-northeast-1"}
	line := formatInstanceRow(inst, IconDisconnected, "", 80)

	if !strings.Contains(line, "Tokyo") {
		t.Errorf("want region label in row, got: %q", line)
	}
	if !strings.Contains(line, "heart-fiction-prod-db") {
		t.Errorf("want id in row, got: %q", line)
	}
}

func TestFormatInstanceRow_NameIDAndRegionShownTogether(t *testing.T) {
	inst := aws.Instance{ID: "i-04ab12cd34", Name: "ec2-prod-app", Region: "ap-northeast-2"}
	line := formatInstanceRow(inst, IconDisconnected, "", 80)

	for _, want := range []string{"ec2-prod-app", "i-04ab12cd34", "Seoul"} {
		if !strings.Contains(line, want) {
			t.Errorf("want %q in row, got: %q", want, line)
		}
	}
}

func TestFormatInstanceRow_NarrowKeepsRegionDropsID(t *testing.T) {
	inst := aws.Instance{ID: "i-04ab12cd34", Name: "ec2-prod-app", Region: "ap-northeast-2"}
	// Width 26: name (12 cells) + "  Seoul" (7 cells) just fits; the
	// 12-char id alongside does not. The region label must stay visible.
	line := formatInstanceRow(inst, IconDisconnected, "", 26)

	if !strings.Contains(line, "Seoul") {
		t.Errorf("want region label kept at narrow width, got: %q", line)
	}
	if strings.Contains(line, "i-04ab12cd34") {
		t.Errorf("want id dropped at narrow width, got: %q", line)
	}
}

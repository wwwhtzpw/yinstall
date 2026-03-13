package cli

import (
	"reflect"
	"testing"

	"github.com/yinstall/internal/runner"
)

func TestParseStepRanges(t *testing.T) {
	steps := []*runner.Step{
		{ID: "B-001"},
		{ID: "B-002"},
		{ID: "C-001"},
		{ID: "C-002"},
		{ID: "C-003"},
	}

	tests := []struct {
		name    string
		specs   []string
		wantIDs []string
		wantErr bool
	}{
		{
			name:    "Single step",
			specs:   []string{"B-001"},
			wantIDs: []string{"B-001"},
		},
		{
			name:    "Single step lowercase",
			specs:   []string{"b001"},
			wantIDs: []string{"B-001"},
		},
		{
			name:    "Comma separated",
			specs:   []string{"B-001,C-001"},
			wantIDs: []string{"B-001", "C-001"},
		},
		{
			name:    "Range start-end",
			specs:   []string{"B-002-C-002"},
			wantIDs: []string{"B-002", "C-001", "C-002"},
		},
		{
			name:    "Range start-",
			specs:   []string{"C-001-"},
			wantIDs: []string{"C-001", "C-002", "C-003"},
		},
		{
			name:    "Range -end",
			specs:   []string{"-B-002"},
			wantIDs: []string{"B-001", "B-002"},
		},
		{
			name:    "Mixed",
			specs:   []string{"B-001", "C-002-"},
			wantIDs: []string{"B-001", "C-002", "C-003"},
		},
		{
			name:    "Normalized range",
			specs:   []string{"c001-c002"},
			wantIDs: []string{"C-001", "C-002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStepRanges(steps, tt.specs)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseStepRanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			var gotIDs []string
			for _, s := range got {
				gotIDs = append(gotIDs, s.ID)
			}
			if !reflect.DeepEqual(gotIDs, tt.wantIDs) {
				t.Errorf("parseStepRanges() = %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

package herdr

import (
	"reflect"
	"testing"

	"github.com/johnendean/herdrcolor/internal/assign"
	"github.com/johnendean/herdrcolor/internal/palette"
)

// The argv shape Herdr 0.9.5 accepts: ID first, then each option and its value
// as separate arguments. `--flag=value` and options before the ID are refused.
func TestReportArgsPutTheIDFirstAndSplitValues(t *testing.T) {
	got := reportArgs("pane", "w3:p1", "herdrcolor", assign.TokensFor("-odd=name", palette.BySlot(2)))
	want := []string{
		"pane", "report-metadata", "w3:p1", "--source", "herdrcolor",
		"--clear-token", "c1",
		"--token", "c2=-odd=name",
		"--clear-token", "c3", "--clear-token", "c4", "--clear-token", "c5", "--clear-token", "c6",
		"--token", "color=#fab387",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

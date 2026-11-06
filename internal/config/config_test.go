package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func mustEnsure(t *testing.T, text string) Result {
	t.Helper()
	res, err := Ensure(text)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestEnsureAddsBothSectionsToAnEmptyConfig(t *testing.T) {
	res := mustEnsure(t, "")
	if !res.Changed || len(res.Managed) != 2 || len(res.HandWritten) != 0 {
		t.Fatalf("%+v", res)
	}
	states, err := Inspect(res.Text)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range states {
		if len(st.Slots) != 6 || st.HandWritten {
			t.Errorf("%s: %d slots, hand-written %v", st.Section.Table, len(st.Slots), st.HandWritten)
		}
	}
}

func TestEnsureIsIdempotent(t *testing.T) {
	first := mustEnsure(t, "onboarding = false\n\n[ui]\nstatus_indicators = \"symbols\"\n")
	second := mustEnsure(t, first.Text)
	if second.Changed || second.Text != first.Text {
		t.Fatal("a second Ensure changed the config")
	}
}

func TestEnsureKeepsEverythingOutsideTheBlock(t *testing.T) {
	user := "# my comment\nonboarding = false   # aligned\n\n[ui]\nstatus_indicators = \"symbols\"\n"
	res := mustEnsure(t, user)
	if !strings.HasPrefix(res.Text, user) {
		t.Fatalf("user text changed:\n%s", res.Text)
	}
	stripped, changed, err := Remove(res.Text)
	if err != nil || !changed || stripped != user {
		t.Fatalf("Remove did not restore the original: %q", stripped)
	}
}

func TestEnsureRewritesAStaleBlockInPlace(t *testing.T) {
	stale := "a = 1\n" + BeginMarker + "\n# old\n" + EndMarker + "\n[keys]\nb = 2\n"
	res := mustEnsure(t, stale)
	if !strings.HasPrefix(res.Text, "a = 1\n"+BeginMarker) || !strings.HasSuffix(res.Text, EndMarker+"\n[keys]\nb = 2\n") {
		t.Fatalf("block not replaced in place:\n%s", res.Text)
	}
}

func TestHandWrittenSectionsAreLeftAlone(t *testing.T) {
	user := "[ui.sidebar.agents]\nrows = [[\"agent\"]]\n"
	res := mustEnsure(t, user)
	if len(res.HandWritten) != 1 || res.HandWritten[0] != "agents" || len(res.Managed) != 1 {
		t.Fatalf("%+v", res)
	}
	if !strings.HasPrefix(res.Text, user) || strings.Count(res.Text, "[ui.sidebar.agents]") != 1 {
		t.Fatalf("hand-written section touched:\n%s", res.Text)
	}
}

func TestADottedKeyCountsAsHandWritten(t *testing.T) {
	res := mustEnsure(t, "[ui]\nsidebar.spaces.rows = [[\"branch\"]]\n")
	if len(res.HandWritten) != 1 || res.HandWritten[0] != "spaces" {
		t.Fatalf("%+v", res)
	}
}

func TestBothHandWrittenRemovesTheBlock(t *testing.T) {
	user := "[ui.sidebar.agents]\nrows = []\n[ui.sidebar.spaces]\nrows = []\n"
	withBlock := BeginMarker + "\n" + EndMarker + "\n" + user
	res := mustEnsure(t, withBlock)
	if res.Text != user {
		t.Fatalf("got:\n%s", res.Text)
	}
}

func TestABrokenConfigIsLeftAlone(t *testing.T) {
	if _, err := Ensure("this is = = not toml"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestAResultThatWouldNotParseIsRefused(t *testing.T) {
	// An inline sidebar table cannot be extended by a [ui.sidebar.agents]
	// header, so writing the block would break the user's config.
	if _, err := Ensure("[ui]\nsidebar = { width = 30 }\n"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestBrokenMarkersAreRefused(t *testing.T) {
	for _, text := range []string{
		BeginMarker + "\n",
		EndMarker + "\n",
		BeginMarker + "\n" + EndMarker + "\n" + BeginMarker + "\n" + EndMarker + "\n",
	} {
		if _, err := Ensure(text); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
}

func TestCRLFConfigsStayCRLF(t *testing.T) {
	res := mustEnsure(t, "onboarding = false\r\n")
	if strings.Contains(strings.ReplaceAll(res.Text, "\r\n", ""), "\n") {
		t.Fatal("bare LF written into a CRLF file")
	}
}

func TestSnippetIsValidAndDeclaresEverySlot(t *testing.T) {
	var doc map[string]any
	if err := toml.Unmarshal([]byte(Snippet()), &doc); err != nil {
		t.Fatal(err)
	}
	states, err := Inspect(Snippet())
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range states {
		if len(st.Slots) != 6 {
			t.Errorf("%s declares %d slots", st.Section.Table, len(st.Slots))
		}
	}
}

func TestInspectSurvivesABrokenConfig(t *testing.T) {
	states, err := Inspect("not = = toml")
	if err == nil || len(states) != 2 || len(states[0].Slots) != 0 {
		t.Fatalf("%v %+v", err, states)
	}
}

func TestWriteKeepsABackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "herdr", "config.toml")
	if err := Write(path, "a = 1\n"); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, "a = 2\n"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	bak, _ := os.ReadFile(path + ".herdrcolor.bak")
	if string(got) != "a = 2\n" || string(bak) != "a = 1\n" {
		t.Fatalf("config %q, backup %q", got, bak)
	}
}

func TestPathHonoursTheOverride(t *testing.T) {
	t.Setenv("HERDR_CONFIG_PATH", "/x/config.toml")
	if p, _ := Path(); p != "/x/config.toml" {
		t.Fatal(p)
	}
}

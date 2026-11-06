package palette

import (
	"regexp"
	"testing"
)

func TestColourIsTheOnePythonGave(t *testing.T) {
	// The README's example, and the Python suite's seed-independence check.
	if got := ColourFor("herdrcolor").Hex; got != "#f38ba8" {
		t.Fatalf("herdrcolor = %s", got)
	}
}

func TestEverySlotIsReachable(t *testing.T) {
	seen := map[int]bool{}
	for i := 0; i < 400; i++ {
		seen[ColourFor(string(rune(0x4e00+i))).Slot] = true
	}
	if len(seen) != len(Palette) {
		t.Fatalf("only %d slots reached", len(seen))
	}
}

func TestSlotsMatchTheirPosition(t *testing.T) {
	for i, c := range Palette {
		if c.Slot != i+1 || BySlot(c.Slot) != c {
			t.Fatalf("palette entry %d has slot %d", i, c.Slot)
		}
	}
}

func TestSlotTokenNamesAreWhatHerdrAccepts(t *testing.T) {
	valid := regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	for _, c := range Palette {
		if !valid.MatchString(SlotToken(c.Slot)) {
			t.Fatalf("%q", SlotToken(c.Slot))
		}
	}
}

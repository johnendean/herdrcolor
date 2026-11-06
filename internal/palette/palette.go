// Package palette holds the six colours and the colour a project prefers.
//
// Six colours, because six is about as many hues as stay distinguishable at a
// glance in a sidebar, and on RGB LEDs later. They are Catppuccin's, so the
// sidebar agrees with Herdr's default theme.
//
// A project's preferred colour comes from a hash of its name rather than from
// the order agents appeared in. That way a project keeps its colour across
// restarts, across sessions, on another machine, and across the Python-to-Go
// port, which internal/assign/testdata/parity.json pins.
package palette

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Source is the metadata source name. Herdr scopes reported metadata by source,
// which is what lets `clear` remove exactly what this plugin set.
const Source = "herdrcolor"

// ColourToken is the hex of the assigned colour. It is published for consumers
// that draw their own pixels rather than reading the sidebar (herdrkeys is
// why it exists). Nothing in the sidebar config uses it.
const ColourToken = "color"

// Colour is one palette entry. Slot is 1-based: the `$cN` token it lives in.
type Colour struct {
	Slot int
	Name string // Catppuccin's name, for `list` and `snippet` output
	Hex  string
}

// Palette is ordered by slot.
var Palette = []Colour{
	{1, "red", "#f38ba8"},
	{2, "peach", "#fab387"},
	{3, "green", "#a6e3a1"},
	{4, "blue", "#89b4fa"},
	{5, "mauve", "#cba6f7"},
	{6, "teal", "#94e2d5"},
}

// SlotToken is the bare token name for a slot. The config refers to it as
// `$c1`; the `$` belongs to the sidebar's syntax, not to the name.
func SlotToken(slot int) string { return fmt.Sprintf("c%d", slot) }

// BySlot returns the colour that lives in a slot.
func BySlot(slot int) Colour { return Palette[slot-1] }

// ColourFor is the colour a project's name hashes to.
//
// SHA-256 of the UTF-8 name, with the first eight bytes read big-endian and
// reduced modulo the palette size. This is the rule the Python version used, so
// existing colours do not move.
func ColourFor(project string) Colour {
	digest := sha256.Sum256([]byte(project))
	n := binary.BigEndian.Uint64(digest[:8])
	return Palette[n%uint64(len(Palette))]
}

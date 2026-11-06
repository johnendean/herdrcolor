// Package config owns herdrcolor's managed block in Herdr's config.toml.
//
// The block holds the two sidebar sections that declare the six colour slots.
// herdrcolor writes and rewrites it from `sync`, and changes text only between
// its markers. A section the user defined outside the block is hand-written:
// it is never edited, only reported. Writes are plain text edits, so the user's
// comments and formatting outside the block survive; the TOML parser is used
// only to read.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/johnendean/herdrcolor/internal/palette"
)

// Section is a sidebar section this plugin colours.
type Section struct {
	Name  string // "agents" or "spaces"
	Table string // its config table
}

// Sections are the two sidebar sections, in the order the block writes them.
var Sections = []Section{
	{"agents", "ui.sidebar.agents"},
	{"spaces", "ui.sidebar.spaces"},
}

const (
	BeginMarker = "# >>> herdrcolor managed block >>>"
	EndMarker   = "# <<< herdrcolor managed block <<<"
)

// Path is Herdr's config.toml: $HERDR_CONFIG_PATH if set, else
// %APPDATA%\herdr\config.toml on Windows and ~/.config/herdr/config.toml
// elsewhere.
func Path() (string, error) {
	if p := os.Getenv("HERDR_CONFIG_PATH"); p != "" {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			return "", errors.New("APPDATA is not set; pass --config")
		}
		return filepath.Join(appdata, "herdr", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "herdr", "config.toml"), nil
}

func slotLines(indent string) []string {
	var lines []string
	for _, c := range palette.Palette {
		lines = append(lines, fmt.Sprintf(`%s{ token = "$%s", fg = "%s", bold = true },  # %s`,
			indent, palette.SlotToken(c.Slot), c.Hex, c.Name))
	}
	return lines
}

// SectionText is one section's TOML, generated from the palette so the two
// always agree.
func SectionText(s Section) string {
	var b []string
	b = append(b, "["+s.Table+"]", "rows = [")
	switch s.Name {
	case "agents":
		b = append(b, `  ["state_icon", "machine",`)
		b = append(b, slotLines("    ")...)
		b = append(b, `    "tab"],`, `  ["agent"],`)
	case "spaces":
		b = append(b, `  ["state_icon",`)
		b = append(b, slotLines("    ")...)
		b = append(b, `  ],`, `  ["branch", "git_status"],`)
	}
	b = append(b, "]")
	return strings.Join(b, "\n") + "\n"
}

// SnippetHeader introduces `herdrcolor snippet` output.
const SnippetHeader = `# herdrcolor: six colour slots per sidebar section.
#
# herdrcolor writes these sections itself, as a managed block in config.toml,
# every time it syncs. Paste this only into a section you keep by hand, then
# run ` + "`herdr server reload-config`" + `.
#
# Exactly one slot ever holds text, so exactly one colour shows. The other five
# cost nothing: an empty token renders as no text, no separator and no padding.
`

// Snippet is both sections, for a person to paste.
func Snippet() string {
	parts := []string{SnippetHeader}
	for _, s := range Sections {
		parts = append(parts, SectionText(s))
	}
	return strings.Join(parts, "\n")
}

// split finds the managed block. before and after exclude it; found is false
// when there is no block.
func split(text string) (before, block, after string, found bool, err error) {
	begin := strings.Index(text, BeginMarker)
	if begin < 0 {
		if strings.Contains(text, EndMarker) {
			return "", "", "", false, errors.New("managed block has an end marker but no begin marker")
		}
		return text, "", "", false, nil
	}
	if strings.Count(text, BeginMarker) > 1 {
		return "", "", "", false, errors.New("more than one managed block")
	}
	rel := strings.Index(text[begin:], EndMarker)
	if rel < 0 {
		return "", "", "", false, errors.New("managed block has a begin marker but no end marker")
	}
	end := begin + rel + len(EndMarker)
	// Take the end marker's line break with the block.
	if strings.HasPrefix(text[end:], "\r\n") {
		end += 2
	} else if strings.HasPrefix(text[end:], "\n") {
		end++
	}
	return text[:begin], text[begin:end], text[end:], true, nil
}

func parse(text string) (map[string]any, error) {
	doc := map[string]any{}
	if err := toml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func lookup(doc map[string]any, table string) (any, bool) {
	var node any = doc
	for _, key := range strings.Split(table, ".") {
		m, ok := node.(map[string]any)
		if !ok {
			return nil, false
		}
		node, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return node, true
}

// handWritten is the sections defined outside the block.
func handWritten(outside string) (map[string]bool, error) {
	doc, err := parse(outside)
	if err != nil {
		return nil, err
	}
	hw := map[string]bool{}
	for _, s := range Sections {
		if _, ok := lookup(doc, s.Table); ok {
			hw[s.Name] = true
		}
	}
	return hw, nil
}

func blockText(sections []Section) string {
	parts := []string{
		BeginMarker + "\n" +
			"# Written by `herdrcolor sync`, which rewrites everything up to the end\n" +
			"# marker. To keep a section by hand, move it out of this block and\n" +
			"# herdrcolor will leave it alone. `herdrcolor clear --config` removes it.\n",
	}
	for _, s := range sections {
		parts = append(parts, SectionText(s))
	}
	return strings.Join(parts, "\n") + EndMarker + "\n"
}

func lineEnding(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func withEnding(text, eol string) string {
	if eol == "\n" {
		return text
	}
	return strings.ReplaceAll(text, "\n", eol)
}

// Result is what Ensure decided.
type Result struct {
	Text        string
	Changed     bool
	Managed     []string // sections the block holds
	HandWritten []string // sections left alone
}

// Ensure returns text with the managed block holding every section the user has
// not written by hand. It refuses, returning an error and no text, when the
// config outside the block does not parse or when the result would not.
func Ensure(text string) (Result, error) {
	before, _, after, found, err := split(text)
	if err != nil {
		return Result{}, err
	}
	hw, err := handWritten(before + after)
	if err != nil {
		return Result{}, fmt.Errorf("config.toml does not parse, so it was left alone: %w", err)
	}

	var res Result
	var managed []Section
	for _, s := range Sections {
		if hw[s.Name] {
			res.HandWritten = append(res.HandWritten, s.Name)
		} else {
			managed = append(managed, s)
			res.Managed = append(res.Managed, s.Name)
		}
	}

	eol := lineEnding(text)
	block := ""
	if len(managed) > 0 {
		block = withEnding(blockText(managed), eol)
	}
	var out string
	switch {
	case found:
		out = before + block + after
	case block == "":
		out = text
	default:
		out = before
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += eol
		}
		if out != "" {
			out += eol
		}
		out += block
	}

	if _, err := parse(out); err != nil {
		return Result{}, fmt.Errorf("adding the managed block would make config.toml invalid, so it was left alone: %w", err)
	}
	res.Text, res.Changed = out, out != text
	return res, nil
}

// Remove returns text without the managed block.
func Remove(text string) (string, bool, error) {
	before, _, after, found, err := split(text)
	if err != nil || !found {
		return text, false, err
	}
	// Drop the blank line Ensure put in front of an appended block.
	if after == "" {
		before = strings.TrimRight(before, "\r\n")
		if before != "" {
			before += lineEnding(text)
		}
	}
	return before + after, true, nil
}

// HasBlock reports whether text contains a managed block.
func HasBlock(text string) bool { return strings.Contains(text, BeginMarker) }

// SectionState is how one section stands, for `doctor`.
type SectionState struct {
	Section     Section
	HandWritten bool
	Slots       map[int]bool // slots it declares
}

// Inspect reports on each section. A config that does not parse declares no
// slots, and the error says why.
func Inspect(text string) ([]SectionState, error) {
	before, _, after, _, splitErr := split(text)
	hw := map[string]bool{}
	if splitErr == nil {
		hw, _ = handWritten(before + after)
	}
	doc, err := parse(text)
	if err == nil {
		err = splitErr
	}
	var states []SectionState
	for _, s := range Sections {
		st := SectionState{Section: s, HandWritten: hw[s.Name], Slots: map[int]bool{}}
		if doc != nil {
			node, _ := lookup(doc, s.Table)
			m, _ := node.(map[string]any)
			rows, _ := m["rows"].([]any)
			for _, row := range rows {
				cells, _ := row.([]any)
				for _, cell := range cells {
					name, _ := cell.(string)
					if t, ok := cell.(map[string]any); ok {
						name, _ = t["token"].(string)
					}
					for _, c := range palette.Palette {
						if name == "$"+palette.SlotToken(c.Slot) {
							st.Slots[c.Slot] = true
						}
					}
				}
			}
		}
		states = append(states, st)
	}
	return states, err
}

// Read returns the config's text, or "" when there is no file yet.
func Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// Write replaces the config, keeping the previous version beside it as
// config.toml.herdrcolor.bak. It writes a temporary file and renames it over
// the original, so Herdr never reads a half-written config.
func Write(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if old, err := os.ReadFile(path); err == nil {
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(path+".herdrcolor.bak", old, mode); err != nil {
			return fmt.Errorf("backing up config: %w", err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.toml.herdrcolor-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp.Name(), mode)
	return os.Rename(tmp.Name(), path)
}

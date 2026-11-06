// Package assign turns a list of agents and spaces into the tokens each row
// should carry.
//
// Everything here is a pure function: nothing talks to Herdr, so the decisions
// that matter can be tested without a running Herdr.
//
// # Why slots
//
// Herdr's sidebar can colour a token, and it can colour one conditionally on
// that token's own text, but a rule cannot look at a second token. So there is
// no way to say "colour the workspace name according to $color". The way round
// it is to make the colour a question of which token holds the text. The config
// declares six tokens, $c1 to $c6, each with a fixed colour. This plugin puts the
// name in exactly one of them and clears the other five.
package assign

import (
	"sort"
	"strings"

	"github.com/johnendean/herdrcolor/internal/palette"
)

// Agent is the part of a Herdr agent record this package reads.
type Agent struct {
	PaneID        string            `json:"pane_id"`
	Cwd           string            `json:"cwd"`
	ForegroundCwd string            `json:"foreground_cwd"`
	Tokens        map[string]any    `json:"tokens"`
}

// Workspace is the part of a Herdr workspace record this package reads.
type Workspace struct {
	WorkspaceID string            `json:"workspace_id"`
	Label       string            `json:"label"`
	Tokens      map[string]any    `json:"tokens"`
}

// PaneAssignment is the colour for one agent pane.
type PaneAssignment struct {
	PaneID  string
	Project string // what was hashed, and the text the sidebar shows
	Colour  palette.Colour
}

// SpaceAssignment is the colour for one space.
type SpaceAssignment struct {
	WorkspaceID string
	Label       string // the text the spaces panel shows
	Colour      palette.Colour
}

// Plan is the colours for both halves of the sidebar.
type Plan struct {
	Panes  []PaneAssignment
	Spaces []SpaceAssignment
}

// ProjectName is the last component of a directory, split on either kind of
// separator so a Windows path and a Unix path name their project the same way.
// It returns "" when there is no name.
func ProjectName(dir string) string {
	trimmed := strings.TrimRight(dir, `/\`)
	if i := strings.LastIndexAny(trimmed, `/\`); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	// A bare drive such as `C:` names no project.
	if len(trimmed) == 2 && trimmed[1] == ':' {
		return ""
	}
	return trimmed
}

// Resolve gives each project a colour: its hashed one where possible, the next
// free one where not.
//
// The hash on its own collides too often (six slots and five projects collide
// more often than not), so it is a preference. Projects are considered in name
// order, never arrival order, so the same set of agents always produces the same
// colours. Past six projects the slots run out and the extras keep their hashed
// colour.
func Resolve(projects map[string]bool) map[string]palette.Colour {
	names := make([]string, 0, len(projects))
	for p := range projects {
		names = append(names, p)
	}
	// Byte-wise order, which for UTF-8 matches Python's code-point order.
	sort.Strings(names)

	n := len(palette.Palette)
	assigned := make(map[string]palette.Colour, len(names))
	taken := make(map[int]bool)
	for _, project := range names {
		preferred := palette.ColourFor(project).Slot
		slot := preferred
		for step := 0; step < n; step++ {
			candidate := (preferred-1+step)%n + 1
			if !taken[candidate] {
				slot = candidate
				break
			}
		}
		taken[slot] = true
		assigned[project] = palette.BySlot(slot)
	}
	return assigned
}

// WorkspaceOf reads the workspace a pane belongs to from its ID (`w6:p1` is in
// `w6`). Herdr reports no other link: `workspace list` carries no pane IDs.
func WorkspaceOf(paneID string) string {
	ws, _, _ := strings.Cut(paneID, ":")
	return ws
}

type namedPane struct{ paneID, project string }

func namedPanes(agents []Agent) []namedPane {
	var named []namedPane
	for _, a := range agents {
		if a.PaneID == "" {
			continue
		}
		project := ProjectName(a.Cwd)
		if project == "" {
			project = ProjectName(a.ForegroundCwd)
		}
		if project == "" {
			continue
		}
		named = append(named, namedPane{a.PaneID, project})
	}
	return named
}

// MakePlan resolves both panels together, so a space and the agents inside it
// always agree.
//
// A space with agents takes their colour; the lowest pane ID wins, so the
// result does not depend on list order. A space with no agents falls back to
// hashing its label, because Herdr reports no directory for a workspace.
func MakePlan(agents []Agent, workspaces []Workspace) Plan {
	named := namedPanes(agents)
	withAgents := map[string]bool{}
	for _, np := range named {
		withAgents[WorkspaceOf(np.paneID)] = true
	}

	var spaces []Workspace
	for _, w := range workspaces {
		if w.WorkspaceID != "" && w.Label != "" {
			spaces = append(spaces, w)
		}
	}

	keys := map[string]bool{}
	for _, np := range named {
		keys[np.project] = true
	}
	for _, w := range spaces {
		if !withAgents[w.WorkspaceID] {
			keys[w.Label] = true
		}
	}
	colours := Resolve(keys)

	var plan Plan
	for _, np := range named {
		plan.Panes = append(plan.Panes, PaneAssignment{np.paneID, np.project, colours[np.project]})
	}

	sorted := append([]PaneAssignment(nil), plan.Panes...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PaneID < sorted[j].PaneID })
	byWorkspace := map[string]PaneAssignment{}
	for _, pa := range sorted {
		ws := WorkspaceOf(pa.PaneID)
		if _, ok := byWorkspace[ws]; !ok {
			byWorkspace[ws] = pa
		}
	}

	for _, w := range spaces {
		colour := colours[w.Label]
		if inherited, ok := byWorkspace[w.WorkspaceID]; ok {
			colour = inherited.Colour
		}
		plan.Spaces = append(plan.Spaces, SpaceAssignment{w.WorkspaceID, w.Label, colour})
	}
	return plan
}

// Tokens is a full token set for one row. A nil value clears the token.
type Tokens map[string]*string

// TokensFor fills one slot with text and clears every other slot. Every slot
// appears on every call, because Herdr leaves an unmentioned token as it was.
func TokensFor(text string, colour palette.Colour) Tokens {
	tokens := Cleared()
	t, hex := text, colour.Hex
	tokens[palette.SlotToken(colour.Slot)] = &t
	tokens[palette.ColourToken] = &hex
	return tokens
}

// Cleared is every token this plugin sets, emptied.
func Cleared() Tokens {
	tokens := Tokens{palette.ColourToken: nil}
	for _, c := range palette.Palette {
		tokens[palette.SlotToken(c.Slot)] = nil
	}
	return tokens
}

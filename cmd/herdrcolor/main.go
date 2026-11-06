// Command herdrcolor gives each Herdr agent its own colour in the sidebar.
//
// `sync` is the whole plugin. The other commands exist because a colour that
// appears by magic is a colour you cannot debug.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/johnendean/herdrcolor/internal/assign"
	"github.com/johnendean/herdrcolor/internal/config"
	"github.com/johnendean/herdrcolor/internal/herdr"
	"github.com/johnendean/herdrcolor/internal/palette"
)

// Version must equal `version` in herdr-plugin.toml, which is also the release
// tag the install fetches. A test holds the two together.
const Version = "0.2.0"

type options struct {
	command     string
	socket      string
	configFile  string
	verbose     bool
	clearConfig bool
}

const usage = `usage: herdrcolor <command> [flags]

commands:
  sync      colour every agent and space now, and keep the config block current
  list      show assigned and reported colours
  snippet   print the sidebar config sections for the palette
  clear     remove the tokens this plugin set (--config: and the managed block)
  doctor    check Herdr, the config, and the colours
  version   print the version

flags (before or after the command):
  --socket PATH       Herdr socket (default: $HERDR_SOCKET_PATH, else Herdr's default)
  --config-file PATH  Herdr config.toml (default: $HERDR_CONFIG_PATH, else the platform default)
  -v, --verbose       print each assignment
`

// parseArgs takes flags on either side of the command: `herdrcolor sync
// --verbose` is what a person types, and `herdrcolor -v sync` should work too.
func parseArgs(args []string) (options, error) {
	var o options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")
		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}
			i++
			return args[i], nil
		}
		var err error
		switch {
		case name == "--socket":
			o.socket, err = takeValue()
		case name == "--config-file":
			o.configFile, err = takeValue()
		case arg == "-v" || arg == "--verbose":
			o.verbose = true
		case arg == "--config":
			o.clearConfig = true
		case arg == "-h" || arg == "--help":
			o.command = "help"
		case strings.HasPrefix(arg, "-"):
			return o, fmt.Errorf("unknown flag %s", arg)
		case o.command == "":
			o.command = arg
		default:
			return o, fmt.Errorf("unexpected argument %q", arg)
		}
		if err != nil {
			return o, err
		}
	}
	if o.clearConfig && o.command != "clear" {
		return o, errors.New("--config belongs to `clear`; the config file is --config-file")
	}
	return o, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	o, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "herdrcolor: %v\n\n%s", err, usage)
		return 2
	}
	app := &app{o: o, out: stdout, err: stderr, herdr: herdr.New(o.socket)}
	commands := map[string]func() int{
		"sync":    app.sync,
		"list":    app.list,
		"snippet": func() int { fmt.Fprint(stdout, config.Snippet()); return 0 },
		"clear":   app.clear,
		"doctor":  app.doctor,
		"version": func() int { fmt.Fprintln(stdout, "herdrcolor", Version); return 0 },
	}
	if o.command == "help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, ok := commands[o.command]
	if !ok {
		if o.command != "" {
			fmt.Fprintf(stderr, "herdrcolor: unknown command %q\n\n", o.command)
		}
		fmt.Fprint(stderr, usage)
		return 2
	}
	return cmd()
}

type app struct {
	o     options
	out   io.Writer
	err   io.Writer
	herdr *herdr.Client
}

func (a *app) fail(err error) int {
	fmt.Fprintf(a.err, "herdrcolor: %v\n", err)
	return 1
}

func (a *app) configPath() (string, error) {
	if a.o.configFile != "" {
		return a.o.configFile, nil
	}
	return config.Path()
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func describe(c palette.Colour) string {
	return fmt.Sprintf("$%s  %s  %s", palette.SlotToken(c.Slot), c.Hex, c.Name)
}

// sync gives every agent and space its colour, then keeps the managed block
// current. It is idempotent: reporting the same tokens again changes nothing
// on screen, which is what makes it safe to run from an event hook, a startup
// hook and a human. Colouring comes first, so a config problem never costs the
// colours themselves.
func (a *app) sync() int {
	agents, err := a.herdr.Agents()
	if err != nil {
		return a.fail(err)
	}
	spaces, err := a.herdr.Workspaces()
	if err != nil {
		return a.fail(err)
	}
	plan := assign.MakePlan(agents, spaces)

	for _, p := range plan.Panes {
		if err := a.herdr.ReportPane(p.PaneID, palette.Source, assign.TokensFor(p.Project, p.Colour)); err != nil {
			return a.fail(err)
		}
		if a.o.verbose {
			fmt.Fprintf(a.out, "%s  %s  %s\n", p.PaneID, p.Project, describe(p.Colour))
		}
	}
	for _, s := range plan.Spaces {
		if err := a.herdr.ReportWorkspace(s.WorkspaceID, palette.Source, assign.TokensFor(s.Label, s.Colour)); err != nil {
			return a.fail(err)
		}
		if a.o.verbose {
			fmt.Fprintf(a.out, "%s  %s  %s\n", s.WorkspaceID, s.Label, describe(s.Colour))
		}
	}
	fmt.Fprintf(a.out, "herdrcolor: coloured %s and %s\n",
		plural(len(plan.Panes), "agent"), plural(len(plan.Spaces), "space"))

	if err := a.ensureConfig(); err != nil {
		return a.fail(fmt.Errorf("config: %w", err))
	}
	return 0
}

// ensureConfig writes the managed block if it is missing or out of date, and
// reloads Herdr only when it changed anything.
func (a *app) ensureConfig() error {
	path, err := a.configPath()
	if err != nil {
		return err
	}
	text, err := config.Read(path)
	if err != nil {
		return err
	}
	res, err := config.Ensure(text)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, name := range res.HandWritten {
		fmt.Fprintf(a.out, "herdrcolor: [ui.sidebar.%s] is hand-written in %s; left alone\n", name, path)
	}
	if !res.Changed {
		return nil
	}
	if err := config.Write(path, res.Text); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "herdrcolor: updated the managed block in %s\n", path)
	return a.herdr.ReloadConfig()
}

func reported(tokens map[string]any) string {
	if v, ok := tokens[palette.ColourToken].(string); ok && v != "" {
		return v
	}
	return "-"
}

// list shows what each row should wear beside what it is wearing. The two can
// differ, after a palette change or a restart that dropped metadata, and seeing
// both tells "not synced" from "synced, config not applied".
func (a *app) list() int {
	agents, err := a.herdr.Agents()
	if err != nil {
		return a.fail(err)
	}
	spaces, err := a.herdr.Workspaces()
	if err != nil {
		return a.fail(err)
	}
	plan := assign.MakePlan(agents, spaces)

	livePanes := map[string]string{}
	for _, ag := range agents {
		livePanes[ag.PaneID] = reported(ag.Tokens)
	}
	liveSpaces := map[string]string{}
	for _, w := range spaces {
		liveSpaces[w.WorkspaceID] = reported(w.Tokens)
	}

	rows := [][]string{{"ROW", "NAME", "SLOT", "COLOUR", "REPORTED"}}
	for _, p := range plan.Panes {
		rows = append(rows, []string{p.PaneID, p.Project, "$" + palette.SlotToken(p.Colour.Slot),
			p.Colour.Hex + " " + p.Colour.Name, livePanes[p.PaneID]})
	}
	for _, s := range plan.Spaces {
		rows = append(rows, []string{s.WorkspaceID, s.Label, "$" + palette.SlotToken(s.Colour.Slot),
			s.Colour.Hex + " " + s.Colour.Name, liveSpaces[s.WorkspaceID]})
	}
	if len(rows) == 1 {
		fmt.Fprintln(a.out, "herdrcolor: nothing to colour")
		return 0
	}
	table(a.out, rows)
	return 0
}

func table(w io.Writer, rows [][]string) {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}
	for _, row := range rows {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-len([]rune(cell))+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}

// clear removes every token this plugin set and leaves other sources alone.
// With --config it also removes the managed block: Herdr has no uninstall hook,
// so this is the documented first step of uninstalling.
func (a *app) clear() int {
	agents, err := a.herdr.Agents()
	if err != nil {
		return a.fail(err)
	}
	spaces, err := a.herdr.Workspaces()
	if err != nil {
		return a.fail(err)
	}
	panes, cleared := 0, 0
	for _, ag := range agents {
		if ag.PaneID == "" {
			continue
		}
		if err := a.herdr.ReportPane(ag.PaneID, palette.Source, assign.Cleared()); err != nil {
			return a.fail(err)
		}
		panes++
	}
	for _, w := range spaces {
		if w.WorkspaceID == "" {
			continue
		}
		if err := a.herdr.ReportWorkspace(w.WorkspaceID, palette.Source, assign.Cleared()); err != nil {
			return a.fail(err)
		}
		cleared++
	}
	fmt.Fprintf(a.out, "herdrcolor: cleared %s and %s\n", plural(panes, "pane"), plural(cleared, "space"))

	if !a.o.clearConfig {
		return 0
	}
	path, err := a.configPath()
	if err != nil {
		return a.fail(err)
	}
	text, err := config.Read(path)
	if err != nil {
		return a.fail(err)
	}
	out, changed, err := config.Remove(text)
	if err != nil {
		return a.fail(fmt.Errorf("%s: %w", path, err))
	}
	if !changed {
		fmt.Fprintf(a.out, "herdrcolor: no managed block in %s\n", path)
		return 0
	}
	if err := config.Write(path, out); err != nil {
		return a.fail(err)
	}
	fmt.Fprintf(a.out, "herdrcolor: removed the managed block from %s\n", path)
	if err := a.herdr.ReloadConfig(); err != nil {
		return a.fail(err)
	}
	return 0
}

// doctor reports on the things that have to be true for a colour to appear.
func (a *app) doctor() int {
	ok := true
	agents, err := a.herdr.Agents()
	var spaces []assign.Workspace
	if err == nil {
		spaces, err = a.herdr.Workspaces()
	}
	if err != nil {
		fmt.Fprintf(a.out, "herdr:    unreachable -- %v\n", err)
		return 1
	}
	plan := assign.MakePlan(agents, spaces)
	fmt.Fprintf(a.out, "herdr:    ok, %s and %s (herdrcolor %s)\n",
		plural(len(plan.Panes), "agent"), plural(len(plan.Spaces), "space"), Version)

	path, err := a.configPath()
	if err != nil {
		fmt.Fprintf(a.out, "config:   %v\n", err)
		return 1
	}
	fmt.Fprintf(a.out, "config:   %s\n", path)
	text, err := config.Read(path)
	if err != nil {
		fmt.Fprintf(a.out, "config:   unreadable -- %v\n", err)
		return 1
	}
	states, parseErr := config.Inspect(text)
	if parseErr != nil {
		fmt.Fprintf(a.out, "config:   does not parse -- %v\n", parseErr)
		ok = false
	}
	want := len(palette.Palette)
	for _, st := range states {
		n := len(st.Slots)
		switch {
		case st.HandWritten && n == want:
			fmt.Fprintf(a.out, "config:   ok, [%s] is hand-written and declares all %d slots\n", st.Section.Table, n)
		case st.HandWritten:
			fmt.Fprintf(a.out, "config:   [%s] is hand-written and declares %d of %d slots; merge in `herdrcolor snippet`\n",
				st.Section.Table, n, want)
			ok = false
		case n == want:
			fmt.Fprintf(a.out, "config:   ok, [%s] is in the managed block\n", st.Section.Table)
		default:
			fmt.Fprintf(a.out, "config:   [%s] is not written yet; run `herdrcolor sync`\n", st.Section.Table)
			ok = false
		}
	}
	if config.HasBlock(text) {
		if installed, err := a.herdr.PluginInstalled(palette.Source); err == nil && !installed {
			fmt.Fprintln(a.out, "config:   a managed block is left over, but herdrcolor is not installed; "+
				"`herdrcolor clear --config` removes it")
			ok = false
		}
	}

	livePanes := map[string]string{}
	for _, ag := range agents {
		livePanes[ag.PaneID] = reported(ag.Tokens)
	}
	liveSpaces := map[string]string{}
	for _, w := range spaces {
		liveSpaces[w.WorkspaceID] = reported(w.Tokens)
	}
	var stale []string
	for _, p := range plan.Panes {
		if livePanes[p.PaneID] != p.Colour.Hex {
			stale = append(stale, p.PaneID)
		}
	}
	for _, s := range plan.Spaces {
		if liveSpaces[s.WorkspaceID] != s.Colour.Hex {
			stale = append(stale, s.WorkspaceID)
		}
	}
	sort.Strings(stale)
	total := len(plan.Panes) + len(plan.Spaces)
	switch {
	case total == 0:
		fmt.Fprintln(a.out, "colours:  nothing to colour")
	case len(stale) > 0:
		fmt.Fprintf(a.out, "colours:  %d of %d not reported (%s); run `herdrcolor sync`\n",
			len(stale), total, strings.Join(stale, ", "))
		ok = false
	default:
		fmt.Fprintf(a.out, "colours:  ok, %d row(s) carry their colour\n", total)
	}
	if !ok {
		return 1
	}
	return 0
}

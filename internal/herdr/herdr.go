// Package herdr talks to Herdr by running its own CLI.
//
// The CLI rather than the socket: Herdr's transport is a Unix socket on macOS
// and Linux and a named pipe on Windows, and the CLI already resolves sessions
// and socket paths the way Herdr does. See docs/adr/0001. Every call is a
// short-lived process, which suits `sync`: it answers a handful of questions,
// reports, and exits.
package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/johnendean/herdrcolor/internal/assign"
)

// Client runs the herdr binary. Socket, when set, is passed to it as
// HERDR_SOCKET_PATH; otherwise the inherited environment decides, which is
// what makes a hook fired by a named session talk back to that session.
type Client struct {
	Bin     string
	Socket  string
	Timeout time.Duration
}

// New finds the herdr binary: $HERDR_BIN_PATH, which Herdr injects into plugin
// hooks, else `herdr` on PATH.
func New(socket string) *Client {
	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	return &Client{Bin: bin, Socket: socket, Timeout: 10 * time.Second}
}

// Error is a failed herdr call.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// Run runs one herdr command and returns its stdout.
func (c *Client) Run(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Env = os.Environ()
	if c.Socket != "" {
		cmd.Env = append(cmd.Env, "HERDR_SOCKET_PATH="+c.Socket)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return nil, &Error{fmt.Sprintf("herdr %s: %s", strings.Join(args[:min(2, len(args))], " "), detail)}
	}
	return stdout.Bytes(), nil
}

// call runs a command that answers with the socket API's JSON envelope and
// decodes its result.
func (c *Client) call(result any, args ...string) error {
	out, err := c.Run(args...)
	if err != nil {
		return err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return &Error{fmt.Sprintf("herdr %s: unreadable reply: %v", strings.Join(args, " "), err)}
	}
	if envelope.Error != nil {
		return &Error{fmt.Sprintf("herdr %s: %s: %s", strings.Join(args, " "), envelope.Error.Code, envelope.Error.Message)}
	}
	if len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}

// Agents is every pane that currently hosts an agent. It is a question, not a
// feed: the event hook decides when to ask, never what the answer is.
func (c *Client) Agents() ([]assign.Agent, error) {
	var r struct {
		Agents []assign.Agent `json:"agents"`
	}
	err := c.call(&r, "agent", "list")
	return r.Agents, err
}

// Workspaces is every space, with the label the spaces panel shows.
func (c *Client) Workspaces() ([]assign.Workspace, error) {
	var r struct {
		Workspaces []assign.Workspace `json:"workspaces"`
	}
	err := c.call(&r, "workspace", "list")
	return r.Workspaces, err
}

// tokenArgs turns a token set into flags, in a fixed order. Herdr's CLI takes no
// `--flag=value` form, so each value is its own argument; it always starts with
// the token name, so a project named `-x` is never read as a flag.
func tokenArgs(tokens assign.Tokens) []string {
	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	sort.Strings(names)
	var args []string
	for _, name := range names {
		if v := tokens[name]; v != nil {
			args = append(args, "--token", name+"="+*v)
		} else {
			args = append(args, "--clear-token", name)
		}
	}
	return args
}

// reportArgs builds a report-metadata call. Herdr's CLI wants the positional ID
// before any option (checked against 0.9.5: `--source x ID` is refused) and has
// no `--` separator; IDs such as `w3:p1` never begin with a dash.
func reportArgs(kind, id, source string, tokens assign.Tokens) []string {
	args := []string{kind, "report-metadata", id, "--source", source}
	return append(args, tokenArgs(tokens)...)
}

// ReportPane sets and clears this source's tokens on a pane in one call, so a
// pane is never briefly wearing two colours.
func (c *Client) ReportPane(paneID, source string, tokens assign.Tokens) error {
	_, err := c.Run(reportArgs("pane", paneID, source, tokens)...)
	return err
}

// ReportWorkspace does the same for a space.
func (c *Client) ReportWorkspace(workspaceID, source string, tokens assign.Tokens) error {
	_, err := c.Run(reportArgs("workspace", workspaceID, source, tokens)...)
	return err
}

// ReloadConfig asks the running server to re-read config.toml.
func (c *Client) ReloadConfig() error {
	_, err := c.Run("server", "reload-config")
	return err
}

// PluginInstalled reports whether Herdr lists this plugin, for `doctor`'s check
// on a managed block left behind after uninstall.
func (c *Client) PluginInstalled(id string) (bool, error) {
	out, err := c.Run("plugin", "list")
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), "("+id+")"), nil
}

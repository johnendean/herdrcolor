# herdrcolor

[![CI](https://github.com/johnendean/herdrcolor/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/johnendean/herdrcolor/actions/workflows/ci.yml?query=branch%3Amain)

Give each [Herdr](https://herdr.dev) agent its own colour in the sidebar.

Agents that all report as `claude` look alike in the agents pane. herdrcolor
gives every project a colour, in both the agents and the spaces panels, so a
glance tells you which pane is which.

```
ROW    NAME        SLOT  COLOUR         REPORTED
w1:p1  api-server  $c4   #89b4fa blue   #89b4fa
w2:p1  dashboard   $c3   #a6e3a1 green  #a6e3a1
w3:p1  firmware    $c2   #fab387 peach  #fab387
w4:p1  herdrcolor  $c1   #f38ba8 red    #f38ba8
w1     api-server  $c4   #89b4fa blue   #89b4fa
w2     dashboard   $c3   #a6e3a1 green  #a6e3a1
```

## Install

```sh
herdr plugin install johnendean/herdrcolor
```

That is the whole install, on macOS, Linux and Windows, x86_64 or arm64 (WSL
counts as Linux). Nothing else needs to be installed first: herdrcolor is a
single static binary.

Herdr clones the plugin and runs `bin/fetch.sh` (or `bin/fetch.ps1` on Windows).
The script downloads the release binary for your platform, refuses it unless its
SHA-256 matches `checksums.txt` in the commit Herdr cloned, and runs a first
`sync`. That sync colours what is running and writes the sidebar config for you
(see [The managed block](#the-managed-block)).

Windows is supported, but Herdr itself still calls plugins on Windows a
preview.

### Uninstall

Herdr has no uninstall hook, so it takes two steps:

```sh
herdrcolor clear --config        # remove the colours and the managed block
herdr plugin uninstall herdrcolor
```

Skip the first and nothing breaks: an empty slot renders as nothing.
`herdrcolor doctor` points out a block left behind.

## How it works

Herdr's sidebar can colour a token, and it can colour one conditionally on
**that token's own text** — but a rule cannot look at a second token. So there
is no way to say "colour the workspace name according to `$color`".

herdrcolor works round it by making the colour a question of *which* token holds
the text. Each sidebar section declares six tokens, `$c1` to `$c6`, each with a
fixed colour. The plugin puts the name in exactly one of them and clears the
other five. The sidebar renders whichever one has text, in that slot's colour,
and the config never changes when a new project appears.

This rests on empty tokens rendering as nothing, which is not documented
anywhere. Checked against a live 0.9.0 sidebar: five empty slots cost no
separator, no padding and no width — a row reads `○ herdrkeys · 1` either way.

Colours are published with `herdr pane report-metadata` for agents and
`herdr workspace report-metadata` for spaces, one call per row, carrying the
whole slot set at once. herdrcolor talks to Herdr only through the `herdr` CLI
(`$HERDR_BIN_PATH` in hooks), never the socket. That gives one transport
everywhere: Herdr uses a Unix socket on macOS and Linux and a named pipe on
Windows, and the CLI already knows which session to talk to. The plugin also
publishes `$color`, the
assigned hex, which nothing in the sidebar uses — it is there for tools that
draw their own pixels, [herdrkeys](https://github.com/johnendean/herdrkeys)
being the reason it exists.

There is no daemon. `sync` asks Herdr what exists, reports, and exits. It runs
from three places:

| Trigger                          | Why                                                |
| -------------------------------- | -------------------------------------------------- |
| `[[build]]` — on install, after the download | colour what is already running, rather than waiting |
| `[[startup]]` — on server start  | reported metadata does not survive a restart        |
| `[[events]]` — `pane.agent_detected` | a new agent is the only thing that can lack a colour |

Every run re-reports every pane and space, so a missed or replayed event cannot
leave anything stale. `agent list` is a question, not a feed: the event decides *when*
to ask, never what the answer is.

### The managed block

The six slots have to be declared in Herdr's `config.toml`
(`%APPDATA%\herdr\config.toml` on Windows, `~/.config/herdr/config.toml`
elsewhere, or `$HERDR_CONFIG_PATH`). `sync` writes them itself, between two
marker lines:

```toml
# >>> herdrcolor managed block >>>
[ui.sidebar.agents]
rows = [ ... ]

[ui.sidebar.spaces]
rows = [ ... ]
# <<< herdrcolor managed block <<<
```

herdrcolor changes text only between the markers, so your comments and settings
elsewhere in the file are kept exactly as they are. It saves the previous file as
`config.toml.herdrcolor.bak` before each change, and runs `herdr server
reload-config` only when it changed something. Every sync makes sure the block
matches the palette, so an update that changes the palette fixes the config on
its own.

A sidebar section you have written yourself, outside the block, is
**hand-written**, and herdrcolor never edits one. It leaves that section out of
the block, and `doctor` tells you whether it declares all six slots. If not,
merge in `herdrcolor snippet` by hand. herdrcolor also leaves the file alone
when it does not parse, or when adding the block would stop it parsing.

## Which colour a project gets

The project is the last component of the agent's directory — the name, not the
path, so the same repository earns the same colour on this machine and on a
remote one where it lives under a different home.

A directory is split on both `/` and `\`, so a Windows checkout and a Unix one
name the same project.

That name is hashed with SHA-256, so every process, every machine and every
version agrees, to pick a **preferred** slot. The hash alone is not
enough: six slots and five projects collide more often than not, and the first
live run put five agents on three colours. So a project keeps its hashed colour
unless a project it shares the screen with already holds it, in which case the
later one walks forward to the next free slot. Projects are considered in name
order, so the same set of agents always produces the same colours however they
started.

A space takes the colour of the agents inside it, which is the point of
colouring the spaces panel at all: the two halves of the sidebar have to agree.
The workspace a pane belongs to comes from its ID — `w6:p1` is in `w6` — since
`workspace.list` reports neither a directory nor its pane IDs. A space with no
agents falls back to hashing its **label**, for the same reason: Herdr reports
no directory for a workspace, so a renamed space can change colour.

## Commands

| Command                  | What it does                                        |
| ------------------------ | --------------------------------------------------- |
| `herdrcolor sync`        | colour every agent and space now, and keep the managed block current; idempotent |
| `herdrcolor list`        | assigned colours beside reported ones               |
| `herdrcolor snippet`     | print both sidebar sections, generated from the palette |
| `herdrcolor doctor`      | check Herdr, the config, and the colours            |
| `herdrcolor clear`       | remove the tokens this plugin set, and nothing else; `--config` also removes the managed block |
| `herdrcolor version`     | print the version                                   |

Flags go before or after the command: `--socket PATH` picks a Herdr socket,
`--config-file PATH` picks a `config.toml`, and `-v` prints each assignment.

`list` shows both columns because they can differ — after a palette change, or
a restart that dropped metadata before any hook ran. Seeing both tells "not
synced" from "synced, config not applied".

## Limits

- **Six colours.** Past six live projects the slots run out and the extras share
  their hashed colour. Six is about as many hues as stay distinguishable at a
  glance, and keeps room for matching LEDs later.
- **A colour can move.** When a colliding project appears or goes away, the
  loser of the tie-break changes colour. Distinctness beat stability: a colour
  is only useful if it tells two panes apart.
- **The project is a directory name.** Two repositories with the same directory
  name are one project as far as colour goes, and an agent started in a
  subdirectory is that subdirectory's project.
- **Agents in one project share a colour.** The colour names the project. Two
  agents in the same repository are not told apart by it.
- **A renamed space can change colour**, but only a space with no agents in
  it: one with agents inherits their colour, which is keyed on the directory.
- **On Windows, the project is where the agent was launched.** Herdr on
  Windows does not track a shell's later `cd`, so an agent started in
  `C:\code` and then moved into `C:\code\api` is coloured as `code`. Start the
  agent in the project's own directory.
- **Sidebar only.** Pane borders and the tab bar are unaffected.

## Development

You need Go, at the exact version in `go.mod`.

```sh
make test    # or: go test ./...
make link    # build bin/herdrcolor, then link this checkout into Herdr
```

A linked checkout uses the binary you built: Herdr runs no build step for
`plugin link`, and the fetch scripts skip the download when a binary is
already there. Rebuild after each change with `make build`.

The tests need no running Herdr: everything that decides a colour is a pure
function in `internal/assign` and `internal/palette`.
`internal/assign/testdata/parity.json` was generated by the Python version
this replaced. It proves every project keeps the colour it had, so never
regenerate it from the Go code.

CI runs the suite on macOS, Linux and Windows. It builds every release target
twice to prove the builds are reproducible, and runs both fetch scripts
against a local "release", checking that a good binary installs and a tampered
one is refused. `main` is protected on the `build` job, which gates the rest.

### Releasing

Install fetches the release named by `version` in `herdr-plugin.toml`, so
changes on `main` reach users only when a release is tagged. See
[ADR 0001](docs/adr/0001-go-binary-fetched-at-install.md) for why it works
this way.

1. Set the new version in `herdr-plugin.toml` and in `Version` in
   `cmd/herdrcolor/main.go`. A test fails if they differ.
2. `make release`, which builds every target into `dist/` and rewrites
   `checksums.txt`.
3. Commit both files, tag the commit `v<version>`, and push the tag.

The release workflow rebuilds from the tag. It publishes only if every binary
reproduces the committed checksums exactly.

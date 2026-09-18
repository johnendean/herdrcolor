# herdrcolor

Give each [Herdr](https://herdr.dev) agent its own colour in the sidebar.

Agents that all report as `claude` look alike in the agents pane. herdrcolor
gives every project a colour, so a glance tells you which pane is which.

```
PANE   PROJECT     SLOT  COLOUR         REPORTED
w1:p1  api-server  $c4   #89b4fa blue   #89b4fa
w2:p1  dashboard   $c3   #a6e3a1 green  #a6e3a1
w3:p1  firmware    $c2   #fab387 peach  #fab387
w4:p1  herdrcolor  $c1   #f38ba8 red    #f38ba8
w5:p1  infra       $c5   #cba6f7 mauve  #cba6f7
```

## Install

```sh
herdr plugin link "$PWD"     # or: make link
./bin/herdrcolor snippet     # print the config block
# paste it into ~/.config/herdr/config.toml
herdr server reload-config
./bin/herdrcolor sync
```

No install step and no dependencies: the system `python3` is enough.

## How it works

Herdr's sidebar can colour a token, and it can colour one conditionally on
**that token's own text** — but a rule cannot look at a second token. So there
is no way to say "colour the workspace name according to `$color`".

herdrcolor works round it by making the colour a question of *which* token holds
the text. The config declares six tokens, `$c1` to `$c6`, each with a fixed
colour. The plugin puts the project's name in exactly one of them and clears the
other five. The sidebar renders whichever one has text, in that slot's colour,
and the config never changes when a new project appears.

This rests on empty tokens rendering as nothing, which is not documented
anywhere. Checked against a live 0.9.0 sidebar: five empty slots cost no
separator, no padding and no width — a row reads `○ herdrkeys · 1` either way.

Colours are published with `pane.report_metadata`, one request per pane,
carrying the whole slot set at once. The plugin also publishes `$color`, the
assigned hex, which nothing in the sidebar uses — it is there for tools that
draw their own pixels, [herdrkeys](https://github.com/johnendean/herdrkeys)
being the reason it exists.

There is no daemon. `sync` asks Herdr what exists, reports, and exits. It runs
from three places:

| Trigger                          | Why                                                |
| -------------------------------- | -------------------------------------------------- |
| `[[build]]` — on install         | colour what is already running, rather than waiting |
| `[[startup]]` — on server start  | reported metadata does not survive a restart        |
| `[[events]]` — `pane.agent_detected` | a new agent is the only thing that can lack a colour |

Every run re-reports every pane, so a missed or replayed event cannot leave
anything stale. `agent.list` is a question, not a feed: the event decides *when*
to ask, never what the answer is.

## Which colour a project gets

The project is the last component of the agent's directory — the name, not the
path, so the same repository earns the same colour on this machine and on a
remote one where it lives under a different home.

That name is hashed (SHA-256, because each hook is a fresh process and `hash()`
is seeded per process) to pick a **preferred** slot. The hash alone is not
enough: six slots and five projects collide more often than not, and the first
live run put five agents on three colours. So a project keeps its hashed colour
unless a project it shares the screen with already holds it, in which case the
later one walks forward to the next free slot. Projects are considered in name
order, so the same set of agents always produces the same colours however they
started.

## Commands

| Command                  | What it does                                        |
| ------------------------ | --------------------------------------------------- |
| `herdrcolor sync`        | colour every agent now; idempotent                  |
| `herdrcolor list`        | assigned colours beside reported ones               |
| `herdrcolor snippet`     | print the config block, generated from the palette  |
| `herdrcolor doctor`      | check Herdr, the config, and the colours            |
| `herdrcolor clear`       | remove the tokens this plugin set, and nothing else |

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
- **Sidebar only.** Pane borders and the tab bar are unaffected.

## Development

```sh
make venv    # only needed for the tests
make test
```

The tests need no running Herdr: everything that decides a colour is a pure
function in `assign.py` and `palette.py`.

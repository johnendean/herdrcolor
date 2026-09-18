"""Turning a list of agents into the tokens each of their panes should carry.

Pure functions: nothing here touches the socket, so the interesting decisions
are testable without a running Herdr.

## Why slots

Herdr's sidebar can colour a token, and it can colour one conditionally on that
token's *own* text -- but a rule cannot look at a second token. So there is no
way to say "colour the workspace name according to `$color`".

The way round it is to make the colour a matter of *which* token holds the text.
The config declares six tokens, `$c1` to `$c6`, each with a fixed colour. This
plugin puts the project's name in exactly one of them and clears the other five.
The sidebar renders whichever one has text, in that slot's colour, and the
config never has to change when a project appears.
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import PurePath
from typing import Any

from . import palette
from .palette import Colour


@dataclass(frozen=True)
class Assignment:
    pane_id: str
    project: str  # what was hashed, and the text the sidebar shows
    colour: Colour


def project_name(cwd: str | None) -> str | None:
    """The project an agent is working on: the last component of its directory.

    The name rather than the whole path, so the same repository earns the same
    colour on this machine and on a remote one, where it sits under a different
    home. The trade is that two checkouts of different repositories sharing a
    directory name are one project as far as colour is concerned -- and that a
    git worktree named `myrepo-feature` is its own project, not a shade of
    `myrepo`, which is the more useful answer of the two for telling panes apart.
    """
    if not cwd:
        return None
    name = PurePath(cwd).name
    return name or None


def resolve(projects: set[str]) -> dict[str, Colour]:
    """A colour per project: the hashed one where possible, a free one where not.

    The hash alone is not good enough in practice. Six slots and five projects
    collide more often than not -- the first live run put five agents on three
    colours -- and two agents wearing one colour is the thing this plugin exists
    to prevent.

    So the hash becomes a preference rather than an answer. A project keeps its
    hashed colour unless a project it shares the screen with already has it, in
    which case the later one walks forward to the next free slot. Projects are
    considered in name order, not arrival order, so the same set of agents
    always produces the same colours however they started.

    Two consequences worth knowing. A project's colour can change when a
    colliding project appears or goes away -- stability gives way to
    distinctness, because a colour is only useful if it is telling two panes
    apart. And past six projects the slots run out; the extras keep their
    hashed colour and share.
    """
    assigned: dict[str, Colour] = {}
    taken: set[int] = set()
    for project in sorted(projects):
        preferred = palette.colour_for(project)
        slot = preferred.slot
        for step in range(len(palette.PALETTE)):
            candidate = (preferred.slot - 1 + step) % len(palette.PALETTE) + 1
            if candidate not in taken:
                slot = candidate
                break
        taken.add(slot)
        assigned[project] = palette.by_slot(slot)
    return assigned


def assignments(agents: list[dict[str, Any]]) -> list[Assignment]:
    """One assignment per agent pane whose directory we can name.

    `foreground_cwd` first: an agent that has followed you into a subdirectory
    is still working on the same project, but if Herdr reports only one of the
    two, use it rather than skipping the pane.

    Two agents in one project deliberately share a colour: the colour names the
    project, and pretending otherwise would mean a pane's colour depended on
    which of its siblings Herdr happened to list first.
    """
    named: list[tuple[str, str]] = []
    for agent in agents:
        pane_id = agent.get("pane_id")
        if not pane_id:
            continue
        project = project_name(agent.get("cwd")) or project_name(
            agent.get("foreground_cwd")
        )
        if not project:
            continue
        named.append((pane_id, project))

    colours = resolve({project for _, project in named})
    return [
        Assignment(pane_id, project, colours[project]) for pane_id, project in named
    ]


def tokens_for(assignment: Assignment) -> dict[str, str | None]:
    """The full token set for one pane: the filled slot, the empty ones, the hex.

    Every slot appears on every call, because a token this plugin does not
    mention is a token Herdr leaves as it was.
    """
    tokens: dict[str, str | None] = {
        palette.slot_token(slot): None for slot in palette.SLOTS
    }
    tokens[palette.slot_token(assignment.colour.slot)] = assignment.project
    tokens[palette.COLOUR_TOKEN] = assignment.colour.hex
    return tokens


def cleared_tokens() -> dict[str, str | None]:
    """Every token this plugin knows how to set, emptied."""
    tokens: dict[str, str | None] = {
        palette.slot_token(slot): None for slot in palette.SLOTS
    }
    tokens[palette.COLOUR_TOKEN] = None
    return tokens

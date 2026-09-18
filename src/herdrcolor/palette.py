"""The palette, and which colour a project gets.

Six colours, because six is about as many hues as stay distinguishable at a
glance in a sidebar -- and, later, on RGB LEDs. They are Catppuccin's, so the
sidebar agrees with Herdr's default theme.

A project's colour comes from a hash of its name rather than from the order
agents appeared in, so a project keeps the same colour across restarts, across
sessions, and on another machine. The cost is collisions: two projects can hash
to the same slot, and nothing here tries to avoid that. Stability was the point.
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass

# The metadata source name. Herdr scopes reported metadata by source, so this is
# what lets `clear` remove exactly what this plugin set and nothing else.
SOURCE = "herdrcolor"

# Token names are bare here and referenced as `$c1` in config: Herdr's token
# names allow only letters, digits, underscore and hyphen, and the `$` belongs
# to the sidebar's syntax, not to the name.
SLOT_TOKEN_PREFIX = "c"

# The hex of the assigned colour, published for consumers that draw their own
# pixels rather than reading the sidebar -- herdrkeys being the reason this
# exists. Nothing in the sidebar config uses it.
COLOUR_TOKEN = "color"


@dataclass(frozen=True)
class Colour:
    slot: int  # 1-based: the `$cN` token it is published in
    name: str  # Catppuccin's name for it, for `list` and `snippet` output
    hex: str


PALETTE: tuple[Colour, ...] = (
    Colour(1, "red", "#f38ba8"),
    Colour(2, "peach", "#fab387"),
    Colour(3, "green", "#a6e3a1"),
    Colour(4, "blue", "#89b4fa"),
    Colour(5, "mauve", "#cba6f7"),
    Colour(6, "teal", "#94e2d5"),
)

SLOTS = tuple(colour.slot for colour in PALETTE)


def slot_token(slot: int) -> str:
    return f"{SLOT_TOKEN_PREFIX}{slot}"


def by_slot(slot: int) -> Colour:
    return PALETTE[slot - 1]


def colour_for(project: str) -> Colour:
    """The colour this project always gets.

    SHA-256 rather than `hash()`, whose seed changes per process: the same
    project must land on the same colour in every invocation, and each event
    hook is its own short-lived process.
    """
    digest = hashlib.sha256(project.encode("utf-8")).digest()
    return PALETTE[int.from_bytes(digest[:8], "big") % len(PALETTE)]

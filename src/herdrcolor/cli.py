"""The command line: sync, list, snippet, clear, doctor.

`sync` is the whole plugin. The rest exist because a colour that appears by
magic is a colour you cannot debug.
"""

from __future__ import annotations

import argparse
import os
import sys
import tomllib
from pathlib import Path

from . import assign, herdr, palette

# The two sidebar sections this plugin colours, and the config table each one is
# configured under.
SECTIONS = {"agents": "ui.sidebar.agents", "spaces": "ui.sidebar.spaces"}


def _socket_path(args: argparse.Namespace) -> Path:
    return Path(args.socket) if args.socket else herdr.default_socket_path()


def _config_path() -> Path:
    explicit = os.environ.get("HERDR_CONFIG_PATH")
    if explicit:
        return Path(explicit)
    return Path.home() / ".config" / "herdr" / "config.toml"


def _plan(path: Path) -> assign.Plan:
    return assign.plan(herdr.agents(path), herdr.workspaces(path))


def cmd_sync(args: argparse.Namespace) -> int:
    """Give every agent and space the colour its project hashes to.

    Idempotent, and cheap enough to be: reporting the same tokens again costs
    one request per row and changes nothing on screen. That is what makes it
    safe to run from an event hook, a startup hook and a human, and why nothing
    here tries to remember what it last reported.
    """
    path = _socket_path(args)
    plan = _plan(path)

    for assignment in plan.panes:
        herdr.report_tokens(
            path,
            assignment.pane_id,
            assign.tokens_for(assignment),
            source=palette.SOURCE,
        )
        if args.verbose:
            print(
                f"{assignment.pane_id}  {assignment.project}  "
                f"${palette.slot_token(assignment.colour.slot)}  "
                f"{assignment.colour.hex}  {assignment.colour.name}"
            )
    for space in plan.spaces:
        herdr.report_workspace_tokens(
            path,
            space.workspace_id,
            assign.tokens_for(space),
            source=palette.SOURCE,
        )
        if args.verbose:
            print(
                f"{space.workspace_id}  {space.label}  "
                f"${palette.slot_token(space.colour.slot)}  "
                f"{space.colour.hex}  {space.colour.name}"
            )

    print(
        f"herdrcolor: coloured {len(plan.panes)} agent"
        f"{'' if len(plan.panes) == 1 else 's'} and "
        f"{len(plan.spaces)} space{'' if len(plan.spaces) == 1 else 's'}"
    )
    return 0


def _table(rows: list[tuple[str, ...]]) -> None:
    widths = [max(len(row[i]) for row in rows) for i in range(len(rows[0]))]
    for row in rows:
        print("  ".join(cell.ljust(widths[i]) for i, cell in enumerate(row)).rstrip())


def cmd_list(args: argparse.Namespace) -> int:
    """What each row is wearing, and what it should be wearing.

    The two can differ -- after a palette change, or a server restart that
    dropped reported metadata before any hook ran again -- and seeing both is
    the fastest way to tell "not synced" from "synced, config not applied".
    """
    path = _socket_path(args)
    agents = herdr.agents(path)
    spaces = herdr.workspaces(path)
    plan = assign.plan(agents, spaces)

    live_panes = {
        agent.get("pane_id"): (agent.get("tokens") or {}).get(palette.COLOUR_TOKEN)
        for agent in agents
    }
    live_spaces = {
        space.get("workspace_id"): (space.get("tokens") or {}).get(
            palette.COLOUR_TOKEN
        )
        for space in spaces
    }

    rows: list[tuple[str, ...]] = [("ROW", "NAME", "SLOT", "COLOUR", "REPORTED")]
    for assignment in plan.panes:
        rows.append(
            (
                assignment.pane_id,
                assignment.project,
                f"${palette.slot_token(assignment.colour.slot)}",
                f"{assignment.colour.hex} {assignment.colour.name}",
                live_panes.get(assignment.pane_id) or "-",
            )
        )
    for space in plan.spaces:
        rows.append(
            (
                space.workspace_id,
                space.label,
                f"${palette.slot_token(space.colour.slot)}",
                f"{space.colour.hex} {space.colour.name}",
                live_spaces.get(space.workspace_id) or "-",
            )
        )

    if len(rows) == 1:
        print("herdrcolor: nothing to colour")
        return 0
    _table(rows)
    return 0


SNIPPET_HEADER = """\
# herdrcolor: six colour slots per sidebar section. Paste into
# ~/.config/herdr/config.toml, then run `herdr server reload-config`.
#
# Exactly one slot ever holds text, so exactly one colour shows. herdrcolor
# decides which; this block only says what each slot looks like. The other five
# slots cost nothing: an empty token renders as no text, no separator and no
# padding."""


def _slot_lines(indent: str) -> list[str]:
    return [
        f'{indent}{{ token = "${palette.slot_token(colour.slot)}", '
        f'fg = "{colour.hex}", bold = true }},  # {colour.name}'
        for colour in palette.PALETTE
    ]


def cmd_snippet(args: argparse.Namespace) -> int:
    """Print the config block, generated from the palette so the two agree."""
    print(SNIPPET_HEADER)
    print()
    print(f"[{SECTIONS['agents']}]")
    print("rows = [")
    print('  ["state_icon", "machine",')
    for line in _slot_lines("    "):
        print(line)
    print('    "tab"],')
    print('  ["agent"],')
    print("]")
    print()
    print(f"[{SECTIONS['spaces']}]")
    print("rows = [")
    print('  ["state_icon",')
    for line in _slot_lines("    "):
        print(line)
    print("  ],")
    print('  ["branch", "git_status"],')
    print("]")
    return 0


def cmd_clear(args: argparse.Namespace) -> int:
    """Remove every token this plugin set, leaving other sources alone."""
    path = _socket_path(args)
    panes = cleared = 0
    for agent in herdr.agents(path):
        pane_id = agent.get("pane_id")
        if not pane_id:
            continue
        herdr.report_tokens(
            path, pane_id, assign.cleared_tokens(), source=palette.SOURCE
        )
        panes += 1
    for space in herdr.workspaces(path):
        workspace_id = space.get("workspace_id")
        if not workspace_id:
            continue
        herdr.report_workspace_tokens(
            path, workspace_id, assign.cleared_tokens(), source=palette.SOURCE
        )
        cleared += 1
    print(f"herdrcolor: cleared {panes} pane(s) and {cleared} space(s)")
    return 0


def declared_slots(config_text: str) -> dict[str, set[int]]:
    """Which slots each sidebar section declares, per the config file.

    Parsed rather than grepped: both sections declare the same six token names,
    so counting `$c1` in the file cannot tell a half-configured sidebar from a
    fully configured one -- which is the failure `doctor` exists to catch.
    """
    try:
        config = tomllib.loads(config_text)
    except tomllib.TOMLDecodeError:
        return {name: set() for name in SECTIONS}

    found: dict[str, set[int]] = {}
    for name, table in SECTIONS.items():
        node: object = config
        for key in table.split("."):
            node = node.get(key, {}) if isinstance(node, dict) else {}
        rows = node.get("rows", []) if isinstance(node, dict) else []
        slots: set[int] = set()
        for row in rows if isinstance(rows, list) else []:
            for token in row if isinstance(row, list) else []:
                name_of = token.get("token") if isinstance(token, dict) else token
                for colour in palette.PALETTE:
                    if name_of == f"${palette.slot_token(colour.slot)}":
                        slots.add(colour.slot)
        found[name] = slots
    return found


def cmd_doctor(args: argparse.Namespace) -> int:
    """Report on the three things that have to be true for a colour to appear."""
    path = _socket_path(args)
    ok = True

    try:
        plan = _plan(path)
    except herdr.HerdrError as exc:
        print(f"herdr:    unreachable -- {exc}")
        return 1
    print(
        f"herdr:    ok, {len(plan.panes)} agent(s) and {len(plan.spaces)} space(s) "
        f"at {path}"
    )

    config = _config_path()
    slots = declared_slots(config.read_text() if config.exists() else "")
    for section, table in SECTIONS.items():
        declared = slots[section]
        if len(declared) == len(palette.PALETTE):
            print(f"config:   ok, [{table}] declares all {len(declared)} slots")
        elif declared:
            print(
                f"config:   [{table}] declares {len(declared)} of "
                f"{len(palette.PALETTE)} slots; run `herdrcolor snippet`"
            )
            ok = False
        else:
            print(f"config:   [{table}] declares no slots; run `herdrcolor snippet`")
            ok = False

    stale = _stale(path, plan)
    total = len(plan.panes) + len(plan.spaces)
    if not total:
        print("colours:  nothing to colour")
    elif stale:
        print(
            f"colours:  {len(stale)} of {total} not reported "
            f"({', '.join(stale)}); run `herdrcolor sync`"
        )
        ok = False
    else:
        print(f"colours:  ok, {total} row(s) carry their colour")

    return 0 if ok else 1


def _stale(path: Path, plan: assign.Plan) -> list[str]:
    """Rows whose reported colour is missing or not the one they should have."""
    reported = {
        agent.get("pane_id"): (agent.get("tokens") or {}).get(palette.COLOUR_TOKEN)
        for agent in herdr.agents(path)
    }
    reported.update(
        {
            space.get("workspace_id"): (space.get("tokens") or {}).get(
                palette.COLOUR_TOKEN
            )
            for space in herdr.workspaces(path)
        }
    )
    stale = [a.pane_id for a in plan.panes if reported.get(a.pane_id) != a.colour.hex]
    stale += [
        s.workspace_id
        for s in plan.spaces
        if reported.get(s.workspace_id) != s.colour.hex
    ]
    return stale


def build_parser() -> argparse.ArgumentParser:
    """A parser that takes its flags on either side of the subcommand.

    `herdrcolor sync --verbose` is what a person types and what the plugin
    manifest ran, and it failed until the flags were declared in both places.
    `SUPPRESS` is what makes that safe: without it the subparser's own default
    would overwrite a value already given before the subcommand.
    """
    flags = argparse.ArgumentParser(add_help=False)
    flags.add_argument(
        "--socket",
        default=argparse.SUPPRESS,
        help="path to herdr.sock (default: $HERDR_SOCKET_PATH, else the default session)",
    )
    flags.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        default=argparse.SUPPRESS,
        help="print each assignment",
    )

    parser = argparse.ArgumentParser(
        prog="herdrcolor",
        description="Give each Herdr agent its own colour.",
        parents=[flags],
    )
    sub = parser.add_subparsers(dest="command")
    for name, help_text in (
        ("sync", "colour every agent and space now (idempotent)"),
        ("list", "show assigned and reported colours"),
        ("snippet", "print the config.toml blocks for the palette"),
        ("clear", "remove the tokens this plugin set"),
        ("doctor", "check Herdr, the config, and the colours"),
    ):
        sub.add_parser(name, help=help_text, parents=[flags])
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    # Suppressed defaults mean these are absent unless someone passed them.
    args.socket = getattr(args, "socket", None)
    args.verbose = getattr(args, "verbose", False)

    commands = {
        "sync": cmd_sync,
        "list": cmd_list,
        "snippet": cmd_snippet,
        "clear": cmd_clear,
        "doctor": cmd_doctor,
    }
    if args.command is None:
        parser.print_help()
        return 2
    try:
        return commands[args.command](args)
    except herdr.HerdrError as exc:
        print(f"herdrcolor: {exc}", file=sys.stderr)
        return 1

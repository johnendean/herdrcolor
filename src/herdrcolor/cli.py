"""The command line: sync, list, snippet, clear, doctor.

`sync` is the whole plugin. The rest exist because a colour that appears by
magic is a colour you cannot debug.
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

from . import assign, herdr, palette


def _socket_path(args: argparse.Namespace) -> Path:
    return Path(args.socket) if args.socket else herdr.default_socket_path()


def _config_path() -> Path:
    explicit = os.environ.get("HERDR_CONFIG_PATH")
    if explicit:
        return Path(explicit)
    return Path.home() / ".config" / "herdr" / "config.toml"


def cmd_sync(args: argparse.Namespace) -> int:
    """Give every agent pane the colour its project hashes to.

    Idempotent, and cheap enough to be: reporting the same tokens again costs
    one request per pane and changes nothing on screen. That is what makes it
    safe to run from an event hook, a startup hook and a human, and why nothing
    here tries to remember what it last reported.
    """
    path = _socket_path(args)
    agents = herdr.agents(path)
    done = 0
    for assignment in assign.assignments(agents):
        herdr.report_tokens(
            path,
            assignment.pane_id,
            assign.tokens_for(assignment),
            source=palette.SOURCE,
        )
        done += 1
        if args.verbose:
            print(
                f"{assignment.pane_id}  {assignment.project}  "
                f"${palette.slot_token(assignment.colour.slot)}  "
                f"{assignment.colour.hex}  {assignment.colour.name}"
            )
    skipped = len(agents) - done
    note = f", {skipped} without a directory" if skipped else ""
    print(f"herdrcolor: coloured {done} agent{'' if done == 1 else 's'}{note}")
    return 0


def cmd_list(args: argparse.Namespace) -> int:
    """What each agent is wearing, and what it should be wearing.

    The two can differ -- after a palette change, or a server restart that
    dropped reported metadata before any hook ran again -- and seeing both is
    the fastest way to tell "not synced" from "synced, config not applied".
    """
    path = _socket_path(args)
    agents = herdr.agents(path)
    by_pane = {agent.get("pane_id"): agent for agent in agents}
    rows = [("PANE", "PROJECT", "SLOT", "COLOUR", "REPORTED")]
    for assignment in assign.assignments(agents):
        reported = (by_pane.get(assignment.pane_id) or {}).get("tokens") or {}
        live = reported.get(palette.COLOUR_TOKEN) or "-"
        rows.append(
            (
                assignment.pane_id,
                assignment.project,
                f"${palette.slot_token(assignment.colour.slot)}",
                f"{assignment.colour.hex} {assignment.colour.name}",
                live,
            )
        )
    if len(rows) == 1:
        print("herdrcolor: no agents with a directory")
        return 0
    widths = [max(len(row[i]) for row in rows) for i in range(len(rows[0]))]
    for row in rows:
        print("  ".join(cell.ljust(widths[i]) for i, cell in enumerate(row)).rstrip())
    return 0


SNIPPET_HEADER = """\
# herdrcolor: six colour slots for the agent rows. Paste into
# ~/.config/herdr/config.toml, then run `herdr server reload-config`.
#
# Exactly one slot ever holds text, so exactly one colour shows. herdrcolor
# decides which; this block only says what each slot looks like."""


def cmd_snippet(args: argparse.Namespace) -> int:
    """Print the config block, generated from the palette so the two agree."""
    print(SNIPPET_HEADER)
    print("[ui.sidebar.agents]")
    print("rows = [")
    print('  ["state_icon", "machine",')
    for colour in palette.PALETTE:
        token = palette.slot_token(colour.slot)
        print(
            f'    {{ token = "${token}", fg = "{colour.hex}" }},'
            f"  # {colour.name}"
        )
    print('    "tab"],')
    print('  ["agent"],')
    print("]")
    return 0


def cmd_clear(args: argparse.Namespace) -> int:
    """Remove every token this plugin set, leaving other sources alone."""
    path = _socket_path(args)
    agents = herdr.agents(path)
    cleared = 0
    for agent in agents:
        pane_id = agent.get("pane_id")
        if not pane_id:
            continue
        herdr.report_tokens(
            path, pane_id, assign.cleared_tokens(), source=palette.SOURCE
        )
        cleared += 1
    print(f"herdrcolor: cleared {cleared} pane{'' if cleared == 1 else 's'}")
    return 0


def cmd_doctor(args: argparse.Namespace) -> int:
    """Report on the three things that have to be true for a colour to appear."""
    path = _socket_path(args)
    ok = True

    try:
        agents = herdr.agents(path)
    except herdr.HerdrError as exc:
        print(f"herdr:    unreachable -- {exc}")
        return 1
    print(f"herdr:    ok, {len(agents)} agent(s) at {path}")

    config = _config_path()
    text = config.read_text() if config.exists() else ""
    declared = [
        palette.slot_token(colour.slot)
        for colour in palette.PALETTE
        if f'"${palette.slot_token(colour.slot)}"' in text
    ]
    if len(declared) == len(palette.PALETTE):
        print(f"config:   ok, all {len(declared)} slots declared in {config}")
    elif declared:
        print(
            f"config:   partial -- {len(declared)} of {len(palette.PALETTE)} slots "
            f"in {config}; run `herdrcolor snippet`"
        )
        ok = False
    else:
        print(f"config:   no slots declared in {config}; run `herdrcolor snippet`")
        ok = False

    wanted = assign.assignments(agents)
    by_pane = {agent.get("pane_id"): agent for agent in agents}
    stale = [
        assignment.pane_id
        for assignment in wanted
        if ((by_pane.get(assignment.pane_id) or {}).get("tokens") or {}).get(
            palette.COLOUR_TOKEN
        )
        != assignment.colour.hex
    ]
    if not wanted:
        print("colours:  nothing to colour")
    elif stale:
        print(
            f"colours:  {len(stale)} of {len(wanted)} not reported "
            f"({', '.join(stale)}); run `herdrcolor sync`"
        )
        ok = False
    else:
        print(f"colours:  ok, {len(wanted)} agent(s) carry their colour")

    return 0 if ok else 1


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
        ("sync", "colour every agent now (idempotent)"),
        ("list", "show assigned and reported colours"),
        ("snippet", "print the config.toml block for the palette"),
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

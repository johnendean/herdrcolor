from herdrcolor import cli


def parse(argv):
    args = cli.build_parser().parse_args(argv)
    return getattr(args, "command", None), getattr(args, "verbose", False), getattr(
        args, "socket", None
    )


def test_flags_are_accepted_after_the_subcommand():
    # The manifest ran `sync --verbose`, which argparse rejected until the
    # flags were declared on the subparsers too.
    assert parse(["sync", "--verbose"]) == ("sync", True, None)


def test_flags_are_accepted_before_the_subcommand():
    assert parse(["--verbose", "sync"]) == ("sync", True, None)


def test_a_flag_given_early_survives_the_subparser():
    assert parse(["--socket", "/tmp/x.sock", "sync"]) == ("sync", False, "/tmp/x.sock")


def test_flags_on_both_sides_do_not_cancel_out():
    command, verbose, socket = parse(["--socket", "/tmp/x.sock", "list", "--verbose"])
    assert (command, verbose, socket) == ("list", True, "/tmp/x.sock")


SNIPPET = """
[ui.sidebar.agents]
rows = [["state_icon", { token = "$c1", fg = "#f38ba8", bold = true },
         { token = "$c2", fg = "#fab387" }]]

[ui.sidebar.spaces]
rows = [["state_icon", { token = "$c1", fg = "#f38ba8" }]]
"""


def test_declared_slots_reads_each_section_separately():
    # Grepping the file could not tell these apart: both sections name `$c1`.
    found = cli.declared_slots(SNIPPET)
    assert found["agents"] == {1, 2}
    assert found["spaces"] == {1}


def test_declared_slots_survives_a_broken_config():
    assert cli.declared_slots("rows = [[[") == {"agents": set(), "spaces": set()}


def test_declared_slots_on_a_config_with_no_sidebar_config():
    assert cli.declared_slots('[theme]\nname = "catppuccin"') == {
        "agents": set(),
        "spaces": set(),
    }

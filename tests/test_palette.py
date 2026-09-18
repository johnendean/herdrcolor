from herdrcolor import palette


def test_colour_is_stable_for_a_name():
    assert palette.colour_for("herdrkeys") == palette.colour_for("herdrkeys")


def test_colour_does_not_depend_on_process_hash_seed():
    # Guards the reason SHA-256 is used: each event hook is a new process, and a
    # colour that changed per process would flicker on every agent it touched.
    assert palette.colour_for("herdrcolor").hex == "#f38ba8"


def test_every_slot_is_reachable():
    names = [f"project-{n}" for n in range(400)]
    assert {palette.colour_for(n).slot for n in names} == set(palette.SLOTS)


def test_slot_token_names_are_what_herdr_accepts():
    import re

    for slot in palette.SLOTS:
        assert re.fullmatch(r"[A-Za-z0-9_-]{1,32}", palette.slot_token(slot))

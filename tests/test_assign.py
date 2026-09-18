from herdrcolor import assign, palette


def agent(pane_id, cwd, **extra):
    return {"pane_id": pane_id, "cwd": cwd, **extra}


def test_project_is_the_last_path_component():
    assert assign.project_name("/Users/dev/code/dashboard") == "dashboard"
    assert assign.project_name("/Users/dev/code/dashboard/") == "dashboard"
    assert assign.project_name("") is None
    assert assign.project_name(None) is None


def test_falls_back_to_foreground_cwd():
    [result] = assign.assignments(
        [{"pane_id": "w1:p1", "foreground_cwd": "/src/infra"}]
    )
    assert result.project == "infra"


def test_panes_without_a_directory_are_skipped():
    assert assign.assignments([{"pane_id": "w1:p1"}]) == []
    assert assign.assignments([agent(None, "/src/infra")]) == []


def test_tokens_fill_one_slot_and_clear_the_rest():
    [result] = assign.assignments([agent("w1:p1", "/src/infra")])
    tokens = assign.tokens_for(result)

    filled = palette.slot_token(result.colour.slot)
    assert tokens[filled] == "infra"
    assert tokens[palette.COLOUR_TOKEN] == result.colour.hex

    others = [
        palette.slot_token(slot) for slot in palette.SLOTS if slot != result.colour.slot
    ]
    assert all(tokens[name] is None for name in others)
    # Every slot on every call: an unmentioned token is one Herdr leaves alone.
    assert set(tokens) == {palette.slot_token(s) for s in palette.SLOTS} | {
        palette.COLOUR_TOKEN
    }


def test_cleared_tokens_empty_everything_this_plugin_sets():
    tokens = assign.cleared_tokens()
    assert set(tokens) == {palette.slot_token(s) for s in palette.SLOTS} | {
        palette.COLOUR_TOKEN
    }
    assert all(value is None for value in tokens.values())


def test_token_count_fits_herdrs_limit():
    [result] = assign.assignments([agent("w1:p1", "/src/infra")])
    assert len(assign.tokens_for(result)) <= 16


def test_colliding_projects_get_different_colours():
    # These four hash onto two slots, which is the situation that made pure
    # hashing unusable: a live run put five agents on three colours.
    projects = {"project-2", "project-5", "project-7", "project-9"}
    assert len({palette.colour_for(p).slot for p in projects}) < len(projects)

    colours = assign.resolve(projects)
    assert len({colour.slot for colour in colours.values()}) == len(projects)


def test_resolution_does_not_depend_on_agent_order():
    forward = [
        agent("w1:p1", "/src/herdrcolor"),
        agent("w2:p1", "/src/api-server"),
    ]
    backward = list(reversed(forward))
    as_seen = {a.pane_id: a.colour for a in assign.assignments(forward)}
    reversed_seen = {a.pane_id: a.colour for a in assign.assignments(backward)}
    assert as_seen == reversed_seen


def test_a_project_keeps_its_hashed_colour_when_nothing_collides():
    colours = assign.resolve({"infra"})
    assert colours["infra"] == palette.colour_for("infra")


def test_panes_in_the_same_project_share_one_colour():
    results = assign.assignments(
        [agent("w1:p1", "/src/infra"), agent("w1:p2", "/src/infra")]
    )
    assert results[0].colour == results[1].colour


def test_a_subdirectory_is_a_project_of_its_own():
    # A known limit of naming the project after the last path component: an
    # agent started in a subdirectory is not recognised as the same project.
    results = assign.assignments(
        [agent("w1:p1", "/src/infra"), agent("w1:p2", "/src/infra/docs")]
    )
    assert results[0].project == "infra"
    assert results[1].project == "docs"


def test_more_projects_than_slots_still_assigns_every_one():
    projects = {f"project-{n}" for n in range(20)}
    colours = assign.resolve(projects)
    assert set(colours) == projects


def space(workspace_id, label):
    return {"workspace_id": workspace_id, "label": label}


def test_a_space_takes_the_colour_of_the_agents_inside_it():
    plan = assign.plan(
        [agent("w6:p1", "/src/api-server")], [space("w6", "api-server")]
    )
    assert plan.spaces[0].colour == plan.panes[0].colour


def test_a_space_keeps_its_colour_when_its_agent_sits_in_a_subdirectory():
    # The pane is coloured for `docs`, the space for `api-server`, and the two
    # deliberately differ -- but the space still inherits the pane's colour so
    # the two panels agree on screen.
    plan = assign.plan(
        [agent("w6:p1", "/src/api-server/docs")], [space("w6", "api-server")]
    )
    assert plan.panes[0].project == "docs"
    assert plan.spaces[0].colour == plan.panes[0].colour


def test_a_space_with_no_agents_is_coloured_by_its_label():
    plan = assign.plan([], [space("w9", "infra")])
    assert plan.spaces[0].colour == palette.colour_for("infra")


def test_an_empty_space_does_not_steal_an_agents_colour():
    # Both hash to the same slot; the sidebar must still show two colours.
    plan = assign.plan(
        [agent("w1:p1", "/src/project-2")], [space("w9", "project-5")]
    )
    assert plan.panes[0].colour != plan.spaces[0].colour


def test_space_colour_does_not_depend_on_pane_order():
    panes = [agent("w6:p2", "/src/dashboard"), agent("w6:p1", "/src/api-server")]
    forward = assign.plan(panes, [space("w6", "api-server")])
    backward = assign.plan(list(reversed(panes)), [space("w6", "api-server")])
    assert forward.spaces[0].colour == backward.spaces[0].colour


def test_spaces_without_a_label_are_skipped():
    plan = assign.plan([], [{"workspace_id": "w9"}, space("wA", "infra")])
    assert [s.workspace_id for s in plan.spaces] == ["wA"]


def test_workspace_of_reads_the_pane_id_prefix():
    assert assign.workspace_of("w6:p1") == "w6"
    assert assign.workspace_of("wB:p12") == "wB"


def test_space_tokens_carry_the_label():
    plan = assign.plan([], [space("w9", "infra")])
    tokens = assign.tokens_for(plan.spaces[0])
    slot = palette.slot_token(plan.spaces[0].colour.slot)
    assert tokens[slot] == "infra"
    assert tokens[palette.COLOUR_TOKEN] == plan.spaces[0].colour.hex

"""Talking to Herdr over its unix socket.

Newline-delimited JSON, one request per connection. Everything here is
short-lived: `sync` runs from an event hook, answers a handful of requests and
exits, so there is no subscription and no connection to keep.
"""

from __future__ import annotations

import json
import os
import socket
from pathlib import Path
from typing import Any


class HerdrError(RuntimeError):
    pass


def default_socket_path() -> Path:
    """The socket to use, resolved the way Herdr's own CLI resolves it.

    `HERDR_SOCKET_PATH` first, because Herdr injects it into plugin hooks and
    into every managed pane: a hook fired by a named session must talk back to
    that same session, not to the default one.
    """
    explicit = os.environ.get("HERDR_SOCKET_PATH")
    if explicit:
        return Path(explicit)
    base = Path.home() / ".config" / "herdr"
    session = os.environ.get("HERDR_SESSION")
    if session:
        return base / "sessions" / session / "herdr.sock"
    return base / "herdr.sock"


def request(
    path: Path,
    method: str,
    params: dict[str, Any] | None = None,
    *,
    timeout: float = 5.0,
) -> dict[str, Any]:
    """One request on its own connection. Raises HerdrError on an error reply."""
    payload = {"id": f"herdrcolor:{method}", "method": method, "params": params or {}}
    try:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(timeout)
        sock.connect(str(path))
    except OSError as exc:
        raise HerdrError(f"cannot reach Herdr at {path}: {exc}") from exc
    with sock:
        sock.sendall((json.dumps(payload) + "\n").encode())
        buffer = b""
        while b"\n" not in buffer:
            chunk = sock.recv(65536)
            if not chunk:
                raise HerdrError(f"{method}: connection closed before a reply")
            buffer += chunk
    reply = json.loads(buffer.split(b"\n", 1)[0])
    if "error" in reply:
        error = reply["error"] or {}
        raise HerdrError(f"{method}: {error.get('code')}: {error.get('message')}")
    return reply.get("result") or {}


def agents(path: Path, *, timeout: float = 5.0) -> list[dict[str, Any]]:
    """Every pane that currently hosts an agent.

    A question rather than a feed, so it cannot fall behind: the event hook only
    decides *when* to ask, never what the answer is. That is also why a missed
    event costs nothing -- the next sync sees the same truth.
    """
    return request(path, "agent.list", timeout=timeout).get("agents") or []


def report_tokens(
    path: Path,
    pane_id: str,
    tokens: dict[str, str | None],
    *,
    source: str,
    timeout: float = 5.0,
) -> None:
    """Set and clear this source's tokens on a pane in one request.

    One call carries the whole slot set -- the assigned colour and nulls for the
    other five -- so a pane is never briefly wearing two colours, and a project
    that rehashes after a palette change cannot leave a stale slot behind.
    """
    request(
        path,
        "pane.report_metadata",
        {"pane_id": pane_id, "source": source, "tokens": tokens},
        timeout=timeout,
    )

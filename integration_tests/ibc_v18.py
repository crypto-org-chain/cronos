"""
IBC test utilities for v1.8 upgrade testing.

Verifies that the ClientStates REST endpoint is panic-free after the v1.8
upgrade, which prunes stale consensusStates subkeys that could previously
cause an unauthenticated REST DoS.
"""

import requests


def preupgrade_ibc_snapshot(port):
    """
    Snapshot IBC client states before the v1.8 upgrade.

    Returns a dict passed to check_ibc_client_states to verify all
    pre-upgrade clients survive the upgrade intact.
    """
    url = f"http://127.0.0.1:{port}/ibc/core/client/v1/client_states"
    resp = requests.get(url)
    assert resp.status_code == 200, (
        f"GET {url} returned {resp.status_code}: {resp.text}"
    )
    data = resp.json()
    assert "client_states" in data, f"missing client_states in response: {data}"
    return {
        "client_ids": [cs["client_id"] for cs in data.get("client_states", [])],
    }


def check_ibc_client_states(port, snapshot=None):
    """
    GET /ibc/core/client/v1/client_states must return 200 OK.

    Stale clients/<id>/consensusStates/<rev>/<h>/clientState keys (left by the
    pre-v9 ibc-go migration) trigger a proto-decoder panic in the ClientStates
    gRPC handler. The v1.8 upgrade handler prunes those keys; this check
    confirms the endpoint is reachable after upgrade.

    If snapshot (from preupgrade_ibc_snapshot) is provided, also asserts that
    all pre-upgrade client IDs are still present — the upgrade must not drop
    any existing IBC clients.
    """
    url = f"http://127.0.0.1:{port}/ibc/core/client/v1/client_states"
    resp = requests.get(url)
    assert resp.status_code == 200, (
        f"GET {url} returned {resp.status_code}: {resp.text}"
    )
    data = resp.json()
    assert "client_states" in data, f"missing client_states field in response: {data}"

    if snapshot is not None:
        post_ids = {cs["client_id"] for cs in data.get("client_states", [])}
        for cid in snapshot["client_ids"]:
            assert cid in post_ids, (
                f"IBC client {cid!r} missing after v1.8 upgrade; "
                f"before: {snapshot['client_ids']}, after: {sorted(post_ids)}"
            )

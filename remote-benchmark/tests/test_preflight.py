from types import SimpleNamespace

from remote_benchmark import preflight as preflight_module
from remote_benchmark.preflight import (
    peer_connectivity_matrix,
    probe_peers,
    resolved_mempool_types,
    unreachable_nodes,
)


def _endpoint(name, rpc, node_config=None):
    return SimpleNamespace(name=name, rpc=rpc, node_config=node_config or {})


def test_resolved_mempool_types_reports_declared_value_or_none():
    endpoints = [
        _endpoint("a", "http://a", {"mempool.type": "app"}),
        _endpoint("b", "http://b"),
    ]

    assert resolved_mempool_types(endpoints) == {"a": "app", "b": None}


def test_peer_connectivity_matrix_marks_connected_pairs(monkeypatch):
    # All three nodes share a host, as they do on a local devnet: only the node
    # IDs distinguish a real peer link from an address coincidence.
    endpoints = [
        _endpoint("a", "http://127.0.0.1:26657"),
        _endpoint("b", "http://127.0.0.1:26667"),
        _endpoint("c", "http://127.0.0.1:26677"),
    ]
    ids = {
        "http://127.0.0.1:26657": "id-a",
        "http://127.0.0.1:26667": "id-b",
        "http://127.0.0.1:26677": "id-c",
    }
    peers_by_rpc = {
        "http://127.0.0.1:26657": ["id-b"],
        "http://127.0.0.1:26667": ["id-a", "id-c"],
        "http://127.0.0.1:26677": [],
    }

    monkeypatch.setattr(preflight_module, "node_id", lambda rpc: ids[rpc])
    monkeypatch.setattr(
        preflight_module,
        "net_info",
        lambda rpc: {
            "result": {"peers": [{"node_info": {"id": i}} for i in peers_by_rpc[rpc]]}
        },
    )

    matrix = peer_connectivity_matrix(endpoints)

    assert matrix["a"] == {"b": True, "c": False}
    assert matrix["b"] == {"a": True, "c": True}
    assert matrix["c"] == {"a": False, "b": False}


def test_peer_connectivity_matrix_degrades_gracefully_for_unreachable_endpoint(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fake_net_info(rpc):
        if rpc == "http://b":
            raise ConnectionError("unreachable")
        return {"result": {"peers": []}}

    def fake_node_id(rpc):
        if rpc == "http://b":
            raise ConnectionError("unreachable")
        return "id-a"

    monkeypatch.setattr(preflight_module, "net_info", fake_net_info)
    monkeypatch.setattr(preflight_module, "node_id", fake_node_id)

    matrix = peer_connectivity_matrix(endpoints)

    # b's identity is unknown, so a's link to it is unknown rather than absent.
    assert matrix["a"] == {"b": None}
    assert matrix["b"] == {"a": None}


def test_unreachable_nodes_names_only_the_node_that_did_not_answer(monkeypatch):
    # With two endpoints the live node's row is all-None too, so the matrix shape
    # can't tell which node is down — the probe can.
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fail_for_b(rpc):
        if rpc == "http://b":
            raise ConnectionError("unreachable")
        return "id-a"

    monkeypatch.setattr(preflight_module, "node_id", fail_for_b)
    monkeypatch.setattr(
        preflight_module,
        "net_info",
        lambda rpc: fail_for_b(rpc) and {"result": {"peers": []}},
    )

    probe = probe_peers(endpoints)

    assert unreachable_nodes(*probe) == ["b"]
    assert peer_connectivity_matrix(endpoints, probe=probe)["a"] == {"b": None}


def test_unreachable_nodes_empty_when_every_node_answers(monkeypatch):
    endpoints = [_endpoint("a", "http://a")]
    monkeypatch.setattr(preflight_module, "node_id", lambda _rpc: "id-a")
    monkeypatch.setattr(preflight_module, "net_info", lambda _rpc: {"result": {"peers": []}})

    assert unreachable_nodes(*probe_peers(endpoints)) == []

from types import SimpleNamespace

from remote_benchmark import preflight as preflight_module
from remote_benchmark.preflight import peer_connectivity_matrix, resolved_mempool_types


def _endpoint(name, rpc, node_config=None):
    return SimpleNamespace(name=name, rpc=rpc, node_config=node_config or {})


def test_resolved_mempool_types_reports_declared_value_or_none():
    endpoints = [
        _endpoint("a", "http://a", {"mempool.type": "app"}),
        _endpoint("b", "http://b"),
    ]

    assert resolved_mempool_types(endpoints) == {"a": "app", "b": None}


def test_peer_connectivity_matrix_marks_connected_pairs(monkeypatch):
    endpoints = [
        _endpoint("a", "http://10.0.0.1"),
        _endpoint("b", "http://10.0.0.2"),
        _endpoint("c", "http://10.0.0.3"),
    ]

    def fake_net_info(rpc):
        peers_by_rpc = {
            "http://10.0.0.1": ["10.0.0.2"],
            "http://10.0.0.2": ["10.0.0.1", "10.0.0.3"],
            "http://10.0.0.3": [],
        }
        return {"result": {"peers": [{"remote_ip": ip} for ip in peers_by_rpc[rpc]]}}

    monkeypatch.setattr(preflight_module, "net_info", fake_net_info)

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

    monkeypatch.setattr(preflight_module, "net_info", fake_net_info)

    matrix = peer_connectivity_matrix(endpoints)

    assert matrix["a"] == {"b": False}
    assert matrix["b"] == {"a": None}

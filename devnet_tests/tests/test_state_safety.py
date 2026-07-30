from types import SimpleNamespace

from devnet_tests.conftest import Node
from devnet_tests.state_safety import check_app_hash_agreement, historical_query_soak


def _node(name: str) -> Node:
    return Node(name, w3=None, rpc=f"http://{name}:26657")


NODE_A, NODE_B, NODE_C = _node("a"), _node("b"), _node("c")


def _stub_blocks(monkeypatch, hashes):
    """hashes: [(node, app_hash)]; an app_hash of None makes that node error."""
    hash_by_rpc = {node.rpc: app_hash for node, app_hash in hashes}

    def get(url, timeout=None):
        rpc, _, height = url.partition("/block?height=")
        app_hash = hash_by_rpc[rpc]
        if app_hash is None:
            raise RuntimeError(f"no block at height {height}")
        body = {"result": {"block": {"header": {"app_hash": app_hash}}}}
        return SimpleNamespace(json=lambda: body)

    monkeypatch.setattr("devnet_tests.state_safety.requests.get", get)


def test_check_app_hash_agreement_flags_a_disagreeing_height(monkeypatch):
    _stub_blocks(monkeypatch, [(NODE_A, "hashA"), (NODE_B, "hashB")])

    divergences = check_app_hash_agreement([NODE_A, NODE_B], 5, 5)

    assert divergences == [
        {"height": 5, "hashes": {"a": "hashA", "b": "hashB"}, "reason": "mismatch"}
    ]


def test_check_app_hash_agreement_ignores_agreeing_heights(monkeypatch):
    _stub_blocks(monkeypatch, [(NODE_A, "same"), (NODE_B, "same")])

    assert check_app_hash_agreement([NODE_A, NODE_B], 5, 5) == []


def test_check_app_hash_agreement_ignores_a_node_that_errored(monkeypatch):
    _stub_blocks(monkeypatch, [(NODE_A, "hashA"), (NODE_B, "hashA"), (NODE_C, None)])

    assert check_app_hash_agreement([NODE_A, NODE_B, NODE_C], 5, 5) == []


def test_check_app_hash_agreement_flags_a_height_no_node_could_answer(monkeypatch):
    _stub_blocks(monkeypatch, [(NODE_A, None), (NODE_B, None)])

    divergences = check_app_hash_agreement([NODE_A, NODE_B], 5, 5)

    assert divergences == [{"height": 5, "hashes": {}, "reason": "unverifiable"}]


def test_historical_query_soak_captures_errors_without_raising():
    calls = []

    def get_balance(address, height):
        calls.append((address, height))
        if len(calls) == 2:
            raise RuntimeError("too many open files")

    w3 = SimpleNamespace(eth=SimpleNamespace(get_balance=get_balance))

    result = historical_query_soak(w3, "0xabc", height=1, iterations=3)

    assert result.iterations == 3
    assert result.errors == ["too many open files"]
    assert calls == [("0xabc", 1)] * 3


def test_historical_query_soak_reports_no_errors_on_a_clean_run():
    w3 = SimpleNamespace(eth=SimpleNamespace(get_balance=lambda address, height: 0))

    result = historical_query_soak(w3, "0xabc", height=1, iterations=5)

    assert result.errors == []

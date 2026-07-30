from types import SimpleNamespace

from remote_benchmark import divergence as divergence_module
from remote_benchmark.divergence import (
    app_hash_at,
    check_app_hash_agreement,
    collect_heights,
    height_skew,
)


def _endpoint(name, rpc):
    return SimpleNamespace(name=name, rpc=rpc)


def test_collect_heights_returns_none_for_unreachable_endpoint(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fake_block_height(rpc):
        if rpc == "http://a":
            return 100
        raise ConnectionError("unreachable")

    monkeypatch.setattr(divergence_module, "block_height", fake_block_height)

    heights = collect_heights(endpoints)

    assert heights == {"a": 100, "b": None}


def test_height_skew_ignores_unreachable_nodes():
    assert height_skew({"a": 100, "b": 95, "c": None}) == 5


def test_height_skew_none_when_fewer_than_two_reachable():
    assert height_skew({"a": 100, "b": None}) is None
    assert height_skew({}) is None


def test_app_hash_at_reads_block_header(monkeypatch):
    def fake_block(height, rpc):
        assert height == 42
        assert rpc == "http://a"
        return {"result": {"block": {"header": {"app_hash": "deadbeef"}}}}

    monkeypatch.setattr(divergence_module, "block", fake_block)

    assert app_hash_at(42, "http://a") == "deadbeef"


def test_check_app_hash_agreement_flags_divergent_heights(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fake_app_hash_at(height, rpc):
        if height == 10:
            return "same-hash"
        return "hash-a" if rpc == "http://a" else "hash-b"

    monkeypatch.setattr(divergence_module, "app_hash_at", fake_app_hash_at)

    divergences = check_app_hash_agreement(endpoints, 10, 11)

    assert divergences == [
        {"height": 11, "hashes": {"a": "hash-a", "b": "hash-b"}}
    ]


def test_check_app_hash_agreement_skips_nodes_that_error_at_a_height(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fake_app_hash_at(height, rpc):
        if rpc == "http://b":
            raise ConnectionError("node b hasn't reached this height")
        return "hash-a"

    monkeypatch.setattr(divergence_module, "app_hash_at", fake_app_hash_at)

    divergences = check_app_hash_agreement(endpoints, 5, 5)

    assert divergences == []


def test_check_app_hash_agreement_no_divergence_when_all_agree(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    monkeypatch.setattr(divergence_module, "app_hash_at", lambda height, rpc: "same")

    assert check_app_hash_agreement(endpoints, 1, 3) == []

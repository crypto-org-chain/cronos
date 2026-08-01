from types import SimpleNamespace

from remote_benchmark import divergence as divergence_module
from remote_benchmark.divergence import (
    abci_app_hash,
    check_app_hash_agreement,
    collect_heights,
    height_skew,
)


def _endpoint(name, rpc):
    return SimpleNamespace(name=name, rpc=rpc)


def _check(endpoints, blocks=1, timeout=0):
    return check_app_hash_agreement(endpoints, blocks=blocks, interval=0, timeout=timeout)


def _reasons(divergences):
    return " | ".join(entry.get("reason", "") for entry in divergences)


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


def test_abci_app_hash_reads_the_nodes_own_last_commit(monkeypatch):
    monkeypatch.setattr(
        divergence_module,
        "abci_info",
        lambda rpc: {
            "result": {
                "response": {"last_block_height": "42", "last_block_app_hash": "beef"}
            }
        },
    )

    assert abci_app_hash("http://a") == (42, "beef")


def test_abci_app_hash_is_zero_before_anything_is_committed(monkeypatch):
    monkeypatch.setattr(
        divergence_module, "abci_info", lambda rpc: {"result": {"response": {}}}
    )

    assert abci_app_hash("http://a") == (0, "")


def test_check_app_hash_agreement_flags_nodes_disagreeing_at_a_height(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    monkeypatch.setattr(
        divergence_module,
        "abci_app_hash",
        lambda rpc: (10, "hash-a" if rpc == "http://a" else "hash-b"),
    )

    divergences = _check(endpoints)

    assert {"height": 10, "hashes": {"a": "hash-a", "b": "hash-b"}} in [
        {"height": e["height"], "hashes": e["hashes"]}
        for e in divergences
        if "hashes" in e
    ]


def test_check_app_hash_agreement_flags_a_node_that_never_reports(monkeypatch):
    # A diverged node panics on "wrong Block.Header.AppHash" and stops
    # answering; dropping it from the comparison set would pass on a broken run.
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fake_abci_app_hash(rpc):
        if rpc == "http://b":
            raise ConnectionError("node b is down")
        return 5, "hash-a"

    monkeypatch.setattr(divergence_module, "abci_app_hash", fake_abci_app_hash)

    divergences = _check(endpoints)

    assert any(entry.get("unreachable") == ["b"] for entry in divergences)


def test_check_app_hash_agreement_flags_a_chain_that_does_not_advance(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    monkeypatch.setattr(divergence_module, "abci_app_hash", lambda rpc: (7, "same"))

    divergences = _check(endpoints, blocks=3)

    assert "chain only advanced to 7" in _reasons(divergences)


def test_check_app_hash_agreement_flags_a_node_reporting_two_hashes_for_a_height(
    monkeypatch,
):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    hashes = iter(["h1", "h1", "h2", "h1"])
    monkeypatch.setattr(divergence_module, "abci_app_hash", lambda rpc: (9, next(hashes)))

    # timeout=1 lets the loop poll twice, so node a reports height 9 twice.
    divergences = check_app_hash_agreement(endpoints, blocks=5, interval=0, timeout=1)

    assert "reported two app hashes for height 9" in _reasons(divergences)


def test_check_app_hash_agreement_flags_a_window_no_two_nodes_shared(monkeypatch):
    # Each node only ever reports its own distinct height, so no height was
    # compared across nodes - an empty result would be a vacuous pass.
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    monkeypatch.setattr(
        divergence_module,
        "abci_app_hash",
        lambda rpc: (10, "hash") if rpc == "http://a" else (11, "hash"),
    )

    divergences = _check(endpoints)

    assert "never actually compared" in _reasons(divergences)


def test_check_app_hash_agreement_clean_when_all_nodes_agree_and_advance(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    heights = iter([10, 10, 11, 11, 12, 12])
    monkeypatch.setattr(
        divergence_module, "abci_app_hash", lambda rpc: (next(heights), "same")
    )

    assert check_app_hash_agreement(endpoints, blocks=2, interval=0, timeout=5) == []


def test_check_app_hash_agreement_needs_two_endpoints():
    divergences = _check([_endpoint("a", "http://a")])

    assert "need two endpoints" in _reasons(divergences)


def test_check_app_hash_agreement_flags_an_empty_app_hash(monkeypatch):
    # last_block_app_hash is omitempty: an absent or renamed field reads as ""
    # on every node, so all of them "match" having compared nothing. A committed
    # height always carries a commit hash, so this is a build/schema defect and
    # has to hard-fail rather than warn.
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    monkeypatch.setattr(divergence_module, "abci_app_hash", lambda rpc: (10, ""))

    divergences = _check(endpoints)

    empty = [e for e in divergences if "empty last_block_app_hash" in e["reason"]]

    # reported once per node, not once per poll
    assert len(empty) == 2
    assert {e["kind"] for e in empty} == {"diverged"}
    assert all(e["height"] == 10 for e in empty)


def test_check_app_hash_agreement_marks_unverifiable_outcomes_as_such(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]

    def fake_abci_app_hash(rpc):
        if rpc == "http://b":
            raise ConnectionError("node b is down")
        return 5, "hash-a"

    monkeypatch.setattr(divergence_module, "abci_app_hash", fake_abci_app_hash)

    divergences = _check(endpoints)

    # A node that never answered didn't establish a mismatch, so nothing here
    # may be reported as a confirmed divergence — but something must be
    # reported: an empty result would mean "checked and agreed".
    assert divergences
    assert all(entry["kind"] == "unverified" for entry in divergences)


def test_check_app_hash_agreement_marks_a_real_mismatch_as_diverged(monkeypatch):
    endpoints = [_endpoint("a", "http://a"), _endpoint("b", "http://b")]
    monkeypatch.setattr(
        divergence_module,
        "abci_app_hash",
        lambda rpc: (10, "hash-a" if rpc == "http://a" else "hash-b"),
    )

    divergences = _check(endpoints)

    assert any(entry["kind"] == "diverged" for entry in divergences)

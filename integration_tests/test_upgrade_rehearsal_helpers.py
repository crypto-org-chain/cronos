import time
from unittest import mock

import pytest

from .upgrade_rehearsal import LoadGenerator, assert_no_divergence

REQUESTS_GET = "integration_tests.upgrade_rehearsal.requests.get"
PORTS = [26657, 26667]


def _wait_for_results(gen, n, timeout=5):
    deadline = time.monotonic() + timeout
    while len(gen.results) < n:
        if time.monotonic() > deadline:
            raise TimeoutError(
                f"LoadGenerator only produced {len(gen.results)}/{n} results"
            )
        time.sleep(0.01)


def test_load_generator_records_successes():
    gen = LoadGenerator(send_fn=lambda: None, interval=0)
    gen.start()
    _wait_for_results(gen, 1)
    gen.stop()
    assert all(gen.results)


def test_load_generator_records_failures_without_stopping():
    calls = []

    def flaky():
        calls.append(1)
        if len(calls) <= 2:
            raise RuntimeError("node unavailable")

    gen = LoadGenerator(send_fn=flaky, interval=0)
    gen.start()
    _wait_for_results(gen, 3)
    gen.stop()
    assert gen.results[:2] == [False, False]
    assert gen.results[2] is True


def _fake_response(height, app_hash):
    rsp = mock.Mock()
    rsp.json.return_value = {
        "result": {
            "response": {
                "last_block_height": str(height),
                "last_block_app_hash": app_hash,
            }
        }
    }
    return rsp


def _advancing(hash_for, heights):
    """A /abci_info stub whose nodes climb through `heights`, one height per
    poll round, with each node's app hash decided by hash_for(port, height)."""
    rounds = iter(heights)
    state = {"height": next(rounds), "pending": set()}

    def get(url, timeout):
        port = next(p for p in PORTS if str(p) in url)
        height = state["height"]
        state["pending"].add(port)
        if state["pending"] == set(PORTS):
            state["pending"] = set()
            state["height"] = next(rounds, height)
        return _fake_response(height, hash_for(port, height))

    return get


def test_assert_no_divergence_passes_when_hashes_agree():
    get = _advancing(lambda port, height: f"hash{height}", range(1, 6))
    with mock.patch(REQUESTS_GET, side_effect=get):
        assert_no_divergence(PORTS, blocks=2, interval=0)


def test_assert_no_divergence_raises_on_mismatch():
    get = _advancing(
        lambda port, height: f"hash{height}-{port}" if height == 2 else f"hash{height}",
        range(1, 6),
    )
    with mock.patch(REQUESTS_GET, side_effect=get):
        with pytest.raises(AssertionError, match="divergence at height 2"):
            assert_no_divergence(PORTS, blocks=2, interval=0)


def test_assert_no_divergence_raises_when_a_node_never_answers():
    def get(url, timeout):
        if str(PORTS[1]) in url:
            raise ConnectionError("node unreachable")
        return _fake_response(5, "same")

    with mock.patch(REQUESTS_GET, side_effect=get):
        with pytest.raises(AssertionError, match="no committed app hash"):
            assert_no_divergence(PORTS, blocks=1, interval=0, timeout=0)


def test_assert_no_divergence_raises_when_the_chain_does_not_advance():
    with mock.patch(REQUESTS_GET, return_value=_fake_response(5, "same")):
        with pytest.raises(AssertionError, match="only advanced"):
            assert_no_divergence(PORTS, blocks=2, interval=0, timeout=0)


def test_assert_no_divergence_raises_when_a_node_reports_two_hashes_for_a_height():
    hashes = iter(["a", "b"])

    def get(url, timeout):
        if str(PORTS[0]) in url:
            return _fake_response(1, next(hashes, "b"))
        return _fake_response(1, "a")

    with mock.patch(REQUESTS_GET, side_effect=get):
        with pytest.raises(AssertionError, match="two app hashes for height 1"):
            assert_no_divergence(PORTS, blocks=2, interval=0, timeout=1)


def test_assert_no_divergence_ignores_a_height_only_one_node_answered():
    # node1 misses height 1 but catches up from height 2 on, so there is still
    # a height compared by both.
    def hash_for(port, height):
        return f"hash{height}"

    inner = _advancing(hash_for, range(1, 6))

    def get(url, timeout):
        rsp = inner(url, timeout)
        height = int(rsp.json()["result"]["response"]["last_block_height"])
        if str(PORTS[1]) in url and height == 1:
            raise ConnectionError("node not caught up yet")
        return rsp

    with mock.patch(REQUESTS_GET, side_effect=get):
        assert_no_divergence(PORTS, blocks=2, interval=0)


def test_assert_no_divergence_raises_when_the_app_hash_is_absent():
    # what a renamed or dropped last_block_app_hash field looks like: every node
    # reads back "", which would otherwise "match" without comparing anything
    with mock.patch(REQUESTS_GET, return_value=_fake_response(5, "")):
        with pytest.raises(AssertionError, match="empty app hash"):
            assert_no_divergence(PORTS, blocks=1, interval=0, timeout=0)


def test_assert_no_divergence_raises_on_nonpositive_blocks():
    with mock.patch(REQUESTS_GET) as get:
        with pytest.raises(AssertionError, match="blocks must be at least 1"):
            assert_no_divergence(PORTS, blocks=0)
    assert get.call_count == 0


def test_assert_no_divergence_raises_on_a_single_node():
    with mock.patch(REQUESTS_GET) as get:
        with pytest.raises(AssertionError, match="need two nodes"):
            assert_no_divergence(PORTS[:1], blocks=1)
    assert get.call_count == 0

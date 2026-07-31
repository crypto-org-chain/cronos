import time
from unittest import mock

import pytest

from .upgrade_rehearsal import LoadGenerator, assert_no_divergence

REQUESTS_GET = "integration_tests.upgrade_rehearsal.requests.get"


def _wait_for_results(gen, n, timeout=5):
    deadline = time.monotonic() + timeout
    while len(gen.results) < n:
        if time.monotonic() > deadline:
            raise TimeoutError(f"LoadGenerator only produced {len(gen.results)}/{n} results")
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


def _fake_response(app_hash):
    rsp = mock.Mock()
    rsp.json.return_value = {"result": {"block": {"header": {"app_hash": app_hash}}}}
    return rsp


def test_assert_no_divergence_passes_when_hashes_agree():
    with mock.patch(REQUESTS_GET, return_value=_fake_response("same")):
        assert_no_divergence([26657, 26667], start=1, end=2)


def test_assert_no_divergence_raises_on_mismatch():
    def get(url, timeout):
        app_hash = "a" if "26657" in url else "b"
        return _fake_response(app_hash)

    with mock.patch(REQUESTS_GET, side_effect=get):
        with pytest.raises(AssertionError, match="divergence"):
            assert_no_divergence([26657, 26667], start=1, end=1)


def test_assert_no_divergence_ignores_a_height_only_one_node_answered():
    def get(url, timeout):
        if "26667" in url:
            raise ConnectionError("node not caught up yet")
        return _fake_response("same")

    with mock.patch(REQUESTS_GET, side_effect=get):
        assert_no_divergence([26657, 26667], start=1, end=1)


def test_assert_no_divergence_raises_when_a_node_fails_every_height():
    def get(url, timeout):
        if "26667" in url:
            raise ConnectionError("node unreachable")
        return _fake_response("same")

    with mock.patch(REQUESTS_GET, side_effect=get):
        with pytest.raises(AssertionError, match="unreachable"):
            assert_no_divergence([26657, 26667], start=1, end=2)

from types import SimpleNamespace

import pytest

from devnet_tests.rpc_methods import (
    METHODS,
    DiffContext,
    _equal_compare,
    _shape_only_compare,
    run_method,
)

FILTER_METHOD = next(m for m in METHODS if m.name == "eth_newFilter+eth_getFilterLogs")
CTX = DiffContext(
    height=1, block_hash="0x0", tx_hash=None, address="0x0", calldata="0x", sender="0x0"
)


def _fake_w3(make_request):
    return SimpleNamespace(provider=SimpleNamespace(make_request=make_request))


def test_equal_compare_matches_identical_responses():
    a = {"jsonrpc": "2.0", "id": 1, "result": {"foo": "bar"}}
    b = {"jsonrpc": "2.0", "id": 2, "result": {"foo": "bar"}}
    assert _equal_compare(a, b) == []


def test_equal_compare_flags_mismatched_results():
    a = {"jsonrpc": "2.0", "id": 1, "result": {"foo": "bar"}}
    b = {"jsonrpc": "2.0", "id": 1, "result": {"foo": "baz"}}
    assert _equal_compare(a, b) != []


def test_shape_only_compare_ignores_value_differences():
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": {"0x1": {}}, "queued": {}}}
    b = {"result": {"pending": {}, "queued": {"0x2": {}}}}
    assert compare(a, b) == []


def test_shape_only_compare_flags_missing_keys():
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": {}, "queued": {}}}
    b = {"result": {"pending": {}}}
    assert compare(a, b) != []


def test_shape_only_compare_flags_error_responses():
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": {}, "queued": {}}}
    b = {"error": {"code": -32601, "message": "method not found"}}
    assert compare(a, b) != []


def test_filter_getlogs_failure_is_not_masked_by_uninstall_failure():
    def make_request(method, params):
        if method == "eth_newFilter":
            return {"result": "0x1"}
        if method == "eth_getFilterLogs":
            raise RuntimeError("boom-getlogs")
        if method == "eth_uninstallFilter":
            raise RuntimeError("boom-uninstall")
        raise AssertionError(f"unexpected method {method}")

    with pytest.raises(RuntimeError, match="boom-getlogs"):
        run_method(FILTER_METHOD, _fake_w3(make_request), CTX)


def test_filter_cleanup_uses_the_created_filter_id():
    uninstalled = []

    def make_request(method, params):
        if method == "eth_newFilter":
            return {"result": "0xabc"}
        if method == "eth_getFilterLogs":
            return {"result": []}
        if method == "eth_uninstallFilter":
            uninstalled.append(params[0])
            return {"result": True}
        raise AssertionError(f"unexpected method {method}")

    run_method(FILTER_METHOD, _fake_w3(make_request), CTX)
    assert uninstalled == ["0xabc"]


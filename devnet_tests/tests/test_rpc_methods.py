from types import SimpleNamespace

import pytest
from hexbytes import HexBytes

from devnet_tests.rpc_methods import (
    METHODS,
    DiffContext,
    _equal_compare,
    _shape_only_compare,
    build_context,
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


def test_shape_only_compare_flags_a_differing_result_key_set():
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": {}, "queued": {}}}
    b = {"result": {"pending": {}, "queued": {}, "extra": {}}}
    assert compare(a, b) == [
        "result keys differ: ['pending', 'queued'] != ['extra', 'pending', 'queued']"
    ]


def test_shape_only_compare_flags_a_value_type_difference():
    # ethermint returns txpool_status counts as hex strings; a node answering
    # with a map instead is a real cross-version shape difference.
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": "0x1", "queued": "0x0"}}
    b = {"result": {"pending": {}, "queued": "0x0"}}
    assert compare(a, b) == ["pending value type differs: str != dict"]


def test_shape_only_compare_flags_error_responses():
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": {}, "queued": {}}}
    b = {"error": {"code": -32601, "message": "method not found"}}
    assert compare(a, b) != []


def test_shape_only_compare_leaves_both_errored_to_the_caller():
    # Both nodes lacking the txpool namespace exercises nothing; the runner
    # classifies it as both_errored, so it must not read as a cross-node mismatch.
    compare = _shape_only_compare({"pending", "queued"})
    error = {"error": {"code": -32601, "message": "method not found"}}
    assert compare(dict(error), dict(error)) == []
    assert compare(error, {"error": {"code": -32000, "message": "other"}}) == []


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


def test_shape_only_compare_flags_null_result_instead_of_raising():
    compare = _shape_only_compare({"pending", "queued"})
    a = {"result": {"pending": {}, "queued": {}}}
    b = {"result": None}
    assert compare(a, b) == ["b result missing keys: ['pending', 'queued']"]


def _fake_block_w3(transactions, code_by_address=None):
    code_by_address = code_by_address or {}
    return SimpleNamespace(
        eth=SimpleNamespace(
            get_block=lambda height, full_transactions=True: SimpleNamespace(
                hash=HexBytes("0xdeadbeef"), transactions=transactions
            ),
            get_code=lambda address: code_by_address.get(address, b""),
        )
    )


def _tx(to, data="0x", tx_hash="0xaa"):
    return {"to": to, "input": HexBytes(data), "hash": HexBytes(tx_hash)}


def test_build_context_prefers_a_tx_that_calls_contract_bytecode():
    eoa_tx = _tx("0xeoa", data="0x")
    contract_tx = _tx("0xcontract", data="0xa9059cbb")
    w3 = _fake_block_w3([eoa_tx, contract_tx], {"0xcontract": b"\x60\x00"})

    ctx = build_context(w3, height=7, sender="0xsender")

    assert (ctx.address, ctx.calldata) == ("0xcontract", "0xa9059cbb")
    assert ctx.call_target_is_contract
    # The tx-hash methods still sample the block's first tx, not the call target.
    assert ctx.tx_hash == "0xaa"


def test_build_context_falls_back_to_an_empty_call_without_contract_txs():
    w3 = _fake_block_w3([_tx("0xeoa", data="0x")])

    ctx = build_context(w3, height=7, sender="0xsender")

    assert (ctx.address, ctx.calldata) == ("0xeoa", "0x")
    # Flags the degraded path: the call category executes no EVM code here.
    assert not ctx.call_target_is_contract


def test_build_context_falls_back_to_sender_on_an_empty_block():
    ctx = build_context(_fake_block_w3([]), height=7, sender="0xsender")

    assert (ctx.address, ctx.calldata, ctx.tx_hash) == ("0xsender", "0x", None)
    assert not ctx.call_target_is_contract

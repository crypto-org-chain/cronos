from types import SimpleNamespace

import pytest
from click.testing import CliRunner

from devnet_tests import rpc_diff as rpc_diff_module
from devnet_tests.devnet import Devnet, Node
from devnet_tests.rpc_diff import DiffRow, RpcDiffReport, cli, run_rpc_diff


def _fake_node(name, block_number, make_request):
    eth = SimpleNamespace(
        block_number=block_number,
        get_block=lambda height, full_transactions=True: SimpleNamespace(
            hash=SimpleNamespace(hex=lambda: "0xblockhash"), transactions=[]
        ),
    )
    provider = SimpleNamespace(make_request=make_request)
    return Node(name, SimpleNamespace(eth=eth, provider=provider), rpc="http://fake")


def _fixed_response(response):
    def make_request(method, params):
        return dict(response)

    return make_request


def _raising(message):
    def make_request(method, params):
        raise RuntimeError(message)

    return make_request


# Satisfies both _equal_compare and the txpool methods' _shape_only_compare.
OK_RESPONSE = {"result": {"pending": {}, "queued": {}}}


def test_exception_on_reference_flows_into_compare_as_mismatch():
    reference = _fake_node("ref", 10, _raising("reference down"))
    target = _fake_node("target", 10, _fixed_response(OK_RESPONSE))
    devnet = Devnet([reference, target], funded_account=None)

    report = run_rpc_diff(devnet, 5, 5)

    assert report.compared > 0
    assert len(report.rows) == report.compared
    assert all(
        any("reference down" in m for m in row.mismatches) for row in report.rows
    )


def test_exception_on_target_flows_into_compare_as_mismatch():
    reference = _fake_node("ref", 10, _fixed_response(OK_RESPONSE))
    target = _fake_node("target", 10, _raising("target down"))
    devnet = Devnet([reference, target], funded_account=None)

    report = run_rpc_diff(devnet, 5, 5)

    assert report.compared > 0
    assert all(any("target down" in m for m in row.mismatches) for row in report.rows)


def test_identical_nodes_report_full_equivalence():
    make_request = _fixed_response(OK_RESPONSE)
    reference = _fake_node("ref", 10, make_request)
    target = _fake_node("target", 10, make_request)
    devnet = Devnet([reference, target], funded_account=None)

    report = run_rpc_diff(devnet, 5, 5)

    assert report.rows == []
    assert report.equivalence_rate == 1.0


def test_height_clamp_raises_when_a_node_has_not_caught_up():
    reference = _fake_node("ref", 10, _fixed_response(OK_RESPONSE))
    target = _fake_node("target", 3, _fixed_response(OK_RESPONSE))
    devnet = Devnet([reference, target], funded_account=None)

    with pytest.raises(ValueError, match="hasn't reached"):
        run_rpc_diff(devnet, 5, 8)


def test_methods_with_no_tx_to_sample_are_named_in_the_report():
    make_request = _fixed_response(OK_RESPONSE)
    reference = _fake_node("ref", 10, make_request)
    target = _fake_node("target", 10, make_request)
    devnet = Devnet([reference, target], funded_account=None)

    # _fake_node's blocks are always empty, so every tx-hash method skips.
    report = run_rpc_diff(devnet, 5, 6)

    assert report.skipped_methods == {
        "eth_getTransactionByHash": 2,
        "eth_getTransactionReceipt": 2,
        "debug_traceTransaction": 2,
    }
    assert report.skipped == 6
    assert report.to_dict()["skipped_methods"] == report.skipped_methods
    assert report.heights_sampled == 2
    assert report.never_compared == [
        "debug_traceTransaction",
        "eth_getTransactionByHash",
        "eth_getTransactionReceipt",
    ]


def test_heights_sampled_follows_the_clamped_range():
    # The requested end is clamped to the least-caught-up node, so a skip count
    # must be judged against the heights actually walked, not the requested ones.
    make_request = _fixed_response(OK_RESPONSE)
    devnet = Devnet(
        [_fake_node("ref", 10, make_request), _fake_node("target", 6, make_request)],
        funded_account=None,
    )

    report = run_rpc_diff(devnet, 5, 9)

    assert report.heights_sampled == 2
    assert "eth_getTransactionByHash" in report.never_compared


def test_equivalence_rate_is_none_when_nothing_was_compared():
    # 1.0 here would let an empty run read as "fully equivalent".
    assert RpcDiffReport().equivalence_rate is None


def _invoke_rpc_diff_cmd(monkeypatch, report):
    monkeypatch.setattr(rpc_diff_module, "load_devnet", lambda path: None)
    monkeypatch.setattr(rpc_diff_module, "run_rpc_diff", lambda *a: report)
    return CliRunner().invoke(
        cli, ["rpc-diff", "--config", "cfg.yaml", "--start", "1", "--end", "1"]
    )


def test_cli_exits_non_zero_on_mismatches(monkeypatch):
    report = RpcDiffReport(
        rows=[DiffRow("eth_getBalance", "state", 1, "target", ["differs"])],
        compared=2,
        matched=1,
    )

    result = _invoke_rpc_diff_cmd(monkeypatch, report)

    assert result.exit_code != 0
    assert "1 mismatch(es)" in result.output


def test_cli_exits_non_zero_when_nothing_was_compared(monkeypatch):
    result = _invoke_rpc_diff_cmd(monkeypatch, RpcDiffReport())

    assert result.exit_code != 0
    assert "nothing was compared" in result.output


def test_cli_exits_zero_when_every_response_matched(monkeypatch):
    report = RpcDiffReport(
        compared=2, matched=2, heights_sampled=1, contract_call_heights=1
    )

    result = _invoke_rpc_diff_cmd(monkeypatch, report)

    assert result.exit_code == 0
    assert "equivalence rate: 100.00%" in result.output


def test_cli_exits_non_zero_when_a_method_never_ran_at_any_height(monkeypatch):
    report = RpcDiffReport(
        compared=2,
        matched=2,
        heights_sampled=2,
        contract_call_heights=1,
        skipped=2,
        skipped_methods={"debug_traceTransaction": 2},
    )

    result = _invoke_rpc_diff_cmd(monkeypatch, report)

    assert result.exit_code != 0
    assert "never compared at any sampled height" in result.output


def test_cli_exits_zero_when_a_method_only_skipped_some_heights(monkeypatch):
    report = RpcDiffReport(
        compared=2,
        matched=2,
        heights_sampled=2,
        contract_call_heights=1,
        skipped=1,
        skipped_methods={"debug_traceTransaction": 1},
    )

    result = _invoke_rpc_diff_cmd(monkeypatch, report)

    assert result.exit_code == 0


def test_cli_exits_non_zero_when_no_height_called_a_contract(monkeypatch):
    report = RpcDiffReport(
        compared=2, matched=2, heights_sampled=1, contract_call_heights=0
    )

    result = _invoke_rpc_diff_cmd(monkeypatch, report)

    assert result.exit_code != 0
    assert "no sampled height" in result.output


def test_mismatches_are_broken_down_per_method():
    # An aggregate rate would dilute one consistently-broken method; the
    # per-method counts keep it visible.
    report = RpcDiffReport(
        rows=[
            DiffRow("eth_call", "call", 1, "target", ["differs"]),
            DiffRow("eth_call", "call", 2, "target", ["differs"]),
            DiffRow("eth_getLogs", "logs", 1, "target", ["differs"]),
        ],
        compared=60,
        matched=57,
    )

    assert report.mismatches_by_method == {"eth_call": 2, "eth_getLogs": 1}
    assert report.to_dict()["mismatches_by_method"] == report.mismatches_by_method


def test_mismatches_by_method_is_empty_on_a_clean_run():
    assert RpcDiffReport(compared=4, matched=4).mismatches_by_method == {}


def test_identical_errors_are_neither_matched_nor_mismatched():
    # Both nodes lacking a namespace exercises nothing, so it must not read as
    # equivalence.
    error = {"error": {"code": -32601, "message": "method not found"}}
    make_request = _fixed_response(error)
    devnet = Devnet(
        [_fake_node("ref", 10, make_request), _fake_node("target", 10, make_request)],
        funded_account=None,
    )

    report = run_rpc_diff(devnet, 5, 5)

    assert report.matched == 0
    assert report.both_errored > 0
    assert "eth_getBlockByNumber" in report.never_responded
    assert report.to_dict()["never_responded"] == report.never_responded


def test_a_method_that_responded_once_is_not_reported_as_never_responded():
    responses = iter([OK_RESPONSE])

    def make_request(method, params):
        # Only the first call of the run gets a real response.
        return dict(next(responses, {"error": {"message": "boom"}}))

    devnet = Devnet(
        [_fake_node("ref", 10, make_request), _fake_node("target", 10, make_request)],
        funded_account=None,
    )

    report = run_rpc_diff(devnet, 5, 5)

    assert report.both_errored > 0
    assert "eth_getBlockByNumber" not in report.never_responded


def test_contract_call_heights_counts_heights_with_real_bytecode():
    # _fake_node's blocks are always empty, so no height has a contract to call.
    make_request = _fixed_response(OK_RESPONSE)
    devnet = Devnet(
        [_fake_node("ref", 10, make_request), _fake_node("target", 10, make_request)],
        funded_account=None,
    )

    report = run_rpc_diff(devnet, 5, 6)

    assert report.contract_call_heights == 0
    assert report.to_dict()["contract_call_heights"] == 0


def test_cli_exits_non_zero_when_a_method_never_responded(monkeypatch):
    report = RpcDiffReport(
        compared=2,
        matched=1,
        both_errored=1,
        both_errored_methods={"debug_traceTransaction": 1},
    )

    result = _invoke_rpc_diff_cmd(monkeypatch, report)

    assert result.exit_code != 0
    assert "never got a real response" in result.output

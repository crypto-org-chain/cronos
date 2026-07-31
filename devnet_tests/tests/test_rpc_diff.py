from types import SimpleNamespace

import pytest

from devnet_tests.devnet import Devnet, Node
from devnet_tests.rpc_diff import run_rpc_diff


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

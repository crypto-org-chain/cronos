import pytest

from .ibc_utils import (
    RATIO,
    assert_ready,
    get_balance,
    hermes_transfer,
    prepare_network,
)
from .utils import ADDRS, eth_to_bech32, wait_for_fn

pytestmark = pytest.mark.ibc_timeout


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc_timeout"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name)
    yield from network


def test_ibc(ibc):
    src_amount = hermes_transfer(ibc)
    dst_amount = src_amount * RATIO  # the decimal places difference
    dst_denom = "basetcro"
    dst_addr = eth_to_bech32(ADDRS["signer2"])
    old_dst_balance = get_balance(ibc.cronos, dst_addr, dst_denom)

    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = get_balance(ibc.cronos, dst_addr, dst_denom)
        return new_dst_balance != old_dst_balance

    wait_for_fn("balance change", check_balance_change)
    assert old_dst_balance + dst_amount == new_dst_balance


def test_cronos_transfer_timeout(ibc):
    """
    test sending basetcro from cronos to crypto-org-chain using cli transfer_tokens.
    depends on `test_ibc` to send the original coins.
    """
    assert_ready(ibc)
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    dst_denom = "basecro"
    cli = ibc.cronos.cosmos_cli()
    src_amount = dst_amount * RATIO  # the decimal places difference
    src_addr = cli.address("signer2")
    src_denom = "basetcro"

    # case 1: use cronos cli
    old_src_balance = get_balance(ibc.cronos, src_addr, src_denom)
    old_dst_balance = get_balance(ibc.chainmain, dst_addr, dst_denom)
    rsp = cli.transfer_tokens(
        src_addr,
        dst_addr,
        f"{src_amount}{src_denom}",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    new_src_balance = 0

    def check_balance_change():
        nonlocal new_src_balance
        new_src_balance = get_balance(ibc.cronos, src_addr, src_denom)
        get_balance(ibc.chainmain, dst_addr, dst_denom)
        return old_src_balance == new_src_balance

    wait_for_fn("balance no change", check_balance_change)
    new_dst_balance = get_balance(ibc.chainmain, dst_addr, dst_denom)
    assert old_dst_balance == new_dst_balance

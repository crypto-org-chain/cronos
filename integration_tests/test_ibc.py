import pytest

from .ibc_utils import (
    RATIO,
    assert_ready,
    cronos_transfer_source_tokens,
    cronos_transfer_source_tokens_with_proxy,
    find_duplicate,
    get_balance,
    ibc_incentivized_transfer,
    ibc_transfer,
    prepare_network,
)
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    parse_events_rpc,
    send_transaction,
    wait_for_fn,
)

pytestmark = pytest.mark.ibc


@pytest.fixture(scope="module", params=[True, False])
def ibc(request, tmp_path_factory):
    "prepare-network"
    incentivized = request.param
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(path, name, incentivized=incentivized)


def test_ibc_transfer_with_hermes(ibc):
    """
    test ibc transfer tokens with hermes cli
    """
    ibc_transfer(ibc)
    dst_denom = "basetcro"
    # assert that the relayer transactions do enables the dynamic fee extension option.
    cli = ibc.cronos.cosmos_cli()
    criteria = "message.action=/ibc.core.channel.v1.MsgChannelOpenInit"
    tx = cli.tx_search(criteria)["txs"][0]
    events = parse_events_rpc(tx["events"])
    fee = int(events["tx"]["fee"].removesuffix(dst_denom))
    gas = int(tx["gas_wanted"])
    # the effective fee is decided by the max_priority_fee (base fee is zero)
    # rather than the normal gas price
    assert fee == gas * 1000000

    # check duplicate OnRecvPacket events
    criteria = "message.action=/ibc.core.channel.v1.MsgRecvPacket"
    tx = cli.tx_search(criteria)["txs"][0]
    events = tx["logs"][1]["events"]
    for event in events:
        dup = find_duplicate(event["attributes"])
        assert not dup, f"duplicate {dup} in {event['type']}"


def test_ibc_incentivized_transfer(ibc):
    if not ibc.incentivized:
        # this test case only works for incentivized channel.
        return
    ibc_incentivized_transfer(ibc)


def test_cronos_transfer_tokens(ibc):
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
    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = get_balance(ibc.chainmain, dst_addr, dst_denom)
        return old_dst_balance != new_dst_balance

    wait_for_fn("balance change", check_balance_change)
    assert old_dst_balance + dst_amount == new_dst_balance
    new_src_balance = get_balance(ibc.cronos, src_addr, src_denom)
    assert old_src_balance - src_amount == new_src_balance


def test_cronos_transfer_tokens_acknowledgement_error(ibc):
    """
    test sending basetcro from cronos to crypto-org-chain using cli transfer_tokens
    with invalid receiver for acknowledgement error.
    depends on `test_ibc` to send the original coins.
    """
    assert_ready(ibc)
    dst_addr = "invalid_address"
    dst_amount = 2
    cli = ibc.cronos.cosmos_cli()
    src_amount = dst_amount * RATIO  # the decimal places difference
    src_addr = cli.address("signer2")
    src_denom = "basetcro"

    old_src_balance = get_balance(ibc.cronos, src_addr, src_denom)
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
        return old_src_balance == new_src_balance

    wait_for_fn("balance no change", check_balance_change)
    new_src_balance = get_balance(ibc.cronos, src_addr, src_denom)


def test_cro_bridge_contract(ibc):
    """
    test sending basetcro from cronos to crypto-org-chain using CroBridge contract.
    depends on `test_ibc` to send the original coins.
    """
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    dst_amount = 2
    dst_denom = "basecro"
    src_amount = dst_amount * RATIO  # the decimal places difference
    old_dst_balance = get_balance(ibc.chainmain, dst_addr, dst_denom)

    # case 2: use CroBridge contract
    w3 = ibc.cronos.w3
    contract = deploy_contract(w3, CONTRACTS["CroBridge"])
    tx = contract.functions.send_cro_to_crypto_org(dst_addr).build_transaction(
        {
            "from": ADDRS["signer2"],
            "value": src_amount,
        }
    )
    receipt = send_transaction(w3, tx)
    assert receipt.status == 1

    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = get_balance(ibc.chainmain, dst_addr, dst_denom)
        return old_dst_balance != new_dst_balance

    wait_for_fn("check balance change", check_balance_change)
    assert old_dst_balance + dst_amount == new_dst_balance


def test_cronos_transfer_source_tokens(ibc):
    """
    test sending crc20 tokens originated from cronos to crypto-org-chain
    """
    assert_ready(ibc)
    cronos_transfer_source_tokens(ibc)


def test_cronos_transfer_source_tokens_with_proxy(ibc):
    """
    test sending crc20 tokens originated from cronos to crypto-org-chain
    """
    assert_ready(ibc)
    cronos_transfer_source_tokens_with_proxy(ibc)

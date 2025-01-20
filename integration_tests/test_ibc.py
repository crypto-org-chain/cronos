import json

import pytest

from .cosmoscli import module_address
from .ibc_utils import (
    RATIO,
    cronos_transfer_source_tokens,
    cronos_transfer_source_tokens_with_proxy,
    find_duplicate,
    get_balance,
    ibc_incentivized_transfer,
    ibc_transfer,
    prepare_network,
    register_fee_payee,
    wait_for_check_channel_ready,
)
from .utils import (
    ADDRS,
    CONTRACTS,
    approve_proposal,
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


def test_ibc_transfer(ibc):
    """
    test ibc transfer tokens with hermes cli
    """
    ibc_transfer(ibc)
    dst_denom = "basetcro"
    # assert that the relayer transactions do enables the dynamic fee extension option.
    cli = ibc.cronos.cosmos_cli()
    criteria = "message.action='/ibc.core.channel.v1.MsgChannelOpenInit'"
    tx = cli.tx_search(criteria)["txs"][0]
    events = parse_events_rpc(tx["events"])
    fee = int(events["tx"]["fee"].removesuffix(dst_denom))
    gas = int(tx["gas_wanted"])
    # the effective fee is decided by the max_priority_fee (base fee is zero)
    # rather than the normal gas price
    assert fee == gas * 1000000

    # check duplicate OnRecvPacket events
    criteria = "message.action='/ibc.core.channel.v1.MsgRecvPacket'"
    tx = cli.tx_search(criteria)["txs"][0]
    events = tx["events"]
    for event in events:
        dup = find_duplicate(event["attributes"])
        assert not dup, f"duplicate {dup} in {event['type']}"


def test_ibc_incentivized_transfer(ibc, tmp_path):
    if not ibc.incentivized:
        # rly: ibc_upgrade_channels not work
        return
        # upgrade to incentivized
        src_chain = ibc.cronos.cosmos_cli()
        version = {"fee_version": "ics29-1", "app_version": "ics20-1"}
        community = "community"
        authority = module_address("gov")
        connid = "connection-0"
        channel_id = "channel-0"
        deposit = "1basetcro"
        proposal_src = src_chain.ibc_upgrade_channels(
            version,
            community,
            deposit=deposit,
            title="channel-upgrade-title",
            summary="summary",
            port_pattern="transfer",
            channel_ids=channel_id,
        )
        proposal_src["deposit"] = deposit
        proposal_src["proposer"] = authority
        proposal_src["messages"][0]["signer"] = authority
        proposal = tmp_path / "proposal.json"
        proposal.write_text(json.dumps(proposal_src))
        rsp = src_chain.submit_gov_proposal(proposal, from_=community)
        assert rsp["code"] == 0, rsp["raw_log"]
        approve_proposal(ibc.cronos, rsp["events"])
        wait_for_check_channel_ready(
            src_chain, connid, channel_id, "STATE_FLUSHCOMPLETE"
        )
        wait_for_check_channel_ready(src_chain, connid, channel_id)
        register_fee_payee(ibc.cronos, ibc.chainmain)
    ibc_incentivized_transfer(ibc)


def test_cronos_transfer_tokens(ibc):
    """
    test sending basetcro from cronos to crypto-org-chain using cli transfer_tokens.
    depends on `test_ibc` to send the original coins.
    """
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
    cronos_transfer_source_tokens(ibc)


def test_cronos_transfer_source_tokens_with_proxy(ibc):
    """
    test sending crc20 tokens originated from cronos to crypto-org-chain
    """
    cronos_transfer_source_tokens_with_proxy(ibc)

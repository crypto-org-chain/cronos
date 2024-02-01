import json

import pytest
from eth_utils import keccak, to_checksum_address
from pystarport import cluster
from web3.datastructures import AttributeDict

from .ibc_utils import (
    RATIO,
    assert_duplicate,
    cronos_transfer_source_tokens,
    cronos_transfer_source_tokens_with_proxy,
    get_balance,
    ibc_denom,
    ibc_incentivized_transfer,
    prepare_network,
    rly_transfer,
)
from .utils import (
    ADDRS,
    CONTRACT_ABIS,
    bech32_to_eth,
    eth_to_bech32,
    get_logs_since,
    get_method_map,
    get_topic_data,
    module_address,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.ibc_rly_evm

CONTRACT = "0x0000000000000000000000000000000000000065"
contract_info = json.loads(CONTRACT_ABIS["IRelayerModule"].read_text())
method_map = get_method_map(contract_info)
cronos_signer2 = ADDRS["signer2"]
src_amount = 10
src_denom = "basecro"
dst_amount = src_amount * RATIO  # the decimal places difference
dst_denom = "basetcro"
channel = "channel-0"


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc_rly_evm"
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(
        path,
        name,
        relayer=cluster.Relayer.RLY.value,
    )


def coin_received(receiver, amt, denom):
    return {
        "receiver": receiver,
        "amount": [(amt, denom)],
    }


def coin_base(minter, amt, denom):
    return {
        "minter": minter,
        "amount": [(amt, denom)],
    }


def coin_spent(spender, amt, denom):
    return {
        "spender": spender,
        "amount": [(amt, denom)],
    }


def distribute_fee(receiver, fee):
    return {
        "receiver": receiver,
        "fee": keccak(text=fee),
    }


def fungible(dst, src, amt, denom):
    return {
        "receiver": dst,
        "sender": src,
        "denom": keccak(text=denom),
        "amount": amt,
    }


def transfer(src, dst, amt, denom):
    return {
        "recipient": dst,
        "sender": src,
        "amount": [(amt, denom)],
    }


def burn(burner, amt, denom):
    return {
        "burner": burner,
        "amount": [(amt, denom)],
    }


def recv_packet(src, dst, amt, denom):
    return {
        "packetDataHex": (dst, src, [(amt, denom)]),
    }


def acknowledge_packet():
    return {
        "packetSrcPort": keccak(text="transfer"),
        "packetSrcChannel": keccak(text=channel),
        "packetDstPort": keccak(text="transfer"),
        "packetDstChannel": channel,
        "packetChannelOrdering": "ORDER_UNORDERED",
        "packetConnection": "connection-0",
    }


def denom_trace(denom):
    return {
        "denom": keccak(text=denom),
    }


def write_ack(src, dst, amt, denom):
    return {
        "packetConnection": "connection-0",
        "packetDataHex": (dst, src, [(amt, denom)]),
    }


def send_coins(src, dst, amt, denom):
    return [
        coin_spent(src, amt, denom),
        coin_received(dst, amt, denom),
        transfer(src, dst, amt, denom),
    ]


def send_from_module_to_acc(src, dst, amt, denom):
    return [
        coin_received(src, amt, denom),
        coin_base(src, amt, denom),
        *send_coins(src, dst, amt, denom),
    ]


def send_from_acc_to_module(src, dst, amt, denom):
    return [
        *send_coins(src, dst, amt, denom),
    ]


def test_ibc(ibc):
    # chainmain-1 relayer -> cronos_777-1 signer2
    w3 = ibc.cronos.w3
    wait_for_new_blocks(ibc.cronos.cosmos_cli(), 1)
    start = w3.eth.get_block_number()
    rly_transfer(ibc)
    denom = ibc_denom(channel, src_denom)
    dst_addr = eth_to_bech32(cronos_signer2)
    old_dst_balance = get_balance(ibc.cronos, dst_addr, dst_denom)
    new_dst_balance = 0

    def check_balance_change():
        nonlocal new_dst_balance
        new_dst_balance = get_balance(ibc.cronos, dst_addr, dst_denom)
        return new_dst_balance != old_dst_balance

    wait_for_fn("balance change", check_balance_change)
    assert old_dst_balance + dst_amount == new_dst_balance
    logs = get_logs_since(w3, CONTRACT, start)
    relayer0 = ibc.chainmain.cosmos_cli().address("relayer")
    relayer = to_checksum_address(bech32_to_eth(relayer0))
    cronos_addr = module_address("cronos")
    transfer_addr = module_address("transfer")
    expected = [
        recv_packet(relayer0, cronos_signer2, src_amount, src_denom),
        denom_trace(denom),
        *send_from_module_to_acc(transfer_addr, cronos_signer2, src_amount, denom),
        fungible(cronos_signer2, relayer, src_amount, src_denom),
        *send_from_acc_to_module(cronos_signer2, cronos_addr, src_amount, denom),
        *send_from_module_to_acc(cronos_addr, cronos_signer2, dst_amount, dst_denom),
        write_ack(relayer0, cronos_signer2, src_amount, src_denom),
    ]
    assert len(logs) == len(expected)
    height = logs[0]["blockNumber"]
    assert_duplicate(ibc.cronos.base_port(0), height)
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]


def get_escrow_address(cli, channel):
    return to_checksum_address(
        bech32_to_eth(cli.ibc_escrow_address("transfer", channel)),
    )


def test_ibc_incentivized_transfer(ibc):
    w3 = ibc.cronos.w3
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    start = w3.eth.get_block_number()
    amount = ibc_incentivized_transfer(ibc)
    logs = get_logs_since(w3, CONTRACT, start)
    fee_denom = "ibcfee"
    fee = f"{src_amount}{fee_denom}"
    transfer_denom = "transfer/channel-0/basetcro"
    dst_adr = ibc.chainmain.cosmos_cli().address("signer2")
    src_relayer = ADDRS["signer1"]
    checksum_dst_adr = to_checksum_address(bech32_to_eth(dst_adr))
    feeibc_addr = module_address("feeibc")
    escrow = get_escrow_address(cli, channel)
    expected = [
        acknowledge_packet(),
        distribute_fee(src_relayer, fee),
        *send_coins(feeibc_addr, src_relayer, src_amount, fee_denom),
        distribute_fee(src_relayer, fee),
        *send_coins(feeibc_addr, src_relayer, src_amount, fee_denom),
        distribute_fee(cronos_signer2, fee),
        *send_coins(feeibc_addr, cronos_signer2, src_amount, fee_denom),
        fungible(checksum_dst_adr, cronos_signer2, amount, dst_denom),
        recv_packet(dst_adr, cronos_signer2, amount, transfer_denom),
        *send_coins(escrow, cronos_signer2, amount, dst_denom),
        fungible(cronos_signer2, checksum_dst_adr, amount, transfer_denom),
        write_ack(dst_adr, cronos_signer2, amount, transfer_denom),
    ]
    assert len(logs) == len(expected)
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]


def get_transfer_source_tokens_topics(dst_adr, amount, contract, escrow):
    checksum_dst_adr = to_checksum_address(bech32_to_eth(dst_adr))
    cronos_addr = module_address("cronos")
    cronos_denom = f"cronos{contract}"
    transfer_cronos_denom = f"transfer/{channel}/{cronos_denom}"
    return [
        acknowledge_packet(),
        fungible(checksum_dst_adr, ADDRS["validator"], amount, cronos_denom),
        recv_packet(dst_adr, cronos_signer2, amount, transfer_cronos_denom),
        *send_coins(escrow, cronos_signer2, amount, cronos_denom),
        fungible(cronos_signer2, checksum_dst_adr, amount, transfer_cronos_denom),
        *send_coins(cronos_signer2, cronos_addr, amount, cronos_denom),
        coin_spent(cronos_addr, amount, cronos_denom),
        burn(cronos_addr, amount, cronos_denom),
        write_ack(dst_adr, cronos_signer2, amount, transfer_cronos_denom),
    ]


def test_cronos_transfer_source_tokens(ibc):
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    w3 = ibc.cronos.w3
    start = w3.eth.get_block_number()
    amount, contract = cronos_transfer_source_tokens(ibc)
    logs = get_logs_since(w3, CONTRACT, start)
    escrow = get_escrow_address(cli, channel)
    dst_adr = ibc.chainmain.cosmos_cli().address("signer2")
    expected = get_transfer_source_tokens_topics(dst_adr, amount, contract, escrow)
    assert len(logs) == len(expected)
    height = logs[0]["blockNumber"]
    assert_duplicate(ibc.cronos.base_port(0), height)
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]


def test_cronos_transfer_source_tokens_with_proxy(ibc):
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    w3 = ibc.cronos.w3
    start = w3.eth.get_block_number()
    amount, contract = cronos_transfer_source_tokens_with_proxy(ibc)
    logs = get_logs_since(w3, CONTRACT, start)
    escrow = get_escrow_address(cli, channel)
    dst_adr = ibc.chainmain.cosmos_cli().address("signer2")
    expected = get_transfer_source_tokens_topics(dst_adr, amount, contract, escrow)
    assert len(logs) == len(expected)
    height = logs[0]["blockNumber"]
    assert_duplicate(ibc.cronos.base_port(0), height)
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]

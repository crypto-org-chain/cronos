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
    hermes_transfer,
    ibc_denom,
    ibc_incentivized_transfer,
    prepare_network,
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
    parse_events_rpc,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.ibc_rly_evm

CONTRACT = "0x0000000000000000000000000000000000000065"
contract_info = json.loads(CONTRACT_ABIS["IRelayerModule"].read_text())
method_map = get_method_map(contract_info)
method_name_map = get_method_map(contract_info, by_name=True)
method_with_seq = ["RecvPacket", "WriteAcknowledgement", "AcknowledgePacket"]
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
        relayer=cluster.Relayer.HERMES.value,
    )


def amount_dict(amt, denom):
    return [
        AttributeDict(
            {
                "amount": amt,
                "denom": denom,
            }
        )
    ]


def coin_received(receiver, amt, denom):
    return {
        "receiver": receiver,
        "amount": amount_dict(amt, denom),
    }


def coin_base(minter, amt, denom):
    return {
        "minter": minter,
        "amount": amount_dict(amt, denom),
    }


def coin_spent(spender, amt, denom):
    return {
        "spender": spender,
        "amount": amount_dict(amt, denom),
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
        "amount": amount_dict(amt, denom),
    }


def burn(burner, amt, denom):
    return {
        "burner": burner,
        "amount": amount_dict(amt, denom),
    }


def recv_packet(seq, src, dst, amt, denom):
    return {
        "packetSequence": keccak(text=f"{seq}"),
        "packetSrcPort": keccak(text="transfer"),
        "packetSrcChannel": keccak(text=channel),
        "packetDstPort": "transfer",
        "packetDstChannel": channel,
        "connectionId": "connection-0",
        "packetDataHex": AttributeDict(
            {
                "receiver": dst,
                "sender": src,
                "amount": amount_dict(amt, denom),
            }
        ),
    }


def acknowledge_packet(seq):
    return {
        "packetSequence": keccak(text=f"{seq}"),
        "packetSrcPort": keccak(text="transfer"),
        "packetSrcChannel": keccak(text=channel),
        "packetDstPort": "transfer",
        "packetDstChannel": channel,
        "connectionId": "connection-0",
    }


def denom_trace(denom):
    return {
        "denom": keccak(text=denom),
    }


def write_ack(seq, src, dst, amt, denom):
    return {
        "packetSequence": keccak(text=f"{seq}"),
        "packetSrcPort": keccak(text="transfer"),
        "packetSrcChannel": keccak(text=channel),
        "packetDstPort": "transfer",
        "packetDstChannel": channel,
        "connectionId": "connection-0",
        "packetDataHex": AttributeDict(
            {
                "receiver": dst,
                "sender": src,
                "amount": amount_dict(amt, denom),
            }
        ),
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


def get_send_packet_seq(
    cli,
    criteria="message.action='/ibc.applications.transfer.v1.MsgTransfer'",
):
    txs = cli.tx_search_rpc(
        criteria,
        order="desc",
    )
    for tx in txs:
        res = tx["tx_result"]
        events = parse_events_rpc(res["events"])
        target = events.get("send_packet")
        if target and target["packet_sequence"]:
            return target["packet_sequence"]
    return None


def filter_logs_since(w3, start, name, seq):
    topic = method_name_map.get(name)
    assert topic
    return w3.eth.get_logs(
        {
            "fromBlock": start,
            "address": [CONTRACT],
            "topics": [topic, "0x" + keccak(text=f"{seq}").hex()],
        }
    )


def test_ibc(ibc):
    # chainmain-1 relayer -> cronos_777-1 signer2
    w3 = ibc.cronos.w3
    wait_for_new_blocks(ibc.cronos.cosmos_cli(), 1)
    start = w3.eth.get_block_number()
    hermes_transfer(ibc)
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
    chainmain_cli = ibc.chainmain.cosmos_cli()
    relayer0 = chainmain_cli.address("relayer")
    relayer = to_checksum_address(bech32_to_eth(relayer0))
    cronos_addr = module_address("cronos")
    transfer_addr = module_address("transfer")
    seq = get_send_packet_seq(chainmain_cli)
    expected = [
        recv_packet(seq, relayer0, cronos_signer2, src_amount, src_denom),
        denom_trace(denom),
        *send_from_module_to_acc(transfer_addr, cronos_signer2, src_amount, denom),
        fungible(cronos_signer2, relayer, src_amount, src_denom),
        *send_from_acc_to_module(cronos_signer2, cronos_addr, src_amount, denom),
        *send_from_module_to_acc(cronos_addr, cronos_signer2, dst_amount, dst_denom),
        write_ack(seq, relayer0, cronos_signer2, src_amount, src_denom),
    ]
    assert len(logs) == len(expected)
    height = logs[0]["blockNumber"]
    assert_duplicate(ibc.cronos.base_port(0), height)
    for i, log in enumerate(logs):
        method_name, topic = get_topic_data(w3, method_map, contract_info, log)
        assert topic == AttributeDict(expected[i]), [i, method_name]
        # test filter by seq
        if method_name in method_with_seq:
            flogs = filter_logs_since(w3, start, method_name, seq)[0]
            _, ftopic = get_topic_data(w3, method_map, contract_info, flogs)
            assert ftopic == topic, method_name


def get_escrow_address(cli, channel):
    return to_checksum_address(
        bech32_to_eth(cli.ibc_escrow_address("transfer", channel)),
    )


def test_ibc_incentivized_transfer(ibc):
    w3 = ibc.cronos.w3
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    start = w3.eth.get_block_number()
    amount, seq0 = ibc_incentivized_transfer(ibc)
    logs = get_logs_since(w3, CONTRACT, start)
    fee_denom = "ibcfee"
    fee = f"{src_amount}{fee_denom}"
    transfer_denom = "transfer/channel-0/basetcro"
    dst_adr = ibc.chainmain.cosmos_cli().address("signer2")
    src_relayer = ADDRS["signer1"]
    checksum_dst_adr = to_checksum_address(bech32_to_eth(dst_adr))
    feeibc_addr = module_address("feeibc")
    escrow = get_escrow_address(cli, channel)
    seq1 = get_send_packet_seq(ibc.chainmain.cosmos_cli())
    expected = [
        acknowledge_packet(seq0),
        distribute_fee(src_relayer, fee),
        *send_coins(feeibc_addr, src_relayer, src_amount, fee_denom),
        distribute_fee(src_relayer, fee),
        *send_coins(feeibc_addr, src_relayer, src_amount, fee_denom),
        distribute_fee(cronos_signer2, fee),
        *send_coins(feeibc_addr, cronos_signer2, src_amount, fee_denom),
        fungible(checksum_dst_adr, cronos_signer2, amount, dst_denom),
        recv_packet(seq1, dst_adr, cronos_signer2, amount, transfer_denom),
        *send_coins(escrow, cronos_signer2, amount, dst_denom),
        fungible(cronos_signer2, checksum_dst_adr, amount, transfer_denom),
        write_ack(seq1, dst_adr, cronos_signer2, amount, transfer_denom),
    ]
    assert len(logs) == len(expected)
    for i, log in enumerate(logs):
        method_name, topic = get_topic_data(w3, method_map, contract_info, log)
        assert topic == AttributeDict(expected[i]), [i, method_name]
        # test filter by seq
        if method_name in method_with_seq:
            seq = seq0 if method_name == "AcknowledgePacket" else seq1
            flogs = filter_logs_since(w3, start, method_name, seq)[0]
            _, ftopic = get_topic_data(w3, method_map, contract_info, flogs)
            assert ftopic == topic, method_name


def assert_transfer_source_tokens_topics(ibc, fn):
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    w3 = ibc.cronos.w3
    start = w3.eth.get_block_number()
    amount, contract = fn(ibc)
    logs = get_logs_since(w3, CONTRACT, start)
    escrow = get_escrow_address(cli, channel)
    dst_adr = ibc.chainmain.cosmos_cli().address("signer2")
    seq0 = get_send_packet_seq(
        ibc.cronos.cosmos_cli(),
        criteria="message.action='/ethermint.evm.v1.MsgEthereumTx'",
    )
    seq1 = get_send_packet_seq(ibc.chainmain.cosmos_cli())
    checksum_dst_adr = to_checksum_address(bech32_to_eth(dst_adr))
    cronos_addr = module_address("cronos")
    cronos_denom = f"cronos{contract}"
    transfer_denom = f"transfer/{channel}/{cronos_denom}"
    expected = [
        acknowledge_packet(seq0),
        fungible(checksum_dst_adr, ADDRS["validator"], amount, cronos_denom),
        recv_packet(seq1, dst_adr, cronos_signer2, amount, transfer_denom),
        *send_coins(escrow, cronos_signer2, amount, cronos_denom),
        fungible(cronos_signer2, checksum_dst_adr, amount, transfer_denom),
        *send_coins(cronos_signer2, cronos_addr, amount, cronos_denom),
        coin_spent(cronos_addr, amount, cronos_denom),
        burn(cronos_addr, amount, cronos_denom),
        write_ack(seq1, dst_adr, cronos_signer2, amount, transfer_denom),
    ]
    assert len(logs) == len(expected)
    height = logs[0]["blockNumber"]
    assert_duplicate(ibc.cronos.base_port(0), height)
    for i, log in enumerate(logs):
        method_name, topic = get_topic_data(w3, method_map, contract_info, log)
        assert topic == AttributeDict(expected[i]), [i, method_name]
        # test filter by seq
        if method_name in method_with_seq:
            seq = seq0 if method_name == "AcknowledgePacket" else seq1
            flogs = filter_logs_since(w3, start, method_name, seq)[0]
            _, ftopic = get_topic_data(w3, method_map, contract_info, flogs)
            assert ftopic == topic, method_name


def test_cronos_transfer_source_tokens(ibc):
    assert_transfer_source_tokens_topics(ibc, cronos_transfer_source_tokens)


def test_cronos_transfer_source_tokens_with_proxy(ibc):
    assert_transfer_source_tokens_topics(ibc, cronos_transfer_source_tokens_with_proxy)

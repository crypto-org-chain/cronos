import base64
import hashlib
import json
import os
import subprocess
from contextlib import contextmanager
from enum import Enum, IntEnum
from pathlib import Path
from typing import NamedTuple

import requests
from cprotobuf import Field, ProtoEntity
from eth_utils import to_checksum_address
from pystarport import cluster, ports

from .network import Chainmain, Cronos, Hermes, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    bech32_to_eth,
    deploy_contract,
    derive_new_account,
    eth_to_bech32,
    find_log_event_attrs,
    parse_events_rpc,
    send_transaction,
    setup_token_mapping,
    wait_for_fn,
    wait_for_new_blocks,
    wait_for_port,
)

RATIO = 10**10
RELAYER_CALLER = "0x6F1805D56bF05b7be10857F376A5b1c160C8f72C"


class Status(IntEnum):
    PENDING, SUCCESS, FAIL = range(3)


class ChannelOrder(Enum):
    ORDERED = "ORDER_ORDERED"
    UNORDERED = "ORDER_UNORDERED"


class IBCNetwork(NamedTuple):
    cronos: Cronos
    chainmain: Chainmain
    hermes: Hermes | None
    incentivized: bool


def call_hermes_cmd(
    hermes,
    connection_only,
    incentivized,
    version,
    is_ibc_transfer=False,
):
    if connection_only:
        subprocess.check_call(
            [
                "hermes",
                "--config",
                hermes.configpath,
                "create",
                "connection",
                "--a-chain",
                "cronos_777-1",
                "--b-chain",
                "chainmain-1",
            ]
        )
    else:
        subprocess.check_call(
            [
                "hermes",
                "--config",
                hermes.configpath,
                "create",
                "channel",
                "--a-port",
                "transfer",
                "--b-port",
                "transfer",
                "--a-chain",
                "cronos_777-1",
                "--b-chain",
                "chainmain-1",
                "--new-client-connection",
                "--yes",
            ]
            + (
                [
                    "--channel-version",
                    str(version) if is_ibc_transfer else json.dumps(version),
                ]
                if incentivized or is_ibc_transfer
                else []
            )
        )


def call_rly_cmd(path, connection_only, incentivized, version, hostchain="chainmain-1"):
    cmd = [
        "rly",
        "pth",
        "new",
        "cronos_777-1",
        hostchain,
        "chainmain-cronos",
        "--home",
        str(path),
    ]
    subprocess.check_call(cmd)
    if connection_only:
        cmd = [
            "rly",
            "tx",
            "connect",
            "chainmain-cronos",
            "--home",
            str(path),
        ]
    else:
        cmd = [
            "rly",
            "tx",
            "connect",
            "chainmain-cronos",
            "--src-port",
            "transfer",
            "--dst-port",
            "transfer",
            "--order",
            "unordered",
            "--home",
            str(path),
        ]
        if incentivized:
            cmd.extend(["--version", json.dumps(version)])
    subprocess.check_call(cmd)


def prepare_network(
    tmp_path,
    file,
    incentivized=True,
    is_relay=True,
    connection_only=False,
    grantee=None,
    need_relayer_caller=False,
    relayer=cluster.Relayer.HERMES.value,
    is_ibc_transfer=False,
):
    print("incentivized", incentivized)
    print("is_relay", is_relay)
    print("connection_only", connection_only)
    print("relayer", relayer)
    print("need_relayer_caller", need_relayer_caller)
    print("is_ibc_transfer", is_ibc_transfer)
    is_hermes = relayer == cluster.Relayer.HERMES.value
    hermes = None

    # We ignore the ibc_rly_evm settings if it is hermes relayer
    config_file = file
    if is_hermes:
        if file == "ibc_rly_evm":
            config_file = "ibc_rly"
        if file == "ibc_timeout":
            config_file = "ibc_timeout_hermes"

    file_path = f"configs/{config_file}.jsonnet"

    with contextmanager(setup_custom_cronos)(
        tmp_path,
        26700,
        Path(__file__).parent / file_path,
        relayer=relayer,
    ) as cronos:
        cli = cronos.cosmos_cli()
        path = cronos.base_dir.parent / "relayer"
        if grantee:
            granter_addr = cli.address("signer1")
            grantee_addr = cli.address(grantee)
            max_gas = 1000000
            gas_price = 10000000000000000
            limit = f"{max_gas*gas_price*2}basetcro"
            rsp = cli.grant(granter_addr, grantee_addr, limit)
            assert rsp["code"] == 0, rsp["raw_log"]
            grant_detail = cli.query_grant(granter_addr, grantee_addr)
            assert grant_detail["granter"] == granter_addr
            assert grant_detail["grantee"] == grantee_addr
            if not is_hermes:
                subprocess.run(
                    [
                        "rly",
                        "keys",
                        "restore",
                        "cronos_777-1",
                        granter_addr,
                        os.getenv("SIGNER1_MNEMONIC"),
                        "--home",
                        path,
                    ],
                    check=True,
                )

        chainmain = Chainmain(cronos.base_dir.parent / "chainmain-1")
        # wait for grpc ready
        wait_for_port(ports.grpc_port(chainmain.base_port(0)))  # chainmain grpc
        wait_for_port(ports.grpc_port(cronos.base_port(0)))  # cronos grpc
        wait_for_new_blocks(chainmain.cosmos_cli(), 1)
        wait_for_new_blocks(cli, 1)
        connid = os.getenv("CONNECTION_ID", "connection-0")

        channel_version = {
            "version": "ics27-1",
            "encoding": "proto3",
            "tx_type": "sdk_multi_msg",
            "controller_connection_id": connid,
            "host_connection_id": connid,
        }
        version = "ics20-1" if is_ibc_transfer else json.dumps(channel_version)

        w3 = cronos.w3
        contract = None
        acc = None
        if need_relayer_caller:
            acc = derive_new_account(2)
            sender = acc.address
            # fund new sender to deploy contract with same address
            if w3.eth.get_balance(sender, "latest") == 0:
                fund = 3000000000000000000
                tx = {"to": sender, "value": fund, "gasPrice": w3.eth.gas_price}
                send_transaction(w3, tx)
                assert w3.eth.get_balance(sender, "latest") == fund
            contract = deploy_contract(w3, CONTRACTS["TestRelayer"], key=acc.key)
            caller = contract.address
            assert caller == RELAYER_CALLER, caller
        if is_hermes:
            hermes = Hermes(path.with_suffix(".toml"))
            call_hermes_cmd(
                hermes,
                connection_only,
                incentivized,
                version,
                is_ibc_transfer,
            )
        else:
            call_rly_cmd(path, connection_only, incentivized, version)

        port = None
        if is_relay:
            cronos.supervisorctl("start", "relayer-demo")
            if is_hermes:
                port = hermes.port
        yield IBCNetwork(cronos, chainmain, hermes, incentivized)
        if port:
            wait_for_port(port)


def register_fee_payee(src_chain, dst_chain, contract=None, acc=None):
    port_id = "transfer"
    channel_id = "channel-0"
    chains = [src_chain.cosmos_cli(), dst_chain.cosmos_cli()]
    relayer0 = chains[0].address("signer1")
    relayer1 = chains[1].address("relayer")
    rsp = chains[1].register_counterparty_payee(
        port_id,
        channel_id,
        relayer1,
        relayer0,
        from_=relayer1,
        fees="100000000basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    if contract is None:
        rsp = chains[0].register_payee(
            port_id,
            channel_id,
            relayer0,
            relayer0,
            from_="signer1",
            fees="100000000basetcro",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        rsp = chains[0].register_counterparty_payee(
            port_id,
            channel_id,
            relayer0,
            relayer1,
            from_=relayer0,
            fees="100000000basetcro",
        )
        assert rsp["code"] == 0, rsp["raw_log"]
    else:
        data = {"from": acc.address}
        tx = contract.functions.callRegisterPayee(
            port_id, channel_id, to_checksum_address(bech32_to_eth(relayer0))
        ).build_transaction(data)
        receipt = send_transaction(src_chain.w3, tx, acc.key)
        assert receipt.status == 1, receipt
        tx = contract.functions.callRegisterCounterpartyPayee(
            port_id, channel_id, relayer1
        ).build_transaction(data)
        receipt = send_transaction(src_chain.w3, tx, acc.key)
        assert receipt.status == 1, receipt


def assert_ready(ibc):
    # wait for hermes
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{ibc.hermes.port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"


def hermes_transfer(ibc):
    assert_ready(ibc)
    # chainmain-1 -> cronos_777-1
    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    my_channel = "channel-0"
    dst_addr = eth_to_bech32(ADDRS["signer2"])
    src_amount = 10
    src_denom = "basecro"
    # dstchainid srcchainid srcportid srchannelid
    cmd = (
        f"hermes --config {ibc.hermes.configpath} tx ft-transfer "
        f"--dst-chain {my_ibc1} --src-chain {my_ibc0} --src-port transfer "
        f"--src-channel {my_channel} --amount {src_amount} "
        f"--timeout-height-offset 1000 --number-msgs 1 "
        f"--denom {src_denom} --receiver {dst_addr} --key-name relayer"
    )
    subprocess.run(cmd, check=True, shell=True)
    return src_amount


def rly_transfer(ibc):
    # chainmain-1 -> cronos_777-1
    my_ibc0 = "chainmain-1"
    my_ibc1 = "cronos_777-1"
    channel = "channel-0"
    dst_addr = eth_to_bech32(ADDRS["signer2"])
    src_amount = 10
    src_denom = "basecro"
    path = ibc.cronos.base_dir.parent / "relayer"
    # srcchainid dstchainid amount dst_addr srchannelid
    cmd = (
        f"rly tx transfer {my_ibc0} {my_ibc1} {src_amount}{src_denom} "
        f"{dst_addr} {channel} "
        f"--path chainmain-cronos "
        f"--home {str(path)}"
    )
    subprocess.run(cmd, check=True, shell=True)
    return src_amount


def assert_duplicate(base_port, height):
    port = ports.rpc_port(base_port)
    url = f"http://127.0.0.1:{port}/block_results?height={height}"
    res = requests.get(url).json().get("result")
    events = res["txs_results"][0]["events"]
    values = set()
    for event in events:
        if event["type"] == "message":
            continue
        str = json.dumps(event)
        assert str not in values, f"dup event find: {str}"
        values.add(str)


def find_duplicate(attributes):
    res = set()
    key = attributes[0]["key"]
    for attribute in attributes:
        if attribute["key"] == key:
            value0 = attribute["value"]
        elif attribute["key"] == "amount":
            amount = attribute["value"]
            value_pair = f"{value0}:{amount}"
            if value_pair in res:
                return value_pair
            res.add(value_pair)
    return None


def ibc_transfer(ibc, transfer_fn=hermes_transfer):
    src_amount = transfer_fn(ibc)
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


def get_balance(chain, addr, denom):
    balance = chain.cosmos_cli().balance(addr, denom)
    print("balance", balance, addr, denom)
    return balance


def get_balances(chain, addr):
    return chain.cosmos_cli().balances(addr)


def ibc_multi_transfer(ibc):
    chains = [ibc.cronos.cosmos_cli(), ibc.chainmain.cosmos_cli()]
    users = [f"user{i}" for i in range(1, 50)]
    addrs0 = [chains[0].address(user) for user in users]
    addrs1 = [chains[1].address(user) for user in users]
    denom0 = "basetcro"
    denom1 = "basecro"
    channel0 = "channel-0"
    channel1 = "channel-0"
    old_balance0 = 30000000000000000000000
    old_balance1 = 1000000000000000000000
    path = f"transfer/{channel1}/{denom0}"
    denom_hash = hashlib.sha256(path.encode()).hexdigest().upper()
    amount = 1000
    expected = [
        {"denom": denom1, "amount": f"{old_balance1}"},
        {"denom": f"ibc/{denom_hash}", "amount": f"{amount}"},
    ]

    for i, _ in enumerate(users):
        rsp = chains[0].ibc_transfer(
            addrs0[i],
            addrs1[i],
            f"{amount}{denom0}",
            channel0,
            fees=f"1000{denom1}",
            event_query_tx_for=True,
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        balance = chains[1].balance(addrs1[i], denom1)
        assert balance == old_balance1, balance
        balance = chains[0].balance(addrs0[i], denom0)
        assert balance == old_balance0 - amount, balance

    def assert_trace_balance(addr):
        balance = chains[1].balances(addr)
        if len(balance) > 1:
            assert balance == expected, balance
            return True
        else:
            return False

    denom_trace = chains[1].ibc_denom_trace(path, ibc.chainmain.node_rpc(0))
    print("denom_trace", denom_trace)

    assert denom_trace["base"] == denom0
    assert denom_trace["trace"] == [{"port_id": "transfer", "channel_id": channel1}]

    for i, _ in enumerate(users):
        wait_for_fn("assert balance", lambda: assert_trace_balance(addrs1[i]))

    # chainmain-1 -> cronos_777-1
    amt = amount // 2

    def assert_balance(addr):
        balance = chains[0].balance(addr, denom0)
        if balance > old_balance0 - amount:
            assert balance == old_balance0 - amt, balance
            return True
        else:
            return False

    for _ in range(0, 2):
        for i, _ in enumerate(users):
            rsp = chains[1].ibc_transfer(
                addrs1[i],
                addrs0[i],
                f"{amt}ibc/{denom_hash}",
                channel1,
                fees=f"100000000{denom1}",
            )
            assert rsp["code"] == 0, rsp["raw_log"]

        for i, _ in enumerate(users):
            wait_for_fn("assert balance", lambda: assert_balance(addrs0[i]))

        old_balance0 += amt


def ibc_incentivized_transfer(ibc):
    chains = [ibc.cronos.cosmos_cli(), ibc.chainmain.cosmos_cli()]
    user0 = chains[0].address("signer2")
    relayer0 = chains[0].address("signer1")
    user1 = chains[1].address("signer2")
    relayer1 = chains[1].address("relayer")
    amount = 1000
    fee_denom = "ibcfee"
    base_denom0 = "basetcro"
    base_denom1 = "basecro"
    old_relayer0_fee = chains[0].balance(relayer0, fee_denom)
    old_user0_fee = chains[0].balance(user0, fee_denom)
    old_user0_base = chains[0].balance(user0, base_denom0)
    old_relayer1_fee = chains[1].balance(relayer1, fee_denom)
    old_user1_fee = chains[1].balance(user1, fee_denom)
    old_user1_base = chains[1].balance(user1, base_denom1)
    assert old_user0_base == 30000000000100000000000
    assert old_user1_base == 1000000000000000000000
    src_channel = "channel-0"
    dst_channel = "channel-0"
    rsp = chains[0].ibc_transfer(
        user0,
        user1,
        f"{amount}{base_denom0}",
        src_channel,
        fees=f"0{base_denom0}",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = chains[0].event_query_tx_for(rsp["txhash"])
    evt = find_log_event_attrs(
        rsp["events"], "send_packet", lambda attrs: "packet_sequence" in attrs
    )
    print("packet event", evt)
    packet_seq = int(evt["packet_sequence"])
    recv_fee = 10
    ack_fee = 11
    timeout_fee = 12
    rsp = chains[0].pay_packet_fee(
        "transfer",
        src_channel,
        packet_seq,
        recv_fee=f"{recv_fee}{fee_denom}",
        ack_fee=f"{ack_fee}{fee_denom}",
        timeout_fee=f"{timeout_fee}{fee_denom}",
        from_=user0,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # fee is locked
    user0_fee = chains[0].balance(user0, fee_denom)
    # https://github.com/cosmos/ibc-go/pull/5571
    assert user0_fee == old_user0_fee - recv_fee - ack_fee, user0_fee

    # wait for relayer receive the fee
    def check_fee():
        relayer0_fee = chains[0].balance(relayer0, fee_denom)
        if relayer0_fee > old_relayer0_fee:
            assert relayer0_fee == old_relayer0_fee + recv_fee + ack_fee, relayer0_fee
            return True
        else:
            return False

    wait_for_fn("wait for relayer to receive the fee", check_fee)

    # timeout fee is refunded
    user0_balances = get_balances(ibc.cronos, user0)
    expected = [
        {"denom": base_denom0, "amount": f"{old_user0_base - amount}"},
        {"denom": fee_denom, "amount": f"{old_user0_fee - recv_fee - ack_fee}"},
    ]
    assert user0_balances == expected, user0_balances
    path = f"transfer/{dst_channel}/{base_denom0}"
    denom_hash = ibc_denom(dst_channel, base_denom0)
    denom_trace = chains[1].ibc_denom_trace(path, ibc.chainmain.node_rpc(0))
    assert denom_trace == {"path": f"transfer/{dst_channel}", "base_denom": base_denom0}
    user1_balances = get_balances(ibc.chainmain, user1)
    expected = [
        {"denom": base_denom1, "amount": f"{old_user1_base}"},
        {"denom": f"{denom_hash}", "amount": f"{amount}"},
        {"denom": f"{fee_denom}", "amount": f"{old_user1_fee}"},
    ]
    assert user1_balances == expected, user1_balances
    # transfer back
    rsp = chains[1].ibc_transfer(
        user1,
        user0,
        f"{amount}{denom_hash}",
        dst_channel,
        fees=f"0{base_denom1}",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = chains[1].event_query_tx_for(rsp["txhash"])
    evt = find_log_event_attrs(
        rsp["events"], "send_packet", lambda attrs: "packet_sequence" in attrs
    )
    print("packet event", evt)
    rsp = chains[1].pay_packet_fee(
        "transfer",
        dst_channel,
        int(evt["packet_sequence"]),
        recv_fee=f"{recv_fee}{fee_denom}",
        ack_fee=f"{ack_fee}{fee_denom}",
        timeout_fee=f"{timeout_fee}{fee_denom}",
        from_=user1,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    # fee is locked
    user1_fee = chains[1].balance(user1, fee_denom)
    assert user1_fee == old_user1_fee - recv_fee - ack_fee, user1_fee

    # wait for relayer receive the fee
    def check_fee():
        relayer1_fee = chains[1].balance(relayer1, fee_denom)
        if relayer1_fee > old_relayer1_fee:
            assert relayer1_fee == old_relayer1_fee + recv_fee + ack_fee, relayer1_fee
            return True
        else:
            return False

    wait_for_fn("wait for relayer to receive the fee", check_fee)

    def check_balance_change():
        return chains[0].balance(user0, base_denom0) != old_user0_base - amount

    wait_for_fn("balance change", check_balance_change)
    return amount, packet_seq, recv_fee, ack_fee


def ibc_denom(channel, denom):
    h = hashlib.sha256(f"transfer/{channel}/{denom}".encode()).hexdigest().upper()
    return f"ibc/{h}"


def cronos_transfer_source_tokens(ibc):
    # deploy crc21 contract
    w3 = ibc.cronos.w3
    contract, denom = setup_token_mapping(ibc.cronos, "TestERC21Source", "DOG")
    # send token to crypto.org
    print("send to crypto.org")
    chainmain_receiver = ibc.chainmain.cosmos_cli().address("signer2")
    dest_denom = ibc_denom("channel-0", denom)
    amount = 1000

    # check and record receiver balance
    chainmain_receiver_balance = get_balance(
        ibc.chainmain, chainmain_receiver, dest_denom
    )
    assert chainmain_receiver_balance == 0

    # send to ibc
    tx = contract.functions.send_to_ibc_v2(
        chainmain_receiver, amount, 0, b""
    ).build_transaction({"from": ADDRS["validator"]})
    txreceipt = send_transaction(w3, tx)
    assert txreceipt.status == 1, "should success"

    # check balance
    chainmain_receiver_new_balance = 0

    def check_chainmain_balance_change():
        nonlocal chainmain_receiver_new_balance
        chainmain_receiver_new_balance = get_balance(
            ibc.chainmain, chainmain_receiver, dest_denom
        )
        chainmain_receiver_all_balance = get_balances(ibc.chainmain, chainmain_receiver)
        print("receiver all balance:", chainmain_receiver_all_balance)
        return chainmain_receiver_balance != chainmain_receiver_new_balance

    wait_for_fn("check balance change", check_chainmain_balance_change)
    assert chainmain_receiver_new_balance == amount

    # check legacy send to ibc
    tx = contract.functions.send_to_ibc(chainmain_receiver, 1).build_transaction(
        {"from": ADDRS["validator"]}
    )
    txreceipt = send_transaction(w3, tx)
    assert txreceipt.status == 0, "should fail"

    # send back the token to cronos
    # check receiver balance
    cronos_balance_before_send = contract.caller.balanceOf(ADDRS["signer2"])
    assert cronos_balance_before_send == 0

    # send back token through ibc
    print("Send back token through ibc")
    chainmain_cli = ibc.chainmain.cosmos_cli()
    cronos_receiver = eth_to_bech32(ADDRS["signer2"])

    coin = "1000" + dest_denom
    fees = "100000000basecro"
    rsp = chainmain_cli.ibc_transfer(
        chainmain_receiver, cronos_receiver, coin, "channel-0", fees=fees
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # check contract balance
    cronos_balance_after_send = 0

    def check_contract_balance_change():
        nonlocal cronos_balance_after_send
        cronos_balance_after_send = contract.caller.balanceOf(ADDRS["signer2"])
        return cronos_balance_after_send != cronos_balance_before_send

    wait_for_fn("check contract balance change", check_contract_balance_change)
    assert cronos_balance_after_send == amount
    return amount, contract.address


def cronos_transfer_source_tokens_with_proxy(ibc):
    w3 = ibc.cronos.w3
    symbol = "TEST"
    contract, denom = setup_token_mapping(ibc.cronos, "TestCRC20", symbol)

    # deploy crc20 proxy contract
    proxycrc20 = deploy_contract(
        w3,
        CONTRACTS["TestCRC20Proxy"],
        (contract.address, True),
    )

    print("proxycrc20 contract deployed at address: ", proxycrc20.address)
    assert proxycrc20.caller.is_source()
    assert proxycrc20.caller.crc20() == contract.address

    cronos_cli = ibc.cronos.cosmos_cli()
    # change token mapping
    rsp = cronos_cli.update_token_mapping(
        denom, proxycrc20.address, symbol, 6, from_="validator"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cronos_cli, 1)

    print("check the contract mapping exists now")
    rsp = cronos_cli.query_denom_by_contract(proxycrc20.address)
    assert rsp["denom"] == denom

    # send token to crypto.org
    print("send to crypto.org")
    chainmain_receiver = ibc.chainmain.cosmos_cli().address("signer2")
    dest_denom = ibc_denom("channel-0", denom)
    amount = 1000
    sender = ADDRS["validator"]

    # First we need to approve the proxy contract to move asset
    tx = contract.functions.approve(proxycrc20.address, amount).build_transaction(
        {"from": sender}
    )
    txreceipt = send_transaction(w3, tx)
    assert txreceipt.status == 1, "should success"
    assert contract.caller.allowance(ADDRS["validator"], proxycrc20.address) == amount

    # check and record receiver balance
    chainmain_receiver_balance = get_balance(
        ibc.chainmain, chainmain_receiver, dest_denom
    )
    assert chainmain_receiver_balance == 0

    # send to ibc
    tx = proxycrc20.functions.send_to_ibc(
        chainmain_receiver, amount, 0, b""
    ).build_transaction({"from": sender})
    txreceipt = send_transaction(w3, tx)
    print(txreceipt)
    assert txreceipt.status == 1, "should success"

    # check balance
    chainmain_receiver_new_balance = 0

    def check_chainmain_balance_change():
        nonlocal chainmain_receiver_new_balance
        chainmain_receiver_new_balance = get_balance(
            ibc.chainmain, chainmain_receiver, dest_denom
        )
        chainmain_receiver_all_balance = get_balances(ibc.chainmain, chainmain_receiver)
        print("receiver all balance:", chainmain_receiver_all_balance)
        return chainmain_receiver_balance != chainmain_receiver_new_balance

    wait_for_fn("check balance change", check_chainmain_balance_change)
    assert chainmain_receiver_new_balance == amount

    # send back the token to cronos
    # check receiver balance
    cronos_balance_before_send = contract.caller.balanceOf(ADDRS["signer2"])
    assert cronos_balance_before_send == 0

    # send back token through ibc
    print("Send back token through ibc")
    chainmain_cli = ibc.chainmain.cosmos_cli()
    cronos_receiver = eth_to_bech32(ADDRS["signer2"])

    coin = f"{amount}{dest_denom}"
    fees = "100000000basecro"
    rsp = chainmain_cli.ibc_transfer(
        chainmain_receiver, cronos_receiver, coin, "channel-0", fees=fees
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    # check contract balance
    cronos_balance_after_send = 0

    def check_contract_balance_change():
        nonlocal cronos_balance_after_send
        cronos_balance_after_send = contract.caller.balanceOf(ADDRS["signer2"])
        return cronos_balance_after_send != cronos_balance_before_send

    wait_for_fn("check contract balance change", check_contract_balance_change)
    assert cronos_balance_after_send == amount
    return amount, contract.address


def wait_for_check_channel_ready(cli, connid, channel_id, target="STATE_OPEN"):
    print("wait for channel ready", channel_id, target)

    def check_channel_ready():
        channels = cli.ibc_query_channels(connid)["channels"]
        try:
            state = next(
                channel["state"]
                for channel in channels
                if channel["channel_id"] == channel_id
            )
        except StopIteration:
            return False
        return state == target

    wait_for_fn("channel ready", check_channel_ready, timeout=30)


def get_next_channel(cli, connid):
    prefix = "channel-"
    channels = cli.ibc_query_channels(connid)["channels"]
    c = 0
    if len(channels) > 0:
        c = max(channel["channel_id"] for channel in channels)
        c = int(c.removeprefix(prefix)) + 1
    return f"{prefix}{c}"


def wait_for_check_tx(cli, adr, num_txs, timeout=None):
    print("wait for tx arrive")

    def check_tx():
        current = len(cli.query_all_txs(adr)["txs"])
        print("current", current)
        return current > num_txs

    if timeout is None:
        wait_for_fn("transfer tx", check_tx)
    else:
        try:
            print(f"should assert timeout err when pass {timeout}s")
            wait_for_fn("transfer tx", check_tx, timeout=timeout)
        except TimeoutError:
            raised = True
        assert raised


def wait_for_status_change(tcontract, channel_id, seq, timeout=None):
    print(f"wait for status change for {seq}")

    def check_status():
        status = tcontract.caller.getStatus(channel_id, seq)
        return status

    if timeout is None:
        wait_for_fn("current status", check_status)
    else:
        try:
            print(f"should assert timeout err when pass {timeout}s")
            wait_for_fn("current status", check_status, timeout=timeout)
        except TimeoutError:
            raised = True
        assert raised


def register_acc(cli, connid, ordering=ChannelOrder.ORDERED.value, signer="signer2"):
    print("register ica account")
    v = json.dumps(
        {
            "version": "ics27-1",
            "encoding": "proto3",
            "tx_type": "sdk_multi_msg",
            "controller_connection_id": connid,
            "host_connection_id": connid,
        }
    )
    rsp = cli.ica_register_account(
        connid,
        from_=signer,
        gas="400000",
        version=v,
        ordering=ordering,
    )
    port_id, channel_id = assert_channel_open_init(rsp)
    wait_for_check_channel_ready(cli, connid, channel_id)

    print("query ica account")
    ica_address = cli.ica_query_account(
        connid,
        cli.address(signer),
    )["address"]
    print("ica address", ica_address, "channel_id", channel_id)
    return ica_address, port_id, channel_id


def funds_ica(cli, adr, signer="signer2"):
    # initial balance of interchain account should be zero
    assert cli.balance(adr) == 0

    # send some funds to interchain account
    rsp = cli.transfer(signer, adr, "1cro", gas_prices="1000000basecro")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)
    amt = 100000000
    # check if the funds are received in interchain account
    assert cli.balance(adr, denom="basecro") == amt
    return amt


def assert_channel_open_init(rsp):
    assert rsp["code"] == 0, rsp["raw_log"]
    port_id, channel_id = next(
        (
            evt["attributes"][0]["value"],
            evt["attributes"][1]["value"],
        )
        for evt in rsp["events"]
        if evt["type"] == "channel_open_init"
    )
    print("port-id", port_id, "channel-id", channel_id)
    return port_id, channel_id


def gen_send_msg(sender, receiver, denom, amount):
    return {
        "@type": "/cosmos.bank.v1beta1.MsgSend",
        "from_address": sender,
        "to_address": receiver,
        "amount": [{"denom": denom, "amount": f"{amount}"}],
    }


def ica_send_tx(
    cli_host,
    cli_controller,
    connid,
    ica_address,
    msg_num,
    receiver,
    denom,
    amount,
    memo=None,
    incentivized_cb=None,
    signer="signer2",
    **kwargs,
):
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    # generate a transaction to send to host chain
    m = gen_send_msg(ica_address, receiver, denom, amount)
    msgs = []
    for i in range(msg_num):
        msgs.append(m)
    data = json.dumps(msgs)
    packet = cli_controller.ica_generate_packet_data(data, json.dumps(memo))
    # submit transaction on host chain on behalf of interchain account
    rsp = cli_controller.ica_send_tx(
        connid,
        json.dumps(packet),
        from_=signer,
        **kwargs,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    events = parse_events_rpc(rsp["events"])
    seq = int(events.get("send_packet")["packet_sequence"])
    if incentivized_cb:
        incentivized_cb(seq)
    wait_for_check_tx(cli_host, ica_address, num_txs)
    return seq


def log_gas_records(cli):
    criteria = "tx.height >= 0"
    txs = cli.tx_search_rpc(criteria)
    records = []
    for tx in txs:
        res = tx["tx_result"]
        if res["gas_used"]:
            records.append(int(res["gas_used"]))
    return records


class QueryBalanceRequest(ProtoEntity):
    address = Field("string", 1)
    denom = Field("string", 2)


def gen_query_balance_packet(cli, ica_address):
    query = QueryBalanceRequest(address=ica_address, denom="basecro")
    data = json.dumps(
        {
            "@type": "/ibc.applications.interchain_accounts.host.v1.MsgModuleQuerySafe",
            "signer": ica_address,
            "requests": [
                {
                    "path": "/cosmos.bank.v1beta1.Query/Balance",
                    "data": base64.b64encode(query.SerializeToString()).decode(),
                }
            ],
        }
    )
    return cli.ica_generate_packet_data(data)

import hashlib
import json
import os
import subprocess
from pathlib import Path
from typing import NamedTuple

from pystarport import cluster, ports

from .network import Chainmain, Cronos, Hermes, setup_custom_cronos
from .utils import (
    ADDRS,
    CONTRACTS,
    deploy_contract,
    eth_to_bech32,
    parse_events,
    parse_events_rpc,
    send_transaction,
    setup_token_mapping,
    wait_for_fn,
    wait_for_new_blocks,
    wait_for_port,
)

RATIO = 10**10


class IBCNetwork(NamedTuple):
    cronos: Cronos
    chainmain: Chainmain
    hermes: Hermes | None
    incentivized: bool
    proc: subprocess.Popen[bytes] | None


def call_hermes_cmd(
    hermes,
    connection_only,
    incentivized,
    version,
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
                    json.dumps(version),
                ]
                if incentivized
                else []
            )
        )


def call_rly_cmd(path, version):
    cmd = [
        "rly",
        "pth",
        "new",
        "chainmain-1",
        "cronos_777-1",
        "chainmain-cronos",
        "--home",
        str(path),
    ]
    subprocess.check_call(cmd)
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
        "--version",
        json.dumps(version),
        "--home",
        str(path),
    ]
    subprocess.check_call(cmd)


def prepare_network(
    tmp_path,
    file,
    incentivized=True,
    is_relay=True,
    connection_only=False,
    relayer=cluster.Relayer.HERMES.value,
):
    print("incentivized", incentivized)
    print("is_relay", is_relay)
    print("connection_only", connection_only)
    print("relayer", relayer)
    is_hermes = relayer == cluster.Relayer.HERMES.value
    hermes = None
    file = f"configs/{file}.jsonnet"
    gen = setup_custom_cronos(
        tmp_path,
        26700,
        Path(__file__).parent / file,
        relayer=relayer,
    )
    cronos = next(gen)
    chainmain = Chainmain(cronos.base_dir.parent / "chainmain-1")
    # wait for grpc ready
    wait_for_port(ports.grpc_port(chainmain.base_port(0)))  # chainmain grpc
    wait_for_port(ports.grpc_port(cronos.base_port(0)))  # cronos grpc

    version = {"fee_version": "ics29-1", "app_version": "ics20-1"}
    path = cronos.base_dir.parent / "relayer"
    if is_hermes:
        hermes = Hermes(path.with_suffix(".toml"))
        call_hermes_cmd(
            hermes,
            connection_only,
            incentivized,
            version,
        )
    else:
        call_rly_cmd(path, version)

    proc = None
    if incentivized:
        # register fee payee
        src_chain = cronos.cosmos_cli()
        dst_chain = chainmain.cosmos_cli()
        rsp = dst_chain.register_counterparty_payee(
            "transfer",
            "channel-0",
            dst_chain.address("relayer"),
            src_chain.address("signer1"),
            from_="relayer",
            fees="100000000basecro",
        )
        assert rsp["code"] == 0, rsp["raw_log"]

    port = None
    if is_relay:
        if is_hermes:
            cronos.supervisorctl("start", "relayer-demo")
            port = hermes.port
        else:
            proc = subprocess.Popen(
                [
                    "rly",
                    "start",
                    "chainmain-cronos",
                    "--home",
                    str(path),
                ],
                preexec_fn=os.setsid,
            )
            port = 5183
    yield IBCNetwork(cronos, chainmain, hermes, incentivized, proc)
    if port:
        wait_for_port(port)


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


def ibc_transfer_with_hermes(ibc):
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


def get_balance(chain, addr, denom):
    balance = chain.cosmos_cli().balance(addr, denom)
    print("balance", balance, addr, denom)
    return balance


def get_balances(chain, addr):
    return chain.cosmos_cli().balances(addr)


def ibc_incentivized_transfer(ibc):
    chains = [ibc.cronos.cosmos_cli(), ibc.chainmain.cosmos_cli()]
    receiver = chains[1].address("signer2")
    sender = chains[0].address("signer2")
    relayer = chains[0].address("signer1")
    amount = 1000
    fee_denom = "ibcfee"
    base_denom = "basetcro"
    old_amt_fee = chains[0].balance(relayer, fee_denom)
    old_amt_sender_fee = chains[0].balance(sender, fee_denom)
    old_amt_sender_base = chains[0].balance(sender, base_denom)
    old_amt_receiver_base = chains[1].balance(receiver, "basecro")
    assert old_amt_sender_base == 30000000000100000000000
    assert old_amt_receiver_base == 1000000000000000000000
    src_channel = "channel-0"
    dst_channel = "channel-0"
    rsp = chains[0].ibc_transfer(
        sender,
        receiver,
        f"{amount}{base_denom}",
        src_channel,
        1,
        "0basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    src_chain = ibc.cronos.cosmos_cli()
    rsp = src_chain.event_query_tx_for(rsp["txhash"])
    evt = parse_events(rsp["logs"])["send_packet"]
    print("packet event", evt)
    packet_seq = int(evt["packet_sequence"])
    fee = f"10{fee_denom}"
    rsp = chains[0].pay_packet_fee(
        "transfer",
        src_channel,
        packet_seq,
        recv_fee=fee,
        ack_fee=fee,
        timeout_fee=fee,
        from_=sender,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = src_chain.event_query_tx_for(rsp["txhash"])
    # fee is locked
    assert chains[0].balance(sender, fee_denom) == old_amt_sender_fee - 30

    # wait for relayer receive the fee
    def check_fee():
        amount = chains[0].balance(relayer, fee_denom)
        if amount > old_amt_fee:
            assert amount == old_amt_fee + 20
            return True
        else:
            return False

    wait_for_fn("wait for relayer to receive the fee", check_fee)

    # timeout fee is refunded
    assert get_balances(ibc.cronos, sender) == [
        {"denom": base_denom, "amount": f"{old_amt_sender_base - amount}"},
        {"denom": fee_denom, "amount": f"{old_amt_sender_fee - 20}"},
    ]
    path = f"transfer/{dst_channel}/{base_denom}"
    denom_hash = hashlib.sha256(path.encode()).hexdigest().upper()
    assert json.loads(
        chains[0].raw(
            "query",
            "ibc-transfer",
            "denom-trace",
            denom_hash,
            node=ibc.chainmain.node_rpc(0),
            output="json",
        )
    )["denom_trace"] == {"path": f"transfer/{dst_channel}", "base_denom": base_denom}
    assert get_balances(ibc.chainmain, receiver) == [
        {"denom": "basecro", "amount": f"{old_amt_receiver_base}"},
        {"denom": f"ibc/{denom_hash}", "amount": f"{amount}"},
    ]
    # transfer back
    fee_amount = 100000000
    rsp = chains[1].ibc_transfer(
        receiver,
        sender,
        f"{amount}ibc/{denom_hash}",
        dst_channel,
        1,
        f"{fee_amount}basecro",
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    def check_balance_change():
        return chains[0].balance(sender, base_denom) != old_amt_sender_base - amount

    wait_for_fn("balance change", check_balance_change)
    assert chains[0].balance(sender, base_denom) == old_amt_sender_base
    assert chains[1].balance(receiver, "basecro") == old_amt_receiver_base - fee_amount
    return amount


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
    rsp = chainmain_cli.ibc_transfer(
        chainmain_receiver, cronos_receiver, coin, "channel-0", 1, "100000000basecro"
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
    rsp = chainmain_cli.ibc_transfer(
        chainmain_receiver, cronos_receiver, coin, "channel-0", 1, "100000000basecro"
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


def wait_for_check_channel_ready(cli, connid, channel_id):
    print("wait for channel ready")

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
        return state == "STATE_OPEN"

    wait_for_fn("channel ready", check_channel_ready)


def wait_for_check_tx(cli, adr, num_txs):
    print("wait for tx arrive")

    def check_tx():
        current = len(cli.query_all_txs(adr)["txs"])
        print("current", current)
        return current > num_txs

    wait_for_fn("transfer tx", check_tx)


def funds_ica(cli, adr):
    # initial balance of interchain account should be zero
    assert cli.balance(adr) == 0

    # send some funds to interchain account
    rsp = cli.transfer("signer2", adr, "1cro", gas_prices="1000000basecro")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)

    # check if the funds are received in interchain account
    assert cli.balance(adr, denom="basecro") == 100000000


def generate_ica_packet(cli, ica_address, to):
    # generate a transaction to send to host chain
    generated_tx_msg = {
        "@type": "/cosmos.bank.v1beta1.MsgSend",
        "from_address": ica_address,
        "to_address": to,
        "amount": [{"denom": "basecro", "amount": "50000000"}],
    }
    str = json.dumps(generated_tx_msg)
    generated_packet = cli.ica_generate_packet_data(str)
    return json.dumps(generated_packet)

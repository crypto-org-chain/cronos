import json

import pytest
from web3.datastructures import AttributeDict

from .ibc_utils import (
    funds_ica,
    prepare_network,
    wait_for_check_channel_ready,
    wait_for_check_tx,
)
from .utils import (
    ADDRS,
    CONTRACT_ABIS,
    CONTRACTS,
    KEYS,
    deploy_contract,
    eth_to_bech32,
    get_logs_since,
    get_method_map,
    get_topic_data,
    send_transaction,
)

CONTRACT = "0x0000000000000000000000000000000000000066"
contract_info = json.loads(CONTRACT_ABIS["IICAModule"].read_text())
method_map = get_method_map(contract_info)
connid = "connection-0"
timeout = 300000000000
denom = "basecro"
keys = KEYS["signer2"]
validator = "validator"
amt = 1000
amt1 = 100


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    path = tmp_path_factory.mktemp(name)
    network = prepare_network(path, name, incentivized=False, connection_only=True)
    yield from network


def register_acc(cli, w3, register, query, data, addr, channel_id):
    print("register ica account")
    tx = register(connid, "").build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    owner = eth_to_bech32(addr)
    wait_for_check_channel_ready(cli, connid, channel_id)
    res = cli.ica_query_account(connid, owner)
    ica_address = res["interchain_account_address"]
    print("query ica account", ica_address)
    res = query(connid, addr).call()
    assert ica_address == res, res
    return ica_address


def submit_msgs(ibc, func, data, ica_address, is_multi, seq):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    to = cli_host.address(validator)
    # generate msg send to host chain
    msgs = [
        {
            "@type": "/cosmos.bank.v1beta1.MsgSend",
            "from_address": ica_address,
            "to_address": to,
            "amount": [{"denom": denom, "amount": f"{amt}"}],
        }
    ]
    if is_multi:
        to = cli_host.address(validator, bech="val")
        # generate msg delegate to host chain
        msgs.append(
            {
                "@type": "/cosmos.staking.v1beta1.MsgDelegate",
                "delegator_address": ica_address,
                "validator_address": to,
                "amount": {"denom": denom, "amount": f"{amt1}"},
            }
        )
    generated_packet = cli_controller.ica_generate_packet_data(json.dumps(msgs))
    num_txs = len(cli_host.query_all_txs(ica_address)["txs"])
    start = w3.eth.get_block_number()
    str = json.dumps(generated_packet)
    # submit transaction on host chain on behalf of interchain account
    tx = func(connid, str, timeout).build_transaction(data)
    receipt = send_transaction(w3, tx, keys)
    assert receipt.status == 1
    logs = get_logs_since(w3, CONTRACT, start)
    expected = [{"seq": seq}]
    assert len(logs) == len(expected)
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]
    wait_for_check_tx(cli_host, ica_address, num_txs)
    return str


def test_call(ibc):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    name = "signer2"
    addr = ADDRS[name]
    contract = w3.eth.contract(address=CONTRACT, abi=contract_info)
    data = {"from": ADDRS[name], "gas": 200000}
    ica_address = register_acc(
        cli_controller,
        w3,
        contract.functions.registerAccount,
        contract.functions.queryAccount,
        data,
        addr,
        "channel-0",
    )
    balance = funds_ica(cli_host, ica_address)
    submit_msgs(ibc, contract.functions.submitMsgs, data, ica_address, False, 1)
    balance -= amt
    assert cli_host.balance(ica_address, denom=denom) == balance
    submit_msgs(ibc, contract.functions.submitMsgs, data, ica_address, True, 2)
    balance -= amt
    balance -= amt1
    assert cli_host.balance(ica_address, denom=denom) == balance


def test_sc_call(ibc):
    cli_host = ibc.chainmain.cosmos_cli()
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    contract = w3.eth.contract(address=CONTRACT, abi=contract_info)
    tcontract = deploy_contract(w3, CONTRACTS["TestICA"])
    addr = tcontract.address
    name = "signer2"
    signer = ADDRS[name]
    keys = KEYS[name]
    data = {"from": signer, "gas": 200000}
    ica_address = register_acc(
        cli_controller,
        w3,
        tcontract.functions.callRegister,
        contract.functions.queryAccount,
        data,
        addr,
        "channel-1",
    )
    balance = funds_ica(cli_host, ica_address)
    assert tcontract.caller.getAccount() == signer
    assert tcontract.functions.callQueryAccount(connid, addr).call() == ica_address

    # register from another user should fail
    name = "signer1"
    data = {"from": ADDRS[name], "gas": 200000}
    version = ""
    tx = tcontract.functions.callRegister(connid, version).build_transaction(data)
    res = send_transaction(w3, tx, KEYS[name])
    assert res.status == 0
    assert tcontract.caller.getAccount() == signer

    assert tcontract.functions.delegateQueryAccount(connid, addr).call() == ica_address
    assert tcontract.functions.staticQueryAccount(connid, addr).call() == ica_address

    # readonly call should fail
    def register_ro(func):
        tx = func(connid, version).build_transaction(data)
        assert send_transaction(w3, tx, keys).status == 0

    register_ro(tcontract.functions.delegateRegister)
    register_ro(tcontract.functions.staticRegister)

    # readonly call should fail
    def submit_msgs_ro(func, str):
        tx = func(connid, str, timeout).build_transaction(data)
        assert send_transaction(w3, tx, keys).status == 0

    seq = 1
    str = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        False,
        seq,
    )
    submit_msgs_ro(tcontract.functions.delegateSubmitMsgs, str)
    submit_msgs_ro(tcontract.functions.staticSubmitMsgs, str)
    assert tcontract.caller.getLastAckSeq() == seq
    balance -= amt
    assert cli_host.balance(ica_address, denom=denom) == balance
    seq = 2
    str = submit_msgs(
        ibc,
        tcontract.functions.callSubmitMsgs,
        data,
        ica_address,
        True,
        seq,
    )
    submit_msgs_ro(tcontract.functions.delegateSubmitMsgs, str)
    submit_msgs_ro(tcontract.functions.staticSubmitMsgs, str)
    balance -= amt
    balance -= amt1
    assert cli_host.balance(ica_address, denom=denom) == balance

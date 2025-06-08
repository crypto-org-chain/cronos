import json

import pytest
import requests
from eth_account.account import Account
from eth_utils import abi
from hexbytes import HexBytes
from pystarport import ports
from web3.exceptions import BadFunctionCallOutput

from .gravity_utils import prepare_gravity, setup_cosmos_erc20_contract
from .network import setup_cronos, setup_geth
from .utils import (
    ACCOUNTS,
    ADDRS,
    CONTRACTS,
    KEYS,
    approve_proposal,
    deploy_contract,
    eth_to_bech32,
    multiple_send_to_cosmos,
    send_to_cosmos,
    send_transaction,
    setup_token_mapping,
    w3_wait_for_new_blocks,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gravity

# skip gravity-bridge integration tests since it's not enabled by default.
pytest.skip("skipping gravity-bridge tests", allow_module_level=True)

Account.enable_unaudited_hdwallet_features()


def cronos_crc21_abi():
    path = CONTRACTS["ModuleCRC21"]
    return json.load(path.open())["abi"]


def check_auto_deployment(cli, denom, cronos_w3, recipient, amount):
    "check crc21 contract auto deployed, and the crc21 balance"
    try:
        rsp = cli.query_contract_by_denom(denom)
    except AssertionError:
        # not deployed yet
        return None
    assert len(rsp["auto_contract"]) > 0
    crc21_contract = cronos_w3.eth.contract(
        address=rsp["auto_contract"], abi=cronos_crc21_abi()
    )
    try:
        if crc21_contract.caller.balanceOf(recipient) == amount:
            return crc21_contract
    except BadFunctionCallOutput:
        # there's a chance the contract is not ready for call,
        # maybe due to inconsistency between different rpc services.
        return None
    return None


def get_id_from_receipt(receipt):
    "check the id after sendToEvmChain call"
    target = HexBytes(
        abi.event_signature_to_log_topic("__CronosSendToEvmChainResponse(uint256)")
    )
    for _, log in enumerate(receipt.logs):
        if log.topics[0] == target:
            return log.data
    res = "0x0000000000000000000000000000000000000000000000000000000000000000"
    return HexBytes(res)


@pytest.fixture(scope="module")
def custom_geth(tmp_path_factory):
    yield from setup_geth(tmp_path_factory.mktemp("geth"), 8555)


@pytest.fixture(scope="module", params=[True, False])
def custom_cronos(request, tmp_path_factory):
    yield from setup_cronos(tmp_path_factory.mktemp("cronos"), 26600, request.param)


@pytest.fixture(scope="module")
def gravity(custom_cronos, custom_geth):
    yield from prepare_gravity(custom_cronos, custom_geth)


def test_gravity_transfer(gravity):
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        CONTRACTS["TestERC20A"],
    )

    balance = erc20.caller.balanceOf(ADDRS["validator"])
    assert balance == 100000000000000000000000000
    amount = 1000

    print("send to cronos crc20")
    recipient = HexBytes(ADDRS["community"])
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, geth, recipient, amount, KEYS["validator"]
    )
    assert txreceipt.status == 1, "should success"
    assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

    denom = f"gravity{erc20.address}"

    def check_gravity_native_tokens():
        "check the balance of gravity native token"
        return cli.balance(eth_to_bech32(recipient), denom=denom) == amount

    if gravity.cronos.enable_auto_deployment:
        crc21_contract = None

        def local_check_auto_deployment():
            nonlocal crc21_contract
            crc21_contract = check_auto_deployment(
                cli, denom, cronos_w3, recipient, amount
            )
            return crc21_contract

        wait_for_fn("send-to-crc21", local_check_auto_deployment)

        # send it back to erc20
        tx = crc21_contract.functions.send_to_evm_chain(
            ADDRS["validator"], amount, 1, 0, b""
        ).build_transaction({"from": ADDRS["community"]})
        txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
        # CRC20 emit 3 logs for send_to_evm_chain:
        # burn
        # __CronosSendToEvmChain
        # __CronosSendToEvmChainResponse
        assert len(txreceipt.logs) == 3
        data = "0x0000000000000000000000000000000000000000000000000000000000000001"
        match = get_id_from_receipt(txreceipt) == HexBytes(data)
        assert match, "should be able to get id"
        assert txreceipt.status == 1, "should success"
    else:
        wait_for_fn("send-to-gravity-native", check_gravity_native_tokens)
        # send back the gravity native tokens
        rsp = cli.send_to_ethereum(
            ADDRS["validator"], f"{amount}{denom}", f"0{denom}", from_="community"
        )
        assert rsp["code"] == 0, rsp["raw_log"]

    def check():
        v = erc20.caller.balanceOf(ADDRS["validator"])
        return v == balance

    wait_for_fn("send-to-ethereum", check)


def test_multiple_attestation_processing(gravity):
    if not gravity.cronos.enable_auto_deployment:
        geth = gravity.geth
        cli = gravity.cronos.cosmos_cli()

        # deploy test erc20 contract
        erc20 = deploy_contract(
            geth,
            CONTRACTS["TestERC20A"],
        )

        balance = erc20.caller.balanceOf(ADDRS["validator"])
        assert balance == 100000000000000000000000000

        amount = 10
        # Send some eth and erc20 to all accounts
        print("fund all accounts")
        for name in ACCOUNTS:
            address = ACCOUNTS[name].address
            data = {"to": address, "value": 10**17}
            send_transaction(geth, data, KEYS["validator"])
            tx = erc20.functions.transfer(address, amount).build_transaction(
                {"from": ADDRS["validator"]}
            )
            tx_receipt = send_transaction(geth, tx, KEYS["validator"])
            assert tx_receipt.status == 1, "should success"

        print("generate multiple send to cosmos")
        recipient = HexBytes(ADDRS["community"])

        denom = f"gravity{erc20.address}"
        previous = cli.balance(eth_to_bech32(recipient), denom=denom)
        height_to_check = cli.block_height()

        multiple_send_to_cosmos(
            gravity.contract, erc20, geth, recipient, amount, KEYS.values()
        )

        def check_gravity_balance():
            """
            check the all attestation are processed at once by comparing
            with previous block balance
            """
            nonlocal previous
            nonlocal height_to_check
            current = cli.balance(
                eth_to_bech32(recipient), denom=denom, height=height_to_check
            )
            check = current == previous + (10 * len(ACCOUNTS))
            previous = current
            height_to_check = height_to_check + 1
            return check

        # we are checking the difference of balance for each height to ensure
        # attestation are processed within the same block
        wait_for_fn(
            "send-to-gravity-native", check_gravity_balance, timeout=600, interval=2
        )


def submit_proposal(cli, tmp_path, is_legacy, denom, conctract):
    if is_legacy:
        return cli.gov_propose_token_mapping_change_legacy(
            denom, conctract, "", 0, from_="community", deposit="1basetcro"
        )
    proposal = tmp_path / "proposal.json"
    # governance module account as signer
    signer = "crc10d07y265gmmuvt4z0w9aw880jnsr700jdufnyd"
    proposal_src = {
        "messages": [
            {
                "@type": "/cosmos.gov.v1.MsgExecLegacyContent",
                "content": {
                    "@type": "/cronos.TokenMappingChangeProposal",
                    "denom": denom,
                    "contract": conctract,
                    "symbol": "",
                    "decimal": 0,
                },
                "authority": signer,
            }
        ],
        "deposit": "1basetcro",
        "title": "title",
        "summary": "summary",
    }
    proposal.write_text(json.dumps(proposal_src))
    return cli.submit_gov_proposal(
        "community", "submit-proposal", proposal, broadcast_mode="sync"
    )


@pytest.mark.parametrize("is_legacy", [True, False])
def test_gov_token_mapping(gravity, tmp_path, is_legacy):
    """
    Test adding a token mapping through gov module
    - deploy test erc20 contract on geth
    - deploy corresponding contract on cronos
    - add the token mapping on cronos using gov module
    - do a gravity transfer, check the balances
    """
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        CONTRACTS["TestERC20A"],
    )
    print("erc20 contract", erc20.address)
    crc21 = deploy_contract(
        cronos_w3,
        CONTRACTS["TestERC20Utility"],
    )
    print("crc21 contract", crc21.address)
    denom = f"gravity{erc20.address}"

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cli.query_contract_by_denom(denom)

    rsp = submit_proposal(cli, tmp_path, is_legacy, denom, crc21.address)
    assert rsp["code"] == 0, rsp["raw_log"]

    approve_proposal(gravity.cronos, rsp["events"])

    print("check the contract mapping exists now")
    rsp = cli.query_contract_by_denom(denom)
    print("contract", rsp)
    assert rsp["contract"] == crc21.address

    print("try to send token from ethereum to cronos")
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, geth, ADDRS["community"], 10, KEYS["validator"]
    )
    assert txreceipt.status == 1

    def check():
        balance = crc21.caller.balanceOf(ADDRS["community"])
        print("crc20 balance", balance)
        return balance == 10

    wait_for_fn("check balance on cronos", check)

    # check duplicate end_block_events
    height = cli.block_height()
    port = ports.rpc_port(gravity.cronos.base_port(0))
    url = f"http://127.0.0.1:{port}/block_results?height={height}"
    res = requests.get(url).json().get("result")
    if res:
        events = res["end_block_events"]
        target = "ethereum_send_to_cosmos_handled"
        count = sum(1 for evt in events if evt["type"] == target)
        assert count <= 2, f"duplicate {target}"


def test_direct_token_mapping(gravity):
    """
    Test adding a token mapping directly
    - deploy test erc20 contract on geth
    - deploy corresponding contract on cronos
    - add the token mapping on cronos using gov module
    - do a gravity transfer, check the balances
    """
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        CONTRACTS["TestERC20A"],
    )
    print("erc20 contract", erc20.address)
    crc21 = deploy_contract(
        cronos_w3,
        CONTRACTS["TestERC20Utility"],
    )
    print("crc21 contract", crc21.address)
    denom = f"gravity{erc20.address}"

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cli.query_contract_by_denom(denom)

    rsp = cli.update_token_mapping(denom, crc21.address, "", 0, from_="community")
    assert rsp["code"] != 0, "should not have the permission"

    rsp = cli.update_token_mapping(denom, crc21.address, "", 0, from_="validator")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)

    print("check the contract mapping exists now")
    rsp = cli.query_contract_by_denom(denom)
    print("contract", rsp)
    assert rsp["contract"] == crc21.address

    print("try to send token from ethereum to cronos")
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, geth, ADDRS["community"], 10, KEYS["validator"]
    )
    assert txreceipt.status == 1

    def check():
        balance = crc21.caller.balanceOf(ADDRS["community"])
        print("crc20 balance", balance)
        return balance == 10

    wait_for_fn("check balance on cronos", check)


def test_gravity_cancel_transfer(gravity):
    if gravity.cronos.enable_auto_deployment:
        geth = gravity.geth
        cli = gravity.cronos.cosmos_cli()
        cronos_w3 = gravity.cronos.w3

        # deploy test erc20 contract
        erc20 = deploy_contract(
            geth,
            CONTRACTS["TestERC20A"],
        )

        # deploy gravity cancellation contract
        cancel_contract = deploy_contract(
            cronos_w3,
            CONTRACTS["CronosGravityCancellation"],
        )

        balance = erc20.caller.balanceOf(ADDRS["validator"])
        assert balance == 100000000000000000000000000
        amount = 1000

        print("send to cronos crc21")
        community = HexBytes(ADDRS["community"])
        key = KEYS["validator"]
        send_to_cosmos(gravity.contract, erc20, geth, community, amount, key)
        assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

        denom = f"gravity{erc20.address}"
        crc21_contract = None

        def local_check_auto_deployment():
            nonlocal crc21_contract
            crc21_contract = check_auto_deployment(
                cli, denom, cronos_w3, community, amount
            )
            return crc21_contract

        wait_for_fn("send-to-crc21", local_check_auto_deployment)

        # batch are created every 10 blocks, wait til block number reaches
        # a multiple of 10 to lower the chance to have the transaction include
        # in the batch right away
        current_block = cronos_w3.eth.get_block_number()
        print("current_block: ", current_block)
        batch_block = 10
        diff_block = batch_block - current_block % batch_block
        wait_for_new_blocks(cli, diff_block)

        # send it back to erc20
        tx = crc21_contract.functions.send_to_evm_chain(
            ADDRS["validator"], amount, 1, 0, b""
        ).build_transaction({"from": community})
        txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
        # CRC20 emit 3 logs for send_to_evm_chain:
        # burn
        # __CronosSendToEvmChain
        # __CronosSendToEvmChainResponse
        assert len(txreceipt.logs) == 3
        tx_id = get_id_from_receipt(txreceipt)
        assert txreceipt.status == 1, "should success"

        # Check_deduction
        balance_after_send = crc21_contract.caller.balanceOf(community)
        assert balance_after_send == 0

        # Cancel the send_to_evm_chain from another contract
        canceltx = cancel_contract.functions.cancelTransaction(
            int.from_bytes(tx_id, "big")
        ).build_transaction({"from": community})
        canceltxreceipt = send_transaction(cronos_w3, canceltx, KEYS["community"])
        # Should fail because it was not called from the CRC21 contract
        assert canceltxreceipt.status == 0, "should fail"

        canceltx = crc21_contract.functions.cancel_send_to_evm_chain(
            int.from_bytes(tx_id, "big")
        ).build_transaction({"from": community})
        canceltxreceipt = send_transaction(cronos_w3, canceltx, KEYS["community"])
        assert canceltxreceipt.status == 1, "should success"

        def check_refund():
            v = crc21_contract.caller.balanceOf(community)
            return v == amount

        wait_for_fn("cancel-send-to-ethereum", check_refund)


def test_gravity_source_tokens(gravity):
    if not gravity.cronos.enable_auto_deployment:
        # deploy contracts
        w3 = gravity.cronos.w3
        symbol = "DOG"
        contract, denom = setup_token_mapping(gravity.cronos, "TestERC21Source", symbol)
        cosmos_erc20_contract = setup_cosmos_erc20_contract(
            gravity,
            denom,
            symbol,
        )
        # Send token to ethereum
        amount = 1000
        ethereum_receiver = ADDRS["validator"]
        balance_before_send_to_ethereum = cosmos_erc20_contract.caller.balanceOf(
            ethereum_receiver
        )

        print("send to ethereum")
        tx = contract.functions.send_to_evm_chain(
            ethereum_receiver, amount, 1, 0, b""
        ).build_transaction({"from": ADDRS["validator"]})
        txreceipt = send_transaction(w3, tx)
        assert txreceipt.status == 1, "should success"

        balance_after_send_to_ethereum = balance_before_send_to_ethereum

        def check_ethereum_balance_change():
            nonlocal balance_after_send_to_ethereum
            balance_after_send_to_ethereum = cosmos_erc20_contract.caller.balanceOf(
                ethereum_receiver
            )
            return balance_before_send_to_ethereum != balance_after_send_to_ethereum

        wait_for_fn("check ethereum balance change", check_ethereum_balance_change)
        assert (
            balance_after_send_to_ethereum == balance_before_send_to_ethereum + amount
        )

        # Send back token to cronos
        cronos_receiver = "0x0000000000000000000000000000000000000001"
        balance_before_send_to_cosmos = contract.caller.balanceOf(cronos_receiver)
        amount = 15
        txreceipt = send_to_cosmos(
            gravity.contract,
            cosmos_erc20_contract,
            gravity.geth,
            HexBytes(cronos_receiver),
            amount,
            KEYS["validator"],
        )
        assert txreceipt.status == 1, "should success"

        balance_after_send_to_cosmos = balance_before_send_to_cosmos

        def check_cronos_balance_change():
            nonlocal balance_after_send_to_cosmos
            balance_after_send_to_cosmos = contract.caller.balanceOf(cronos_receiver)
            return balance_before_send_to_cosmos != balance_after_send_to_cosmos

        wait_for_fn("check cronos balance change", check_cronos_balance_change)
        assert balance_after_send_to_cosmos == balance_before_send_to_cosmos + amount


def test_gravity_blacklisted_contract(gravity):
    if gravity.cronos.enable_auto_deployment:
        geth = gravity.geth
        cli = gravity.cronos.cosmos_cli()
        cronos_w3 = gravity.cronos.w3

        # deploy test blacklisted contract with signer1 as blacklisted
        erc20 = deploy_contract(
            geth,
            CONTRACTS["TestBlackListERC20"],
            (ADDRS["signer1"],),
        )

        balance = erc20.caller.balanceOf(ADDRS["validator"])
        assert balance == 100000000000000000000000000
        amount = 1000

        print("send to cronos crc20")
        recipient = HexBytes(ADDRS["community"])
        txreceipt = send_to_cosmos(
            gravity.contract, erc20, geth, recipient, amount, KEYS["validator"]
        )
        assert txreceipt.status == 1, "should success"
        assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

        denom = f"gravity{erc20.address}"
        crc21_contract = None

        def local_check_auto_deployment():
            nonlocal crc21_contract
            crc21_contract = check_auto_deployment(
                cli, denom, cronos_w3, recipient, amount
            )
            return crc21_contract

        wait_for_fn("send-to-crc21", local_check_auto_deployment)

        # get voucher nonce
        old_nonce = gravity.contract.caller.state_lastRevertedNonce()
        old_balance1 = erc20.caller.balanceOf(ADDRS["signer1"])

        # send it back to blacklisted address
        tx = crc21_contract.functions.send_to_evm_chain(
            ADDRS["signer1"], amount, 1, 0, b""
        ).build_transaction({"from": ADDRS["community"]})
        txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
        assert txreceipt.status == 1, "should success"

        def check():
            nonce = gravity.contract.caller.state_lastRevertedNonce()
            return old_nonce + 1 == nonce

        wait_for_fn("send-to-ethereum", check)

        # check that voucher has been created
        voucher = gravity.contract.caller.state_RevertedVouchers(old_nonce)
        assert voucher[0] == erc20.address
        assert voucher[1] == ADDRS["signer1"]
        assert voucher[2] == amount

        # check balance is the same
        new_balance1 = erc20.caller.balanceOf(ADDRS["signer1"])
        assert old_balance1 == new_balance1

        old_balance2 = erc20.caller.balanceOf(ADDRS["signer2"])

        # try to redeem voucher with non recipient address
        with pytest.raises(Exception):
            gravity.contract.functions.redeemVoucher(
                old_nonce, ADDRS["signer2"]
            ).build_transaction({"from": ADDRS["validator"]})

        # send user1 some fund for gas
        send_transaction(
            geth, {"to": ADDRS["signer1"], "value": 10**17}, KEYS["validator"]
        )
        # redeem voucher
        tx = gravity.contract.functions.redeemVoucher(
            old_nonce, ADDRS["signer2"]
        ).build_transaction({"from": ADDRS["signer1"]})
        txreceipt = send_transaction(geth, tx, KEYS["signer1"])
        assert txreceipt.status == 1, "should success"
        w3_wait_for_new_blocks(geth, 1)
        new_balance2 = erc20.caller.balanceOf(ADDRS["signer2"])
        assert old_balance2 + amount == new_balance2

        # asset cannot redeem twice
        with pytest.raises(Exception):
            gravity.contract.functions.redeemVoucher(
                old_nonce, ADDRS["signer2"]
            ).build_transaction({"from": ADDRS["signer1"]})


def test_gravity_turn_bridge(gravity):
    geth = gravity.geth
    cli = gravity.cronos.cosmos_cli()
    cronos_w3 = gravity.cronos.w3

    # deploy test erc20 contract
    erc20 = deploy_contract(
        geth,
        CONTRACTS["TestERC20A"],
    )

    balance = erc20.caller.balanceOf(ADDRS["validator"])
    assert balance == 100000000000000000000000000
    amount = 1000

    print("send to cronos crc20")
    recipient = HexBytes(ADDRS["community"])
    txreceipt = send_to_cosmos(
        gravity.contract, erc20, geth, recipient, amount, KEYS["validator"]
    )
    assert txreceipt.status == 1, "should success"
    assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

    denom = f"gravity{erc20.address}"

    def check_gravity_native_tokens():
        "check the balance of gravity native token"
        return cli.balance(eth_to_bech32(recipient), denom=denom) == amount

    if gravity.cronos.enable_auto_deployment:
        crc21_contract = None

        def local_check_auto_deployment():
            nonlocal crc21_contract
            crc21_contract = check_auto_deployment(
                cli, denom, cronos_w3, recipient, amount
            )
            return crc21_contract

        wait_for_fn("send-to-crc21", local_check_auto_deployment)
    else:
        wait_for_fn("send-to-gravity-native", check_gravity_native_tokens)

    # turn off bridge
    rsp = cli.turn_bridge("false", from_="community")
    assert rsp["code"] != 0, "should not have the permission"

    rsp = cli.turn_bridge("false", from_="validator")
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)

    if gravity.cronos.enable_auto_deployment:
        # send it back to erc20, should fail
        tx = crc21_contract.functions.send_to_evm_chain(
            ADDRS["validator"], amount, 1, 0, b""
        ).build_transaction({"from": ADDRS["community"]})
        txreceipt = send_transaction(cronos_w3, tx, KEYS["community"])
        assert txreceipt.status == 0, "should fail"
    else:
        # send back the gravity native tokens, should fail
        rsp = cli.send_to_ethereum(
            ADDRS["validator"], f"{amount}{denom}", f"0{denom}", from_="community"
        )
        assert rsp["code"] == 3, rsp["raw_log"]

    wait_for_new_blocks(cli, 10)
    # check no new batch is created
    rsp = cli.query_batches()
    assert len(rsp["batches"]) == 0

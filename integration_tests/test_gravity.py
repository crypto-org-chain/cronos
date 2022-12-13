import json

import pytest
import sha3
import toml
from dateutil.parser import isoparse
from eth_account.account import Account
from eth_utils import abi, to_checksum_address
from hexbytes import HexBytes
from pystarport import ports

from .gorc import GoRc
from .network import GravityBridge, setup_cronos_experimental, setup_geth
from .utils import (
    ACCOUNTS,
    ADDRS,
    CONTRACTS,
    KEYS,
    add_ini_sections,
    deploy_contract,
    deploy_erc20,
    dump_toml,
    eth_to_bech32,
    get_contract,
    multiple_send_to_cosmos,
    parse_events,
    send_to_cosmos,
    send_transaction,
    supervisorctl,
    w3_wait_for_new_blocks,
    wait_for_block_time,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gravity

Account.enable_unaudited_hdwallet_features()


def cronos_crc21_abi():
    path = CONTRACTS["ModuleCRC21"]
    return json.load(path.open())["abi"]


def gorc_config(keystore, gravity_contract, eth_rpc, cosmos_grpc, metrics_listen):
    return {
        "keystore": str(keystore),
        "gravity": {
            "contract": gravity_contract,
            "fees_denom": "basetcro",
        },
        "ethereum": {
            "key_derivation_path": "m/44'/60'/0'/0/0",
            "rpc": eth_rpc,
        },
        "cosmos": {
            "gas_price": {
                "amount": 5000000000000,
                "denom": "basetcro",
            },
            "gas_limit": 500000,
            "grpc": cosmos_grpc,
            "key_derivation_path": "m/44'/60'/0'/0/0",
            "prefix": "crc",
        },
        "metrics": {
            "listen_addr": metrics_listen,
        },
    }


def update_gravity_contract(tomlfile, contract):
    with open(tomlfile) as fp:
        obj = toml.load(fp)
    obj["gravity"]["contract"] = contract
    tomlfile.write_text(dump_toml(obj))


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
    if crc21_contract.caller.balanceOf(recipient) == amount:
        return crc21_contract
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
def geth(tmp_path_factory):
    "start-geth"
    for network in setup_geth(tmp_path_factory.mktemp("geth"), 8555):
        yield network.w3


@pytest.fixture(scope="module", params=[True, False])
def cronos(request, tmp_path_factory):
    """start-cronos
    params: enable_auto_deployment
    """
    yield from setup_cronos_experimental(
        tmp_path_factory.mktemp("cronos_experimental"), 26600, request.param
    )


@pytest.fixture(scope="module")
def gravity(cronos, geth):
    """
    - set-delegator-keys
    - deploy gravity contract
    - start orchestrator
    """
    chain_id = "cronos_777-1"

    # set-delegate-keys
    for i, val in enumerate(cronos.config["validators"]):
        # generate gorc config file
        gorc_config_path = cronos.base_dir / f"node{i}/gorc.toml"
        grpc_port = ports.grpc_port(val["base_port"])
        metrics_port = 3000 + i
        gorc_config_path.write_text(
            dump_toml(
                gorc_config(
                    cronos.base_dir / f"node{i}/orchestrator_keystore",
                    "",  # to be filled later after the gravity contract deployed
                    geth.provider.endpoint_uri,
                    f"http://localhost:{grpc_port}",
                    f"127.0.0.1:{metrics_port}",
                )
            )
        )

        gorc = GoRc(gorc_config_path)

        # generate new accounts on both chain
        gorc.add_eth_key("eth")
        gorc.add_eth_key("cronos")  # cronos and eth key derivation are the same

        # fund the orchestrator accounts
        eth_addr = to_checksum_address(gorc.show_eth_addr("eth"))
        print("fund 0.1 eth to address", eth_addr)
        send_transaction(geth, {"to": eth_addr, "value": 10**17}, KEYS["validator"])
        acc_addr = gorc.show_cosmos_addr("cronos")
        print("fund 100cro to address", acc_addr)
        rsp = cronos.cosmos_cli().transfer(
            "community", acc_addr, "%dbasetcro" % (100 * (10**18))
        )
        assert rsp["code"] == 0, rsp["raw_log"]

        cli = cronos.cosmos_cli(i)
        val_addr = cli.address("validator", bech="val")
        val_acct_addr = cli.address("validator")
        nonce = int(cli.account(val_acct_addr)["base_account"]["sequence"])
        signature = gorc.sign_validator("eth", val_addr, nonce)
        rsp = cli.set_delegate_keys(
            val_addr, acc_addr, eth_addr, HexBytes(signature).hex(), from_=val_acct_addr
        )
        assert rsp["code"] == 0, rsp["raw_log"]
    # wait for gravity signer tx get generated
    wait_for_new_blocks(cli, 2)

    # create admin account and fund it
    admin, _ = Account.create_with_mnemonic()
    print("fund 0.1 eth to address", admin.address)
    send_transaction(geth, {"to": admin.address, "value": 10**17}, KEYS["validator"])

    # deploy gravity contract to geth
    gravity_id = cli.query_gravity_params()["params"]["gravity_id"]
    signer_set = cli.query_latest_signer_set_tx()["signer_set"]["signers"]
    powers = [int(signer["power"]) for signer in signer_set]
    threshold = int(2**32 * 0.66)  # gravity normalize the power to [0, 2**32]
    eth_addresses = [signer["ethereum_address"] for signer in signer_set]
    assert sum(powers) >= threshold, "not enough validator on board"

    contract = deploy_contract(
        geth,
        CONTRACTS["Gravity"],
        (gravity_id.encode(), threshold, eth_addresses, powers, admin.address),
    )
    print("gravity contract deployed", contract.address)

    # make all the orchestrator "Relayer" roles
    k_relayer = sha3.keccak_256()
    k_relayer.update(b"RELAYER")
    for _, address in enumerate(eth_addresses):
        set_role_tx = contract.functions.grantRole(
            k_relayer.hexdigest(), address
        ).build_transaction({"from": admin.address})
        set_role_receipt = send_transaction(geth, set_role_tx, admin.key)
        print("set_role_tx", set_role_receipt)

    # start orchestrator:
    # a) add process into the supervisord config file
    # b) reload supervisord
    programs = {}
    for i, val in enumerate(cronos.config["validators"]):
        # update gravity contract in gorc config
        gorc_config_path = cronos.base_dir / f"node{i}/gorc.toml"
        update_gravity_contract(gorc_config_path, contract.address)

        programs[f"program:{chain_id}-orchestrator{i}"] = {
            "command": (
                f'gorc -c "{gorc_config_path}" orchestrator start '
                "--cosmos-key cronos --ethereum-key eth --mode AlwaysRelay"
            ),
            "environment": "RUST_BACKTRACE=full",
            "autostart": "true",
            "autorestart": "true",
            "startsecs": "3",
            "redirect_stderr": "true",
            "stdout_logfile": f"%(here)s/orchestrator{i}.log",
        }

    add_ini_sections(cronos.base_dir / "tasks.ini", programs)
    supervisorctl(cronos.base_dir / "../tasks.ini", "update")

    yield GravityBridge(cronos, geth, contract)


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
            send_transaction(
                geth, {"to": address, "value": 10**17}, KEYS["validator"]
            )
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
            "send-to-gravity-native", check_gravity_balance, timeout=400, interval=1
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
    }
    proposal.write_text(json.dumps(proposal_src))
    return cli.submit_gov_proposal(proposal, from_="community")


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

    # get proposal_id
    ev = parse_events(rsp["logs"])["submit_proposal"]
    proposal_id = ev["proposal_id"]
    print("gov proposal submitted", proposal_id)

    # not sure why, but sometimes can't find the proposal immediatelly
    wait_for_new_blocks(cli, 1)
    proposal = cli.query_proposal(proposal_id)

    # each validator vote yes
    for i in range(len(gravity.cronos.config["validators"])):
        rsp = gravity.cronos.cosmos_cli(i).gov_vote("validator", proposal_id, "yes")
        assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)
    assert (
        int(cli.query_tally(proposal_id)["yes_count"]) == cli.staking_pool()
    ), "all validators should have voted yes"
    print("wait for proposal to be activated")
    wait_for_block_time(cli, isoparse(proposal["voting_end_time"]))
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
        # deploy crc21 contract
        w3 = gravity.cronos.w3
        contract = deploy_contract(w3, CONTRACTS["TestERC21Source"])

        # setup the contract mapping
        cronos_cli = gravity.cronos.cosmos_cli()

        print("crc21 contract", contract.address)
        denom = f"cronos{contract.address}"

        print("check the contract mapping not exists yet")
        with pytest.raises(AssertionError):
            cronos_cli.query_contract_by_denom(denom)

        rsp = cronos_cli.update_token_mapping(
            denom, contract.address, "DOG", 6, from_="validator"
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        wait_for_new_blocks(cronos_cli, 1)

        print("check the contract mapping exists now")
        rsp = cronos_cli.query_denom_by_contract(contract.address)
        assert rsp["denom"] == denom

        # Create cosmos erc20 contract
        print("Deploy cosmos erc20 contract on ethereum")
        tx_receipt = deploy_erc20(
            gravity.contract, gravity.geth, denom, denom, "DOG", 6, KEYS["validator"]
        )
        assert tx_receipt.status == 1, "should success"

        # Wait enough for orchestrator to relay the event
        w3_wait_for_new_blocks(gravity.geth, 120)

        # Check mapping is done on gravity side
        cosmos_erc20 = cronos_cli.query_gravity_contract_by_denom(denom)
        print("cosmos_erc20:", cosmos_erc20)
        assert cosmos_erc20 != ""
        cosmos_erc20_contract = get_contract(
            gravity.geth, cosmos_erc20["erc20"], CONTRACTS["TestERC21Source"]
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

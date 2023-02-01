import pytest
import sha3
from eth_account.account import Account
from eth_utils import to_checksum_address
from hexbytes import HexBytes
from pystarport import ports

from .gorc import GoRc
from .network import GravityBridge, setup_cronos_experimental, setup_geth
from .test_gravity import gorc_config, update_gravity_contract
from .utils import (
    ADDRS,
    CONTRACTS,
    KEYS,
    add_ini_sections,
    deploy_contract,
    dump_toml,
    eth_to_bech32,
    send_to_cosmos,
    send_transaction,
    supervisorctl,
    wait_for_fn,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gravity

Account.enable_unaudited_hdwallet_features()


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


def test_gravity_proxy_contract(gravity):
    if not gravity.cronos.enable_auto_deployment:
        geth = gravity.geth

        # deploy test erc20 contract
        erc20 = deploy_contract(
            geth,
            CONTRACTS["TestERC20A"],
        )
        balance = erc20.caller.balanceOf(ADDRS["validator"])
        assert balance == 100000000000000000000000000

        denom = f"gravity{erc20.address}"

        # deploy crc20 contract
        w3 = gravity.cronos.w3
        crc20 = deploy_contract(w3, CONTRACTS["TestCRC20"])

        print("crc20 contract deployed at address: ", crc20.address)

        # setup the contract mapping
        cronos_cli = gravity.cronos.cosmos_cli()

        print("check the contract mapping not exists yet")
        with pytest.raises(AssertionError):
            cronos_cli.query_contract_by_denom(denom)

        rsp = cronos_cli.update_token_mapping(
            denom, crc20.address, "TEST", 18, from_="validator"
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        wait_for_new_blocks(cronos_cli, 1)

        print("check the contract mapping exists now")
        rsp = cronos_cli.query_denom_by_contract(crc20.address)
        assert rsp["denom"] == denom

        # Send some tokens
        print("send to cronos crc20")
        amount = 1000
        recipient = HexBytes(ADDRS["community"])
        txreceipt = send_to_cosmos(
            gravity.contract, erc20, geth, recipient, amount, KEYS["validator"]
        )
        assert txreceipt.status == 1, "should success"
        assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

        def check_gravity_tokens():
            "check the balance of gravity native token"
            return crc20.caller.balanceOf(ADDRS["community"]) == amount

        wait_for_fn("check_gravity_tokens", check_gravity_tokens)

        # deploy crc20 proxy contract
        proxycrc20 = deploy_contract(
            w3,
            CONTRACTS["TestCRC20Proxy"],
            (crc20.address, False),
        )

        print("proxycrc20 contract deployed at address: ", proxycrc20.address)
        assert not proxycrc20.caller.is_source()
        assert proxycrc20.caller.crc20() == crc20.address

        # change token mapping
        rsp = cronos_cli.update_token_mapping(
            denom, proxycrc20.address, "DOG", 18, from_="validator"
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        wait_for_new_blocks(cronos_cli, 1)

        print("check the contract mapping exists now")
        rsp = cronos_cli.query_denom_by_contract(proxycrc20.address)
        assert rsp["denom"] == denom

        # Fund the proxy contract cosmos account with original supply
        # by sending tokens to dead address
        # (because mint to zero address is forbidden in ERC20 contract)
        print("restore original supply crc20 by sending token to dead address")
        amount = 1000
        balance = erc20.caller.balanceOf(ADDRS["validator"])
        dead_address = "0x000000000000000000000000000000000000dEaD"
        cosmos_dead_address = HexBytes(dead_address)
        txreceipt = send_to_cosmos(
            gravity.contract,
            erc20,
            geth,
            cosmos_dead_address,
            amount,
            KEYS["validator"],
        )
        assert txreceipt.status == 1, "should success"
        assert erc20.caller.balanceOf(ADDRS["validator"]) == balance - amount

        def check_dead_gravity_tokens():
            "check the balance of gravity token"
            return crc20.caller.balanceOf(dead_address) == amount

        wait_for_fn("check_dead_gravity_tokens", check_dead_gravity_tokens)

        # Try to send back token to ethereum
        amount = 500
        ethereum_receiver = ADDRS["validator2"]
        # community_balance_before_send = crc20.caller.balanceOf(community)
        balance_before_send_to_ethereum = erc20.caller.balanceOf(ethereum_receiver)

        print("send to ethereum")
        # First we need to approve the proxy contract to move asset
        tx = crc20.functions.approve(proxycrc20.address, amount).build_transaction(
            {"from": ADDRS["community"]}
        )
        txreceipt = send_transaction(w3, tx, key=KEYS["community"])
        assert txreceipt.status == 1, "should success"
        assert crc20.caller.allowance(ADDRS["community"], proxycrc20.address) == amount

        # Then trigger the send to evm chain
        sender = ADDRS["community"]
        community_balance_before_send = crc20.caller.balanceOf(sender)
        print(
            "sender address : ",
        )
        tx2 = proxycrc20.functions.send_to_evm_chain(
            ethereum_receiver, amount, 1, 0, b""
        ).build_transaction({"from": ADDRS["community"]})
        txreceipt2 = send_transaction(w3, tx2, key=KEYS["community"])
        print("receipt : ", txreceipt2)
        assert txreceipt2.status == 1, "should success"
        # Check deduction
        assert crc20.caller.balanceOf(sender) == community_balance_before_send - amount

        balance_after_send_to_ethereum = balance_before_send_to_ethereum

        def check_ethereum_balance_change():
            nonlocal balance_after_send_to_ethereum
            balance_after_send_to_ethereum = erc20.caller.balanceOf(ethereum_receiver)
            print("balance dead address", crc20.caller.balanceOf(dead_address))
            return balance_before_send_to_ethereum != balance_after_send_to_ethereum

        wait_for_fn(
            "ethereum balance change", check_ethereum_balance_change, timeout=60
        )
        assert (
            balance_after_send_to_ethereum == balance_before_send_to_ethereum + amount
        )


def test_gravity_detect_malicious_supply(gravity):
    if not gravity.cronos.enable_auto_deployment:
        geth = gravity.geth
        cli = gravity.cronos.cosmos_cli()

        # deploy fake contract to trigger the malicious supply
        # any transfer made with this contract will send an amount of token
        # equal to max uint256
        erc20 = deploy_contract(
            geth,
            CONTRACTS["TestMaliciousSupply"],
        )
        denom = f"gravity{erc20.address}"
        print(denom)

        # check that the bridge is activated
        activate = cli.query_gravity_params()["params"]["bridge_active"]
        assert activate is True

        max_int = 2**256 - 1
        print("send max_int to community address using gravity bridge")
        recipient = HexBytes(ADDRS["community"])
        txreceipt = send_to_cosmos(
            gravity.contract, erc20, geth, recipient, max_int, KEYS["validator"]
        )
        assert txreceipt.status == 1, "should success"

        # check that amount has been received
        def check_gravity_native_tokens():
            "check the balance of gravity native token"
            return cli.balance(eth_to_bech32(recipient), denom=denom) == max_int

        wait_for_fn("balance", check_gravity_native_tokens)

        # check that the bridge is still activated
        activate = cli.query_gravity_params()["params"]["bridge_active"]
        assert activate is True

        # need a random transferFrom to increment the counter in the contract
        # (see logic) to be able to redo a max uint256 transfer
        print("do a random send to increase contract nonce")
        txtransfer = erc20.functions.transferFrom(
            ADDRS["validator"], ADDRS["validator2"], 1
        ).build_transaction({"from": ADDRS["validator"]})
        txreceipt = send_transaction(geth, txtransfer, KEYS["validator"])
        assert txreceipt.status == 1, "should success"

        print("send again max_int to community address using gravity bridge")
        txreceipt = send_to_cosmos(
            gravity.contract, erc20, geth, recipient, max_int, KEYS["validator"]
        )
        assert txreceipt.status == 1, "should success"

        # Wait enough for orchestrator to relay the event
        wait_for_new_blocks(cli, 30)

        # check that the bridge has not been desactivated
        activate = cli.query_gravity_params()["params"]["bridge_active"]
        assert activate is True

        # check that balance is still same
        assert cli.balance(eth_to_bech32(recipient), denom=denom) == max_int

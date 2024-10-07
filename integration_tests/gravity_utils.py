import toml
from eth_account.account import Account
from eth_hash.auto import keccak
from eth_utils import to_checksum_address
from hexbytes import HexBytes
from pystarport import ports

from .gorc import GoRc
from .network import GravityBridge
from .utils import (
    CONTRACTS,
    KEYS,
    add_ini_sections,
    deploy_contract,
    deploy_erc20,
    dump_toml,
    get_contract,
    send_transaction,
    wait_for_new_blocks,
)


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
            "msg_batch_size": 10,
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


def prepare_gravity(custom_cronos, custom_geth):
    """
    - set-delegator-keys
    - deploy gravity contract
    - start orchestrator
    """
    chain_id = "cronos_777-1"
    w3 = custom_geth.w3
    # set-delegate-keys
    for i, val in enumerate(custom_cronos.config["validators"]):
        # generate gorc config file
        gorc_config_path = custom_cronos.base_dir / f"node{i}/gorc.toml"
        grpc_port = ports.grpc_port(val["base_port"])
        metrics_port = 3000 + i
        gorc_config_path.write_text(
            dump_toml(
                gorc_config(
                    custom_cronos.base_dir / f"node{i}/orchestrator_keystore",
                    "",  # to be filled later after the gravity contract deployed
                    w3.provider.endpoint_uri,
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
        send_transaction(w3, {"to": eth_addr, "value": 10**17}, KEYS["validator"])
        acc_addr = gorc.show_cosmos_addr("cronos")
        print("fund 100cro to address", acc_addr)
        rsp = custom_cronos.cosmos_cli().transfer(
            "community", acc_addr, "%dbasetcro" % (100 * (10**18))
        )
        assert rsp["code"] == 0, rsp["raw_log"]

        cli = custom_cronos.cosmos_cli(i)
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
    send_transaction(w3, {"to": admin.address, "value": 10**17}, KEYS["validator"])

    # deploy gravity contract to geth
    gravity_id = cli.query_gravity_params()["gravity_id"]
    signer_set = cli.query_latest_signer_set_tx()["signer_set"]["signers"]
    powers = [int(signer["power"]) for signer in signer_set]
    threshold = int(2**32 * 0.66)  # gravity normalize the power to [0, 2**32]
    eth_addresses = [signer["ethereum_address"] for signer in signer_set]
    assert sum(powers) >= threshold, "not enough validator on board"

    contract = deploy_contract(
        w3,
        CONTRACTS["Gravity"],
        (gravity_id.encode(), threshold, eth_addresses, powers, admin.address),
    )
    print("gravity contract deployed", contract.address)

    # make all the orchestrator "Relayer" roles
    k_relayer = keccak.new(b"RELAYER")
    for _, address in enumerate(eth_addresses):
        set_role_tx = contract.functions.grantRole(
            k_relayer.digest().hex(), address
        ).build_transaction({"from": admin.address})
        set_role_receipt = send_transaction(w3, set_role_tx, admin.key)
        print("set_role_tx", set_role_receipt)

    # start orchestrator:
    # a) add process into the supervisord config file
    # b) reload supervisord
    programs = {}
    for i, val in enumerate(custom_cronos.config["validators"]):
        # update gravity contract in gorc config
        gorc_config_path = custom_cronos.base_dir / f"node{i}/gorc.toml"
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

    add_ini_sections(custom_cronos.base_dir / "tasks.ini", programs)
    custom_cronos.supervisorctl("update")

    yield GravityBridge(custom_cronos, w3, contract)


def setup_cosmos_erc20_contract(cluster, denom, symbol):
    # Create cosmos erc20 contract
    print("Deploy cosmos erc20 contract on ethereum")
    tx_receipt = deploy_erc20(
        cluster.contract, cluster.geth, denom, denom, symbol, 6, KEYS["validator"]
    )
    assert tx_receipt.status == 1, "should success"
    # Wait enough for orchestrator to relay the event
    cronos_cli = cluster.cronos.cosmos_cli()
    wait_for_new_blocks(cronos_cli, 30)
    # Check mapping is done on cluster side
    cosmos_erc20 = cronos_cli.query_gravity_contract_by_denom(denom)
    print("cosmos_erc20:", cosmos_erc20)
    assert cosmos_erc20 != ""
    cosmos_erc20_contract = get_contract(
        cluster.geth, cosmos_erc20["erc20"], CONTRACTS["CosmosERC20"]
    )
    return cosmos_erc20_contract

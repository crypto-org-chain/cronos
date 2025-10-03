import base64
import configparser
import hashlib
import json
import os
import re
import socket
import subprocess
import sys
import time
from collections import defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from decimal import Decimal
from pathlib import Path

import bech32
import eth_utils
import pytest
import requests
import rlp
import toml
from dateutil.parser import isoparse
from dotenv import load_dotenv
from eth_account import Account
from eth_utils import abi, to_checksum_address
from hexbytes import HexBytes
from pystarport import ledger
from web3._utils.contracts import abi_to_signature, find_matching_event_abi
from web3._utils.events import get_event_data
from web3._utils.method_formatters import receipt_formatter
from web3._utils.transactions import fill_nonce, fill_transaction_defaults
from web3.datastructures import AttributeDict

load_dotenv(Path(__file__).parent.parent / "scripts/.env")
Account.enable_unaudited_hdwallet_features()
ACCOUNTS = {
    "validator": Account.from_mnemonic(os.getenv("VALIDATOR1_MNEMONIC")),
    "validator2": Account.from_mnemonic(os.getenv("VALIDATOR2_MNEMONIC")),
    "validator3": Account.from_mnemonic(os.getenv("VALIDATOR3_MNEMONIC")),
    "community": Account.from_mnemonic(os.getenv("COMMUNITY_MNEMONIC")),
    "signer1": Account.from_mnemonic(os.getenv("SIGNER1_MNEMONIC")),
    "signer2": Account.from_mnemonic(os.getenv("SIGNER2_MNEMONIC")),
}
KEYS = {name: account.key for name, account in ACCOUNTS.items()}
ADDRS = {name: account.address for name, account in ACCOUNTS.items()}
CRONOS_ADDRESS_PREFIX = "crc"
TEST_CONTRACTS = {
    "Gravity": "Gravity.sol",
    "Greeter": "Greeter.sol",
    "TestERC20A": "TestERC20A.sol",
    "TestERC21Source": "TestERC21Source.sol",
    "TestRevert": "TestRevert.sol",
    "TestERC20Utility": "TestERC20Utility.sol",
    "TestMessageCall": "TestMessageCall.sol",
    "TestBlackListERC20": "TestBlackListERC20.sol",
    "CroBridge": "CroBridge.sol",
    "CronosGravityCancellation": "CronosGravityCancellation.sol",
    "TestCRC20": "TestCRC20.sol",
    "TestCRC20Proxy": "TestCRC20Proxy.sol",
    "TestMaliciousSupply": "TestMaliciousSupply.sol",
    "CosmosERC20": "CosmosToken.sol",
    "TestBank": "TestBank.sol",
    "TestICA": "TestICA.sol",
    "Random": "Random.sol",
    "TestRelayer": "TestRelayer.sol",
}


def contract_path(name, filename):
    return (
        Path(__file__).parent
        / "contracts/artifacts/contracts"
        / filename
        / (name + ".json")
    )


CONTRACTS = {
    "ModuleCRC20": Path(__file__).parent.parent
    / "x/cronos/types/contracts/ModuleCRC20.json",
    "ModuleCRC21": Path(__file__).parent.parent
    / "x/cronos/types/contracts/ModuleCRC21.json",
    "ModuleCRC20Proxy": Path(__file__).parent.parent
    / "x/cronos/types/contracts/ModuleCRC20Proxy.json",
    **{
        name: contract_path(name, filename) for name, filename in TEST_CONTRACTS.items()
    },
}

CONTRACT_ABIS = {
    "IRelayerModule": Path(__file__).parent.parent / "build/IRelayerModule.abi",
    "IICAModule": Path(__file__).parent.parent / "build/IICAModule.abi",
}


def wait_for_fn(name, fn, *, timeout=240, interval=1):
    for i in range(int(timeout / interval)):
        result = fn()
        if result:
            return result
        time.sleep(interval)
    else:
        raise TimeoutError(f"wait for {name} timeout")


def get_sync_info(s):
    return s.get("SyncInfo") or s.get("sync_info")


def wait_for_block(cli, height, timeout=240):
    for i in range(timeout * 2):
        try:
            status = cli.status()
        except AssertionError as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            current_height = int(get_sync_info(status)["latest_block_height"])
            print("current block height", current_height)
            if current_height >= height:
                break
        time.sleep(0.5)
    else:
        raise TimeoutError(f"wait for block {height} timeout")


def wait_for_new_blocks(cli, n, sleep=0.5, timeout=240):
    cur_height = begin_height = int(get_sync_info(cli.status())["latest_block_height"])
    start_time = time.time()
    while cur_height - begin_height < n:
        time.sleep(sleep)
        cur_height = int(get_sync_info(cli.status())["latest_block_height"])
        if time.time() - start_time > timeout:
            raise TimeoutError(f"wait for block {begin_height + n} timeout")
    return cur_height


def wait_for_block_time(cli, t):
    print("wait for block time", t)
    while True:
        now = isoparse(get_sync_info(cli.status())["latest_block_time"])
        print("block time now:", now)
        if now >= t:
            break
        time.sleep(0.5)


def get_proposal_id(rsp, msg="/cosmos.staking.v1beta1.MsgUpdateParams"):
    def cb(attrs):
        return "proposal_id" in attrs

    ev = find_log_event_attrs(rsp["events"], "submit_proposal", cb)
    assert ev["proposal_messages"] == "," + msg, rsp
    return ev["proposal_id"]


def approve_proposal(
    n,
    events,
    vote_option="yes",
    msg="/cosmos.gov.v1.MsgExecLegacyContent",
    wait_tx=True,
    broadcast_mode="sync",
):
    cli = n.cosmos_cli()

    # get proposal_id
    ev = find_log_event_attrs(
        events, "submit_proposal", lambda attrs: "proposal_id" in attrs
    )
    proposal_id = ev["proposal_id"]
    proposal = cli.query_proposal(proposal_id)
    if msg == "/cosmos.gov.v1.MsgExecLegacyContent":
        assert proposal["status"] == "PROPOSAL_STATUS_DEPOSIT_PERIOD", proposal
    rsp = cli.gov_deposit(
        "community",
        proposal_id,
        "100000000basetcro",
        event_query_tx=wait_tx,
        broadcast_mode=broadcast_mode,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    proposal = cli.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_VOTING_PERIOD", proposal

    if vote_option is not None:
        for i in range(len(n.config["validators"])):
            rsp = n.cosmos_cli(i).gov_vote(
                "validator",
                proposal_id,
                vote_option,
                event_query_tx=wait_tx,
                broadcast_mode=broadcast_mode,
            )
            assert rsp["code"] == 0, rsp["raw_log"]

        wait_for_new_blocks(cli, 1)
        assert (
            int(cli.query_tally(proposal_id)[vote_option + "_count"])
            == cli.staking_pool()
        ), "all voted"
    else:
        assert cli.query_tally(proposal_id) == {
            "yes_count": "0",
            "no_count": "0",
            "abstain_count": "0",
            "no_with_veto_count": "0",
        }

    wait_for_block_time(cli, isoparse(proposal["voting_end_time"]))
    proposal = cli.query_proposal(proposal_id)
    if vote_option == "yes":
        assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal
    else:
        assert proposal["status"] == "PROPOSAL_STATUS_REJECTED", proposal


def submit_gov_proposal(cronos, msg, **kwargs):
    proposal_json = {
        "title": "title",
        "summary": "summary",
        "deposit": "1basetcro",
        **kwargs,
    }
    rsp = cronos.cosmos_cli().submit_gov_proposal(
        "community", "submit-proposal", proposal_json, broadcast_mode="sync"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cronos, rsp["events"], msg=msg)
    print("proposal has passed now")


def wait_for_port(port, host="127.0.0.1", timeout=40.0):
    start_time = time.perf_counter()
    while True:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                break
        except OSError as ex:
            time.sleep(0.1)
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    "Waited too long for the port {} on host {} to start accepting "
                    "connections.".format(port, host)
                ) from ex


def wait_for_ipc(path, timeout=40.0):
    print("wait for unix socket", path, "to be available")
    start_time = time.perf_counter()
    while True:
        if os.path.exists(path):
            break
        time.sleep(0.1)
        if time.perf_counter() - start_time >= timeout:
            raise TimeoutError(
                "Waited too long for the unix socket {path} to be available"
            )


def w3_wait_for_block(w3, height, timeout=240):
    for _ in range(timeout * 2):
        try:
            current_height = w3.eth.block_number
        except Exception as e:
            print(f"get json-rpc block number failed: {e}", file=sys.stderr)
        else:
            if current_height >= height:
                break
            print("current block height", current_height)
        time.sleep(0.5)
    else:
        raise TimeoutError(f"wait for block {height} timeout")


def w3_wait_for_new_blocks(w3, n):
    begin_height = w3.eth.block_number
    while True:
        time.sleep(0.5)
        cur_height = w3.eth.block_number
        if cur_height - begin_height >= n:
            break


def get_ledger():
    return ledger.Ledger()


def find_log_event_attrs(events, ev_type, cond=None):
    for ev in events:
        if ev["type"] == ev_type:
            attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
            if cond is None or cond(attrs):
                return attrs
    return None


def decode_base64(raw):
    try:
        return base64.b64decode(raw.encode()).decode()
    except Exception:
        return raw


def parse_events_rpc(events):
    result = defaultdict(dict)
    for ev in events:
        for attr in ev["attributes"]:
            if attr["key"] is None:
                continue
            key = decode_base64(attr["key"])
            if attr["value"] is not None:
                value = decode_base64(attr["value"])
            else:
                value = None
            result[ev["type"]][key] = value
    return result


_next_unique = 0


def gen_base_port(worker_index):
    global _next_unique
    base_port = 10000 + (worker_index * 10 + _next_unique) * 100
    _next_unique += 1
    return base_port


def sign_single_tx_with_options(cli, tx_file, singer_name, **options):
    return json.loads(
        cli.cosmos_cli(0).raw(
            "tx",
            "sign",
            tx_file,
            from_=singer_name,
            home=cli.cosmos_cli(0).data_dir,
            keyring_backend="test",
            chain_id=cli.cosmos_cli(0).chain_id,
            node=cli.cosmos_cli(0).node_rpc,
            **options,
        )
    )


def find_balance(balances, denom):
    "find a denom in the coin list, return the amount, if not exists, return 0"
    for balance in balances:
        if balance["denom"] == denom:
            return int(balance["amount"])
    return 0


class ContractAddress(rlp.Serializable):
    fields = [
        ("from", rlp.sedes.Binary()),
        ("nonce", rlp.sedes.big_endian_int),
    ]


def contract_address(addr, nonce):
    return eth_utils.to_checksum_address(
        eth_utils.to_hex(
            eth_utils.keccak(
                rlp.encode(ContractAddress(eth_utils.to_bytes(hexstr=addr), nonce))
            )[12:]
        )
    )


def decode_bech32(addr):
    _, bz = bech32.bech32_decode(addr)
    return HexBytes(bytes(bech32.convertbits(bz, 5, 8)))


def bech32_to_eth(addr):
    return decode_bech32(addr).hex()


def eth_to_bech32(addr, prefix=CRONOS_ADDRESS_PREFIX):
    bz = bech32.convertbits(HexBytes(addr), 8, 5)
    return bech32.bech32_encode(prefix, bz)


def add_ini_sections(inipath, sections):
    ini = configparser.RawConfigParser()
    ini.read(inipath)
    for name, value in sections.items():
        ini.add_section(name)
        ini[name].update(value)
    with inipath.open("w") as fp:
        ini.write(fp)


def edit_ini_sections(chain_id, ini_path, callback):
    ini = configparser.RawConfigParser()
    ini.read(ini_path)
    reg = re.compile(rf"^program:{chain_id}-node(\d+)")
    for section in ini.sections():
        m = reg.match(section)
        if m:
            i = m.group(1)
            old = ini[section]
            ini[section].update(callback(i, old))
    with ini_path.open("w") as fp:
        ini.write(fp)


def supervisorctl(inipath, *args):
    return subprocess.check_output(
        (sys.executable, "-msupervisor.supervisorctl", "-c", inipath, *args),
    ).decode()


def deploy_contract(w3, jsonfile, args=(), key=KEYS["validator"], exp_gas_used=None):
    """
    deploy contract and return the deployed contract instance
    """
    acct = Account.from_key(key)
    info = json.loads(jsonfile.read_text())
    bytecode = ""
    if "bytecode" in info:
        bytecode = info["bytecode"]
    if "byte" in info:
        bytecode = info["byte"]
    contract = w3.eth.contract(abi=info["abi"], bytecode=bytecode)
    tx = contract.constructor(*args).build_transaction({"from": acct.address})
    txreceipt = send_transaction(w3, tx, key)
    assert txreceipt.status == 1
    if exp_gas_used is not None:
        assert (
            exp_gas_used == txreceipt.gasUsed
        ), f"exp {exp_gas_used}, got {txreceipt.gasUsed}"
    address = txreceipt.contractAddress
    return w3.eth.contract(address=address, abi=info["abi"])


def get_contract(w3, address, jsonfile):
    """
    get contract from address and abi
    """
    info = json.loads(jsonfile.read_text())
    return w3.eth.contract(address=address, abi=info["abi"])


def sign_transaction(w3, tx, key=KEYS["validator"]):
    "fill default fields and sign"
    acct = Account.from_key(key)
    tx["from"] = acct.address
    tx = fill_transaction_defaults(w3, tx)
    tx = fill_nonce(w3, tx)
    return acct.sign_transaction(tx)


def get_account_nonce(w3, key=KEYS["validator"]):
    acct = Account.from_key(key)
    return w3.eth.get_transaction_count(acct.address)


def send_transaction(w3, tx, key=KEYS["validator"]):
    signed = sign_transaction(w3, tx, key)
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    return w3.eth.wait_for_transaction_receipt(txhash)


def replace_transaction(w3, old_tx, new_tx, key=KEYS["validator"]):
    signed = sign_transaction(w3, old_tx, key)
    old_tx_hash = w3.eth.send_raw_transaction(signed.rawTransaction)
    new_txhash = w3.eth.replace_transaction(old_tx_hash, new_tx)
    return w3.eth.wait_for_transaction_receipt(new_txhash)


def cronos_address_from_mnemonics(mnemonics, prefix=CRONOS_ADDRESS_PREFIX):
    "return cronos address from mnemonics"
    acct = Account.from_mnemonic(mnemonics)
    return eth_to_bech32(acct.address, prefix)


def derive_new_account(n=1):
    # derive a new address
    account_path = f"m/44'/60'/0'/0/{n}"
    mnemonic = os.getenv("COMMUNITY_MNEMONIC")
    return Account.from_mnemonic(mnemonic, account_path=account_path)


def send_to_cosmos(gravity_contract, token_contract, w3, recipient, amount, key=None):
    """
    do approve and sendToCronos on ethereum side
    """
    acct = Account.from_key(key)
    txreceipt = send_transaction(
        w3,
        token_contract.functions.approve(
            gravity_contract.address, amount
        ).build_transaction({"from": acct.address}),
        key,
    )
    assert txreceipt.status == 1, "approve failed"

    return send_transaction(
        w3,
        gravity_contract.functions.sendToCronos(
            token_contract.address, HexBytes(recipient), amount
        ).build_transaction({"from": acct.address}),
        key,
    )


def deploy_erc20(gravity_contract, w3, denom, name, symbol, decimal, key=None):
    acct = Account.from_key(key)

    return send_transaction(
        w3,
        gravity_contract.functions.deployERC20(
            denom, name, symbol, decimal
        ).build_transaction({"from": acct.address}),
        key,
    )


class InlineTable(dict, toml.decoder.InlineTableDict):
    "a hack to dump inline table with toml library"

    pass


def dump_toml(obj):
    return toml.dumps(obj, encoder=toml.TomlPreserveInlineDictEncoder())


class Contract:
    "General contract."

    def __init__(self, contract_path, private_key=KEYS["validator"], chain_id=777):
        self.chain_id = chain_id
        self.account = Account.from_key(private_key)
        self.address = self.account.address
        self.private_key = private_key
        with open(contract_path) as f:
            json_data = f.read()
            contract_json = json.loads(json_data)
        self.bytecode = contract_json["bytecode"]
        self.abi = contract_json["abi"]
        self.contract = None
        self.w3 = None

    def deploy(self, w3):
        "Deploy Greeter contract on `w3` and return the receipt."
        if self.contract is None:
            self.w3 = w3
            contract = self.w3.eth.contract(abi=self.abi, bytecode=self.bytecode)
            transaction = contract.constructor().build_transaction(
                {"chainId": self.chain_id, "from": self.address}
            )
            receipt = send_transaction(self.w3, transaction, self.private_key)
            self.contract = self.w3.eth.contract(
                address=receipt.contractAddress, abi=self.abi
            )
            return receipt
        else:
            return receipt


class Greeter(Contract):
    "Greeter contract."

    def transfer(self, string):
        "Call contract on `w3` and return the receipt."
        transaction = self.contract.functions.setGreeting(string).build_transaction(
            {
                "chainId": self.chain_id,
                "from": self.address,
            }
        )
        receipt = send_transaction(self.w3, transaction, self.private_key)
        assert string == self.contract.functions.greet().call()
        return receipt


class RevertTestContract(Contract):
    "RevertTestContract contract."

    def transfer(self, value):
        "Call contract on `w3` and return the receipt."
        transaction = self.contract.functions.transfer(value).build_transaction(
            {
                "chainId": self.chain_id,
                "from": self.address,
                "gas": 100000,  # skip estimateGas error
            }
        )
        receipt = send_transaction(self.w3, transaction, self.private_key)
        return receipt


def modify_command_in_supervisor_config(ini: Path, fn, **kwargs):
    "replace the first node with the instrumented binary"
    ini.write_text(
        re.sub(
            r"^command = (cronosd .*$)",
            lambda m: f"command = {fn(m.group(1))}",
            ini.read_text(),
            flags=re.M,
            **kwargs,
        )
    )


def build_batch_tx(w3, cli, txs, key=KEYS["validator"]):
    "return cosmos batch tx and eth tx hashes"
    signed_txs = [sign_transaction(w3, tx, key) for tx in txs]
    tmp_txs = [cli.build_evm_tx(signed.rawTransaction.hex()) for signed in signed_txs]

    msgs = [tx["body"]["messages"][0] for tx in tmp_txs]
    fee = sum(int(tx["auth_info"]["fee"]["amount"][0]["amount"]) for tx in tmp_txs)
    gas_limit = sum(int(tx["auth_info"]["fee"]["gas_limit"]) for tx in tmp_txs)

    tx_hashes = [signed.hash for signed in signed_txs]

    # build batch cosmos tx
    return {
        "body": {
            "messages": msgs,
            "memo": "",
            "timeout_height": "0",
            "extension_options": [
                {"@type": "/ethermint.evm.v1.ExtensionOptionsEthereumTx"}
            ],
            "non_critical_extension_options": [],
        },
        "auth_info": {
            "signer_infos": [],
            "fee": {
                "amount": [{"denom": "basetcro", "amount": str(fee)}],
                "gas_limit": str(gas_limit),
                "payer": "",
                "granter": "",
            },
        },
        "signatures": [],
    }, tx_hashes


def get_receipts_by_block(w3, blk):
    if isinstance(blk, int):
        blk = hex(blk)
    rsp = w3.provider.make_request("cronos_getTransactionReceiptsByBlock", [blk])
    if "error" not in rsp:
        rsp["result"] = [
            AttributeDict(receipt_formatter(item)) for item in rsp["result"]
        ]
    return rsp


def send_raw_transactions(w3, raw_transactions):
    with ThreadPoolExecutor(len(raw_transactions)) as exec:
        tasks = [
            exec.submit(w3.eth.send_raw_transaction, raw) for raw in raw_transactions
        ]
        sended_hash_set = {future.result() for future in as_completed(tasks)}
    return sended_hash_set


def send_txs(w3, cli, to, keys, params):
    tx = {"to": to, "value": 10000} | params
    # use different sender accounts to be able be send concurrently
    raw_transactions = []
    for key_from in keys:
        signed = sign_transaction(w3, tx, key_from)
        raw_transactions.append(signed.rawTransaction)

    # wait block update
    block_num_0 = wait_for_new_blocks(cli, 1, sleep=0.1)
    print(f"block number start: {block_num_0}")

    # send transactions
    sended_hash_set = send_raw_transactions(w3, raw_transactions)

    return block_num_0, sended_hash_set


def multiple_send_to_cosmos(gcontract, tcontract, w3, recipient, amount, keys):
    # use different sender accounts to be able be send concurrently
    raw_transactions = []
    for key_from in keys:
        acct = Account.from_key(key_from)
        acct_address = HexBytes(acct.address)
        # approve first
        approve = tcontract.functions.approve(gcontract.address, amount)
        txreceipt = send_transaction(
            w3,
            approve.build_transaction({"from": acct.address}),
            key_from,
        )
        assert txreceipt.status == 1, "approve failed"

        # generate the tx
        tx = gcontract.functions.sendToCronos(
            tcontract.address, HexBytes(recipient), amount
        ).build_transaction({"from": acct_address})
        signed = sign_transaction(w3, tx, key_from)
        raw_transactions.append(signed.rawTransaction)

    # wait for new block
    w3_wait_for_new_blocks(w3, 1)
    return send_raw_transactions(w3, raw_transactions)


def setup_token_mapping(cronos, name, symbol):
    # deploy contract
    w3 = cronos.w3
    contract = deploy_contract(w3, CONTRACTS[name])

    # setup the contract mapping
    cronos_cli = cronos.cosmos_cli()

    print("contract", contract.address)
    denom = f"cronos{contract.address}"
    balance = contract.caller.balanceOf(ADDRS["validator"])
    assert balance == 100000000000000000000000000

    print("check the contract mapping not exists yet")
    with pytest.raises(AssertionError):
        cronos_cli.query_contract_by_denom(denom)

    rsp = cronos_cli.update_token_mapping(
        denom, contract.address, symbol, 6, from_="validator"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cronos_cli, 1)

    print("check the contract mapping exists now")
    rsp = cronos_cli.query_denom_by_contract(contract.address)
    assert rsp["denom"] == denom
    return contract, denom


def module_address(name):
    data = hashlib.sha256(name.encode()).digest()[:20]
    return to_checksum_address(decode_bech32(eth_to_bech32(data)).hex())


def submit_any_proposal(cronos):
    # governance module account as granter
    cli = cronos.cosmos_cli()
    granter_addr = "crc10d07y265gmmuvt4z0w9aw880jnsr700jdufnyd"
    grantee_addr = cli.address("signer1")

    msg = "/cosmos.feegrant.v1beta1.MsgGrantAllowance"
    proposal_json = {
        "title": "title",
        "summary": "summary",
        "deposit": "1basetcro",
        "messages": [
            {
                "@type": msg,
                "granter": granter_addr,
                "grantee": grantee_addr,
                "allowance": {
                    "@type": "/cosmos.feegrant.v1beta1.BasicAllowance",
                    "spend_limit": [],
                    "expiration": None,
                },
            }
        ],
    }

    rsp = cli.submit_gov_proposal(
        "community", "submit-proposal", proposal_json, broadcast_mode="sync"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(cronos, rsp["events"], msg=msg)
    grant_detail = cli.query_grant(granter_addr, grantee_addr)
    assert grant_detail["granter"] == granter_addr
    assert grant_detail["grantee"] == grantee_addr


def get_method_map(contract_info, by_name=False):
    method_map = {}
    for item in contract_info:
        if item["type"] != "event":
            continue
        event_abi = find_matching_event_abi(contract_info, item["name"])
        signature = abi_to_signature(event_abi)
        key = f"0x{abi.event_signature_to_log_topic(signature).hex()}"
        if by_name:
            name = signature.split("(")[0]
            method_map[name] = key
        else:
            method_map[key] = signature
    return method_map


def get_topic_data(w3, method_map, contract_info, log):
    method = method_map[log.topics[0].hex()]
    name = method.split("(")[0]
    event_abi = find_matching_event_abi(contract_info, name)
    event_data = get_event_data(w3.codec, event_abi, log)
    return name, event_data.args


def get_logs_since(w3, addr, start):
    end = w3.eth.get_block_number()
    return w3.eth.get_logs(
        {
            "fromBlock": start,
            "toBlock": end,
            "address": [addr],
        }
    )


def get_consensus_params(port, height):
    url = f"http://127.0.0.1:{port}/consensus_params?height={height}"
    return requests.get(url).json()["result"]["consensus_params"]


def get_send_enable(port):
    url = f"http://127.0.0.1:{port}/cosmos/bank/v1beta1/params"
    raw = requests.get(url).json()
    return raw["params"]["send_enabled"]


def get_expedited_params(param):
    min_deposit = param["min_deposit"][0]
    voting_period = param["voting_period"]
    tokens_ratio = 5
    threshold_ratio = 1.334
    period_ratio = 0.5
    expedited_threshold = float(param["threshold"]) * threshold_ratio
    expedited_threshold = Decimal(f"{expedited_threshold}")
    expedited_voting_period = int(int(voting_period[:-1]) * period_ratio)
    return {
        "expedited_min_deposit": [
            {
                "denom": min_deposit["denom"],
                "amount": str(int(min_deposit["amount"]) * tokens_ratio),
            }
        ],
        "expedited_threshold": f"{expedited_threshold:.18f}",
        "expedited_voting_period": f"{expedited_voting_period}s",
    }


def assert_gov_params(cli, old_param):
    param = cli.query_params("gov")
    expedited_param = get_expedited_params(old_param)
    for key, value in expedited_param.items():
        assert param[key] == value, param


def fund_acc(w3, acc, fund=3000000000000000000):
    addr = acc.address
    if w3.eth.get_balance(addr, "latest") == 0:
        tx = {"to": addr, "value": fund, "gasPrice": w3.eth.gas_price}
        send_transaction(w3, tx)
        assert w3.eth.get_balance(addr, "latest") == fund


def remove_cancun_prague_params(cronos):
    from .cosmoscli import module_address as cosmos_module_address

    cli = cronos.cosmos_cli()
    p = cli.query_params("evm")
    del p["chain_config"]["cancun_time"]
    del p["chain_config"]["prague_time"]
    authority = cosmos_module_address("gov")
    msg = "/ethermint.evm.v1.MsgUpdateParams"
    submit_gov_proposal(
        cronos,
        msg,
        messages=[
            {
                "@type": msg,
                "authority": authority,
                "params": p,
            }
        ],
    )
    p = cli.query_params("evm")
    assert not p["chain_config"]["cancun_time"]
    assert not p["chain_config"]["prague_time"]

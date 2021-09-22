import configparser
import datetime
import json
import os
import re
import shutil
import socket
import subprocess
import sys
import time
import uuid

import bech32
import eth_utils
import rlp
import yaml
from cprotobuf import Field, ProtoEntity
from dateutil.parser import isoparse
from eth_account import Account
from eth_account.messages import encode_defunct
from hexbytes import HexBytes
from pystarport import cluster, ledger
from pystarport.ports import rpc_port
from web3._utils.transactions import fill_nonce, fill_transaction_defaults

KEYS = {
    "validator": "826E479F5385C8C32CD96B0C0ACCDB8CC4FA5CACCC1BE54C1E3AA4D676A6EFF5",
    "community": "5D665FBD2FB40CB8E9849263B04457BA46D5F948972D0FE4C1F19B6B0F243574",
}
ADDRS = {name: Account.from_key(key).address for name, key in KEYS.items()}
CRONOS_ADDRESS_PREFIX = "crc"


def wait_for_fn(name, fn, *, timeout=120, interval=1):
    for i in range(int(timeout / interval)):
        result = fn()
        print("check", name, result)
        if result:
            break
        time.sleep(interval)
    else:
        raise TimeoutError(f"wait for {name} timeout")


def wait_for_block(cli, height, timeout=240):
    for i in range(timeout * 2):
        try:
            status = cli.status()
        except AssertionError as e:
            print(f"get sync status failed: {e}", file=sys.stderr)
        else:
            current_height = int(status["SyncInfo"]["latest_block_height"])
            if current_height >= height:
                break
            print("current block height", current_height)
        time.sleep(0.5)
    else:
        raise TimeoutError(f"wait for block {height} timeout")


def wait_for_new_blocks(cli, n):
    begin_height = int((cli.status())["SyncInfo"]["latest_block_height"])
    while True:
        time.sleep(0.5)
        cur_height = int((cli.status())["SyncInfo"]["latest_block_height"])
        if cur_height - begin_height >= n:
            break


def wait_for_block_time(cli, t):
    print("wait for block time", t)
    while True:
        now = isoparse((cli.status())["SyncInfo"]["latest_block_time"])
        print("block time now:", now)
        if now >= t:
            break
        time.sleep(0.5)


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


def cluster_fixture(
    config_path,
    worker_index,
    data,
    quiet=False,
    post_init=None,
    enable_cov=None,
    cmd=None,
):
    """
    init a single devnet
    """
    if enable_cov is None:
        enable_cov = os.environ.get("GITHUB_ACTIONS") == "true"
    base_port = gen_base_port(worker_index)
    print("init cluster at", data, ", base port:", base_port)
    cluster.init_cluster(data, config_path, base_port, cmd=cmd)

    config = yaml.safe_load(open(config_path))
    clis = {}
    for key in config:
        if key == "relayer":
            continue

        chain_id = key
        chain_data = data / chain_id

        if post_init:
            post_init(chain_id, chain_data)

        if enable_cov:
            # replace the first node with the instrumented binary
            ini = chain_data / cluster.SUPERVISOR_CONFIG_FILE
            ini.write_text(
                re.sub(
                    r"^command = (.*/)?chain-maind",
                    "command = chain-maind-inst "
                    "-test.coverprofile=%(here)s/coverage.txt",
                    ini.read_text(),
                    count=1,
                    flags=re.M,
                )
            )
        clis[chain_id] = cluster.ClusterCLI(data, chain_id=chain_id)

    supervisord = cluster.start_cluster(data)
    if not quiet:
        tailer = cluster.start_tail_logs_thread(data)

    try:
        begin = time.time()
        for cli in clis.values():
            # wait for first node rpc port available before start testing
            wait_for_port(rpc_port(cli.config["validators"][0]["base_port"]))
            # wait for the first block generated before start testing
            wait_for_block(cli, 2)

        if len(clis) == 1:
            yield list(clis.values())[0]
        else:
            yield clis

        if enable_cov:
            # wait for server startup complete to generate the coverage report
            duration = time.time() - begin
            if duration < 15:
                time.sleep(15 - duration)
    finally:
        supervisord.terminate()
        supervisord.wait()
        if not quiet:
            tailer.stop()
            tailer.join()

    if enable_cov:
        # collect the coverage results
        try:
            shutil.move(
                str(chain_data / "coverage.txt"), f"coverage.{uuid.uuid1()}.txt"
            )
        except FileNotFoundError:
            ts = time.time()
            st = datetime.datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M:%S")
            print(st + " FAILED TO FIND COVERAGE")
            print(os.listdir(chain_data))
            data = [
                (int(p), c)
                for p, c in [
                    x.rstrip("\n").split(" ", 1)
                    for x in os.popen("ps h -eo pid:1,command")
                ]
            ]
            print(data)


def get_ledger():
    return ledger.Ledger()


def parse_events(logs):
    return {
        ev["type"]: {attr["key"]: attr["value"] for attr in ev["attributes"]}
        for ev in logs[0]["events"]
    }


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


class DelegateKeysSignMsg(ProtoEntity):
    validator_address = Field("string", 1)
    nonce = Field("uint64", 2)


def sign_validator(acct, val_addr, nonce):
    if nonce > 0:
        msg = DelegateKeysSignMsg(validator_address=val_addr, nonce=nonce)
    else:
        msg = DelegateKeysSignMsg(validator_address=val_addr)
    sign_bytes = eth_utils.keccak(msg.SerializeToString())
    signed = acct.sign_message(encode_defunct(sign_bytes))
    return eth_utils.to_hex(signed.signature)


def add_ini_sections(inipath, sections):
    ini = configparser.RawConfigParser()
    ini.read_file(inipath.open())
    for name, value in sections.items():
        ini.add_section(name)
        ini[name].update(value)
    with inipath.open("w") as fp:
        ini.write(fp)


def supervisorctl(inipath, *args):
    subprocess.run(
        (sys.executable, "-msupervisor.supervisorctl", "-c", inipath, *args),
        check=True,
    )


def deploy_contract(w3, jsonfile, args=(), key=KEYS["validator"]):
    """
    deploy contract and return the deployed contract instance
    """
    acct = Account.from_key(key)
    info = json.load(open(jsonfile))
    contract = w3.eth.contract(abi=info["abi"], bytecode=info["bytecode"])
    tx = contract.constructor(*args).buildTransaction({"from": acct.address})
    txreceipt = send_transaction(w3, tx, key)
    assert txreceipt.status == 1
    address = txreceipt.contractAddress
    return w3.eth.contract(address=address, abi=info["abi"])


def send_transaction(w3, tx, key):
    acct = Account.from_key(key)
    tx["from"] = acct.address
    tx = fill_transaction_defaults(w3, tx)
    tx = fill_nonce(w3, tx)
    signed = acct.sign_transaction(tx)
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    return w3.eth.wait_for_transaction_receipt(txhash)


def cronos_address_from_mnemonics(mnemonics, prefix=CRONOS_ADDRESS_PREFIX):
    "return cronos address from mnemonics"
    acct = Account.from_mnemonic(mnemonics)
    return eth_to_bech32(acct.address, prefix)

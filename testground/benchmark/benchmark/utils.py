import json
import socket
import time
from pathlib import Path

import bech32
import jsonmerge
import requests
import tomlkit
import web3
from eth_account import Account
from hexbytes import HexBytes
from web3._utils.transactions import fill_nonce, fill_transaction_defaults

DEFAULT_DENOM = "basecro"
CRONOS_ADDRESS_PREFIX = "crc"
LOCAL_RPC = "http://127.0.0.1:26657"
LOCAL_JSON_RPC = "http://127.0.0.1:8545"


def patch_toml_doc(doc, patch):
    for k, v in patch.items():
        if isinstance(v, dict):
            patch_toml_doc(doc.setdefault(k, {}), v)
        else:
            doc[k] = v


def patch_toml(path: Path, patch):
    doc = tomlkit.parse(path.read_text())
    patch_toml_doc(doc, patch)
    path.write_text(tomlkit.dumps(doc))
    return doc


_merger = jsonmerge.Merger(
    {
        "properties": {
            "app_state": {
                "properties": {
                    "auth": {"properties": {"accounts": {"mergeStrategy": "append"}}},
                    "evm": {"properties": {"accounts": {"mergeStrategy": "append"}}},
                }
            }
        }
    }
)


def merge_genesis(base, head):
    return _merger.merge(base, head)


def patch_genesis(path: Path, patch):
    doc = json.loads(path.read_text())
    doc = merge_genesis(doc, patch)
    path.write_text(json.dumps(doc))
    return doc


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


def wait_for_block(cli, target: int, timeout=40):
    height = -1
    for i in range(timeout):
        status = json.loads(cli("status", output="json"))
        height = int(status["sync_info"]["latest_block_height"])

        if height >= target:
            break

        time.sleep(1)
    else:
        raise TimeoutError(
            f"Waited too long for block {target} to be reached. "
            f"Current height: {height}"
        )

    return height


def wait_for_w3(timeout=40):
    for i in range(timeout):
        try:
            w3 = web3.Web3(web3.providers.HTTPProvider(LOCAL_JSON_RPC))
            w3.eth.get_balance("0x0000000000000000000000000000000000000001")
        except:  # noqa
            time.sleep(1)
            continue

        break
    else:
        raise TimeoutError("Waited too long for web3 json-rpc to be ready.")


def decode_bech32(addr):
    _, bz = bech32.bech32_decode(addr)
    return HexBytes(bytes(bech32.convertbits(bz, 5, 8)))


def bech32_to_eth(addr):
    return decode_bech32(addr).hex()


def eth_to_bech32(addr, prefix=CRONOS_ADDRESS_PREFIX):
    bz = bech32.convertbits(HexBytes(addr), 8, 5)
    return bech32.bech32_encode(prefix, bz)


def sign_transaction(w3, tx, acct):
    "fill default fields and sign"
    tx["from"] = acct.address
    tx = fill_transaction_defaults(w3, tx)
    tx = fill_nonce(w3, tx)
    return acct.sign_transaction(tx)


def send_transaction(w3, tx, acct, wait=True):
    signed = sign_transaction(w3, tx, acct)
    txhash = w3.eth.send_raw_transaction(signed.rawTransaction)
    if wait:
        return w3.eth.wait_for_transaction_receipt(txhash)
    return txhash


def send_transactions(w3, txs, acct, wait=True):
    """
    send a batch of transactions from same account
    """
    signed_txs = [sign_transaction(w3, tx, acct) for tx in txs]
    txhashes = [
        w3.eth.send_raw_transaction(signed.rawTransaction) for signed in signed_txs
    ]
    if wait:
        return [w3.eth.wait_for_transaction_receipt(txhash) for txhash in txhashes]
    return txhashes


def export_eth_account(cli, name: str, **kwargs) -> Account:
    kwargs.setdefault("keyring_backend", "test")
    return Account.from_key(cli("keys", "unsafe-export-eth-key", name, **kwargs))


def gen_account(global_seq: int, index: int) -> Account:
    """
    deterministically generate test private keys,
    index 0 is reserved for validator account.
    """
    return Account.from_key(((global_seq + 1) << 32 | index).to_bytes(32))


def block_height(rpc=LOCAL_RPC):
    rsp = requests.get(f"{rpc}/status").json()
    return int(rsp["result"]["sync_info"]["latest_block_height"])


def block(height, rpc=LOCAL_RPC):
    return requests.get(f"{rpc}/block?height={height}").json()


def block_eth(height: int, json_rpc=LOCAL_JSON_RPC):
    return requests.post(
        json_rpc,
        json={
            "jsonrpc": "2.0",
            "method": "eth_getBlockByNumber",
            "params": [hex(height), False],
            "id": 1,
        },
    ).json()["result"]


def block_txs(height, rpc=LOCAL_RPC):
    return block(height, rpc=rpc)["result"]["block"]["data"]["txs"]


def split(a: int, n: int):
    """
    Split range(0, a) into n parts
    """
    k, m = divmod(a, n)
    return [(i * k + min(i, m), (i + 1) * k + min(i + 1, m)) for i in range(n)]


def split_batch(a: int, size: int):
    """
    Split range(0, a) into batches with size
    """
    if size < 1:
        size = 1

    k, m = divmod(a, size)
    parts = [(i * size, (i + 1) * size) for i in range(k)]
    if m:
        parts.append((k * size, a))
    return parts


class Tee:
    def __init__(self, f1, f2):
        self.f1 = f1
        self.f2 = f2

    def write(self, s) -> int:
        s1 = self.f1.write(s)
        s2 = self.f2.write(s)
        assert s1 == s2
        return s1

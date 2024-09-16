import json
import socket
import time
from pathlib import Path

import bech32
import tomlkit
import web3
from eth_account import Account
from hexbytes import HexBytes
from web3._utils.transactions import fill_nonce, fill_transaction_defaults

CRONOS_ADDRESS_PREFIX = "crc"


def patch_dict(doc, kwargs):
    for k, v in kwargs.items():
        keys = k.split(".")
        assert len(keys) > 0
        cur = doc
        for section in keys[:-1]:
            cur = cur[section]
        cur[keys[-1]] = v


def patch_toml(path: Path, kwargs):
    doc = tomlkit.parse(path.read_text())
    patch_dict(doc, kwargs)
    path.write_text(tomlkit.dumps(doc))
    return doc


def patch_json(path: Path, kwargs):
    doc = json.loads(path.read_text())
    patch_dict(doc, kwargs)
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
            w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
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

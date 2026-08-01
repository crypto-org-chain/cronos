import time

import bech32
import requests
import web3
from eth_account import Account
from hexbytes import HexBytes

DEFAULT_DENOM = "basecro"
CRONOS_ADDRESS_PREFIX = "crc"

# Every RPC helper here is called in per-height loops across every endpoint
# (divergence checks issue blocks x nodes requests), so an unbounded read on a
# single hung node would stall the whole run.
HTTP_TIMEOUT_S = 10


def decode_bech32(addr):
    _, bz = bech32.bech32_decode(addr)
    return HexBytes(bytes(bech32.convertbits(bz, 5, 8)))


def bech32_to_eth(addr):
    return decode_bech32(addr).hex()


def eth_to_bech32(addr, prefix=CRONOS_ADDRESS_PREFIX):
    bz = bech32.convertbits(HexBytes(addr), 8, 5)
    return bech32.bech32_encode(prefix, bz)


def gen_account(global_seq: int, index: int) -> Account:
    """
    deterministically generate test private keys,
    index 0 is reserved for the funding account.
    """
    return Account.from_key(((global_seq + 1) << 32 | index).to_bytes(32))


def status(rpc):
    return requests.get(f"{rpc}/status", timeout=HTTP_TIMEOUT_S).json()


def node_id(rpc):
    return status(rpc)["result"]["node_info"]["id"]


def block_height(rpc):
    return int(status(rpc)["result"]["sync_info"]["latest_block_height"])


def block(height, rpc):
    return requests.get(f"{rpc}/block?height={height}", timeout=HTTP_TIMEOUT_S).json()


def abci_info(rpc):
    return requests.get(f"{rpc}/abci_info", timeout=HTTP_TIMEOUT_S).json()


def block_eth(height: int, json_rpc):
    return requests.post(
        json_rpc,
        json={
            "jsonrpc": "2.0",
            "method": "eth_getBlockByNumber",
            "params": [hex(height), False],
            "id": 1,
        },
        timeout=HTTP_TIMEOUT_S,
    ).json()["result"]


def eth_block_number(json_rpc) -> int:
    rsp = requests.post(
        json_rpc,
        json={"jsonrpc": "2.0", "method": "eth_blockNumber", "params": [], "id": 1},
        timeout=HTTP_TIMEOUT_S,
    ).json()
    return int(rsp["result"], 16)


def block_results(height, rpc):
    return requests.get(
        f"{rpc}/block_results?height={height}", timeout=HTTP_TIMEOUT_S
    ).json()


def net_info(rpc):
    return requests.get(f"{rpc}/net_info", timeout=HTTP_TIMEOUT_S).json()


def mempool_status(rpc):
    """Return (n_txs, total_bytes) from CometBFT's unconfirmed txs endpoint."""
    rsp = requests.get(f"{rpc}/num_unconfirmed_txs", timeout=HTTP_TIMEOUT_S).json()
    r = rsp.get("result", {})
    return int(r.get("n_txs", 0)), int(r.get("total_bytes", 0))


def block_txs(height, rpc):
    return block(height, rpc=rpc)["result"]["block"]["data"]["txs"]


def wait_for_w3(json_rpc, timeout=40):
    w3 = web3.Web3(web3.providers.HTTPProvider(json_rpc))
    for _ in range(timeout):
        try:
            w3.eth.get_balance("0x0000000000000000000000000000000000000001")
            return w3
        except Exception:  # noqa
            time.sleep(1)
    raise TimeoutError(f"Waited too long for web3 json-rpc {json_rpc} to be ready.")


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

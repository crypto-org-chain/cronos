import itertools
import queue
import sys
import threading
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
HTTP_RETRIES = 3

# A wedged SSH-tunneled loopback socket has been observed to sit in poll()
# well past requests' own `timeout=`, which only re-arms on each individual
# recv - it doesn't bound the call as a whole. A dedicated thread per call
# plus a wall-clock deadline on the result queue is the actual backstop: a
# fresh connection on retry reliably succeeds even while the old thread's
# socket stays stuck forever (and is abandoned). This must NOT run on a
# bounded shared pool - an abandoned worker never returns its slot, so a
# shared pool eventually saturates with permanently-stuck workers and every
# later call (even ones that would've succeeded on a fresh connection) queues
# forever behind them. A leaked daemon thread per stuck call is cheap; a
# starved shared pool reproduces the exact stall this is meant to fix.
def _with_deadline(fn, deadline=HTTP_TIMEOUT_S):
    result = queue.Queue(maxsize=1)

    def runner():
        try:
            result.put((True, fn()))
        except Exception as exc:  # noqa
            result.put((False, exc))

    threading.Thread(target=runner, daemon=True).start()
    try:
        ok, value = result.get(timeout=deadline)
    except queue.Empty:
        raise TimeoutError(f"RPC call exceeded {deadline}s deadline")
    if not ok:
        raise value
    return value


_session_lock = threading.Lock()
_sessions = {}


def _get_session(base_url):
    """One persistent connection-pooled Session per tunnel base URL.

    A bare requests.get/post opens a fresh TCP connection every call, so
    each one pays full slow-start ramp-up - measured over the VPN path here
    at ~1 Mbit/s climbing to ~18 Mbit/s over 10s. Keep-alive reuse avoids
    re-paying that ramp on every poll.
    """
    with _session_lock:
        session = _sessions.get(base_url)
        if session is None:
            session = requests.Session()
            # _with_deadline abandons (never cancels) a stuck call's thread,
            # so a stuck request keeps its pooled connection checked out
            # forever. block=False (default) means an exhausted pool just
            # opens an unpooled connection rather than hanging, but a low
            # maxsize would throw away keep-alive reuse for most calls once
            # a few stack up stuck - size well above what one node's traffic
            # needs so exhaustion is rare, not the default block-free escape
            # hatch.
            adapter = requests.adapters.HTTPAdapter(pool_maxsize=50)
            session.mount("http://", adapter)
            session.mount("https://", adapter)
            _sessions[base_url] = session
        return session


_pick_lock = threading.Lock()
_pick_cycles = {}


def _pick(candidates):
    """Round-robin across `candidates` on every call (not just retries), so
    steady-state churn is spread across all of them before any one connection
    gets overloaded - a retry then also naturally lands on a different
    tunnel most of the time, for free."""
    if len(candidates) == 1:
        return candidates[0]
    with _pick_lock:
        cycle = _pick_cycles.setdefault(candidates, itertools.cycle(candidates))
        return next(cycle)


def request_json(method, rpc, path, retries=HTTP_RETRIES, **kwargs):
    """requests.get/post `f"{base}{path}"`, retrying with backoff on timeout
    or any request error - the deadline is wall-clock (see _with_deadline),
    not requests' own unreliable `timeout=`.

    `rpc` is either a single base URL or a list of candidate base URLs (all
    reaching the same node, e.g. via independent SSH tunnels); each attempt
    round-robins across the candidates via `_pick`.
    """
    candidates = tuple(rpc) if isinstance(rpc, (list, tuple)) else (rpc,)
    verb = method.__name__  # "get" or "post"
    last_exc = None
    for attempt in range(retries):
        base = _pick(candidates)
        call = getattr(_get_session(base), verb)
        url = f"{base}{path}"
        try:
            return _with_deadline(
                lambda: call(url, timeout=HTTP_TIMEOUT_S, **kwargs)
            ).json()
        except (TimeoutError, requests.exceptions.RequestException) as exc:
            last_exc = exc
            if attempt < retries - 1:
                time.sleep(min(2**attempt, 5))
    raise last_exc


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
    return request_json(requests.get, rpc, "/status")


def node_id(rpc):
    return status(rpc)["result"]["node_info"]["id"]


def block_height(rpc):
    return int(status(rpc)["result"]["sync_info"]["latest_block_height"])


def block(height, rpc):
    return request_json(requests.get, rpc, f"/block?height={height}")


def abci_info(rpc):
    return request_json(requests.get, rpc, "/abci_info")


def block_eth(height: int, json_rpc):
    return request_json(
        requests.post,
        json_rpc,
        "",
        json={
            "jsonrpc": "2.0",
            "method": "eth_getBlockByNumber",
            "params": [hex(height), False],
            "id": 1,
        },
    )["result"]


def eth_block_number(json_rpc) -> int:
    rsp = request_json(
        requests.post,
        json_rpc,
        "",
        json={"jsonrpc": "2.0", "method": "eth_blockNumber", "params": [], "id": 1},
    )
    return int(rsp["result"], 16)


def block_results(height, rpc):
    return request_json(requests.get, rpc, f"/block_results?height={height}")


def net_info(rpc):
    return request_json(requests.get, rpc, "/net_info")


def mempool_status(rpc):
    """Return (n_txs, total_bytes) from CometBFT's unconfirmed txs endpoint."""
    rsp = request_json(requests.get, rpc, "/num_unconfirmed_txs")
    r = rsp.get("result", {})
    return int(r.get("n_txs", 0)), int(r.get("total_bytes", 0))


def block_txs(height, rpc):
    return block(height, rpc=rpc)["result"]["block"]["data"]["txs"]


# CometBFT caps /blockchain at this many block_metas per call regardless of
# the requested span, so a wide range needs this many round trips chunked.
BLOCKCHAIN_PAGE_SIZE = 20


# A page whose request_json call had to retry (wedged tunnel, backoff sleep)
# takes several seconds instead of tens of milliseconds. A range spanning
# many pages can silently rack up minutes of retry cost between the caller's
# own progress prints, which reads identically to a hang - so surface any
# single page slow enough to indicate a retry happened.
SLOW_PAGE_THRESHOLD_S = 2.0


def blockchain_range(min_height, max_height, rpc):
    """Fetch {height: (num_txs, time)} for [min_height, max_height] via
    CometBFT's /blockchain endpoint, chunked by BLOCKCHAIN_PAGE_SIZE, instead
    of one /block call per height."""
    metas = {}
    lo = min_height
    while lo <= max_height:
        hi = min(lo + BLOCKCHAIN_PAGE_SIZE - 1, max_height)
        started = time.monotonic()
        rsp = request_json(requests.get, rpc, f"/blockchain?minHeight={lo}&maxHeight={hi}")
        elapsed = time.monotonic() - started
        if elapsed >= SLOW_PAGE_THRESHOLD_S:
            print(
                f"blockchain page {lo}-{hi} took {elapsed:.1f}s (likely retried)",
                file=sys.stderr,
            )
        for meta in rsp["result"]["block_metas"]:
            header = meta["header"]
            metas[int(header["height"])] = (int(meta["num_txs"]), header["time"])
        lo = hi + 1
    return metas


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

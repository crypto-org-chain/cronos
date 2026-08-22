import asyncio
import base64
import hashlib
import itertools
import multiprocessing
import os
import sys
import time
from collections import namedtuple

import aiohttp
import backoff
import eth_abi
import ujson
from eth_account._utils.legacy_transactions import Transaction
from hexbytes import HexBytes

from . import cosmostx
from .contracts import NFT_ADDRESS, POOL_ADDRESS
from .erc20 import CONTRACT_ADDRESS
from .utils import DEFAULT_DENOM, gen_account, split, split_batch

GAS_PRICE = 1000000000
CHAIN_ID = 777
# raised from 1024: with send_batch_size=8000, a lower cap queued requests at
# the connector instead of the node, which just moved the bottleneck client-side.
CONNECTION_POOL_SIZE = 10000
# Default per-host cap for tunneled runs, where a large send_batch_size can
# open hundreds/thousands of connections through a single SSH tunnel at once
# and crash the ssh -L process outright (observed with send_batch_size=4000,
# 3-way tunnel pool, ~1300 concurrent opens per tunnel). Callers on direct
# loopback endpoints, with no tunnel to protect, can override via
# Config.send_conn_per_host.
CONNECTION_POOL_PER_HOST = 200
PROGRESS_INTERVAL_S = 3


def _send_session(conn_per_host, n_hosts):
    """Session whose total limit never binds below the per-host aggregate -
    otherwise CONNECTION_POOL_SIZE could cap concurrency below what
    conn_per_host * n_hosts is meant to allow."""
    connector = aiohttp.TCPConnector(
        limit=max(CONNECTION_POOL_SIZE, conn_per_host * n_hosts),
        limit_per_host=conn_per_host,
    )
    return aiohttp.ClientSession(connector=connector, json_serialize=ujson.dumps)


Job = namedtuple(
    "Job",
    [
        "chunk",
        "global_seq",
        "num_txs",
        "create_tx",
        "batch",
        "nonce",
        "msg_version",
        "tx_options",
        "evm_denom",
        "wire_format",
        "sender_strategy",
        "start_account",
    ],
)
EthTx = namedtuple("EthTx", ["tx", "raw", "sender"])


ERC20_TRANSFER_SELECTOR = "0xa9059cbb"  # transfer(address,uint256)
ERC20_TRANSFER_GAS = 51630

# Fixed EOA every erc20-transfer-hot tx sends to, so every sender writes the
# same recipient balance slot - the intended contended hot spot. It's a
# plain EOA, not a contract, so it needs no genesis entry: the ERC20
# mapping slot it maps to defaults to zero until the first transfer.
HOT_RECEIVER_ADDRESS = "0x4" + "0" * 39

SWAP_SELECTOR = "0x8693ed2e"  # swap(uint112,bool)
SWAP_AMOUNT = 1000
SWAP_GAS = 51630

MINT_SELECTOR = "0x1249c58b"  # mint()
# balanceOf[sender] is cold on every mint from a fresh sender (e.g.
# sender_strategy=unique-per-tx), and totalMinted is cold on the very first
# mint - both cases pay the full SSTORE-to-nonzero cost.
MINT_GAS = 80000


def simple_transfer_tx(sender: str, nonce: int, options: dict):
    return {
        "to": sender,
        "value": 1,
        "nonce": nonce,
        "gas": 21000,
        "gasPrice": options.get("gas_price", GAS_PRICE),
        "chainId": options.get("chain_id", CHAIN_ID),
    }


def _contract_call_tx(to: str, data: str, gas: int, nonce: int, options: dict):
    return {
        "to": to,
        "value": 0,
        "nonce": nonce,
        "gas": gas,
        "gasPrice": options.get("gas_price", GAS_PRICE),
        "chainId": options.get("chain_id", CHAIN_ID),
        "data": data,
    }


def _erc20_transfer_data(recipient: str) -> str:
    return (
        ERC20_TRANSFER_SELECTOR
        + eth_abi.encode(["address", "uint256"], [recipient, 1]).hex()
    )


def erc20_transfer_tx(sender: str, nonce: int, options: dict):
    return _contract_call_tx(
        CONTRACT_ADDRESS,
        _erc20_transfer_data(sender),
        ERC20_TRANSFER_GAS,
        nonce,
        options,
    )


def erc20_transfer_hot_tx(sender: str, nonce: int, options: dict):
    return _contract_call_tx(
        CONTRACT_ADDRESS,
        _erc20_transfer_data(HOT_RECEIVER_ADDRESS),
        ERC20_TRANSFER_GAS,
        nonce,
        options,
    )


def uniswap_swap_tx(sender: str, nonce: int, options: dict):
    zero_for_one = nonce % 2 == 0
    data = (
        SWAP_SELECTOR
        + eth_abi.encode(["uint112", "bool"], [SWAP_AMOUNT, zero_for_one]).hex()
    )
    return _contract_call_tx(POOL_ADDRESS, data, SWAP_GAS, nonce, options)


def nft_mint_tx(sender: str, nonce: int, options: dict):
    return _contract_call_tx(NFT_ADDRESS, MINT_SELECTOR, MINT_GAS, nonce, options)


def weighted_mix_tx(sender: str, nonce: int, options: dict):
    """Dispatch to one of options["mix"]'s tx types by weight.

    Picks deterministically from a hash of (sender, nonce) rather than a
    shared random.Random, since tx generation runs across multiprocessing
    workers with no shared RNG state - same (sender, nonce) always maps to
    the same tx type, regardless of worker or run.
    """
    mix = options["mix"]
    digest = hashlib.sha256(f"{sender}:{nonce}".encode()).digest()
    point = (int.from_bytes(digest[:8], "big") / 2**64) * sum(mix.values())

    cumulative = 0.0
    for name, weight in mix.items():
        cumulative += weight
        if point < cumulative:
            return TX_TYPES[name](sender, nonce, options)
    # float rounding can leave point at or past the final cumulative sum
    return TX_TYPES[list(mix)[-1]](sender, nonce, options)


TX_TYPES = {
    "simple-transfer": simple_transfer_tx,
    "erc20-transfer": erc20_transfer_tx,
    "erc20-transfer-hot": erc20_transfer_hot_tx,
    "uniswap-swap": uniswap_swap_tx,
    "nft-mint": nft_mint_tx,
    "weighted-mix": weighted_mix_tx,
}


def build_evm_msg_1_3(tx: EthTx):
    """
    build cronos v1.3 version of MsgEthereumTx
    """
    txn = Transaction.from_bytes(tx.raw)
    return cosmostx.build_any(
        cosmostx.MsgEthereumTx.MSG_URL,
        cosmostx.MsgEthereumTx(
            data=cosmostx.build_any(
                cosmostx.LegacyTx.MSG_URL,
                cosmostx.LegacyTx(
                    nonce=txn.nonce,
                    gas_price=str(txn.gasPrice),
                    gas=txn.gas,
                    to=txn.to.hex(),
                    value=str(txn.value),
                    data=txn.data,
                    v=txn.v.to_bytes(32, byteorder="big"),
                    r=txn.r.to_bytes(32, byteorder="big"),
                    s=txn.s.to_bytes(32, byteorder="big"),
                ),
            ),
            deprecated_hash=txn.hash().hex(),
            from_=tx.sender,
        ),
    )


def build_evm_msg_1_4(tx: EthTx):
    return cosmostx.build_any(
        cosmostx.MsgEthereumTx.MSG_URL,
        cosmostx.MsgEthereumTx(
            from_=tx.sender,
            raw=tx.raw,
        ),
    )


MSG_VERSIONS = {
    "1.3": build_evm_msg_1_3,
    "1.4": build_evm_msg_1_4,
}


_progress_counter = None


def _init_progress(counter):
    global _progress_counter
    _progress_counter = counter


def _do_job(job: Job):
    acct_txs = []
    total = 0
    for account_index in range(*job.chunk):
        txs = []
        acct = None
        if job.sender_strategy == "reuse":
            acct = gen_account(job.global_seq, account_index)

        for i in range(job.num_txs):
            if job.sender_strategy == "unique-per-tx":
                sender_index = (
                    job.start_account
                    + (account_index - job.start_account) * job.num_txs
                    + i
                )
                tx_nonce = job.nonce
                acct = gen_account(job.global_seq, sender_index)
            else:
                tx_nonce = job.nonce + i

            tx = job.create_tx(acct.address, tx_nonce, job.tx_options)
            raw = acct.sign_transaction(tx).rawTransaction
            txs.append(EthTx(tx, raw, HexBytes(acct.address)))
            total += 1

        if job.wire_format == "eth":
            txs = [HexBytes(tx.raw).hex() for tx in txs]
        else:
            # to keep it simple, only build batch inside the account
            txs = [
                build_cosmos_tx(
                    *txs[start:end],
                    msg_version=job.msg_version,
                    evm_denom=job.evm_denom,
                )
                for start, end in split_batch(len(txs), job.batch)
            ]
        acct_txs.append(txs)
        if _progress_counter is not None:
            with _progress_counter.get_lock():
                _progress_counter.value += 1
    print(
        f"generated {total} EVM txs for accounts {job.chunk[0]}-{job.chunk[1] - 1}",
        file=sys.stderr,
    )
    return acct_txs


def gen(
    global_seq,
    num_accounts,
    num_txs,
    tx_type: str,
    batch: int,
    nonce: int = 0,
    start_account: int = 0,
    msg_version: str = "1.4",
    tx_options: dict = None,
    evm_denom: str = DEFAULT_DENOM,
    wire_format: str = "cosmos",
    sender_strategy: str = "reuse",
) -> [str]:
    tx_options = tx_options or {}
    chunks = split(num_accounts, os.cpu_count() or 1)
    create_tx = TX_TYPES[tx_type]
    jobs = [
        Job(
            (start + start_account, end + start_account),
            global_seq,
            num_txs,
            create_tx,
            batch,
            nonce,
            msg_version,
            tx_options,
            evm_denom,
            wire_format,
            sender_strategy,
            start_account,
        )
        for start, end in chunks
    ]

    counter = multiprocessing.Value("i", 0)
    with multiprocessing.Pool(initializer=_init_progress, initargs=(counter,)) as pool:
        result = pool.map_async(_do_job, jobs)
        last_log = time.monotonic()
        while not result.ready():
            result.wait(0.2)
            now = time.monotonic()
            if now - last_log >= PROGRESS_INTERVAL_S:
                print(
                    f"generated txs for {counter.value}/{num_accounts} accounts",
                    file=sys.stderr,
                )
                last_log = now
        acct_txs = result.get()

    # mix the account txs together, ordered by nonce.
    all_txs = []
    for txs in itertools.zip_longest(*itertools.chain(*acct_txs)):
        all_txs += txs

    return all_txs


def physical_account_range(start: int, end: int, num_txs: int, sender_strategy: str):
    """Return the inclusive sender range needed for a logical account range."""
    if sender_strategy == "unique-per-tx":
        return start, start + (end - start + 1) * num_txs - 1
    return start, end


def sender_affinity_accounts(sender_strategy: str, num_accounts: int) -> int | None:
    """The `num_accounts` to hand the send path, or None to disable its
    per-sender endpoint pinning and send serialization.

    Both exist only to keep one sender's nonces from racing each other. Under
    `unique-per-tx` every tx has its own sender at the same nonce, so keying on
    the logical slot would serialize unrelated accounts - and force each one
    through broadcast_tx_sync - for nothing.
    """
    return None if sender_strategy == "unique-per-tx" else num_accounts


def build_cosmos_tx(*txs: EthTx, msg_version="1.4", evm_denom=DEFAULT_DENOM) -> str:
    """
    return base64 encoded cosmos tx, support batch
    """
    build_msg = MSG_VERSIONS[msg_version]
    msgs = [build_msg(tx) for tx in txs]
    fee = sum(tx.tx["gas"] * tx.tx["gasPrice"] for tx in txs)
    gas = sum(tx.tx["gas"] for tx in txs)
    body = cosmostx.TxBody(
        messages=msgs,
        extension_options=[
            cosmostx.build_any("/ethermint.evm.v1.ExtensionOptionsEthereumTx")
        ],
    )
    auth_info = cosmostx.AuthInfo(
        fee=cosmostx.Fee(
            amount=[cosmostx.Coin(denom=evm_denom, amount=str(fee))],
            gas_limit=gas,
        )
    )
    return base64.b64encode(
        cosmostx.TxRaw(
            body=body.SerializeToString(), auth_info=auth_info.SerializeToString()
        ).SerializeToString()
    ).decode()


# CometBFT reports a duplicate send differently depending on which mempool is
# running: the legacy mempool's ErrTxInCache vs app-mempool's ErrSeenTx. Both
# mean the same thing, and a benchmark run must recognize whichever it gets.
DUPLICATE_SEND_MARKERS = ("already exists in cache", "tx already seen")

# sdkerrors.ErrWrongSequence's registered description (x/auth/ante's
# SigVerificationDecorator). A rejection with this text means the tx's own
# bytes are fine and it will succeed if resent once earlier nonces for the
# same sender land - so it's worth retrying instead of counting as failed.
WRONG_SEQUENCE_MARKER = "incorrect account sequence"
# ethermint's own nonce check (ante/eth.go's CheckAndSetEthSenderNonce) raises
# a differently-worded ErrInvalidSequence ("invalid nonce; got X, expected Y").
# It bumps the sender's nonce only in baseapp's checkState, which resets to
# last-committed state on every block commit and is rebuilt by mempool
# recheck; a new envelope for the same sender that lands in the gap between
# reset and recheck completing sees the stale committed nonce and is
# rejected. That gap closes once the predecessor envelope actually commits,
# so this is worth retrying the same as WRONG_SEQUENCE_MARKER.
ETH_INVALID_NONCE_MARKER = "invalid nonce;"
RETRY = "retry"
# Truthy sentinel for a permanent rejection (won't retry) - must be truthy so
# @backoff.on_predicate's default falsy-retries predicate doesn't keep
# resending a tx that will never succeed, and distinct from RETRY so callers
# can still tell "give up" apart from "transient, keep retrying".
INVALID = "invalid"
RETRY_INTERVAL_S = 1.0
MAX_RETRY_ROUNDS = 30
# CometBFT's app-mempool guard holds a rejected tx hash "seen" for
# CheckTxRetryDelay (5s default) before forgetting it. Rounds inside that
# window can still be colliding with our own held-open rejection, so
# own_retry stays True; past it, a duplicate response is CometBFT's cache
# reporting a genuinely accepted tx, and must be trusted as success again.
OWN_RETRY_ROUNDS = 6


def json_rpc_send_body(raw, method="broadcast_tx_async"):
    return {
        "jsonrpc": "2.0",
        "method": method,
        "params": {"tx": raw},
        "id": 1,
    }


def eth_send_raw_body(raw):
    return {
        "jsonrpc": "2.0",
        "method": "eth_sendRawTransaction",
        "params": [raw],
        "id": 1,
    }


@backoff.on_predicate(backoff.expo, max_time=60, max_value=5)
@backoff.on_exception(backoff.expo, aiohttp.ClientError, max_time=60, max_value=5)
async def async_sendtx(session, raw, rpc, sync=False, mode="cosmos", own_retry=False):
    if mode == "eth":
        async with session.post(rpc, json=eth_send_raw_body(raw)) as rsp:
            data = await rsp.json()
            if "error" in data:
                print("send tx error, will retry,", data["error"])
                return False
            return True

    method = "broadcast_tx_sync" if sync else "broadcast_tx_async"
    async with session.post(rpc, json=json_rpc_send_body(raw, method)) as rsp:
        data = await rsp.json()
        if "error" in data:
            # A duplicate-send rejection means this exact tx hash was already
            # accepted by a prior attempt (still pending or already committed -
            # neither mempool distinguishes the two). Retrying it can never
            # succeed differently, so treat it as success rather than burning
            # up to 60s of backoff per tx chasing an already-done send.
            #
            # Exception: own_retry=True (see OWN_RETRY_ROUNDS) - still inside
            # CometBFT's guard window, so this duplicate may be our own
            # held-open rejection rather than genuine acceptance.
            if any(
                marker in str(data["error"].get("data", ""))
                for marker in DUPLICATE_SEND_MARKERS
            ):
                return RETRY if own_retry else True
            print("send tx error, will retry,", data["error"])
            return False
        result = data["result"]
        if result["code"] != 0:
            if mode == "cosmos" and (
                WRONG_SEQUENCE_MARKER in result["log"]
                or ETH_INVALID_NONCE_MARKER in result["log"]
            ):
                return RETRY
            print("tx is invalid, won't retry,", result["log"])
            return INVALID
        return True


async def _drain_retries(session, pending, mode):
    """Resend ``pending`` (raw, rpc) txs rejected with ErrWrongSequence.

    Each round waits ``RETRY_INTERVAL_S`` for earlier nonces to land on-chain,
    then resends via ``broadcast_tx_sync`` (needed to see whether it's still
    a sequence gap). Returns the count still failing once ``MAX_RETRY_ROUNDS``
    is exhausted.
    """
    for round_i in range(MAX_RETRY_ROUNDS):
        if not pending:
            break
        await asyncio.sleep(RETRY_INTERVAL_S)
        own_retry = round_i < OWN_RETRY_ROUNDS
        tasks = [
            asyncio.ensure_future(
                async_sendtx(session, raw, rpc, True, mode, own_retry=own_retry)
            )
            for raw, rpc in pending
        ]
        results = await asyncio.gather(*tasks)
        pending = [item for item, result in zip(pending, results) if result == RETRY]
    return len(pending)


async def _resend_account_tail(session, txs, rpc_for, num_accounts, num_txs, account_index, confirmed, mode):
    """Resend one account's missing nonce tail in order. Returns still-missing count."""
    rpc = rpc_for(account_index)
    for nonce_round in range(confirmed, num_txs):
        raw = txs[nonce_round * num_accounts + account_index]
        result = await async_sendtx(session, raw, rpc, True, mode)
        for round_i in range(MAX_RETRY_ROUNDS):
            if result != RETRY:
                break
            await asyncio.sleep(RETRY_INTERVAL_S)
            result = await async_sendtx(
                session,
                raw,
                rpc,
                True,
                mode,
                own_retry=round_i < OWN_RETRY_ROUNDS,
            )
        if result is not True:
            return num_txs - nonce_round
    return 0


async def resend_missing_nonces(
    txs,
    rpc_for,
    num_accounts,
    num_txs,
    missing,
    mode="cosmos",
    conn_per_host=CONNECTION_POOL_PER_HOST,
    n_hosts=1,
):
    """Heal per-account nonce gaps left by fully-async sending.

    ``missing`` maps ``account_index -> confirmed`` for accounts whose
    on-chain nonce (from a post-hoc query) is behind ``num_txs`` - i.e.
    ``confirmed`` of that account's nonces landed, and ``[confirmed,
    num_txs)`` didn't. Each gap is resent in nonce order via
    ``broadcast_tx_sync``, so - unlike the original async send - this pass
    actually waits out CheckTx before moving to the account's next nonce.
    Different accounts are independent, so their tails resend concurrently -
    only the nonce order within a single account must stay sequential.

    Raw bytes are read directly out of ``txs`` at ``nonce_round *
    num_accounts + account_index`` (``gen()``'s layout - see its docstring),
    so nothing is re-signed. This only holds when every account's txs are
    each their own cosmos tx (``batch_size == 1``); callers must not use this
    otherwise.

    A ``RETRY`` result (transient ``ErrWrongSequence``/invalid-nonce) is
    retried in place up to ``MAX_RETRY_ROUNDS`` times. A ``False`` result is
    a genuine rejection - the account's later nonces can never land without
    it, so its remaining txs are counted as missing and left unsent rather
    than burning rounds on a doomed resend.

    Returns the number of txs from ``missing`` that still didn't land.
    """
    if not missing:
        return 0
    async with _send_session(conn_per_host, n_hosts) as session:
        results = await asyncio.gather(
            *(
                _resend_account_tail(
                    session, txs, rpc_for, num_accounts, num_txs, account_index, confirmed, mode
                )
                for account_index, confirmed in missing.items()
            )
        )
        return sum(results)


def _sender_key(i, j, num_accounts):
    """Map tx position (i+j) to its sender bucket.

    Returns (i+j) unchanged when num_accounts is None: no sender reuses a
    nonce, so any deterministic value works and no two positions collide.
    """
    return (i + j) if num_accounts is None else (i + j) % num_accounts


async def _send_batches(
    session,
    txs,
    rpc_for,
    sync,
    batch_size,
    batch_interval,
    mode,
    probe_batches,
    deadline_s,
    sent_out=None,
):
    """Shared batch-send loop for ``send``/``send_round_robin``.

    ``rpc_for(i, j)`` picks the endpoint for the tx at chunk offset ``i + j``.
    See ``send``'s docstring for the pacing/probe/deadline/retry semantics.

    Same-sender txs are sent fully concurrently, with no wait for a prior
    nonce from the same account to land - two nonces racing CometBFT's
    admission lock can land out of order and get one of them rejected
    forever (recheck disabled). Callers that care about zero-failure use the
    resend pass in ``resend_missing_nonces`` to heal any such gaps after the
    fact, trading a bounded amount of post-hoc repair for not blocking every
    send on its predecessor's CheckTx round-trip.

    ``sent_out``, if given, is a list that gets the count of txs actually
    dispatched appended to it - distinct from ``len(txs)`` when ``deadline_s``
    truncates the loop early. Callers that size ``txs`` with deliberate
    oversupply (so pacing never runs dry) need this to avoid treating the
    untouched surplus as a nonce gap to heal.
    """
    started = time.monotonic()
    last_log = started
    failed = 0
    sent = 0
    pending_retry = []
    for i in range(0, len(txs), batch_size):
        if _past_deadline(started, deadline_s, i, len(txs)):
            break
        chunk = txs[i : i + batch_size]
        batch_sync = sync or (i // batch_size) < probe_batches
        chunk_rpcs = [rpc_for(i, j) for j in range(len(chunk))]
        tasks = [
            asyncio.ensure_future(async_sendtx(session, raw, rpc, batch_sync, mode))
            for raw, rpc in zip(chunk, chunk_rpcs)
        ]
        results = await asyncio.gather(*tasks)
        failed += sum(1 for r in results if r not in (True, RETRY))
        pending_retry.extend(
            (raw, rpc)
            for raw, rpc, result in zip(chunk, chunk_rpcs, results)
            if result == RETRY
        )
        sent = i + len(chunk)
        last_log = _log_progress(started, last_log, sent, len(txs))
        if i + batch_size < len(txs) and batch_interval > 0:
            await asyncio.sleep(batch_interval)
    if sent_out is not None:
        sent_out.append(sent)
    failed += await _drain_retries(session, pending_retry, mode)
    return failed


async def send(
    txs,
    rpc,
    sync=False,
    batch_size=500,
    batch_interval=0.5,
    mode="cosmos",
    probe_batches=1,
    deadline_s=None,
    num_accounts=None,
    conn_per_host=CONNECTION_POOL_PER_HOST,
    sent_out=None,
):
    """Send transactions to a single rpc endpoint in rate-limited batches.

    Sends ``batch_size`` txs concurrently, pauses ``batch_interval`` seconds,
    then sends the next batch.  The pause lets the chain produce blocks and
    drain the mempool between batches, preventing the CheckTx flood that
    overwhelms the proposer and causes repeated consensus round timeouts.

    ``deadline_s`` caps the send loop at that many seconds of wall clock,
    leaving any remaining txs unsent. An open-loop rate target defines the run
    by its duration, so overrunning it to drain the generated txs would
    benchmark a longer window than was asked for.

    The first ``probe_batches`` batches are always sent with
    ``broadcast_tx_sync``/``eth_sendRawTransaction`` regardless of ``sync``,
    so CheckTx rejections are surfaced immediately. Plain
    ``broadcast_tx_async`` returns before CheckTx even runs, so a whole run
    can silently reject every tx (e.g. a bad nonce or unsupported batch
    encoding) while every send still "succeeds" and reports look like
    zero-load runs with no error printed anywhere.

    Returns the number of txs whose ``async_sendtx`` retries were still
    failing when its backoff gave up - those never reached the mempool, so
    counting on them to show up in the commit count would time out waiting
    for txs that are never coming.

    That count only covers give-ups that return ``False`` (HTTP-level error
    responses). A give-up on a persistent ``aiohttp.ClientError`` instead
    raises out of ``async_sendtx``, which propagates through
    ``asyncio.gather`` uncaught - it crashes the send loop rather than being
    counted here.

    A ``sync`` rejection with ``ErrWrongSequence`` isn't counted as failed
    immediately - it means the tx itself is fine and just arrived before an
    earlier nonce for the same sender, so it's queued and resent (unchanged)
    once the run's batches are done, giving the chain time to catch up.

    ``num_accounts`` is accepted only so ``send_round_robin``'s single-rpc
    fallback can call this with the same kwargs it would use for multiple
    endpoints - unused here, since a single endpoint needs no per-sender
    routing.
    """
    async with _send_session(conn_per_host, 1) as session:
        return await _send_batches(
            session,
            txs,
            lambda i, j: rpc,
            sync,
            batch_size,
            batch_interval,
            mode,
            probe_batches,
            deadline_s,
            sent_out=sent_out,
        )


def _past_deadline(started, deadline_s, sent, total):
    """True once the send loop has run past ``deadline_s`` seconds."""
    if deadline_s is None:
        return False
    elapsed = time.monotonic() - started
    if elapsed < deadline_s:
        return False
    print(
        f"deadline {deadline_s:g}s reached after {elapsed:.1f}s, "
        f"stopping with {total - sent}/{total} txs unsent",
        file=sys.stderr,
    )
    return True


def _log_progress(started, last_log, sent, total):
    """Print a "still sending" line at most once every ``PROGRESS_INTERVAL_S``.

    Returns the (possibly updated) last-log timestamp. Without this, a long
    send phase prints nothing between "sending txs..." and the final result,
    which looks indistinguishable from a hang.
    """
    now = time.monotonic()
    if now - last_log < PROGRESS_INTERVAL_S:
        return last_log
    print(
        f"sent {sent}/{total} txs, {now - started:.1f}s elapsed",
        file=sys.stderr,
    )
    return now


async def send_round_robin(
    txs,
    rpcs: [str],
    num_accounts,
    sync=False,
    batch_size=500,
    batch_interval=0.5,
    mode="cosmos",
    probe_batches=1,
    deadline_s=None,
    conn_per_host=CONNECTION_POOL_PER_HOST,
    sent_out=None,
):
    """Send transactions across multiple rpc endpoints in rate-limited batches.

    ``txs`` is laid out by ``gen()`` as consecutive nonce-rounds interleaved
    across accounts, i.e. position ``p`` belongs to account ``p %
    num_accounts``. Txs are assigned to endpoints round-robin by that account
    index (``rpcs[(i % num_accounts) % len(rpcs)]``) so every node always
    sees a given account's txs in nonce order - splitting one account's
    sequential nonces across nodes round-robin by flat position would let a
    later nonce reach a node before an earlier one propagates, which the
    node's CheckTx rejects as a nonce gap. ``num_accounts`` has no safe
    default: omitting it would either serialize everything onto one endpoint
    or break nonce ordering, so callers must always pass it explicitly. Falls
    back to plain ``send`` when only one endpoint is configured.

    See ``send``'s docstring for why ``probe_batches``, ``deadline_s``, and the
    return value exist.
    """
    if len(rpcs) == 1:
        return await send(
            txs,
            rpcs[0],
            sync=sync,
            batch_size=batch_size,
            batch_interval=batch_interval,
            mode=mode,
            probe_batches=probe_batches,
            deadline_s=deadline_s,
            num_accounts=num_accounts,
            conn_per_host=conn_per_host,
            sent_out=sent_out,
        )

    async with _send_session(conn_per_host, len(rpcs)) as session:
        return await _send_batches(
            session,
            txs,
            lambda i, j: rpcs[_sender_key(i, j, num_accounts) % len(rpcs)],
            sync,
            batch_size,
            batch_interval,
            mode,
            probe_batches,
            deadline_s,
            sent_out=sent_out,
        )


def _send_worker(args):
    txs, rpcs, kwargs = args
    return asyncio.run(send_round_robin(txs, rpcs, **kwargs))


def send_multiprocess(
    txs, rpcs, num_accounts, num_workers=None, nonce_ordered=True, **send_kwargs
):
    """Fan tx sending out across multiple OS processes.

    A single asyncio event loop tops out around ~11k tx/s sending locally:
    JSON-RPC serialization and event-loop scheduling per tx are CPU-bound on
    one core, not network-bound (the local devnet's mempool stays empty
    throughout every run, so the node is never the wait). This splits `txs`
    into `num_workers` disjoint account ranges, each sent by its own process
    with its own event loop and connection pool.

    Splits by ACCOUNT range, not flat position: `send_round_robin` requires
    every account's txs to arrive in nonce order, so a worker must own an
    account's entire nonce sequence rather than an arbitrary slice of it.
    `num_accounts` is the raw layout modulus from `gen()` (always a real
    count, even under `unique-per-tx`) - it is only used to compute that
    split, independent of ``nonce_ordered``.

    Each worker's local account index starts back at 0, so without correcting
    for that, every worker's account 0 would route to `rpcs[0]` at the same
    moment - clustering N workers' first sub-batches onto the same low-index
    endpoints instead of spreading them across the full rpc pool. `rpcs` is
    rotated per worker by its account-range start (`lo`) so a worker's local
    index 0 lands on the same endpoint its accounts would get in the
    single-process round robin.

    ``nonce_ordered`` distinguishes the two sender strategies: under `reuse`
    (default, ``True``) a batch must never contain two nonces for the same
    account, so `batch_size` is capped at the worker's own account count and
    `sync=True` is forced - more OS processes hammering CheckTx concurrently
    widens the window for cross-batch reordering per account (round N+1
    landing before round N), which the node rejects as `ErrWrongSequence`, and
    with `recheckDisabled=true` that tx is gone for good unless resent. Under
    `unique-per-tx` (``False``) every tx already has its own sender at its own
    nonce, so there is nothing to reorder or serialize and neither clamp is
    needed.

    The single-process fallback below hits none of the reordering risk (one
    process, one event loop), so it keeps the caller's own ``sync`` regardless
    of ``nonce_ordered``.
    """
    num_workers = num_workers or min(multiprocessing.cpu_count(), num_accounts)
    if num_workers <= 1 or num_accounts <= 1:
        return asyncio.run(
            send_round_robin(
                txs,
                rpcs,
                num_accounts=num_accounts if nonce_ordered else None,
                **send_kwargs,
            )
        )

    if nonce_ordered:
        send_kwargs = {**send_kwargs, "sync": True}
    boundaries = [round(i * num_accounts / num_workers) for i in range(num_workers + 1)]
    jobs = []
    for lo, hi in zip(boundaries, boundaries[1:]):
        if lo == hi:
            continue
        worker_txs = [tx for i, tx in enumerate(txs) if lo <= i % num_accounts < hi]
        offset = lo % len(rpcs)
        worker_rpcs = rpcs[offset:] + rpcs[:offset]
        worker_kwargs = {
            **send_kwargs,
            "num_accounts": (hi - lo) if nonce_ordered else None,
        }
        if nonce_ordered:
            # Only a cap: a batch must not span two nonce rounds for one
            # account, but a caller that asked for smaller batches to throttle
            # CheckTx must not have them widened.
            worker_kwargs["batch_size"] = min(
                send_kwargs.get("batch_size", hi - lo), hi - lo
            )
        jobs.append((worker_txs, worker_rpcs, worker_kwargs))

    with multiprocessing.Pool(len(jobs)) as pool:
        return sum(pool.map(_send_worker, jobs))

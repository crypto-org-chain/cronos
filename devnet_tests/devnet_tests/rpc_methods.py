from collections.abc import Callable
from dataclasses import dataclass


@dataclass
class DiffContext:
    """Fixed input sampled once from a reference node, replayed against every node."""

    height: int
    block_hash: str
    tx_hash: str | None
    address: str
    calldata: str
    sender: str
    # False when the sampled block held no tx targeting deployed bytecode, so
    # `address`/`calldata` are a no-op fallback and the `call` category executes
    # no EVM code at this height.
    call_target_is_contract: bool = False


# Compares two raw JSON-RPC responses ({"result": ...} or {"error": ...}) and
# returns human-readable mismatch descriptions (empty list = match).
Comparator = Callable[[dict, dict], list[str]]
ParamsBuilder = Callable[[DiffContext], list]


def _hex(value) -> str:
    text = value.hex() if hasattr(value, "hex") else str(value)
    return text if text.startswith("0x") else "0x" + text


# Bounds the get_code probing below on a full block; a contract call in a devnet
# block is either near the front or not worth the extra round trips.
_CALL_TARGET_SCAN_LIMIT = 20


def _contract_call_tx(w3, transactions):
    """First tx in the block that invokes deployed bytecode, so the `call`-category
    methods actually execute EVM code instead of no-op'ing against an EOA."""
    for tx in transactions[:_CALL_TARGET_SCAN_LIMIT]:
        if not tx["to"] or not tx["input"]:
            continue
        try:
            if len(w3.eth.get_code(tx["to"])) > 0:
                return tx
        except Exception:  # noqa: BLE001
            continue
    return None


def build_context(w3, height: int, sender: str) -> DiffContext:
    block = w3.eth.get_block(height, full_transactions=True)
    first_tx = block.transactions[0] if block.transactions else None
    call_tx = _contract_call_tx(w3, block.transactions)
    return DiffContext(
        height=height,
        block_hash=_hex(block.hash),
        tx_hash=_hex(first_tx["hash"]) if first_tx else None,
        # Contract creations and empty blocks leave no callee, so fall back to sender.
        address=(call_tx or first_tx or {}).get("to") or sender,
        calldata=_hex(call_tx["input"]) if call_tx else "0x",
        sender=sender,
        call_target_is_contract=call_tx is not None,
    )


def _strip_envelope_id(response: dict) -> dict:
    return {k: v for k, v in response.items() if k != "id"}


def _equal_compare(a: dict, b: dict) -> list[str]:
    a, b = _strip_envelope_id(a), _strip_envelope_id(b)
    return [f"response mismatch: {a} != {b}"] if a != b else []


def _shape_only_compare(required_keys: set) -> Comparator:
    """For methods whose values are legitimately node-local (txpool contents):
    both responses must carry the required keys, agree on their key set, *and*
    agree on each shared key's value type, so a node exposing a different result
    shape — an ethermint version that returns a count where the other returns a
    map — is still flagged."""

    def compare(a: dict, b: dict) -> list[str]:
        # Two nodes erroring identically exercises nothing; the caller classifies
        # that as both_errored rather than as a cross-node mismatch.
        if "error" in a and "error" in b:
            return []
        mismatches = []
        results = {}
        for label, response in (("a", a), ("b", b)):
            if "error" in response:
                mismatches.append(f"{label} returned an error: {response['error']}")
                continue
            # `{"result": null}` is a legal response shape, so coerce before
            # taking keys rather than trusting the default.
            results[label] = response.get("result") or {}
            missing = required_keys - set(results[label])
            if missing:
                mismatches.append(f"{label} result missing keys: {sorted(missing)}")
        # Only worth reporting once both sides are otherwise well-formed;
        # otherwise it just restates the error/missing-keys mismatch above.
        if mismatches:
            return mismatches
        keys_a, keys_b = set(results["a"]), set(results["b"])
        if keys_a != keys_b:
            return [f"result keys differ: {sorted(keys_a)} != {sorted(keys_b)}"]
        for key in sorted(keys_a):
            type_a = type(results["a"][key]).__name__
            type_b = type(results["b"][key]).__name__
            if type_a != type_b:
                mismatches.append(f"{key} value type differs: {type_a} != {type_b}")
        return mismatches

    return compare


def _call(w3, method: str, params: list) -> dict:
    return dict(w3.provider.make_request(method, params))


def _run_filter_logs(w3, ctx: DiffContext) -> dict:
    height = hex(ctx.height)
    created = _call(w3, "eth_newFilter", [{"fromBlock": height, "toBlock": height}])
    if "error" in created:
        return created
    try:
        return _call(w3, "eth_getFilterLogs", [created["result"]])
    finally:
        # Never let a cleanup failure mask the real result/exception above.
        try:
            _call(w3, "eth_uninstallFilter", [created["result"]])
        except Exception:
            pass


@dataclass
class RpcMethod:
    name: str
    category: str
    build_params: ParamsBuilder | None = None
    compare: Comparator = _equal_compare
    # Set for multi-request methods; takes precedence over build_params.
    run: Callable[[object, DiffContext], dict] | None = None


def run_method(method: RpcMethod, w3, ctx: DiffContext) -> dict | None:
    """Returns None when this method has nothing to test against ctx (e.g. no tx
    in the sampled block for a tx-hash method) - callers should skip, not diff."""
    if method.run:
        return method.run(w3, ctx)
    params = method.build_params(ctx)
    if None in params:
        return None
    return _call(w3, method.name, params)


_TXPOOL_SHAPE = _shape_only_compare({"pending", "queued"})

METHODS = [
    RpcMethod("eth_getBlockByNumber", "blocks", lambda ctx: [hex(ctx.height), False]),
    RpcMethod("eth_getBlockByHash", "blocks", lambda ctx: [ctx.block_hash, False]),
    RpcMethod("eth_getTransactionByHash", "transactions", lambda ctx: [ctx.tx_hash]),
    RpcMethod(
        "eth_getTransactionCount",
        "transactions",
        lambda ctx: [ctx.sender, hex(ctx.height)],
    ),
    RpcMethod("eth_getTransactionReceipt", "receipts", lambda ctx: [ctx.tx_hash]),
    RpcMethod(
        "debug_traceTransaction",
        "traces",
        lambda ctx: [ctx.tx_hash, {"tracer": "callTracer"}],
    ),
    RpcMethod(
        "eth_getLogs",
        "logs",
        lambda ctx: [{"fromBlock": hex(ctx.height), "toBlock": hex(ctx.height)}],
    ),
    RpcMethod(
        "eth_call",
        "call",
        lambda ctx: [{"to": ctx.address, "data": ctx.calldata}, hex(ctx.height)],
    ),
    RpcMethod(
        "eth_estimateGas",
        "call",
        lambda ctx: [{"to": ctx.address, "data": ctx.calldata}, hex(ctx.height)],
    ),
    RpcMethod(
        "eth_createAccessList",
        "call",
        lambda ctx: [{"to": ctx.address, "data": ctx.calldata}, hex(ctx.height)],
    ),
    RpcMethod("eth_feeHistory", "fee", lambda ctx: [4, hex(ctx.height), [25, 75]]),
    RpcMethod("txpool_status", "txpool", lambda ctx: [], compare=_TXPOOL_SHAPE),
    RpcMethod("txpool_content", "txpool", lambda ctx: [], compare=_TXPOOL_SHAPE),
    RpcMethod("eth_newFilter+eth_getFilterLogs", "filters", run=_run_filter_logs),
]

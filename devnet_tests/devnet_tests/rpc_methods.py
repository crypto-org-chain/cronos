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


# Compares two raw JSON-RPC responses ({"result": ...} or {"error": ...}) and
# returns human-readable mismatch descriptions (empty list = match).
Comparator = Callable[[dict, dict], list[str]]
ParamsBuilder = Callable[[DiffContext], list]


def build_context(w3, height: int, sender: str) -> DiffContext:
    block = w3.eth.get_block(height, full_transactions=True)
    first_tx = block.transactions[0] if block.transactions else None
    return DiffContext(
        height=height,
        block_hash=block.hash.hex(),
        tx_hash=first_tx["hash"].hex() if first_tx else None,
        # Contract creations and empty blocks leave no callee, so fall back to sender.
        address=(first_tx["to"] if first_tx else None) or sender,
        calldata="0x",
        sender=sender,
    )


def _strip_envelope_id(response: dict) -> dict:
    return {k: v for k, v in response.items() if k != "id"}


def _equal_compare(a: dict, b: dict) -> list[str]:
    a, b = _strip_envelope_id(a), _strip_envelope_id(b)
    return [f"response mismatch: {a} != {b}"] if a != b else []


def _shape_only_compare(required_keys: set) -> Comparator:
    def compare(a: dict, b: dict) -> list[str]:
        mismatches = []
        for label, response in (("a", a), ("b", b)):
            if "error" in response:
                mismatches.append(f"{label} returned an error: {response['error']}")
                continue
            missing = required_keys - response.get("result", {}).keys()
            if missing:
                mismatches.append(f"{label} result missing keys: {sorted(missing)}")
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
        lambda ctx: [{"to": ctx.address, "data": ctx.calldata}],
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

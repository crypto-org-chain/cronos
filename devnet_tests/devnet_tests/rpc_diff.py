import json
from dataclasses import asdict, dataclass, field
from pathlib import Path

import click

from .devnet import Devnet, load_devnet
from .rpc_methods import METHODS, RpcMethod, build_context, run_method

ZERO_ADDRESS = "0x" + "0" * 40


@dataclass
class DiffRow:
    method: str
    category: str
    height: int
    node: str
    mismatches: list[str]


@dataclass
class RpcDiffReport:
    rows: list[DiffRow] = field(default_factory=list)
    skipped: int = 0
    # Heights actually walked, after run_rpc_diff clamps the requested range to
    # the least-caught-up node. A skip count only means "never ran" against this.
    heights_sampled: int = 0
    # method name -> heights it had no usable input at (e.g. a tx-hash method
    # over an empty block). Without this a method that never ran once still
    # counts towards a 100% equivalence rate.
    skipped_methods: dict[str, int] = field(default_factory=dict)
    compared: int = 0
    matched: int = 0
    # Comparisons where both nodes returned an error. Identical errors (both
    # nodes missing the debug namespace, both rejecting the request) exercise
    # nothing, so they are neither a match nor a mismatch.
    both_errored: int = 0
    both_errored_methods: dict[str, int] = field(default_factory=dict)
    responded_methods: dict[str, int] = field(default_factory=dict)
    # Sampled heights whose block held a tx calling deployed bytecode, i.e. where
    # the `call` category actually executed EVM code.
    contract_call_heights: int = 0

    @property
    def equivalence_rate(self) -> float | None:
        """Rate over comparisons that got a real response on at least one side.
        None when there were none — reporting 1.0 for an empty run, or for a run
        where every node errored identically, would be a vacuous pass."""
        substantive = self.compared - self.both_errored
        if substantive == 0:
            return None
        return self.matched / substantive

    @property
    def mismatches_by_method(self) -> dict[str, int]:
        """Mismatch count per method. An aggregate rate hides a method that is
        broken at every height behind the methods that agreed."""
        counts: dict[str, int] = {}
        for row in self.rows:
            counts[row.method] = counts.get(row.method, 0) + 1
        return counts

    @property
    def never_responded(self) -> list[str]:
        """Methods that errored on every comparison at every sampled height, so
        nothing was ever really exercised — two nodes agreeing on an error is not
        evidence of equivalence."""
        return sorted(
            name
            for name in self.both_errored_methods
            if not self.responded_methods.get(name)
        )

    @property
    def never_compared(self) -> list[str]:
        """Methods that had no usable input at every sampled height, so they were
        never diffed once — a 100% equivalence rate over the rest says nothing
        about them."""
        return sorted(
            name
            for name, count in self.skipped_methods.items()
            if count >= self.heights_sampled
        )

    def to_dict(self) -> dict:
        return {
            "equivalence_rate": self.equivalence_rate,
            "compared": self.compared,
            "heights_sampled": self.heights_sampled,
            "skipped": self.skipped,
            "skipped_methods": self.skipped_methods,
            "never_compared": self.never_compared,
            "both_errored": self.both_errored,
            "both_errored_methods": self.both_errored_methods,
            "never_responded": self.never_responded,
            "contract_call_heights": self.contract_call_heights,
            "mismatches_by_method": self.mismatches_by_method,
            "mismatches": [asdict(r) for r in self.rows],
        }


def _run_or_capture(method: RpcMethod, w3, ctx) -> dict | None:
    """A transport/node failure is a diffable outcome here, not a runner crash, so it
    is shaped like an error response and compared alongside the successful ones."""
    try:
        return run_method(method, w3, ctx)
    except Exception as exc:  # noqa: BLE001
        return {"error": {"message": str(exc)}}


def _errored(response: dict | None) -> bool:
    return response is not None and "error" in response


def _compare_or_capture(method: RpcMethod, reference: dict, other: dict | None) -> list[str]:
    """A comparator that blows up on an unexpected response shape is itself a
    diffable outcome, not a reason to abandon the remaining heights."""
    try:
        return method.compare(reference, other)
    except Exception as exc:  # noqa: BLE001
        return [f"comparison failed: {exc}"]


def run_rpc_diff(devnet: Devnet, start: int, end: int) -> RpcDiffReport:
    """Replays every registered method over [start, end] against every node,
    diffing each against devnet.nodes[0] (the reference)."""
    if len(devnet.nodes) < 2:
        raise ValueError("rpc-diff needs at least 2 nodes to compare")

    reference, *others = devnet.nodes
    sender = devnet.funded_account.address if devnet.funded_account else ZERO_ADDRESS
    report = RpcDiffReport()

    # Clamp to the least-caught-up node so a lagging node's "not found yet"
    # responses aren't mistaken for regressions at heights it hasn't reached.
    end = min(end, *(node.w3.eth.block_number for node in devnet.nodes))
    if end < start:
        raise ValueError(
            f"a configured node hasn't reached height {start} yet (caught up to {end})"
        )

    for height in range(start, end + 1):
        report.heights_sampled += 1
        ctx = build_context(reference.w3, height, sender)
        if ctx.call_target_is_contract:
            report.contract_call_heights += 1
        for method in METHODS:
            reference_result = _run_or_capture(method, reference.w3, ctx)
            if reference_result is None:
                report.skipped += 1
                report.skipped_methods[method.name] = (
                    report.skipped_methods.get(method.name, 0) + 1
                )
                continue
            for node in others:
                other_result = _run_or_capture(method, node.w3, ctx)
                mismatches = _compare_or_capture(method, reference_result, other_result)
                report.compared += 1
                if not _errored(reference_result) or not _errored(other_result):
                    report.responded_methods[method.name] = (
                        report.responded_methods.get(method.name, 0) + 1
                    )
                if mismatches:
                    row = DiffRow(
                        method.name, method.category, height, node.name, mismatches
                    )
                    report.rows.append(row)
                elif _errored(reference_result) and _errored(other_result):
                    report.both_errored += 1
                    report.both_errored_methods[method.name] = (
                        report.both_errored_methods.get(method.name, 0) + 1
                    )
                else:
                    report.matched += 1

    return report


@click.group()
def cli():
    pass


@cli.command("rpc-diff")
@click.option("--config", "config_path", required=True)
@click.option("--start", type=int, required=True)
@click.option("--end", type=int, required=True)
@click.option("--out", "out_path", default=None)
def rpc_diff_cmd(config_path, start, end, out_path):
    report = run_rpc_diff(load_devnet(config_path), start, end)

    output = json.dumps(report.to_dict(), indent=2)
    if out_path:
        Path(out_path).write_text(output)
    click.echo(output)

    rate = report.equivalence_rate
    if rate is None:
        raise click.ClickException(
            f"nothing was compared over heights {start}-{end} — "
            "no equivalence rate to report"
        )
    click.echo(f"equivalence rate: {rate:.2%}")
    if report.rows:
        raise click.ClickException(
            f"{len(report.rows)} mismatch(es) across {report.compared} comparisons"
        )
    if report.never_responded:
        raise click.ClickException(
            f"never got a real response from any node: {report.never_responded}"
        )
    # Same two vacuous-pass conditions test_rpc_diff.py asserts on: a method with
    # no usable input at every sampled height was never diffed, and without a tx
    # calling deployed bytecode the `call` category executes no EVM code.
    if report.never_compared:
        raise click.ClickException(
            "never compared at any sampled height (no usable input): "
            f"{report.never_compared}"
        )
    if report.contract_call_heights == 0:
        raise click.ClickException(
            f"no sampled height in {start}-{end} had a tx calling deployed bytecode "
            "— the `call` category executed no EVM code"
        )


if __name__ == "__main__":
    cli()

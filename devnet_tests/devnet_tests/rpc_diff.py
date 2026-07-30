import json
from dataclasses import asdict, dataclass, field
from pathlib import Path

import click

from .conftest import Devnet, load_devnet
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
    compared: int = 0
    matched: int = 0

    @property
    def equivalence_rate(self) -> float:
        if self.compared == 0:
            return 1.0
        return self.matched / self.compared

    def to_dict(self) -> dict:
        return {
            "equivalence_rate": self.equivalence_rate,
            "compared": self.compared,
            "skipped": self.skipped,
            "mismatches": [asdict(r) for r in self.rows],
        }


def _run_or_capture(method: RpcMethod, w3, ctx) -> dict | None:
    """A transport/node failure is a diffable outcome here, not a runner crash, so it
    is shaped like an error response and compared alongside the successful ones."""
    try:
        return run_method(method, w3, ctx)
    except Exception as exc:  # noqa: BLE001
        return {"error": {"message": str(exc)}}


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
        ctx = build_context(reference.w3, height, sender)
        for method in METHODS:
            reference_result = _run_or_capture(method, reference.w3, ctx)
            if reference_result is None:
                report.skipped += 1
                continue
            for node in others:
                mismatches = method.compare(
                    reference_result, _run_or_capture(method, node.w3, ctx)
                )
                report.compared += 1
                if mismatches:
                    row = DiffRow(
                        method.name, method.category, height, node.name, mismatches
                    )
                    report.rows.append(row)
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
    click.echo(f"equivalence rate: {report.equivalence_rate:.2%}")


if __name__ == "__main__":
    cli()

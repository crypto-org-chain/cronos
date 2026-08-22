#!/usr/bin/env python3
"""Fund remote_benchmark's reserved index-0 fund account (see gen_account's
docstring) directly in genesis, for chain-dirs where patch_erc20_genesis.py
already ran without covering it (that script only funds indices 1..N).
"""
import argparse
import glob
import json
import sys
from pathlib import Path

sys.path.insert(0, "scripts/devnet-local")
from patch_erc20_genesis import bank_genesis_balances, eth_to_bech32  # noqa: E402

from remote_benchmark.utils import gen_account  # noqa: E402


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--chain-id", default="cronos_777-1")
    parser.add_argument("--global-seq", type=int, default=0)
    parser.add_argument("--denom", default="basetcro")
    parser.add_argument("--amount", type=int, default=20_000_000 * 10**18)
    args = parser.parse_args()

    fund_address = gen_account(args.global_seq, 0).address
    fund_bech32 = eth_to_bech32(fund_address)

    genesis_paths = sorted(
        Path(p) for p in glob.glob(f"{args.data_dir}/{args.chain_id}/node*/config/genesis.json")
    )
    if not genesis_paths:
        raise SystemExit(f"no genesis.json found under {args.data_dir}/{args.chain_id}")

    for path in genesis_paths:
        genesis = json.loads(path.read_text())
        app_state = genesis["app_state"]

        existing_auth_top = {a["address"] for a in app_state["auth"]["accounts"] if "address" in a}
        if fund_bech32 not in existing_auth_top:
            app_state["auth"]["accounts"].append(
                {"@type": "/cosmos.auth.v1beta1.BaseAccount", "address": fund_bech32}
            )

        existing_bank = {b["address"] for b in app_state["bank"]["balances"]}
        if fund_bech32 not in existing_bank:
            new_balance = bank_genesis_balances(args.denom, args.amount, [fund_address])
            app_state["bank"]["balances"] += new_balance
            supply = app_state["bank"]["supply"]
            for coin in supply:
                if coin["denom"] == args.denom:
                    coin["amount"] = str(int(coin["amount"]) + args.amount)
                    break
            else:
                supply.append({"denom": args.denom, "amount": str(args.amount)})

        path.write_text(json.dumps(genesis))
        print(f"patched {path} with fund account {fund_bech32} ({fund_address})")


if __name__ == "__main__":
    main()

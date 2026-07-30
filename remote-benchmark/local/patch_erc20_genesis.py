#!/usr/bin/env python3
"""Inject predeployed contracts + balances into a pystarport devnet's
genesis, for every node under --data-dir.

remote_benchmark.erc20.genesis_accounts() (ported from
testground/benchmark/benchmark/peer.py) builds the app_state.evm.accounts /
app_state.auth.accounts entries for the fixed contract address that
transaction.py's erc20_transfer_tx already hardcodes. remote_benchmark.contracts
does the same for the ContentionPool and MintCounter contracts used by the
uniswap-swap and nft-mint tx types. pystarport's jsonnet genesis_patch has no
clean way to express "N deterministically-derived accounts with contract
storage slots", so this runs as a separate step between `pystarport init` and
`pystarport start`, patching each node's already-generated genesis.json in
place.

Safe to run for every test case (not just erc20 ones): it only adds entries,
never removes existing validator/community accounts.
"""
import argparse
import glob
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from remote_benchmark.contracts import (  # noqa: E402
    nft_genesis_account,
    pool_genesis_account,
)
from remote_benchmark.erc20 import CONTRACT_ADDRESS, genesis_accounts  # noqa: E402
from remote_benchmark.utils import gen_account  # noqa: E402

# Seed reserves well above any single swap's amountIn so ContentionPool's
# `reserve1 -= amountIn / 2` never underflows.
POOL_RESERVE = 10**24


def patch_genesis(path: Path, addresses: list[str]) -> None:
    genesis = json.loads(path.read_text())
    evm_accounts, auth_accounts = genesis_accounts(CONTRACT_ADDRESS, addresses)
    pool_evm, pool_auth = pool_genesis_account(POOL_RESERVE, POOL_RESERVE)
    nft_evm, nft_auth = nft_genesis_account()
    evm_accounts += [pool_evm, nft_evm]
    auth_accounts += [pool_auth, nft_auth]

    app_state = genesis["app_state"]
    existing_evm = {a["address"] for a in app_state["evm"]["accounts"]}
    app_state["evm"]["accounts"] += [
        a for a in evm_accounts if a["address"] not in existing_evm
    ]

    # plain "/cosmos.auth.v1beta1.BaseAccount" entries (validators, community,
    # signers) have "address" at the top level; only EthAccount entries (like
    # the contract account we're adding) nest it under "base_account".
    existing_auth = {
        a["base_account"]["address"]
        for a in app_state["auth"]["accounts"]
        if "base_account" in a
    }
    app_state["auth"]["accounts"] += [
        a for a in auth_accounts if a["base_account"]["address"] not in existing_auth
    ]

    path.write_text(json.dumps(genesis))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data-dir", required=True, help="pystarport data dir")
    parser.add_argument("--chain-id", default="cronos_777-1")
    parser.add_argument("--global-seq", type=int, default=0)
    parser.add_argument(
        "--num-accounts",
        type=int,
        default=2000,
        help="fund ERC20 balance for account indices 1..num-accounts",
    )
    args = parser.parse_args()

    addresses = [
        gen_account(args.global_seq, i).address for i in range(1, args.num_accounts + 1)
    ]

    genesis_paths = sorted(
        Path(p)
        for p in glob.glob(f"{args.data_dir}/{args.chain_id}/node*/config/genesis.json")
    )
    if not genesis_paths:
        raise SystemExit(f"no genesis.json found under {args.data_dir}/{args.chain_id}")

    for path in genesis_paths:
        patch_genesis(path, addresses)
        print(f"patched {path} with {len(addresses)} ERC20-funded accounts")


if __name__ == "__main__":
    main()

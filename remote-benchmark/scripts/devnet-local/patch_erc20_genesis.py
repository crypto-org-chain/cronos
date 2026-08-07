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

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))

from remote_benchmark.contracts import (  # noqa: E402
    nft_genesis_account,
    pool_genesis_account,
)
from remote_benchmark.erc20 import CONTRACT_ADDRESS, genesis_accounts  # noqa: E402
from remote_benchmark.utils import eth_to_bech32, gen_account  # noqa: E402

# Seed reserves well above any single swap's amountIn so ContentionPool's
# `reserve1 -= amountIn / 2` never underflows.
POOL_RESERVE = 10**24


def bank_genesis_balances(denom: str, amount: int, addresses: list[str]) -> list[dict]:
    """Native-coin balance entries so accounts arrive funded at genesis,
    instead of needing a live post-start funding transaction.

    x/bank genesis balances key off the bech32 cosmos address, not the raw
    hex EVM address that x/evm's accounts use."""
    return [
        {
            "address": eth_to_bech32(addr),
            "coins": [{"denom": denom, "amount": str(amount)}],
        }
        for addr in addresses
    ]


def patch_genesis(
    path: Path,
    erc20_addresses: list[str],
    fund_addresses: list[str],
    denom: str,
    fund_amount: int,
) -> None:
    genesis = json.loads(path.read_text())
    evm_accounts, auth_accounts = genesis_accounts(CONTRACT_ADDRESS, erc20_addresses)
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
    existing_auth_top = {
        a["address"] for a in app_state["auth"]["accounts"] if "address" in a
    }
    existing_auth_nested = {
        a["base_account"]["address"]
        for a in app_state["auth"]["accounts"]
        if "base_account" in a
    }
    app_state["auth"]["accounts"] += [
        a for a in auth_accounts if a["base_account"]["address"] not in existing_auth_nested
    ]

    # x/bank's InitGenesis only writes balances to the store - unlike a live
    # SendCoins tx, it never creates the auth account object, so every funded
    # EOA needs its own explicit BaseAccount entry or its first tx fails
    # ante-handler signature verification with "account not found".
    fund_bech32 = [eth_to_bech32(addr) for addr in fund_addresses]
    app_state["auth"]["accounts"] += [
        {"@type": "/cosmos.auth.v1beta1.BaseAccount", "address": addr}
        for addr in fund_bech32
        if addr not in existing_auth_top
    ]

    bank_balances = bank_genesis_balances(denom, fund_amount, fund_addresses)
    existing_bank = {b["address"] for b in app_state["bank"]["balances"]}
    new_balances = [b for b in bank_balances if b["address"] not in existing_bank]
    app_state["bank"]["balances"] += new_balances

    # cosmos-sdk's InitGenesis panics if total supply doesn't match the sum of
    # every account's balance, so newly-added balances must be reflected here too.
    added_amount = len(new_balances) * fund_amount
    if added_amount:
        supply = app_state["bank"]["supply"]
        for coin in supply:
            if coin["denom"] == denom:
                coin["amount"] = str(int(coin["amount"]) + added_amount)
                break
        else:
            supply.append({"denom": denom, "amount": str(added_amount)})

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
    parser.add_argument(
        "--fund-accounts",
        type=int,
        default=None,
        help=(
            "native-fund account indices 1..fund-accounts, instead of "
            "1..num-accounts. Needed for sender_strategy=unique-per-tx, "
            "where every physical sender (num_accounts * num_txs of them) "
            "needs its own native balance but not its own ERC20 balance."
        ),
    )
    parser.add_argument("--denom", default="basetcro")
    parser.add_argument(
        "--fund-amount",
        type=int,
        default=50 * 10**18,
        help=(
            "native balance per account. 50 CRO: headroom over the "
            "worst-case batch configs' 200 txs x 21000 gas x 5e12 gas_price "
            "= 21 CRO in ante-handler fees per account - too little here "
            "means every batched MsgEthereumTx past the point funds run out "
            "silently fails CheckTx (insufficient funds)"
        ),
    )
    args = parser.parse_args()

    fund_accounts = args.fund_accounts if args.fund_accounts is not None else args.num_accounts
    erc20_addresses = [
        gen_account(args.global_seq, i).address for i in range(1, args.num_accounts + 1)
    ]
    fund_addresses = (
        erc20_addresses
        if fund_accounts == args.num_accounts
        else [
            gen_account(args.global_seq, i).address for i in range(1, fund_accounts + 1)
        ]
    )

    genesis_paths = sorted(
        Path(p)
        for p in glob.glob(f"{args.data_dir}/{args.chain_id}/node*/config/genesis.json")
    )
    if not genesis_paths:
        raise SystemExit(f"no genesis.json found under {args.data_dir}/{args.chain_id}")

    for path in genesis_paths:
        patch_genesis(path, erc20_addresses, fund_addresses, args.denom, args.fund_amount)
        print(
            f"patched {path} with {len(erc20_addresses)} ERC20-funded and "
            f"{len(fund_addresses)} native-funded accounts"
        )


if __name__ == "__main__":
    main()

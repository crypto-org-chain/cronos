import itertools
import json
import tempfile
from pathlib import Path
from typing import List

import jsonmerge
from pydantic.json import pydantic_encoder

from .cli import ChainCommand
from .types import Balance, GenesisAccount, PeerPacket
from .utils import eth_to_bech32, gen_account, patch_json, patch_toml

DEFAULT_DENOM = "basecro"
VAL_ACCOUNT = "validator"
VAL_INITIAL_AMOUNT = Balance(amount="100000000000000000000", denom=DEFAULT_DENOM)
VAL_STAKED_AMOUNT = Balance(amount="10000000000000000000", denom=DEFAULT_DENOM)
ACC_INITIAL_AMOUNT = Balance(amount="10000000000000000000000000", denom=DEFAULT_DENOM)
MEMPOOL_SIZE = 10000
VALIDATOR_GROUP = "validators"
FULLNODE_GROUP = "fullnodes"
CONTAINER_CRONOSD_PATH = "/bin/cronosd"


def init_node(
    cli: ChainCommand,
    home: Path,
    ip: str,
    chain_id: str,
    group: str,
    group_seq: int,
    global_seq: int,
    num_accounts: int = 1,
) -> PeerPacket:
    default_kwargs = {
        "home": home,
        "chain_id": chain_id,
        "keyring_backend": "test",
    }
    cli(
        "init",
        f"{group}-{group_seq}",
        default_denom=DEFAULT_DENOM,
        **default_kwargs,
    )

    val_acct = gen_account(global_seq, 0)
    cli(
        "keys",
        "unsafe-import-eth-key",
        VAL_ACCOUNT,
        val_acct.key.hex(),
        stdin=b"00000000\n",
        **default_kwargs,
    )
    accounts = [
        GenesisAccount(
            address=eth_to_bech32(val_acct.address),
            coins=[VAL_INITIAL_AMOUNT],
        ),
    ] + [
        GenesisAccount(
            address=eth_to_bech32(gen_account(global_seq, i + 1).address),
            coins=[ACC_INITIAL_AMOUNT],
        )
        for i in range(num_accounts)
    ]

    node_id = cli("comet", "show-node-id", **default_kwargs)
    peer_id = f"{node_id}@{ip}:26656"
    peer = PeerPacket(
        ip=str(ip),
        node_id=node_id,
        peer_id=peer_id,
        accounts=accounts,
    )

    if group == VALIDATOR_GROUP:
        peer.gentx = gentx(cli, **default_kwargs)

    return peer


def gen_genesis(
    cli: ChainCommand, leader_home: Path, peers: List[PeerPacket], genesis_patch: dict
):
    accounts = list(itertools.chain(*(peer.accounts for peer in peers)))
    print("adding genesis accounts", len(accounts))
    with tempfile.NamedTemporaryFile() as fp:
        fp.write(json.dumps(accounts, default=pydantic_encoder).encode())
        fp.flush()
        cli(
            "genesis",
            "bulk-add-genesis-account",
            fp.name,
            home=leader_home,
        )
    collect_gen_tx(cli, peers, home=leader_home)
    cli("genesis", "validate", home=leader_home)
    print("genesis validated")
    return patch_json(
        leader_home / "config" / "genesis.json",
        jsonmerge.merge(
            {
                "consensus": {"params": {"block": {"max_gas": "163000000"}}},
                "app_state": {
                    "evm": {"params": {"evm_denom": "basecro"}},
                    "feemarket": {"params": {"no_base_fee": True}},
                },
            },
            genesis_patch,
        ),
    )


def patch_configs(home: Path, peers: str, config_patch: dict, app_patch: dict):
    default_config_patch = {
        "db_backend": "rocksdb",
        "p2p": {"addr_book_strict": False},
        "mempool": {
            "recheck": False,
            "size": MEMPOOL_SIZE,
        },
        "consensus": {"timeout_commit": "1s"},
        "tx_index": {"indexer": "null"},
    }
    default_app_patch = {
        "minimum-gas-prices": "0basecro",
        "index-events": ["ethereum_tx.ethereumTxHash"],
        "memiavl": {
            "enable": True,
            "cache-size": 0,
        },
        "mempool": {"max-txs": MEMPOOL_SIZE},
        "evm": {
            "block-executor": "block-stm",  # or "sequential"
            "block-stm-workers": 0,
            "block-stm-pre-estimate": True,
        },
        "json-rpc": {"enable-indexer": True},
    }
    # update persistent_peers and other configs in config.toml
    config_patch = jsonmerge.merge(
        default_config_patch,
        jsonmerge.merge(
            config_patch,
            {"p2p": {"persistent_peers": peers}},
        ),
    )
    app_patch = jsonmerge.merge(default_app_patch, app_patch)
    patch_toml(home / "config" / "config.toml", config_patch)
    patch_toml(home / "config" / "app.toml", app_patch)


def gentx(cli, **kwargs):
    cli(
        "genesis",
        "add-genesis-account",
        VAL_ACCOUNT,
        str(VAL_INITIAL_AMOUNT),
        **kwargs,
    )
    with tempfile.TemporaryDirectory() as tmp:
        output = Path(tmp) / "gentx.json"
        cli(
            "genesis",
            "gentx",
            VAL_ACCOUNT,
            VAL_STAKED_AMOUNT,
            min_self_delegation=1,
            output_document=output,
            **kwargs,
        )
        return json.loads(output.read_text())


def collect_gen_tx(cli, peers, **kwargs):
    """
    save gentxs to file and call collect-gentxs
    leader node prepare genesis file and broadcast to other nodes
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        tmpdir = Path(tmpdir)
        for i, peer in enumerate(peers):
            if peer.gentx is not None:
                (tmpdir / f"gentx-{i}.json").write_text(json.dumps(peer.gentx))
        cli("genesis", "collect-gentxs", gentx_dir=str(tmpdir), **kwargs)

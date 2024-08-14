import json
import tempfile
from pathlib import Path
from typing import List

from .cli import ChainCommand
from .context import Context
from .network import get_data_ip
from .topology import connect_all
from .types import GenesisAccount, PeerPacket
from .utils import patch_json, patch_toml

VAL_ACCOUNT = "validator"
VAL_INITIAL_AMOUNT = "100000000000000000000basecro"
VAL_STAKED_AMOUNT = "10000000000000000000basecro"
ACC_INITIAL_AMOUNT = "100000000000000000000000basecro"
MEMPOOL_SIZE = 50000
DEFAULT_DENOM = "basecro"
VALIDATOR_GROUP = "validators"
FULLNODE_GROUP = "fullnodes"
CONTAINER_CRONOSD_PATH = "/bin/cronosd"


def bootstrap(ctx: Context, cli) -> PeerPacket:
    home = Path.home() / ".cronos"
    peer = init_node(
        cli,
        home,
        get_data_ip(ctx.params),
        ctx.params.chain_id,
        ctx.params.test_group_id,
        ctx.group_seq,
    )

    data = ctx.sync.publish_subscribe_simple(
        "peers", peer.dict(), ctx.params.test_instance_count
    )
    peers: List[PeerPacket] = [PeerPacket.model_validate(item) for item in data]

    if ctx.is_fullnode_leader:
        # prepare genesis file and publish
        genesis = gen_genesis(cli, home, peers)
        ctx.sync.publish("genesis", genesis)
    else:
        genesis = ctx.sync.subscribe_simple("genesis", 1)[0]
        (home / "config" / "genesis.json").write_text(json.dumps(genesis))
        cli("genesis", "validate", home=home)

    p2p_peers = connect_all(peer, peers)
    patch_configs(home, ctx.params.test_group_id, p2p_peers, "block-stm")
    return peer


def init_node(
    cli: ChainCommand,
    home: Path,
    ip: str,
    chain_id: str,
    group: str,
    group_seq: int,
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
    cli("keys", "add", VAL_ACCOUNT, **default_kwargs)
    cli("keys", "add", "account", **default_kwargs)
    validator_addr = cli("keys", "show", VAL_ACCOUNT, "--address", **default_kwargs)
    account_addr = cli("keys", "show", "account", "--address", **default_kwargs)
    accounts = [
        GenesisAccount(address=validator_addr, balance=VAL_INITIAL_AMOUNT),
        GenesisAccount(address=account_addr, balance=ACC_INITIAL_AMOUNT),
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


def gen_genesis(cli: ChainCommand, leader_home: Path, peers: List[PeerPacket]):
    for peer in peers:
        for account in peer.accounts:
            cli(
                "genesis",
                "add-genesis-account",
                account.address,
                account.balance,
                home=leader_home,
            )
    collect_gen_tx(cli, peers, home=leader_home)
    cli("genesis", "validate", home=leader_home)
    return patch_json(
        leader_home / "config" / "genesis.json",
        {
            "consensus.params.block.max_gas": "81500000",
            "app_state.evm.params.evm_denom": "basecro",
            "app_state.feemarket.params.no_base_fee": True,
        },
    )


def patch_configs(home: Path, group: str, peers: str, block_executor: str):
    # update persistent_peers and other configs in config.toml
    config_patch = {
        "p2p.persistent_peers": peers,
        "p2p.addr_book_strict": False,
        "mempool.recheck": "false",
        "mempool.size": MEMPOOL_SIZE,
        "consensus.timeout_commit": "2s",
    }
    if group == VALIDATOR_GROUP:
        config_patch["tx_index.indexer"] = "null"

    app_patch = {
        "minimum-gas-prices": "0basecro",
        "index-events": ["ethereum_tx.ethereumTxHash"],
        "memiavl.enable": True,
        "mempool.max-txs": MEMPOOL_SIZE,
        "evm.block-executor": block_executor,
    }
    if block_executor == "block-stm":
        app_patch["memiavl.cache-size"] = 0

    patch_toml(home / "config" / "config.toml", config_patch)
    patch_toml(home / "config" / "app.toml", app_patch)


def gentx(cli, **kwargs):
    cli(
        "genesis",
        "add-genesis-account",
        VAL_ACCOUNT,
        VAL_INITIAL_AMOUNT,
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

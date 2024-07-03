import json
import tempfile
from pathlib import Path
from typing import List

from .context import Context
from .network import get_data_ip
from .topology import connect_all
from .types import GenesisAccount, PeerPacket
from .utils import patch_json, patch_toml

VAL_INITIAL_AMOUNT = "100000000000000000000basecro"
VAL_STAKED_AMOUNT = "10000000000000000000basecro"
ACC_INITIAL_AMOUNT = "100000000000000000000basecro"
MEMPOOL_SIZE = 50000


def bootstrap(ctx: Context, cli) -> PeerPacket:
    ip = get_data_ip(ctx.params)
    cli(
        "init",
        f"node{ctx.global_seq}",
        chain_id=ctx.params.chain_id,
        default_denom="basecro",
    )

    cli("keys", "add", "validator", keyring_backend="test")
    cli("keys", "add", "account", keyring_backend="test")
    validator_addr = cli(
        "keys", "show", "validator", "--address", keyring_backend="test"
    )
    account_addr = cli("keys", "show", "account", "--address", keyring_backend="test")
    accounts = [
        GenesisAccount(address=validator_addr, balance=VAL_INITIAL_AMOUNT),
        GenesisAccount(address=account_addr, balance=ACC_INITIAL_AMOUNT),
    ]

    node_id = cli("comet", "show-node-id")
    peer_id = f"{node_id}@{ip}:26656"
    current = PeerPacket(
        ip=str(ip),
        node_id=node_id,
        peer_id=peer_id,
        accounts=accounts,
    )

    if ctx.is_validator:
        current.gentx = gentx(cli, ctx.params.chain_id)

    data = ctx.sync.publish_subscribe_simple(
        "peers", current.dict(), ctx.params.test_instance_count
    )
    peers: List[PeerPacket] = [PeerPacket.model_validate(item) for item in data]

    config_path = Path.home() / ".cronos" / "config"
    if ctx.is_fullnode_leader:
        # prepare genesis file and publish
        for peer in peers:
            for account in peer.accounts:
                cli("genesis", "add-genesis-account", account.address, account.balance)
        collect_gen_tx(cli, peers)
        cli("genesis", "validate")
        genesis = patch_json(
            config_path / "genesis.json",
            {
                "consensus.params.block.max_gas": "81500000",
                "app_state.evm.params.evm_denom": "basecro",
                "app_state.feemarket.params.no_base_fee": True,
            },
        )
        ctx.sync.publish("genesis", genesis)
    else:
        genesis = ctx.sync.subscribe_simple("genesis", 1)[0]
        genesis_file = config_path / "genesis.json"
        genesis_file.write_text(json.dumps(genesis))
        cli("genesis", "validate")

    # update persistent_peers and other configs in config.toml
    config_patch = {
        "p2p.persistent_peers": connect_all(current, peers),
        "mempool.recheck": "false",
        "mempool.size": MEMPOOL_SIZE,
        "consensus.timeout_commit": "2s",
    }
    if ctx.is_validator:
        config_patch["tx_index.indexer"] = "null"

    app_patch = {
        "minimum-gas-prices": "0basecro",
        "index-events": ["ethereum_tx.ethereumTxHash"],
        "memiavl.enable": True,
        "mempool.max-txs": MEMPOOL_SIZE,
    }

    patch_toml(config_path / "config.toml", config_patch)
    patch_toml(config_path / "app.toml", app_patch)

    return current


def gentx(cli, chain_id):
    cli(
        "genesis",
        "add-genesis-account",
        "validator",
        VAL_INITIAL_AMOUNT,
        keyring_backend="test",
    )
    output = Path("gentx.json")
    cli(
        "genesis",
        "gentx",
        "validator",
        VAL_STAKED_AMOUNT,
        min_self_delegation=1,
        chain_id=chain_id,
        output_document=output,
        keyring_backend="test",
    )
    return json.loads(output.read_text())


def collect_gen_tx(cli, peers):
    """
    save gentxs to file and call collect-gentxs
    leader node prepare genesis file and broadcast to other nodes
    """
    with tempfile.TemporaryDirectory() as tmpdir:
        tmpdir = Path(tmpdir)
        for i, peer in enumerate(peers):
            if peer.gentx is not None:
                (tmpdir / f"gentx-{i}.json").write_text(json.dumps(peer.gentx))
        cli("genesis", "collect-gentxs", gentx_dir=str(tmpdir))

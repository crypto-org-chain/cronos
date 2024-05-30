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
        GenesisAccount(
            address=validator_addr,
            balance=VAL_INITIAL_AMOUNT,
        ),
        GenesisAccount(
            address=account_addr,
            balance=ACC_INITIAL_AMOUNT,
        ),
    ]

    node_id = cli("comet", "show-node-id")
    peer_id = f"{node_id}@{ip}:26656"
    peer = PeerPacket(
        ip=str(ip),
        node_id=node_id,
        peer_id=peer_id,
        accounts=accounts,
    )

    if ctx.is_validator:
        peer.gentx = gentx(cli, ctx.params.chain_id)

    data = ctx.sync.publish_subscribe_simple(
        "peers", peer.dict(), ctx.params.test_instance_count
    )
    peers: List[PeerPacket] = [PeerPacket.model_validate(item) for item in data]

    config_path = Path.home() / ".cronos" / "config"
    if ctx.is_leader:
        # prepare genesis file and publish
        for peer in peers:
            for account in peer.accounts:
                if ctx.is_validator and account.address == validator_addr:
                    # if leader is also validator, it's validator account is already
                    # added in gentx
                    continue
                cli("genesis", "add-genesis-account", account.address, account.balance)
        collect_gen_tx(cli, peers)
        cli("genesis", "validate")
        genesis = patch_json(
            config_path / "genesis.json",
            {
                "consensus.params.block.max_gas": "81500000",
                "app_state.evm.params.evm_denom": "basecro",
            },
        )
        ctx.sync.publish("genesis", genesis)
    else:
        genesis = ctx.sync.subscribe_simple("genesis", 1)[0]
        genesis_file = config_path / "genesis.json"
        genesis_file.write_text(json.dumps(genesis))
        cli("genesis", "validate")

    # update persistent_peers in config.toml
    patch_toml(
        config_path / "config.toml",
        {
            "p2p.persistent_peers": connect_all(peer, peers),
            "mempool.recheck": "false",
        },
    )
    patch_toml(
        config_path / "app.toml",
        {
            "minimum-gas-prices": "0basecro",
        },
    )

    return peer


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

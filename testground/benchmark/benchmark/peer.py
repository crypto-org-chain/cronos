import json
import pathlib
import tempfile
from pathlib import Path
from typing import List, Optional

from pydantic import BaseModel

from .context import Context
from .network import get_data_ip

INITIAL_AMOUNT = "10000000basecro"
STAKED_AMOUNT = "10000000basecro"


class GenesisAccount(BaseModel):
    address: str
    balance: str


class PeerPacket(BaseModel):
    ip: str
    node_id: str
    peer_id: str
    accounts: List[GenesisAccount]
    gentx: Optional[dict] = None


def bootstrap(ctx: Context, cli) -> List[PeerPacket]:
    ip = get_data_ip(ctx.params)
    cli(
        "init",
        f"node{ctx.global_seq}",
        chain_id=ctx.params.chain_id,
    )
    cli("keys", "add", "validator", keyring_backend="test")

    validator_addr = show_address(cli, "validator")
    accounts = [
        GenesisAccount(
            address=validator_addr,
            balance=INITIAL_AMOUNT,
        )
    ]

    node_id = get_node_id(cli)
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
    peers: List[PeerPacket] = list(map(PeerPacket.model_validate, data))

    if ctx.is_leader:
        # prepare genesis file and publish
        for peer in peers:
            for account in peer.accounts:
                if ctx.is_validator and account.address == validator_addr:
                    # if leader is also validator, it's validator account is already
                    # added in gentx
                    continue
                cli("genesis", "add-genesis-account", account.address, account.balance)
        genesis = collect_gen_tx(cli, peers)
        cli("genesis", "validate")
        ctx.sync.publish("genesis", genesis)
    else:
        genesis = ctx.sync.subscribe_simple("genesis", 1)[0]
        (pathlib.Path.home() / ".cronos" / "config" / "genesis.json").write_text(
            json.dumps(genesis)
        )

    return peers, genesis


def gentx(cli, chain_id):
    cli(
        "genesis",
        "add-genesis-account",
        "validator",
        INITIAL_AMOUNT,
        keyring_backend="test",
    )
    output = Path("gentx.json")
    cli(
        "genesis",
        "gentx",
        "validator",
        STAKED_AMOUNT,
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
        stdout = cli("genesis", "collect-gentxs", gentx_dir=str(tmpdir))
        return json.loads(stdout.decode())


def get_node_id(cli):
    return cli("comet", "show-node-id").decode().strip()


def show_address(cli, name):
    return (
        cli("keys", "show", name, "--address", keyring_backend="test").decode().strip()
    )

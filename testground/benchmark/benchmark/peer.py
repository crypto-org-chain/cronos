import base64
import hashlib
import itertools
import json
import tempfile
from pathlib import Path
from typing import List, Optional

import jsonmerge
from pydantic.json import pydantic_encoder

from . import erc20
from .cli import ChainCommand
from .types import Balance, GenesisAccount, PeerPacket
from .utils import (
    DEFAULT_DENOM,
    bech32_to_eth,
    eth_to_bech32,
    gen_account,
    merge_genesis,
    patch_genesis,
    patch_toml,
)

VAL_ACCOUNT = "validator"
VAL_INITIAL_AMOUNT = Balance(amount="100000000000000000000", denom=DEFAULT_DENOM)
VAL_STAKED_AMOUNT = Balance(amount="10000000000000000000", denom=DEFAULT_DENOM)
ACC_INITIAL_AMOUNT = Balance(amount="10000000000000000000000000", denom=DEFAULT_DENOM)
MEMPOOL_SIZE = 10000
VALIDATOR_GROUP = "validators"
FULLNODE_GROUP = "fullnodes"
CONTAINER_CRONOSD_PATH = "/bin/cronosd"

# base58btc alphabet
_B58 = b"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"


def _b58encode(data: bytes) -> str:
    n = int.from_bytes(data, "big")
    out = bytearray()
    while n > 0:
        n, r = divmod(n, 58)
        out.append(_B58[r])
    # leading zeros → leading '1'
    for b in data:
        if b == 0:
            out.append(_B58[0])
        else:
            break
    return out[::-1].decode()


def libp2p_id_from_node_key(node_key_path: Path) -> str:
    """Derive libp2p peer ID from CometBFT Ed25519 node_key.json.

    Matches go-libp2p `peer.IDFromPublicKey` for an Ed25519 key:
      - protobuf-marshal PublicKey{Type=Ed25519(1), Data=pub32}
      - keys ≤ 42 bytes use identity multihash (code 0x00)
      - otherwise sha256 multihash (code 0x12)
      - base58btc encode
    """
    nk = json.loads(node_key_path.read_text())
    priv = base64.b64decode(nk["priv_key"]["value"])
    # tendermint Ed25519 priv: 64 bytes (seed-expanded || pub)
    pub = priv[32:]
    if len(pub) != 32:
        raise ValueError(f"unexpected ed25519 pub length: {len(pub)}")
    # protobuf wire: field 1 varint=1 ("\x08\x01"); field 2 lendelim 32B
    marshaled = b"\x08\x01\x12\x20" + pub
    if len(marshaled) <= 42:
        # identity multihash: code 0x00 + length + data
        mh = b"\x00" + bytes([len(marshaled)]) + marshaled
    else:
        h = hashlib.sha256(marshaled).digest()
        mh = b"\x12\x20" + h
    return _b58encode(mh)


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
    libp2p_id = libp2p_id_from_node_key(home / "config" / "node_key.json")
    peer = PeerPacket(
        ip=str(ip),
        node_id=node_id,
        peer_id=peer_id,
        libp2p_id=libp2p_id,
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

    # skip the validator account of the first node, because we use that node's home,
    # and it's already added
    accounts = accounts[1:]

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

    evm_accounts, auth_accounts = erc20.genesis_accounts(
        erc20.CONTRACT_ADDRESS, [bech32_to_eth(acct.address) for acct in accounts]
    )
    return patch_genesis(
        leader_home / "config" / "genesis.json",
        merge_genesis(
            {
                "consensus": {"params": {"block": {"max_gas": "163000000"}}},
                "app_state": {
                    "evm": {
                        "params": {"evm_denom": DEFAULT_DENOM},
                        "accounts": evm_accounts,
                    },
                    "auth": {"accounts": auth_accounts},
                    "feemarket": {"params": {"no_base_fee": True}},
                },
            },
            genesis_patch,
        ),
    )


def patch_configs(
    home: Path,
    peers: str,
    config_patch: dict,
    app_patch: dict,
    libp2p_peers: Optional[List[dict]] = None,
):
    default_config_patch = {
        "db_backend": "rocksdb",
        "p2p": {"addr_book_strict": False},
        "mempool": {
            "recheck": False,
            "size": MEMPOOL_SIZE,
        },
        "consensus": {
            "timeout_commit": "1s",
            "timeout_propose": "500ms",
            "timeout_prevote": "300ms",
            "timeout_precommit": "300ms",
        },
        "tx_index": {"indexer": "null"},
        "instrumentation": {"prometheus": True},
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
        "telemetry": {
            "enabled": True,
            "prometheus-retention-time": 600,
        },
    }
    libp2p_enabled = bool(
        config_patch and config_patch.get("p2p", {}).get("libp2p", {}).get("enabled")
    )
    if libp2p_enabled and libp2p_peers is not None:
        peer_patch = {"p2p": {"libp2p": {"bootstrap_peers": libp2p_peers}}}
    else:
        peer_patch = {"p2p": {"persistent_peers": peers}}
    config_patch = jsonmerge.merge(
        default_config_patch,
        jsonmerge.merge(config_patch, peer_patch),
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

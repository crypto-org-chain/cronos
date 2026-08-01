"""libp2p peer-ID derivation and bootstrap_peers generation for devnet bring-up.

Ported from `testground/benchmark/benchmark/peer.py`
(`libp2p_id_from_node_key`) and `topology.py` (`connect_all_libp2p`) —
remote-benchmark has no import path into that package, so the small,
self-contained derivation logic is copied rather than imported.
"""

import base64
import json
from pathlib import Path

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


def libp2p_id_from_node_key(node_key_path) -> str:
    """Derive libp2p peer ID from CometBFT Ed25519 node_key.json.

    Matches go-libp2p `peer.IDFromPublicKey` for an Ed25519 key:
      - protobuf-marshal PublicKey{Type=Ed25519(1), Data=pub32}
      - identity multihash (code 0x00), since go-libp2p hashes with sha256 only
        above 42 marshaled bytes and an Ed25519 key marshals to 36
      - base58btc encode
    """
    nk = json.loads(Path(node_key_path).read_text())
    priv = base64.b64decode(nk["priv_key"]["value"])
    # tendermint Ed25519 priv: 64 bytes (seed-expanded || pub)
    pub = priv[32:]
    if len(pub) != 32:
        raise ValueError(f"unexpected ed25519 pub length: {len(pub)}")
    # protobuf wire: field 1 varint=1 ("\x08\x01"); field 2 lendelim 32B
    marshaled = b"\x08\x01\x12\x20" + pub
    mh = b"\x00" + bytes([len(marshaled)]) + marshaled
    return _b58encode(mh)


def bootstrap_peers(nodes, port=26656):
    """Derive libp2p peer IDs for `nodes` and build each node's bootstrap_peers list.

    `nodes` is [{"name", "ip", "node_key_path"}, ...]. Returns
    {name: [entry, ...]}, one entry per *other* node, in the shape CometBFT's
    `[[p2p.libp2p.bootstrap_peers]]` TOML expects (same shape as testground's
    `connect_all_libp2p`).
    """
    ids = {node["name"]: libp2p_id_from_node_key(node["node_key_path"]) for node in nodes}
    result = {}
    for node in nodes:
        result[node["name"]] = [
            {
                "host": f"{other['ip']}:{port}",
                "id": ids[other["name"]],
                "persistent": True,
                "unconditional": True,
            }
            for other in nodes
            if other["name"] != node["name"]
        ]
    return result

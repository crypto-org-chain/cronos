from typing import List

from .types import PeerPacket


def connect_all(peer, peers: List[PeerPacket]) -> str:
    """
    connect the peer to all the other peers

    returns the value for persistent-peers config
    """
    return ",".join(other.peer_id for other in peers if other.peer_id != peer.peer_id)


def connect_all_libp2p(peer, peers: List[PeerPacket], port: int = 26656) -> List[dict]:
    """connect the peer to all others via libp2p bootstrap_peers entries.

    returns list of dicts suitable for [[p2p.libp2p.bootstrap_peers]] TOML.
    """
    out = []
    for other in peers:
        if other.libp2p_id == peer.libp2p_id:
            continue
        if not other.libp2p_id:
            continue
        out.append(
            {
                "host": f"{other.ip}:{port}",
                "id": other.libp2p_id,
                "persistent": True,
                "unconditional": True,
            }
        )
    return out

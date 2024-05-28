from typing import List

from .types import PeerPacket


def connect_all(peer, peers: List[PeerPacket]) -> str:
    """
    connect the peer to all the other peers

    returns the value for persistent-peers config
    """
    return ",".join(other.peer_id for other in peers if other.peer_id != peer.peer_id)

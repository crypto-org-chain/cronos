from typing import List, Optional

from pydantic import BaseModel


class GenesisAccount(BaseModel):
    address: str
    balance: str


class PeerPacket(BaseModel):
    ip: str
    node_id: str
    peer_id: str
    accounts: List[GenesisAccount]
    gentx: Optional[dict] = None

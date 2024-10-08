from typing import List, Optional

from pydantic import BaseModel

from .utils import bech32_to_eth


class Balance(BaseModel):
    amount: str
    denom: str

    def __str__(self):
        return f"{self.amount}{self.denom}"


class GenesisAccount(BaseModel):
    address: str
    coins: List[Balance]

    @property
    def eth_address(self) -> str:
        return bech32_to_eth(self.address)


class PeerPacket(BaseModel):
    ip: str
    node_id: str
    peer_id: str
    accounts: List[GenesisAccount]
    gentx: Optional[dict] = None

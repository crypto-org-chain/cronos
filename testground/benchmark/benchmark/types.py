from typing import List, Optional

from pydantic import BaseModel

from .utils import bech32_to_eth, parse_coins

DEFAULT_DENOM = "basecro"


class GenesisAccount(BaseModel):
    address: str
    balance: str

    @property
    def eth_address(self) -> str:
        return bech32_to_eth(self.address)


class PeerPacket(BaseModel):
    ip: str
    node_id: str
    peer_id: str
    accounts: List[GenesisAccount]
    gentx: Optional[dict] = None

    def bulk_genesis_accounts(self):
        """
        convert accounts to the format required in `bulk-add-genesis-account` command
        """
        return [
            {
                "address": acct.address,
                "coins": [parse_coins(acct.balance)],
            }
            for acct in self.accounts
        ]

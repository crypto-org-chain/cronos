from dataclasses import dataclass

from eth_account import Account
from web3 import Web3

from .config import funded_key, load_config


@dataclass
class Node:
    name: str
    w3: Web3
    rpc: str


@dataclass
class Devnet:
    nodes: list[Node]
    funded_account: Account | None
    chain_id: int | None = None

    def verify_chain_ids(self) -> None:
        """Probes broadcast real value-bearing txs with the funded key, so confirm
        the endpoints belong to the chain the config names before touching them.
        No-op when the caller supplied no expected chain id."""
        if self.chain_id is None:
            return
        for node in self.nodes:
            actual = node.w3.eth.chain_id
            if actual != self.chain_id:
                raise ValueError(
                    f"{node.name} reports chain_id {actual}, "
                    f"config expects {self.chain_id}"
                )


def load_devnet(config_path: str) -> Devnet:
    cfg = load_config(config_path)
    nodes = [
        Node(n.name, Web3(Web3.HTTPProvider(n.json_rpc)), n.rpc) for n in cfg.nodes
    ]
    key = funded_key()
    return Devnet(nodes, Account.from_key(key) if key else None, cfg.chain_id)

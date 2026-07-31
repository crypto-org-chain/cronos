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


def load_devnet(config_path: str) -> Devnet:
    cfg = load_config(config_path)
    nodes = [
        Node(n.name, Web3(Web3.HTTPProvider(n.json_rpc)), n.rpc) for n in cfg.nodes
    ]
    key = funded_key()
    return Devnet(nodes, Account.from_key(key) if key else None)

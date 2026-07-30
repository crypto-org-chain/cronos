import os
from pathlib import Path

import yaml
from pydantic import BaseModel, Field


class NodeConfig(BaseModel):
    name: str
    rpc: str
    json_rpc: str


class Config(BaseModel):
    nodes: list[NodeConfig] = Field(min_length=1)
    chain_id: int


def load_config(path: str) -> Config:
    data = yaml.safe_load(Path(path).read_text())
    return Config.model_validate(data)


def funded_key() -> str | None:
    """Private key of the account funded on the target devnet(s).

    Deliberately read from an env var, not the config file, so a live
    devnet's funded key is never committed alongside its config.
    """
    return os.environ.get("DEVNET_FUNDED_KEY")

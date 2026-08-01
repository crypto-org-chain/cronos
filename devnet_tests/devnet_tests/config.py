import os
from pathlib import Path

import yaml
from pydantic import BaseModel, Field, model_validator


class NodeConfig(BaseModel):
    name: str
    rpc: str
    json_rpc: str


class Config(BaseModel):
    nodes: list[NodeConfig] = Field(min_length=1)
    chain_id: int

    @model_validator(mode="after")
    def _nodes_must_be_distinct(self):
        """Two entries sharing an endpoint are one node under two names, which
        makes every cross-node diff compare a node to itself and pass by
        construction."""
        for field in ("name", "rpc", "json_rpc"):
            seen = set()
            for node in self.nodes:
                value = getattr(node, field)
                if value in seen:
                    raise ValueError(f"duplicate {field} across nodes: {value}")
                seen.add(value)
        return self


def load_config(path: str) -> Config:
    data = yaml.safe_load(Path(path).read_text())
    return Config.model_validate(data)


def funded_key() -> str | None:
    """Private key of the account funded on the target devnet(s).

    Deliberately read from an env var, not the config file, so a live
    devnet's funded key is never committed alongside its config.
    """
    return os.environ.get("DEVNET_FUNDED_KEY")

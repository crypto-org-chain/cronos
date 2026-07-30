import json
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, Field, model_validator

from .transaction import TX_TYPES
from .utils import DEFAULT_DENOM


class Endpoint(BaseModel):
    name: str
    rpc: str
    json_rpc: str
    # Operator-declared config not observable over RPC (mempool.type,
    # libp2p enabled, Block-STM workers, ...), recorded verbatim into the
    # node fingerprint by results.py.
    node_config: dict = Field(default_factory=dict)
    # node_exporter base URL for this node's host (disk/network I/O). Host-level,
    # so it's per-endpoint rather than the single global `telemetry` URL.
    node_exporter: str | None = None


class Config(BaseModel):
    endpoints: list[Endpoint] = Field(min_length=1)
    mode: Literal["cosmos", "eth"] = "cosmos"
    chain_id: int
    evm_denom: str = DEFAULT_DENOM
    gas_price: int = 1000000000
    global_seq: int = 0
    tx_type: str = "simple-transfer"
    # Only used when tx_type == "weighted-mix": {tx_type_name: weight}.
    mix_weights: dict[str, float] | None = None
    msg_version: str = "1.4"
    num_accounts: int = 100
    num_txs: int = 1
    sender_strategy: Literal["reuse", "unique-per-tx"] = "reuse"
    batch_size: int = 1
    send_batch_size: int = 500
    send_interval: float = 0.5
    telemetry: str | None = None

    @model_validator(mode="after")
    def _check_mix_weights(self) -> "Config":
        if self.tx_type != "weighted-mix":
            return self
        if not self.mix_weights:
            raise ValueError("mix_weights must be set when tx_type is weighted-mix")
        valid_names = TX_TYPES.keys() - {"weighted-mix"}
        unknown = self.mix_weights.keys() - valid_names
        if unknown:
            raise ValueError(f"mix_weights has unknown tx types: {sorted(unknown)}")
        if any(w < 0 for w in self.mix_weights.values()):
            raise ValueError("mix_weights must not contain negative weights")
        if sum(self.mix_weights.values()) <= 0:
            raise ValueError("mix_weights must sum to a positive total")
        return self

    @property
    def rpcs(self) -> list[str]:
        return [e.rpc for e in self.endpoints]

    @property
    def json_rpcs(self) -> list[str]:
        return [e.json_rpc for e in self.endpoints]

    @property
    def primary(self) -> Endpoint:
        return self.endpoints[0]


def load_config(path: str) -> Config:
    text = Path(path).read_text()
    if path.endswith(".json"):
        data = json.loads(text)
    else:
        data = yaml.safe_load(text)
    return Config.model_validate(data)

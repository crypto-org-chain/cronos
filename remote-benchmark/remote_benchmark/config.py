import json
from pathlib import Path
from typing import Literal

import yaml
from pydantic import BaseModel, Field

from .utils import DEFAULT_DENOM


class Endpoint(BaseModel):
    name: str
    rpc: str
    json_rpc: str


class Config(BaseModel):
    endpoints: list[Endpoint] = Field(min_length=1)
    mode: Literal["cosmos", "eth"] = "cosmos"
    chain_id: int
    evm_denom: str = DEFAULT_DENOM
    gas_price: int = 1000000000
    global_seq: int = 0
    tx_type: str = "simple-transfer"
    msg_version: str = "1.4"
    num_accounts: int = 100
    num_txs: int = 1
    sender_strategy: Literal["reuse", "unique-per-tx"] = "reuse"
    batch_size: int = 1
    send_batch_size: int = 500
    send_interval: float = 0.5
    telemetry: str | None = None

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

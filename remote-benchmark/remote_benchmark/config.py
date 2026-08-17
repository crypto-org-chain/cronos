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
    # Extra tunnels (e.g. independent `ssh -L` processes) to the same node's
    # rpc/json_rpc, so hot polling paths can round-robin across separate
    # underlying TCP connections instead of funneling all churn through one -
    # a single SSH-multiplexed connection head-of-line-blocks every channel
    # riding it on a hiccup, including brand-new ones.
    rpc_pool: list[str] = Field(default_factory=list)
    json_rpc_pool: list[str] = Field(default_factory=list)
    # Operator-declared config not observable over RPC (mempool.type,
    # libp2p enabled, Block-STM workers, ...), recorded verbatim into the
    # node fingerprint by results.py.
    node_config: dict = Field(default_factory=dict)
    # node_exporter base URL for this node's host (disk/network I/O). Host-level,
    # so it's per-endpoint rather than the single global `telemetry` URL.
    node_exporter: str | None = None

    @property
    def rpc_candidates(self) -> list[str]:
        return [self.rpc] + self.rpc_pool

    @property
    def json_rpc_candidates(self) -> list[str]:
        return [self.json_rpc] + self.json_rpc_pool


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
    # Per-host aiohttp connector cap; see CONNECTION_POOL_PER_HOST in
    # transaction.py. 200 protects a tunneled ssh -L endpoint from a connection
    # burst; raise it for direct-loopback endpoints with no tunnel to protect.
    # Applies per sender process, so the burst one host actually sees is
    # send_workers * send_conn_per_host - divide it before raising send_workers
    # on a tunneled endpoint.
    send_conn_per_host: int = 200
    # >1 fans sending out across OS processes via send_multiprocess: a single
    # event loop is CPU-bound on JSON-RPC serialization, not network I/O, so
    # raising conn_per_host alone can't buy more throughput past that wall.
    # Only the benchmark run path honors this; send-txs and soak always send
    # from one process.
    send_workers: int = 1
    # Seconds to keep waiting for the generated txs to commit after the last
    # send. Per-testcase because the floor is set by how many blocks the
    # workload needs: batched txs pack ~100x more gas per Cosmos tx, so they
    # fill far more blocks than the same tx count sent unbatched, and each of
    # those blocks is gas-saturated rather than near-empty.
    commit_timeout: float = 120
    # Throwaway txs sent per account before the measured load, to prime the
    # mempool/JIT/connection-pool warm state instead of paying that cost
    # inside the measured window. 0 disables warm-up.
    warmup_txs: int = 0
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
    def rpc_candidates(self) -> list[str]:
        """Every rpc URL across every endpoint, tunnel pools included - what
        load-sending should round-robin across, as opposed to a single
        endpoint's own `rpc_candidates` used for polling that endpoint."""
        return [url for e in self.endpoints for url in e.rpc_candidates]

    @property
    def json_rpc_candidates(self) -> list[str]:
        return [url for e in self.endpoints for url in e.json_rpc_candidates]

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

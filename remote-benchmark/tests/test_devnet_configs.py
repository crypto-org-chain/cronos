"""Guards on the devnet-local jsonnet configs and the load yamls they pair with.

Every check here corresponds to a defect that shipped silently: a mempool cap
below the offered load drops the overflow through broadcast_tx_async, which
never waits for CheckTx, so the run reports a partial commit count with no
error to explain it.
"""

import json
import shutil
import subprocess
from pathlib import Path

import pytest
import yaml

CONFIG_DIR = (
    Path(__file__).resolve().parents[1] / "scripts" / "devnet-local" / "configs"
)
CHAIN_KEY = "cronos_777-1"
VALIDATOR_COUNTS = (1, 3, 5)
VARIANTS = ("", "-legacy-mempool")

pytestmark = pytest.mark.skipif(
    shutil.which("jsonnet") is None, reason="jsonnet binary not installed"
)


def _devnet(validators, variant=""):
    path = CONFIG_DIR / f"benchmark-{validators}val{variant}.jsonnet"
    return json.loads(subprocess.check_output(["jsonnet", str(path)]))[CHAIN_KEY]


def _load_configs(validators):
    for path in sorted(CONFIG_DIR.glob(f"{validators}val-*.yaml")):
        yield path.name, yaml.safe_load(path.read_text())


def _envelopes(cfg):
    """Cosmos txs the workload puts in flight, which is what a mempool counts -
    batch_size inner MsgEthereumTx ride in one envelope."""
    return cfg["num_accounts"] * cfg["num_txs"] // cfg.get("batch_size", 1)


ALL_DEVNETS = [(v, variant) for v in VALIDATOR_COUNTS for variant in VARIANTS]


@pytest.mark.parametrize("validators,variant", ALL_DEVNETS)
def test_app_mempool_cap_is_never_negative(validators, variant):
    # app/app.go:463 wires PriorityNonceMempool only when max-txs >= 0; a
    # negative value silently falls back to NoOpMempool, which selects txs in
    # gossip-arrival order and so breaks nonce-sequential workloads.
    max_txs = _devnet(validators, variant)["app-config"]["mempool"]["max-txs"]
    assert max_txs >= 0


@pytest.mark.parametrize("validators,variant", ALL_DEVNETS)
def test_app_toml_mempool_carries_only_the_field_it_has(validators, variant):
    # cosmos-sdk's MempoolConfig has exactly one field. A type/broadcast/reap_*
    # copy here reads as load-bearing tuning but is dropped on unmarshal - the
    # real ones live under config_patch's CometBFT mempool.
    assert set(_devnet(validators, variant)["app-config"]["mempool"]) == {"max-txs"}


@pytest.mark.parametrize("validators", VALIDATOR_COUNTS)
def test_reap_gas_matches_the_block_gas_cap(validators):
    devnet = _devnet(validators)
    block_gas = int(devnet["genesis"]["consensus"]["params"]["block"]["max_gas"])
    assert devnet["config"]["mempool"]["reap_max_gas"] == block_gas


@pytest.mark.parametrize("validators", VALIDATOR_COUNTS)
def test_mempool_holds_every_workload_it_will_be_run_against(validators):
    devnet = _devnet(validators)
    # mempool.type=app bypasses CometBFT's mempool, so its size is inert and
    # the app-side cap is the only one that binds.
    cap = devnet["app-config"]["mempool"]["max-txs"]
    too_small = {
        name: _envelopes(cfg)
        for name, cfg in _load_configs(validators)
        if _envelopes(cfg) > cap
    }
    assert not too_small, f"app-config.mempool.max-txs={cap} is below: {too_small}"


@pytest.mark.parametrize("validators", VALIDATOR_COUNTS)
def test_gossip_and_recheck_budget_covers_one_full_block(validators):
    devnet = _devnet(validators)
    block_gas = int(devnet["genesis"]["consensus"]["params"]["block"]["max_gas"])
    # 21000 is simple-transfer's gas, the cheapest tx type and so the one that
    # packs the most into a block.
    txs_per_block = block_gas // 21000
    budget = devnet["app-config"]["cronos"]["mempool-txs-per-block"]
    # 0 means "use cronos's 2900 mainnet default", which starves both the
    # gossip-reap and recheck-batch paths on a benchmark-sized block.
    assert budget >= txs_per_block


@pytest.mark.parametrize("validators", VALIDATOR_COUNTS)
def test_load_configs_agree_with_the_devnet_they_run_against(validators):
    devnet = _devnet(validators)
    evm_chain_id = int(CHAIN_KEY.split("_")[1].split("-")[0])
    for name, cfg in _load_configs(validators):
        assert cfg["chain_id"] == evm_chain_id, name
        assert cfg["evm_denom"] == devnet["genesis"]["app_state"]["evm"]["params"][
            "evm_denom"
        ], name
        endpoints = cfg["endpoints"]
        assert len(endpoints) == validators, name
        for i, endpoint in enumerate(endpoints):
            base = 26650 + i * 10
            assert endpoint["rpc"].endswith(f":{base + 7}"), name
            assert endpoint["json_rpc"].endswith(f":{base + 1}"), name


def test_unique_variants_match_the_control_they_are_compared_against():
    # The -unique configs exist only to isolate same-sender BlockSTM
    # dependencies, so any count difference from their non-unique twin
    # confounds the very thing they measure.
    for path in sorted(CONFIG_DIR.glob("*-unique.yaml")):
        control = yaml.safe_load(
            (CONFIG_DIR / path.name.replace("-unique", "")).read_text()
        )
        unique = yaml.safe_load(path.read_text())
        assert unique["sender_strategy"] == "unique-per-tx", path.name
        for field in ("num_accounts", "num_txs", "batch_size", "tx_type"):
            assert unique[field] == control[field], f"{path.name}: {field}"


def test_warmup_is_not_configured_where_the_runner_would_skip_it():
    # Warm-up needs a sender range disjoint from the measured load, which
    # unique-per-tx has none of - genesis funds exactly the senders the load
    # uses. Setting warmup_txs there reads as tuning but does nothing.
    for path in sorted(CONFIG_DIR.glob("*.yaml")):
        cfg = yaml.safe_load(path.read_text())
        if cfg.get("sender_strategy") == "unique-per-tx":
            assert not cfg.get("warmup_txs"), path.name

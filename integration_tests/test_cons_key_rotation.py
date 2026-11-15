import json
import shutil
from pathlib import Path

import pytest
import requests
from pystarport import ports

from .cosmoscli import module_address
from .network import Cronos, setup_custom_cronos
from .utils import (
    bech32_to_eth,
    find_log_event_attrs,
    submit_gov_proposal,
    wait_for_block,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.gov

ROTATED = 3
MLDSA65 = "ml_dsa_65"


@pytest.fixture(scope="module")
def cronos_rotation(tmp_path_factory):
    yield from setup_custom_cronos(
        tmp_path_factory.mktemp("cons_key_rotation"),
        27200,
        Path(__file__).parent / "configs/cons_key_rotation.jsonnet",
    )


def comet_rpc(cronos, i, path, **params):
    url = f"http://127.0.0.1:{ports.rpc_port(cronos.base_port(i))}/{path}"
    return requests.get(url, params=params).json()["result"]


def comet_validators(cronos, i=0):
    return comet_rpc(cronos, i, "validators", per_page=100)["validators"]


def gen_mldsa65_key(cli, home):
    cli.raw(
        "init",
        "pq",
        consensus_key_algo=MLDSA65,
        chain_id=cli.chain_id,
        home=home,
    )
    pubkey = json.loads(cli.raw("comet", "show-validator", home=home))
    return pubkey, home / "config/priv_validator_key.json"


def update_validator_pubkey_types(cronos, pub_key_types):
    params = cronos.cosmos_cli().query_params("consensus")
    msg = "/cosmos.consensus.v1.MsgUpdateParams"
    # x/consensus rejects the msg unless block, evidence and validator are all set
    content = {
        "@type": msg,
        "authority": module_address("gov"),
        "block": params["block"],
        "evidence": params["evidence"],
        "validator": {"pub_key_types": pub_key_types},
    }
    for field in ("abci", "auth"):
        if params.get(field):
            content[field] = params[field]
    submit_gov_proposal(cronos, msg, messages=[content])


def test_rotate_ed25519_to_mldsa65(cronos_rotation: Cronos, tmp_path):
    cronos = cronos_rotation
    cli = cronos.cosmos_cli(ROTATED)
    w3 = cronos.node_w3(0)
    params = cli.query_params("consensus")
    assert params["validator"]["pub_key_types"] == ["ed25519"]

    new_pubkey, new_key_file = gen_mldsa65_key(cli, tmp_path)
    assert new_pubkey["@type"] == "/cosmos.crypto.mldsa65.PubKey"

    rsp = cli.rotate_cons_pubkey(new_pubkey)
    assert rsp["code"] != 0
    assert "not supported" in rsp["raw_log"], rsp["raw_log"]

    update_validator_pubkey_types(cronos, ["ed25519", MLDSA65])
    params = cli.query_params("consensus")
    assert params["validator"]["pub_key_types"] == ["ed25519", MLDSA65]

    old_key = json.loads(cli.raw("comet", "show-validator", home=cli.data_dir))["key"]
    old_comet_addr = next(
        v["address"]
        for v in comet_validators(cronos)
        if v["pub_key"]["value"] == old_key
    )
    operator = cli.address("validator")
    val_addr = cli.address("validator", bech="val")
    stake_before = cli.balance(operator, "stake")

    rsp = cli.rotate_cons_pubkey(new_pubkey)
    assert rsp["code"] == 0, rsp["raw_log"]
    ev = find_log_event_attrs(rsp["events"], "rotate_cons_pubkey")
    assert ev["fee_burned"] == "1stake", ev
    apply_height = int(ev["apply_height"])
    rotation_height = int(rsp["height"])
    assert cli.balance(operator, "stake") == stake_before - 1
    assert cli.balance(module_address("key_rotation_fee_pool"), "stake") == 0

    # 3 of 4 equal validators keep consensus while the rotated node restarts
    node = f"cronos_777-1-node{ROTATED}"
    cronos.supervisorctl("stop", node)
    shutil.copy(new_key_file, cli.data_dir / "config/priv_validator_key.json")
    wait_for_block(cronos.cosmos_cli(0), apply_height)
    cronos.supervisorctl("start", node)
    wait_for_port(ports.evmrpc_port(cronos.base_port(ROTATED)))

    validators = comet_validators(cronos)
    rotated = [v for v in validators if v["pub_key"]["value"] == new_pubkey["key"]]
    assert len(rotated) == 1, validators
    new_comet_addr = rotated[0]["address"]
    assert rotated[0]["pub_key"]["type"] == "cometbft/PubKeyMlDsa65"
    assert old_comet_addr not in [v["address"] for v in validators]

    val = cli.validator(val_addr)
    val = val.get("validator") or val
    assert val["consensus_pubkey"] == {
        "type": new_pubkey["@type"],
        "value": new_pubkey["key"],
    }

    end = wait_for_new_blocks(cronos.cosmos_cli(0), 12)
    wait_for_block(cli, end)
    hashes = {
        comet_rpc(cronos, i, "block", height=end)["block_id"]["hash"]
        for i in range(len(cronos.config["validators"]))
    }
    assert len(hashes) == 1, hashes

    expected_miner = "0x" + bech32_to_eth(operator)
    signed, proposed = False, []
    for height in range(rotation_height, end + 1):
        block = comet_rpc(cronos, 0, "block", height=height)["block"]
        signers = [s["validator_address"] for s in block["last_commit"]["signatures"]]
        signed |= new_comet_addr in signers
        if block["header"]["proposer_address"] in (old_comet_addr, new_comet_addr):
            proposed.append(height)
            assert w3.eth.get_block(height).miner.lower() == expected_miner, height
    assert signed, "rotated validator never signed with the ML-DSA-65 key"
    assert any(h >= apply_height for h in proposed), proposed

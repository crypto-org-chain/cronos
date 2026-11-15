import base64
import copy
import json
from pathlib import Path

import pytest

from .cosmoscli import DEFAULT_GAS, module_address
from .network import Cronos, setup_custom_cronos
from .utils import (
    MLDSA65,
    bech32_to_eth,
    comet_rpc,
    comet_validator,
    comet_validators,
    find_log_event_attrs,
    gen_mldsa65_key,
    show_validator,
    swap_consensus_keys,
    update_validator_pubkey_types,
    wait_for_block,
    wait_for_new_blocks,
)

pytestmark = pytest.mark.gov

ROTATED = (2, 3)
SHA256_LEAF = {
    "hash": "SHA256",
    "prehash_key": "NO_HASH",
    "prehash_value": "SHA256",
    "length": "VAR_PROTO",
    "prefix": "AA==",
}
# ibc-go GetSDKSpecs: iavl then tendermint
SDK_PROOF_SPECS = [
    {
        "leaf_spec": SHA256_LEAF,
        "inner_spec": {
            "child_order": [0, 1],
            "child_size": 33,
            "min_prefix_length": 4,
            "max_prefix_length": 12,
            "hash": "SHA256",
        },
    },
    {
        "leaf_spec": SHA256_LEAF,
        "inner_spec": {
            "child_order": [0, 1],
            "child_size": 32,
            "min_prefix_length": 1,
            "max_prefix_length": 1,
            "hash": "SHA256",
        },
    },
]


@pytest.fixture(scope="module")
def cronos_rotation(tmp_path_factory):
    yield from setup_custom_cronos(
        tmp_path_factory.mktemp("cons_key_rotation"),
        27200,
        Path(__file__).parent / "configs/cons_key_rotation.jsonnet",
    )


def signed_rotations_tx(clis, pubkeys, path):
    """One tx with every operator's rotation, so they all apply in the same block."""
    first, *rest = clis
    gas = int(DEFAULT_GAS) * len(clis)
    tx = clis[first].rotate_cons_pubkey(pubkeys[first], generate_only=True, gas=gas)
    for i in rest:
        other = clis[i].rotate_cons_pubkey(pubkeys[i], generate_only=True)
        tx["body"]["messages"] += other["body"]["messages"]
    for i in clis:
        path.write_text(json.dumps(tx))
        # amino-json signs neither signer_infos nor other signatures, so each
        # operator can append its signature without invalidating the earlier ones
        tx = clis[i].sign_single_tx(path, "validator", sign_mode="amino-json")
    return tx


def create_cronos_client(cli, height, path):
    """Light client of cronos hosted on cronos itself, trusting the given height."""
    header = cli.ibc_query_client_header(height)["signed_header"]["header"]
    client_state = path / "client_state.json"
    client_state.write_text(
        json.dumps(
            {
                "@type": "/ibc.lightclients.tendermint.v1.ClientState",
                "chain_id": cli.chain_id,
                "trust_level": {"numerator": "1", "denominator": "3"},
                "trusting_period": "1209600s",
                "unbonding_period": "1814400s",
                "max_clock_drift": "10s",
                "frozen_height": {"revision_number": "0", "revision_height": "0"},
                "latest_height": {"revision_number": "1", "revision_height": height},
                "proof_specs": SDK_PROOF_SPECS,
                "upgrade_path": ["upgrade", "upgradedIBCState"],
            }
        )
    )
    consensus_state = path / "consensus_state.json"
    consensus_state.write_text(
        json.dumps(
            {
                "@type": "/ibc.lightclients.tendermint.v1.ConsensusState",
                "timestamp": header["time"],
                "root": {"hash": header["app_hash"]},
                # HexBytes in the consensus state, base64 in the header
                "next_validators_hash": base64.b64decode(
                    header["next_validators_hash"]
                ).hex(),
            }
        )
    )
    rsp = cli.ibc_create_client(client_state, consensus_state, from_="community")
    assert rsp["code"] == 0, rsp["raw_log"]
    return find_log_event_attrs(rsp["events"], "create_client")["client_id"]


def commit_signers(header):
    return {
        base64.b64decode(s["validator_address"]).hex().upper()
        for s in header["signed_header"]["commit"]["signatures"]
        if s["block_id_flag"] == "BLOCK_ID_FLAG_COMMIT"
    }


def test_rotate_ed25519_to_mldsa65(cronos_rotation: Cronos, tmp_path):
    cronos = cronos_rotation
    cli0 = cronos.cosmos_cli(0)
    clis = {i: cronos.cosmos_cli(i) for i in ROTATED}
    w3 = cronos.node_w3(0)
    params = cli0.query_params("consensus")
    assert params["validator"]["pub_key_types"] == ["ed25519"]

    new_keys = {i: gen_mldsa65_key(clis[i], tmp_path / f"node{i}") for i in ROTATED}
    new_pubkeys = {i: new_keys[i][0] for i in ROTATED}
    for pubkey in new_pubkeys.values():
        assert pubkey["@type"] == "/cosmos.crypto.mldsa65.PubKey", pubkey

    rsp = clis[3].rotate_cons_pubkey(new_pubkeys[3])
    assert rsp["code"] != 0
    assert "not supported" in rsp["raw_log"], rsp["raw_log"]

    update_validator_pubkey_types(cronos, ["ed25519", MLDSA65])
    params = cli0.query_params("consensus")
    assert params["validator"]["pub_key_types"] == ["ed25519", MLDSA65]

    old_pubkeys = {i: show_validator(clis[i]) for i in ROTATED}
    old_addrs = {
        i: comet_validator(cronos, old_pubkeys[i]["key"])["address"] for i in ROTATED
    }
    operators = {i: clis[i].address("validator") for i in ROTATED}
    stakes = {i: clis[i].balance(operators[i], "stake") for i in ROTATED}

    tx = signed_rotations_tx(clis, new_pubkeys, tmp_path / "rotate.json")
    rsp = cli0.broadcast_tx_json(tx)
    assert rsp["code"] == 0, rsp["raw_log"]
    apply_heights = set()
    for i in ROTATED:
        valoper = clis[i].address("validator", bech="val")
        ev = find_log_event_attrs(
            rsp["events"], "rotate_cons_pubkey", lambda a: a["validator"] == valoper
        )
        assert ev["fee_burned"] == "1stake", ev
        apply_heights.add(int(ev["apply_height"]))
        assert clis[i].balance(operators[i], "stake") == stakes[i] - 1
    assert len(apply_heights) == 1, apply_heights
    apply_height = apply_heights.pop()
    rotation_height = int(rsp["height"])
    assert cli0.balance(module_address("key_rotation_fee_pool"), "stake") == 0

    swap_consensus_keys(cronos, {i: new_keys[i][1] for i in ROTATED}, apply_height)

    addresses = [v["address"] for v in comet_validators(cronos)]
    new_addrs = {}
    for i in ROTATED:
        rotated = comet_validator(cronos, new_pubkeys[i]["key"])
        assert rotated["pub_key"]["type"] == "cometbft/PubKeyMlDsa65"
        new_addrs[i] = rotated["address"]
        assert old_addrs[i] not in addresses
        # the rotated nodes may still be catching up past the apply height
        val = cli0.validator(clis[i].address("validator", bech="val"))
        val = val.get("validator") or val
        assert val["consensus_pubkey"] == {
            "type": new_pubkeys[i]["@type"],
            "value": new_pubkeys[i]["key"],
        }

    extra_pubkey, _ = gen_mldsa65_key(clis[3], tmp_path / "extra")
    rsp = clis[3].rotate_cons_pubkey(extra_pubkey)
    assert rsp["code"] != 0
    assert "max consensus pubkey rotations" in rsp["raw_log"], rsp["raw_log"]
    # a retired key stays locked for the unbonding window
    rsp = cronos.cosmos_cli(1).rotate_cons_pubkey(old_pubkeys[3])
    assert rsp["code"] != 0
    assert "previously rotated away" in rsp["raw_log"], rsp["raw_log"]

    # a rejoining validator starts with low proposer priority, so allow ~3 rounds
    end = wait_for_new_blocks(cli0, 24)
    nodes = range(len(cronos.config["validators"]))
    for i in nodes:
        wait_for_block(cronos.cosmos_cli(i), end)
    hashes = {
        comet_rpc(cronos, i, "block", height=end)["block_id"]["hash"] for i in nodes
    }
    assert len(hashes) == 1, hashes

    signed = {i: False for i in ROTATED}
    proposed = {i: [] for i in ROTATED}
    for height in range(rotation_height, end + 1):
        block = comet_rpc(cronos, 0, "block", height=height)["block"]
        signers = [s["validator_address"] for s in block["last_commit"]["signatures"]]
        for i in ROTATED:
            signed[i] |= new_addrs[i] in signers
            if block["header"]["proposer_address"] in (old_addrs[i], new_addrs[i]):
                proposed[i].append(height)
                miner = w3.eth.get_block(height).miner.lower()
                assert miner == "0x" + bech32_to_eth(operators[i]), height
    for i in ROTATED:
        assert signed[i], f"node{i} never signed with its ML-DSA-65 key"
        assert any(h >= apply_height for h in proposed[i]), (i, proposed[i])

    # with node1 down, the commit only reaches 2/3 with both ML-DSA-65 signatures,
    # so the light client cannot skip verifying them
    client_id = create_cronos_client(cli0, rotation_height, tmp_path)
    node1 = f"{cli0.chain_id}-node1"
    cronos.supervisorctl("stop", node1)
    target = wait_for_new_blocks(cli0, 3)
    header = cli0.ibc_query_client_header(target)
    assert set(new_addrs.values()) < commit_signers(header), header
    assert len(commit_signers(header)) == 3, header
    header |= {
        "@type": "/ibc.lightclients.tendermint.v1.Header",
        "trusted_height": {
            "revision_number": "1",
            "revision_height": str(rotation_height),
        },
        "trusted_validators": cli0.ibc_query_client_header(rotation_height + 1)[
            "validator_set"
        ],
    }
    header_file = tmp_path / "header.json"
    tampered = copy.deepcopy(header)
    sig = next(
        s
        for s in tampered["signed_header"]["commit"]["signatures"]
        if base64.b64decode(s["validator_address"]).hex().upper() == new_addrs[3]
    )
    raw = bytearray(base64.b64decode(sig["signature"]))
    raw[0] ^= 1
    sig["signature"] = base64.b64encode(raw).decode()
    header_file.write_text(json.dumps(tampered))
    # ML-DSA-65 signatures and keys push the header past the 200k default gas
    rsp = cli0.ibc_update_client_with_header(
        client_id, header_file, from_="community", gas=1000000
    )
    assert rsp["code"] != 0, "tampered ML-DSA-65 signature accepted"
    header_file.write_text(json.dumps(header))
    rsp = cli0.ibc_update_client_with_header(
        client_id, header_file, from_="community", gas=1000000
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    cronos.supervisorctl("start", node1)

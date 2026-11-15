from contextlib import contextmanager
from pathlib import Path

import pytest
from pystarport import cluster, ports

from .ibc_utils import call_rly_cmd, ibc_denom
from .network import Cronos, setup_custom_cronos
from .test_cons_key_rotation import ROTATED, signed_rotations_tx
from .utils import (
    MLDSA65,
    comet_validators,
    find_log_event_attrs,
    gen_mldsa65_key,
    swap_consensus_keys,
    update_validator_pubkey_types,
    wait_for_block,
    wait_for_fn,
    wait_for_new_blocks,
    wait_for_port,
)

pytestmark = pytest.mark.ibc_rly_evm

COUNTERPARTY = "cronos_778-1"
CHANNEL = "channel-0"
DENOM = "basetcro"
AMOUNT = 100


def rotate_to_mldsa65(cronos, tmp_path):
    cli = cronos.cosmos_cli()
    clis = {i: cronos.cosmos_cli(i) for i in ROTATED}
    keys = {i: gen_mldsa65_key(clis[i], tmp_path / f"node{i}") for i in ROTATED}
    update_validator_pubkey_types(cronos, ["ed25519", MLDSA65])
    pubkeys = {i: keys[i][0] for i in ROTATED}
    tx = signed_rotations_tx(clis, pubkeys, tmp_path / "rotate.json")
    rsp = cli.broadcast_tx_json(tx)
    assert rsp["code"] == 0, rsp["raw_log"]
    ev = find_log_event_attrs(rsp["events"], "rotate_cons_pubkey")
    swap_consensus_keys(
        cronos, {i: keys[i][1] for i in ROTATED}, int(ev["apply_height"])
    )
    key_types = [v["pub_key"]["type"] for v in comet_validators(cronos)]
    assert key_types.count("cometbft/PubKeyMlDsa65") == len(ROTATED), key_types
    height = wait_for_new_blocks(cli, 1)
    for i in ROTATED:
        wait_for_block(clis[i], height)


@pytest.fixture(scope="module")
def chains(tmp_path_factory):
    """Two cronos chains joined by rly, the first with ML-DSA-65 validators."""
    with contextmanager(setup_custom_cronos)(
        tmp_path_factory.mktemp("ibc_rly_mldsa65"),
        26700,
        Path(__file__).parent / "configs/ibc_rly_mldsa65.jsonnet",
        relayer=cluster.Relayer.RLY.value,
    ) as cronos:
        counterparty = Cronos(cronos.base_dir.parent / COUNTERPARTY)
        for chain in (cronos, counterparty):
            wait_for_port(ports.grpc_port(chain.base_port(0)))
            wait_for_new_blocks(chain.cosmos_cli(), 1)
        call_rly_cmd(
            cronos.base_dir.parent / "relayer",
            connection_only=False,
            incentivized=False,
            version="ics20-1",
            hostchain=COUNTERPARTY,
        )
        # started before the rotation, so rly has to process the validator update
        cronos.supervisorctl("start", "relayer-demo")
        rotate_to_mldsa65(cronos, tmp_path_factory.mktemp("mldsa65_keys"))
        yield {cronos.cosmos_cli().chain_id: cronos, COUNTERPARTY: counterparty}


@pytest.mark.parametrize(
    "src,dst",
    [("cronos_777-1", COUNTERPARTY), (COUNTERPARTY, "cronos_777-1")],
)
def test_relay_with_mldsa65_validators(chains, src, dst):
    src_cli = chains[src].cosmos_cli()
    dst_cli = chains[dst].cosmos_cli()
    receiver = dst_cli.address("signer2")
    voucher = ibc_denom(CHANNEL, DENOM)
    old_balance = dst_cli.balance(receiver, voucher)

    rsp = src_cli.ibc_transfer(
        "signer2", receiver, f"{AMOUNT}{DENOM}", CHANNEL, event_query_tx_for=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    def received():
        return dst_cli.balance(receiver, voucher) == old_balance + AMOUNT

    wait_for_fn("packet received", received)

    # the source drops the commitment once the acknowledgement is relayed back
    def acknowledged():
        rsp = src_cli.ibc_query_packet_commitments("transfer", CHANNEL)
        return not rsp.get("commitments")

    wait_for_fn("packet acknowledged", acknowledged)

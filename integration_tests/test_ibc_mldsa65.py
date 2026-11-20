from contextlib import contextmanager
from pathlib import Path

import pytest
from pystarport import cluster, ports

from .cosmoscli import signed_rotations_tx
from .ibc_utils import RATIO, call_hermes_cmd, call_rly_cmd, ibc_denom
from .network import Chainmain, Cronos, Hermes, setup_custom_cronos
from .utils import (
    MLDSA65,
    ROTATED,
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

ROTATED_CHAIN = "cronos_777-1"
CHAINMAIN = "chainmain-1"
CRONOS_PEER = "cronos_778-1"
CHANNEL = "channel-0"
AMOUNT = 100
DENOMS = {ROTATED_CHAIN: "basetcro", CRONOS_PEER: "basetcro", CHAINMAIN: "basecro"}
CONFIGS = {
    CHAINMAIN: "configs/ibc_mldsa65_chainmain.jsonnet",
    CRONOS_PEER: "configs/ibc_mldsa65.jsonnet",
}
# the marks only pick the CI shard; nothing here is EVM-specific
RELAYER_MARKS = {
    cluster.Relayer.RLY.value: pytest.mark.ibc_rly_evm,
    cluster.Relayer.HERMES.value: pytest.mark.ibc_update_client,
}


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
    apply_height = int(ev["apply_height"])
    swap_consensus_keys(cronos, {i: keys[i][1] for i in ROTATED}, apply_height)
    key_types = [v["pub_key"]["type"] for v in comet_validators(cronos)]
    assert key_types.count("cometbft/PubKeyMlDsa65") == len(ROTATED), key_types
    height = wait_for_new_blocks(cli, 1)
    for i in ROTATED:
        wait_for_block(clis[i], height)
    return apply_height


@pytest.fixture(
    scope="module",
    params=[
        pytest.param((relayer, peer), marks=mark, id=f"{relayer}-{peer}")
        for relayer, mark in RELAYER_MARKS.items()
        for peer in CONFIGS
    ],
)
def network(request, tmp_path_factory):
    """The rotated cronos chain and a counterparty joined by a relayer."""
    relayer, peer = request.param
    with contextmanager(setup_custom_cronos)(
        tmp_path_factory.mktemp(f"ibc_{relayer}_{peer}_mldsa65"),
        26700,
        Path(__file__).parent / CONFIGS[peer],
        relayer=relayer,
    ) as cronos:
        peer_cls = Chainmain if peer == CHAINMAIN else Cronos
        counterparty = peer_cls(cronos.base_dir.parent / peer)
        for chain in (cronos, counterparty):
            wait_for_port(ports.grpc_port(chain.base_port(0)))
            wait_for_new_blocks(chain.cosmos_cli(), 1)
        path = cronos.base_dir.parent / "relayer"
        if relayer == cluster.Relayer.HERMES.value:
            call_hermes_cmd(
                Hermes(path.with_suffix(".toml")),
                connection_only=False,
                incentivized=False,
                version="ics20-1",
                is_ibc_transfer=True,
                hostchain=peer,
            )
        else:
            call_rly_cmd(
                path,
                connection_only=False,
                incentivized=False,
                version="ics20-1",
                hostchain=peer,
            )
        # started before the rotation, so the relayer must process the validator update
        cronos.supervisorctl("start", "relayer-demo")
        height = rotate_to_mldsa65(cronos, tmp_path_factory.mktemp("mldsa65_keys"))
        yield {ROTATED_CHAIN: cronos, peer: counterparty}, peer, height


@pytest.mark.parametrize("outbound", [True, False], ids=["from_rotated", "to_rotated"])
def test_relay_with_mldsa65_validators(network, outbound):
    chains, peer, apply_height = network
    src, dst = (ROTATED_CHAIN, peer) if outbound else (peer, ROTATED_CHAIN)
    src_cli = chains[src].cosmos_cli()
    dst_cli = chains[dst].cosmos_cli()
    receiver = dst_cli.address("signer2")
    denom = DENOMS[src]
    if src == CHAINMAIN:
        # cronos converts the CRO voucher to basetcro, with 10 more decimals
        dst_denom, dst_amount = DENOMS[ROTATED_CHAIN], AMOUNT * RATIO
    else:
        dst_denom, dst_amount = ibc_denom(CHANNEL, denom), AMOUNT
    old_balance = dst_cli.balance(receiver, dst_denom)

    rsp = src_cli.ibc_transfer(
        "signer2", receiver, f"{AMOUNT}{denom}", CHANNEL, event_query_tx_for=True
    )
    assert rsp["code"] == 0, rsp["raw_log"]

    def received():
        return dst_cli.balance(receiver, dst_denom) == old_balance + dst_amount

    wait_for_fn("packet received", received)

    # the source drops the commitment once the acknowledgement is relayed back
    def acknowledged():
        rsp = src_cli.ibc_query_packet_commitments("transfer", CHANNEL)
        return not rsp.get("commitments")

    wait_for_fn("packet acknowledged", acknowledged)

    # either direction needs the counterparty to verify a post-rotation header
    cli = chains[peer].cosmos_cli()
    client = cli.ibc_query_client_state("transfer", CHANNEL)
    latest = int(client["client_state"]["latest_height"]["revision_height"])
    assert latest > apply_height, latest

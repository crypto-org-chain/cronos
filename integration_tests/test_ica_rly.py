import json
import os
import signal

import pytest
from eth_utils import keccak
from pystarport import cluster
from web3.datastructures import AttributeDict

from .ibc_utils import (
    assert_channel_open_init,
    prepare_network,
    update_client,
    wait_for_check_channel_ready,
    wait_for_new_blocks,
)
from .utils import CONTRACT_ABIS, get_logs_since, get_method_map, get_topic_data

CONTRACT = "0x0000000000000000000000000000000000000065"
contract_info = json.loads(CONTRACT_ABIS["IRelayerModule"].read_text())
method_map = get_method_map(contract_info)


def channel_open_ack(
    port_id,
    channel_id,
    counterparty_port_id,
    counterparty_channel_id,
    connection_id,
):
    return {
        "portId": keccak(text=port_id),
        "channelId": keccak(text=channel_id),
        "counterpartyPortId": keccak(text=counterparty_port_id),
        "counterpartyChannelId": counterparty_channel_id,
        "connectionId": connection_id,
    }


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    path = tmp_path_factory.mktemp("ica_rly")
    procs = []
    try:
        for network in prepare_network(
            path,
            "ibc",
            incentivized=False,
            connection_only=True,
            relayer=cluster.Relayer.RLY.value,
        ):
            if network.proc:
                procs.append(network.proc)
                print("append:", network.proc)
            yield network
    finally:
        print("finally:", procs)
        for proc in procs:
            try:
                os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
            except ProcessLookupError:
                pass
            # proc.terminate()
            proc.wait()


def test_ica(ibc, tmp_path):
    connid = "connection-0"
    cli_controller = ibc.cronos.cosmos_cli()
    w3 = ibc.cronos.w3
    wait_for_new_blocks(cli_controller, 1)
    start = w3.eth.get_block_number()
    print("register ica account")
    rsp = cli_controller.icaauth_register_account(
        connid, from_="signer2", gas="400000", fees="100000000basetcro"
    )
    port_id, channel_id = assert_channel_open_init(rsp)
    wait_for_check_channel_ready(cli_controller, connid, channel_id)
    logs = get_logs_since(w3, CONTRACT, start)
    expected = [
        update_client(),
        channel_open_ack(port_id, channel_id, "icahost", "channel-1", connid),
    ]
    assert len(logs) == len(expected)
    for i, log in enumerate(logs):
        method_name, args = get_topic_data(w3, method_map, contract_info, log)
        assert args == AttributeDict(expected[i]), [i, method_name]
    print("query ica account")
    ica_address = cli_controller.ica_query_account(
        connid, cli_controller.address("signer2")
    )["interchain_account_address"]
    print("ica address", ica_address)

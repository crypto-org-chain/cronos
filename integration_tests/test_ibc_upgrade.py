import pytest
from pystarport import ports

from .ibc_utils import RATIO, get_balance, ibc_transfer, prepare_network
from .upgrade_utils import post_init, prepare_upgrade_binaries, propose_software_upgrade
from .utils import find_log_event_attrs, wait_for_block, wait_for_fn, wait_for_port

pytestmark = pytest.mark.upgrade


@pytest.fixture(scope="module")
def ibc(tmp_path_factory):
    path = tmp_path_factory.mktemp("ibc_upgrade")
    upgrades = prepare_upgrade_binaries(path, genesis="v1.8", plans=["v1.9"])
    genesis_binary = str(upgrades / "genesis/bin/cronosd")
    with pytest.MonkeyPatch.context() as mp:
        mp.setenv("CRONOS_GENESIS_BINARY", genesis_binary)
        yield from prepare_network(
            path,
            "ibc_upgrade",
            incentivized=False,
            is_ibc_transfer=True,
            post_init=post_init,
            cronos_binary=genesis_binary,
        )


def send_to_chainmain(ibc, relayed=True):
    """
    send 1basecro worth of basetcro back to chainmain, return the packet sequence
    """
    cli = ibc.cronos.cosmos_cli()
    src_addr = cli.address("signer2")
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    old_dst_balance = get_balance(ibc.chainmain, dst_addr, "basecro")
    rsp = cli.transfer_tokens(src_addr, dst_addr, f"{RATIO}basetcro")
    assert rsp["code"] == 0, rsp["raw_log"]
    if relayed:
        wait_for_balance(ibc.chainmain, dst_addr, old_dst_balance + 1)
    ev = find_log_event_attrs(rsp["events"], "send_packet")
    return int(ev["packet_sequence"])


def wait_for_balance(chain, addr, expected):
    wait_for_fn(
        "balance change",
        lambda: get_balance(chain, addr, "basecro") == expected,
    )


def test_ibc_upgrade(ibc):
    """
    IBC transfers keep their channel send sequence across the v1.9 upgrade,
    including a packet sent before the upgrade and relayed after it
    """
    c = ibc.cronos
    ibc_transfer(ibc)
    seq = send_to_chainmain(ibc)

    # leave a packet in flight over the upgrade
    c.supervisorctl("stop", "relayer-demo")
    assert send_to_chainmain(ibc, relayed=False) == seq + 1
    dst_addr = ibc.chainmain.cosmos_cli().address("signer2")
    pending_balance = get_balance(ibc.chainmain, dst_addr, "basecro") + 1

    cli = c.cosmos_cli()
    target = cli.block_height() + 15
    propose_software_upgrade(c, "v1.9", target)
    c.chain_binary = str(c.base_dir.parent / "upgrades/v1.9/bin/cronosd")
    wait_for_block(c.cosmos_cli(), target + 2, timeout=480)
    wait_for_port(ports.rpc_port(c.base_port(0)))

    c.supervisorctl("start", "relayer-demo")
    wait_for_port(ibc.hermes.port)
    wait_for_balance(ibc.chainmain, dst_addr, pending_balance)

    assert send_to_chainmain(ibc) == seq + 2
    ibc_transfer(ibc)

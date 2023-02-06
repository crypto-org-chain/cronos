import shutil
import tempfile

from pystarport import ports

from .network import Cronos
from .utils import ADDRS, wait_for_port


def test_versiondb_migration(cronos: Cronos):
    """
    restore the app db of the second validator using changeset commands,
    check the chain still works, and query result of iavl tree is correct.
    """
    w3 = cronos.w3
    w3.eth.wait_for_transaction_receipt(
        w3.eth.send_transaction(
            {
                "from": ADDRS["validator"],
                "to": ADDRS["community"],
                "value": 1000,
            }
        )
    )
    old_balance = w3.eth.get_balance(ADDRS["community"])

    # stop the network first
    print("stop all nodes")
    print(cronos.supervisorctl("stop", "all"))
    cli = cronos.cosmos_cli(i=1)

    changeset_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print("dump to:", changeset_dir)
    print(cli.changeset_dump(changeset_dir))
    snapshot_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print("verify and save to snapshot:", snapshot_dir)
    print(cli.changeset_verify(changeset_dir, save_snapshot=snapshot_dir))
    # remove existing `application.db`
    app_db = cli.data_dir / "data/application.db"
    print("remove and restore app db:", app_db)
    shutil.rmtree(app_db)
    print(cli.changeset_restore_app_db(snapshot_dir, app_db))

    print("start all nodes")
    print(cronos.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1"))
    wait_for_port(ports.evmrpc_port(cronos.base_port(0)))
    wait_for_port(ports.evmrpc_port(cronos.base_port(1)))

    # check query still works, node1 don't enable versiondb,
    # so we are testing iavl query here.
    w3_1 = cronos.node_w3(1)
    assert w3_1.eth.get_balance(ADDRS["community"]) == old_balance

    # check the chain is still growing
    w3.eth.wait_for_transaction_receipt(
        w3.eth.send_transaction(
            {
                "from": ADDRS["community"],
                "to": ADDRS["validator"],
                "value": 1000,
            }
        )
    )

import shutil
import tempfile

import tomlkit
from pystarport import ports

from .network import Cronos
from .utils import ADDRS, send_transaction, w3_wait_for_new_blocks, wait_for_port


def test_versiondb_migration(cronos: Cronos):
    """
    test versiondb migration commands.
    node0 has memiavl and versiondb enabled while node1 don't,
    - stop all the nodes
    - dump change set from node1's application.db
    - verify change set and save snapshot
    - restore pruned application.db from the snapshot
    - replace node1's application.db with the restored one
    - rebuild versiondb for node0
    - start the nodes, now check:
      - the network can grow
      - node0 do support historical queries
      - node1 don't support historical queries
    """
    w3 = cronos.w3
    community = ADDRS["community"]
    balance0 = w3.eth.get_balance(community)
    block0 = w3.eth.block_number

    tx = {
        "from": ADDRS["validator"],
        "to": community,
        "value": 1000,
    }
    send_transaction(w3, tx)
    balance1 = w3.eth.get_balance(community)
    block1 = w3.eth.block_number

    # wait for a few blocks
    w3_wait_for_new_blocks(w3, 5)

    # stop the network first
    print("stop all nodes")
    print(cronos.supervisorctl("stop", "all"))
    cli0 = cronos.cosmos_cli(i=0)
    cli1 = cronos.cosmos_cli(i=1)

    changeset_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print("dump to:", changeset_dir)

    # only restore to an intermediate version to test version mismatch behavior
    print(cli1.changeset_dump(changeset_dir, end_version=block1 + 1))

    snapshot_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print("verify and save to snapshot:", snapshot_dir)
    _, commit_info = cli0.changeset_verify(changeset_dir, save_snapshot=snapshot_dir)
    latest_version = commit_info["version"]

    # replace existing `application.db`
    app_db1 = cli1.data_dir / "data/application.db"
    print("replace node db:", app_db1)
    shutil.rmtree(app_db1)
    print(cli1.changeset_restore_app_db(snapshot_dir, app_db1))

    print("restore versiondb for node0")
    sst_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print(cli0.changeset_build_versiondb_sst(changeset_dir, sst_dir))
    # ingest-versiondb-sst expects an empty database
    shutil.rmtree(cli0.data_dir / "data/versiondb")
    print(
        cli0.changeset_ingest_versiondb_sst(
            cli0.data_dir / "data/versiondb", sst_dir, maximum_version=latest_version
        )
    )

    # force node1's app-db-backend to be rocksdb
    patch_app_db_backend(cli1.data_dir / "config/app.toml", "rocksdb")

    print("start all nodes")
    print(
        cronos.supervisorctl(
            "start", "cronos_777-1-node0", "cronos_777-1-node1", "cronos_777-1-node2"
        )
    )
    for i in range(len(cronos.config["validators"])):
        wait_for_port(ports.evmrpc_port(cronos.base_port(i)))

    assert w3.eth.get_balance(community, block_identifier=block0) == balance0
    assert w3.eth.get_balance(community, block_identifier=block1) == balance1
    assert w3.eth.get_balance(community) == balance1

    # check query still works, node1 don't enable versiondb,
    # so we are testing iavl query here.
    w3_1 = cronos.node_w3(1)
    assert w3_1.eth.get_balance(community) == balance1

    # check the chain is still growing
    send_transaction(
        w3,
        {
            "from": community,
            "to": ADDRS["validator"],
            "value": 1000,
        },
    )


def patch_app_db_backend(path, backend):
    cfg = tomlkit.parse(path.read_text())
    cfg["app-db-backend"] = backend
    path.write_text(tomlkit.dumps(cfg))

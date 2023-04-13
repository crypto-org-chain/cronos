import shutil
import tempfile

import tomlkit
from pystarport import ports

from .network import Cronos
from .utils import ADDRS, send_transaction, wait_for_port


def test_versiondb_migration(cronos: Cronos):
    """
    test versiondb migration commands.
    node0 has versiondb enabled while node1 don't,
    - stop all the nodes
    - dump change set
    - verify change set and save snapshot
    - restore pruned application.db from the snapshot
    - replace node0/node1's application.db with the restored one
    - build versiondb for node0
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

    # stop the network first
    print("stop all nodes")
    print(cronos.supervisorctl("stop", "all"))
    cli0 = cronos.cosmos_cli(i=0)
    cli1 = cronos.cosmos_cli(i=1)

    changeset_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print("dump to:", changeset_dir)
    print(cli0.changeset_dump(changeset_dir))
    snapshot_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print("verify and save to snapshot:", snapshot_dir)
    _, commit_info = cli0.changeset_verify(changeset_dir, save_snapshot=snapshot_dir)
    latest_version = commit_info["version"]

    # replace existing `application.db`
    app_db0 = cli0.data_dir / "data/application.db"
    app_db1 = cli1.data_dir / "data/application.db"
    print("replace node db:", app_db0)
    shutil.rmtree(app_db0)
    print(cli0.changeset_restore_app_db(snapshot_dir, app_db0))
    print("replace node db:", app_db1)
    shutil.rmtree(app_db1)
    shutil.copytree(app_db0, app_db1)

    print("restore versiondb for node0")
    sst_dir = tempfile.mkdtemp(dir=cronos.base_dir)
    print(cli0.changeset_build_versiondb_sst(changeset_dir, sst_dir))
    tmpdir = tempfile.mkdtemp(dir=cronos.base_dir)
    print(
        cli0.changeset_ingest_versiondb_sst(
            tmpdir, sst_dir, maximum_version=latest_version
        )
    )
    versiondb_dir = cli0.data_dir / "data/versiondb"
    shutil.rmtree(versiondb_dir)
    shutil.move(tmpdir, versiondb_dir)

    # force app-db-backend to be rocksdb
    patch_app_db_backend(cli0.data_dir / "config/app.toml", "rocksdb")
    patch_app_db_backend(cli1.data_dir / "config/app.toml", "rocksdb")

    print("start all nodes")
    print(cronos.supervisorctl("start", "cronos_777-1-node0", "cronos_777-1-node1"))
    wait_for_port(ports.evmrpc_port(cronos.base_port(0)))
    wait_for_port(ports.evmrpc_port(cronos.base_port(1)))

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

import pytest
from pystarport import cluster

from .ibc_utils import log_gas_records, prepare_network, rly_transfer
from .utils import wait_for_new_blocks

pytestmark = pytest.mark.ibc_rly_gas


@pytest.fixture(scope="module", params=["ibc_rly", "ibc_rly_evm"])
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = request.param
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(path, name, relayer=cluster.Relayer.RLY.value)


records = []


def test_ibc(ibc):
    # chainmain-1 relayer -> cronos_777-1 signer2
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    rly_transfer(ibc)
    diff = 0.01
    record = log_gas_records(cli)
    if record:
        records.append(record)
    if len(records) == 2:
        for e1, e2 in zip(*records):
            res = float(e2) / float(e1)
            assert 1 - diff <= res <= 1 + diff, res

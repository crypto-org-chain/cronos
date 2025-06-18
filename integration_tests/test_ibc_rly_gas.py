import pytest

from .ibc_utils import (
    ibc_incentivized_transfer,
    ibc_multi_transfer,
    ibc_transfer,
    log_gas_records,
    prepare_network,
)
from .utils import wait_for_new_blocks

pytestmark = pytest.mark.ibc_rly_gas


@pytest.fixture(scope="module", params=["ibc_rly_evm", "ibc_rly"])
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = request.param
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(
        path,
        name,
        need_relayer_caller=name == "ibc_rly_evm",
        is_ibc_transfer=True,
    )


records = []


def test_ibc(ibc):
    # chainmain-1 relayer -> cronos_777-1 signer2
    cli = ibc.cronos.cosmos_cli()
    wait_for_new_blocks(cli, 1)
    ibc_transfer(ibc)
    
    if ibc.hermes is None:
        ibc_incentivized_transfer(ibc)
    
    ibc_multi_transfer(ibc)
    diff = 0.15
    record = log_gas_records(cli)
    if record:
        records.append(record)
    if len(records) == 2:
        res = float(sum(records[0]) / sum(records[1]))
        assert 1 - diff <= res <= 1 + diff, res

from pathlib import Path

import pytest

from .network import setup_custom_cronos
from .utils import submit_any_proposal

pytestmark = pytest.mark.gov


@pytest.fixture(scope="module")
def custom_cronos(tmp_path_factory):
    path = tmp_path_factory.mktemp("cronos")
    yield from setup_custom_cronos(
        path, 26400, Path(__file__).parent / "configs/broadcast.jsonnet"
    )


def test_submit_any_proposal(custom_cronos):
    submit_any_proposal(custom_cronos)

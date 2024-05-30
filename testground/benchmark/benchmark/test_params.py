import ipaddress
from datetime import datetime

import pytest
from pydantic import ValidationError

from .params import RunParams, parse_dict, run_params


def test_params():
    exp = RunParams(
        test_case="entrypoint",
        test_group_id="single",
        test_group_instance_count=2,
        test_instance_count=2,
        test_instance_params=parse_dict(
            "latency=0|timeout=21m|bandwidth=420Mib|chain_id=testground"
        ),
        test_outputs_path="/outputs",
        test_plan="benchmark",
        test_run="cp9va5nae0pksdti05vg",
        test_start_time=datetime.fromisoformat("2024-05-27T10:52:08+08:00"),
        test_subnet=ipaddress.IPv4Network("16.20.0.0/16"),
        test_sidecar=True,
        test_temp_path="/temp",
    )
    actual = run_params(
        {
            "TEST_BRANCH": "",
            "TEST_CASE": "entrypoint",
            "TEST_GROUP_ID": "single",
            "TEST_GROUP_INSTANCE_COUNT": "2",
            "TEST_INSTANCE_COUNT": "2",
            "TEST_INSTANCE_PARAMS": (
                "latency=0|timeout=21m|bandwidth=420Mib|chain_id=testground"
            ),
            "TEST_INSTANCE_ROLE": "",
            "TEST_OUTPUTS_PATH": "/outputs",
            "TEST_PLAN": "benchmark",
            "TEST_REPO": "",
            "TEST_RUN": "cp9va5nae0pksdti05vg",
            "TEST_START_TIME": "2024-05-27T10:52:08+08:00",
            "TEST_SIDECAR": "true",
            "TEST_SUBNET": "16.20.0.0/16",
            "TEST_TAG": "",
            "TEST_CAPTURE_PROFILES": "",
            "TEST_TEMP_PATH": "/temp",
        }
    )
    assert exp == actual


def test_params_bool():
    assert RunParams(test_sidecar=True) == run_params({"TEST_SIDECAR": "true"})
    assert RunParams(test_sidecar=False) == run_params({"TEST_SIDECAR": "false"})
    with pytest.raises(ValidationError):
        run_params({"TEST_SIDECAR": "fse"})

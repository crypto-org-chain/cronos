import ipaddress
import os
from dataclasses import dataclass, fields
from datetime import datetime
from typing import Dict

VALIDATOR_GROUP_ID = "validators"


@dataclass
class RunParams:
    test_branch: str = ""
    test_case: str = ""
    test_group_id: str = ""
    test_group_instance_count: int = 0
    test_instance_count: int = 0
    test_instance_params: dict = None
    test_instance_role: str = ""
    test_outputs_path: str = ""
    test_plan: str = ""
    test_repo: str = ""
    test_run: str = ""
    test_sidecar: bool = False
    test_start_time: datetime = None
    test_subnet: ipaddress.IPv4Network = None
    test_tag: str = ""
    test_capture_profiles: str = ""
    test_temp_path: str = ""
    log_level: str = ""

    def events_key(self):
        return (
            f"run:{self.test_run}"
            f":plan:{self.test_plan}"
            f":case:{self.test_case}"
            ":run_events"
        )

    def topic_key(self, topic: str):
        return (
            f"run:{self.test_run}"
            f":plan:{self.test_plan}"
            f":case:{self.test_case}"
            f":topics:{topic}"
        )

    def state_key(self, name: str):
        return (
            f"run:{self.test_run}"
            f":plan:{self.test_plan}"
            f":case:{self.test_case}"
            f":states:{name}"
        )

    def ipaddress(self, seq: int) -> ipaddress.IPv4Address:
        # add 256 to avoid conflict with system services
        return self.test_subnet.network_address + (seq + 256)

    def network_config(
        self, global_seq: int, callback_state="network-configured"
    ) -> dict:
        """
        config ip address based on global seq
        """
        return {
            "network": "default",
            "enable": True,
            # using the assigned `GlobalSequencer` id per each of instance
            # to fill in the last 2 octets of the new IP address for the instance
            "IPv4": str(self.ipaddress(global_seq)) + "/16",
            "IPv6": None,
            "rules": None,
            "default": {
                "latency": 0,
                "jitter": 0,
                "bandwidth": 0,
                "filter": 0,
                "loss": 0,
                "corrupt": 0,
                "corrupt_corr": 0,
                "reorder": 0,
                "reorder_corr": 0,
                "duplicate": 0,
                "duplicate_corr": 0,
            },
            "callback_state": callback_state,
            "routing_policy": "allow_all",
        }

    @property
    def is_validator(self) -> bool:
        return self.test_group_id == VALIDATOR_GROUP_ID


def run_params(env=None) -> RunParams:
    if env is None:
        env = os.environ
    p = RunParams()
    for f in fields(RunParams):
        value = env.get(f.name.upper())
        if value is None:
            continue
        if f.type == bool:
            value = parse_bool(value)
        elif f.type == datetime:
            value = datetime.fromisoformat(value)
        elif f.type == dict:
            value = parse_dict(value)
        else:
            value = f.type(value)
        setattr(p, f.name, value)
    return p


def parse_bool(s: str) -> bool:
    return s == "true"


def parse_dict(s: str) -> Dict[str, str]:
    return dict(part.split("=") for part in s.split("|"))

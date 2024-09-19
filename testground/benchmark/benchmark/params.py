import ipaddress
import os
from datetime import datetime
from typing import Dict, Optional

from pydantic import BaseModel
from pydantic.functional_validators import BeforeValidator
from typing_extensions import Annotated

VALIDATOR_GROUP_ID = "validators"


def parse_dict(s: any) -> Dict[str, str]:
    if isinstance(s, str):
        return dict(part.split("=") for part in s.split("|"))
    return s


Params = Annotated[dict, BeforeValidator(parse_dict)]


class RunParams(BaseModel):
    test_branch: Optional[str] = ""
    test_case: Optional[str] = ""
    test_group_id: Optional[str] = ""
    test_group_instance_count: Optional[int] = 0
    test_instance_count: Optional[int] = 0
    test_instance_params: Optional[Params] = None
    test_instance_role: Optional[str] = ""
    test_outputs_path: Optional[str] = ""
    test_plan: Optional[str] = ""
    test_repo: Optional[str] = ""
    test_run: Optional[str] = ""
    test_sidecar: Optional[bool] = False
    test_start_time: Optional[datetime] = None
    test_subnet: Optional[ipaddress.IPv4Network] = None
    test_tag: Optional[str] = ""
    test_capture_profiles: Optional[str] = ""
    test_temp_path: Optional[str] = ""
    log_level: Optional[str] = ""

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

    @property
    def is_validator(self) -> bool:
        return self.test_group_id == VALIDATOR_GROUP_ID

    @property
    def chain_id(self) -> str:
        return self.test_instance_params["chain_id"]

    @property
    def num_accounts(self) -> int:
        return int(self.test_instance_params["num_accounts"])

    @property
    def num_txs(self) -> int:
        return int(self.test_instance_params["num_txs"])


def run_params(env=None) -> RunParams:
    if env is None:
        env = os.environ

    d = {
        name: env[name.upper()]
        for name in RunParams.model_fields
        if name.upper() in env
    }
    return RunParams(**d)

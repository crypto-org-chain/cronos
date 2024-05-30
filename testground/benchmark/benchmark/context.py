import os
import socket

from .params import RunParams, run_params
from .sync import SyncService

LEADER_GLOBAL_SEQUENCE = 1


class Context:
    def __init__(self, params: RunParams = None):
        if params is None:
            params = run_params()
        self.params = params
        self.sync = SyncService(params)

    def init_common(self):
        self.wait_network_ready()

        self.global_seq = self.sync.signal_entry("initialized_global")
        self.group_seq = self.sync.signal_and_wait(
            f"initialized_group_{self.params.test_group_id}",
            self.params.test_group_instance_count,
        )

        print("global_seq:", self.global_seq, "group_seq:", self.group_seq)

        print("start initializing network address")
        self.config_network(self.params.network_config(self.global_seq))

        os.environ["TMPDIR"] = self.params.test_temp_path

    def wait_network_ready(self):
        self.record_stage_start("network-initialized")

        if self.params.test_sidecar:
            self.sync.barrier("network-initialized", self.params.test_instance_count)

        print("network initialisation successful")

        self.record_stage_end("network-initialized")

    def config_network(self, config: dict):
        if not self.params.test_sidecar:
            print(
                "ignoring network change request; running in a sidecar-less environment"
            )
            return

        assert config.get("callback_state"), "no callback state provided"

        return self.sync.publish_and_wait(
            "network:" + socket.gethostname(),
            config,
            config["callback_state"],
            self.params.test_instance_count,
        )

    def record_success(self):
        return self.sync.signal_event(
            {
                "success_event": {
                    "group": self.params.test_group_id,
                },
            }
        )

    def record_failure(self, error: str):
        return self.sync.signal_event(
            {
                "failure_event": {
                    "group": self.params.test_group_id,
                    "error": error,
                },
            }
        )

    def record_stage_start(self, name: str):
        self.sync.signal_event(
            {
                "stage_start_event": {
                    "name": name,
                    "group": self.params.test_group_id,
                },
            }
        )

    def record_stage_end(self, name: str):
        self.sync.signal_event(
            {
                "stage_end_event": {
                    "name": name,
                    "group": self.params.test_group_id,
                }
            }
        )

    @property
    def is_leader(self) -> bool:
        return self.global_seq == LEADER_GLOBAL_SEQUENCE

    @property
    def is_validator(self) -> bool:
        return self.params.is_validator

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.sync.close()

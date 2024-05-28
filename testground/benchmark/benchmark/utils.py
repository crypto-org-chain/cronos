import json
import socket
import time
from pathlib import Path

import tomlkit


def patch_dict(doc, kwargs):
    for k, v in kwargs.items():
        keys = k.split(".")
        assert len(keys) > 0
        cur = doc
        for section in keys[:-1]:
            cur = cur[section]
        cur[keys[-1]] = v


def patch_toml(path: Path, kwargs):
    doc = tomlkit.parse(path.read_text())
    patch_dict(doc, kwargs)
    path.write_text(tomlkit.dumps(doc))


def patch_json(path: Path, kwargs):
    doc = json.loads(path.read_text())
    patch_dict(doc, kwargs)
    path.write_text(json.dumps(doc))


def wait_for_port(port, host="127.0.0.1", timeout=40.0):
    start_time = time.perf_counter()
    while True:
        try:
            with socket.create_connection((host, port), timeout=timeout):
                break
        except OSError as ex:
            time.sleep(0.1)
            if time.perf_counter() - start_time >= timeout:
                raise TimeoutError(
                    "Waited too long for the port {} on host {} to start accepting "
                    "connections.".format(port, host)
                ) from ex

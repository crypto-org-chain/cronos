import json
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

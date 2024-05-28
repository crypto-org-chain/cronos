from pathlib import Path

import tomlkit


def patch_toml(path: Path, kwargs):
    doc = tomlkit.parse(path.read_text())
    for k, v in kwargs.items():
        keys = k.split(".")
        assert len(keys) > 0
        cur = doc
        for section in keys[:-1]:
            cur = cur[section]
        cur[keys[-1]] = v
    path.write_text(tomlkit.dumps(doc))

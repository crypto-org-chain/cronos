import os
from pathlib import Path

import yaml
from deepdiff import DeepDiff
from pystarport.expansion import expand_yaml


def test_expansion():
    template_with_dotenv = Path(__file__).parent / "template_with_dotenv.yaml"
    template_no_dotenv = Path(__file__).parent / "template_no_dotenv.yaml"

    # `expand_yaml` is backward compatible, not expanded, and no diff
    assert yaml.safe_load(open(template_no_dotenv)) == expand_yaml(
        template_no_dotenv, None
    )

    # `expand_yaml` is expanded but no diff
    assert not DeepDiff(
        yaml.safe_load(open(template_no_dotenv)),
        expand_yaml(template_with_dotenv, None),
        ignore_order=True,
    )

    # overriding dotenv with relative path is expanded and has diff)
    assert DeepDiff(
        yaml.safe_load(open(template_no_dotenv)),
        expand_yaml(template_with_dotenv, ".env1"),
        ignore_order=True,
    ) == {
        "values_changed": {
            "root['cronos_777-1']['validators'][0]['mnemonic']": {
                "new_value": "good",
                "old_value": "visit craft resemble online window solution west chuckle "
                "music diesel vital settle comic tribe project blame bulb armed flower "
                "region sausage mercy arrive release",
            }
        }
    }

    # overriding dotenv with absolute path is expanded and has diff
    assert DeepDiff(
        yaml.safe_load(open(template_no_dotenv)),
        expand_yaml(template_with_dotenv, os.path.abspath("test_expansion/.env1")),
        ignore_order=True,
    ) == {
        "values_changed": {
            "root['cronos_777-1']['validators'][0]['mnemonic']": {
                "new_value": "good",
                "old_value": "visit craft resemble online window solution west chuckle "
                "music diesel vital settle comic tribe project blame bulb armed flower "
                "region sausage mercy arrive release",
            }
        }
    }

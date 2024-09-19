from .utils import merge_genesis


def test_merge_genesis():
    g = merge_genesis(
        {"app_state": {"auth": {"accounts": [1]}}},
        {"app_state": {"auth": {"accounts": [2]}}},
    )
    assert g["app_state"]["auth"]["accounts"] == [1, 2]

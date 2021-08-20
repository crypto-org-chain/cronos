#!/usr/bin/env python
import getpass
from pathlib import Path

import fire
from cprotobuf import Field, ProtoEntity
from eth_account import Account

from .utils import sign_valiadtor


class DelegateKeysSignMsg(ProtoEntity):
    validator_address = Field("string", 1)
    nonce = Field("uint64", 2)


class CLI:
    def sign_validator(
        self, keystore: str, validator_address: str, nonce: int, passphrase=None
    ):
        if passphrase is None:
            passphrase = getpass.getpass("keystore passphrase:")
        key = Account.decrypt(Path(keystore).read_text(), passphrase)
        acct = Account.from_key(key)
        return sign_valiadtor(acct, validator_address, nonce)

    def decrypt_keystore(self, keystore: str, passphrase=None):
        if passphrase is None:
            passphrase = getpass.getpass("keystore passphrase:")
        return Account.decrypt(Path(keystore).read_text(), passphrase).hex()


if __name__ == "__main__":
    fire.Fire(CLI())

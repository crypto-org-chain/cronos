#!/usr/bin/env python
import getpass
from pathlib import Path

import eth_utils
import fire
from cprotobuf import Field, ProtoEntity
from eth_account import Account
from eth_account.messages import encode_defunct


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
        msg = DelegateKeysSignMsg(validator_address=validator_address, nonce=nonce)
        sign_bytes = eth_utils.keccak(msg.SerializeToString())

        signed = acct.sign_message(encode_defunct(sign_bytes))
        return eth_utils.to_hex(signed.signature)

    def decrypt_keystore(self, keystore: str, passphrase=None):
        if passphrase is None:
            passphrase = getpass.getpass("keystore passphrase:")
        return Account.decrypt(Path(keystore).read_text(), passphrase).hex()


if __name__ == "__main__":
    fire.Fire(CLI())

import web3
from eth_account import Account

from .types import PeerPacket
from .utils import send_transaction

TX_AMOUNT = 1000


def sendtx(cli, peer: PeerPacket):
    w3 = web3.Web3(web3.providers.HTTPProvider("http://localhost:8545"))
    assert w3.eth.chain_id == 777
    acct = export_eth_account(cli, "account")
    print("test address", acct.address, "balance", w3.eth.get_balance(acct.address))

    nonce = w3.eth.get_transaction_count(acct.address)
    for i in range(TX_AMOUNT):
        tx = {
            "to": "0x0000000000000000000000000000000000000000",
            "value": 1,
            "nonce": nonce,
            "gas": 21000,
        }
        try:
            send_transaction(w3, tx, acct, wait=False)
        except ValueError as e:
            if "invalid nonce" in str(e):
                # reset nonce and continue
                nonce = w3.eth.get_transaction_count(acct.address)
                continue
            raise
        nonce += 1


def export_eth_account(cli, name: str) -> Account:
    return Account.from_key(
        cli("keys", "unsafe-export-eth-key", name, keyring_backend="test")
    )

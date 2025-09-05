from hexbytes import HexBytes

from integration_tests.utils import (
    CONTRACTS,
    deploy_contract,
    derive_new_account,
    fund_acc,
    get_contract,
)

DELEGATION_PREFIX = "0xef0100"


def address_to_delegation(address: str):
    return DELEGATION_PREFIX + address[2:]


def send_eip7702_transaction(w3, account, target_address):
    nonce = w3.eth.get_transaction_count(account.address)
    auth = {
        "chainId": w3.eth.chain_id,
        "address": target_address,
        "nonce": nonce + 1,
    }
    signed_auth = account.sign_authorization(auth)
    tx = account.sign_transaction(
        {
            "chainId": w3.eth.chain_id,
            "type": 4,
            "to": account.address,
            "value": 0,
            "gas": 100000,
            "maxFeePerGas": 1000000000000,
            "maxPriorityFeePerGas": 10000,
            "nonce": nonce,
            "authorizationList": [signed_auth],
        }
    )
    tx_hash = w3.eth.send_raw_transaction(tx.raw_transaction)
    w3.eth.wait_for_transaction_receipt(tx_hash, timeout=30)

    # Verify the code was set correctly
    code = w3.eth.get_code(account.address, "latest")
    expected_code = address_to_delegation(target_address)
    expected_code_hex = HexBytes(expected_code)
    assert code == expected_code_hex, f"Expected code {expected_code_hex}, got {code}"

    # Verify the nonce was incremented correctly
    new_nonce = w3.eth.get_transaction_count(account.address)
    assert new_nonce == nonce + 2, f"Expected nonce {nonce + 2}, got {new_nonce}"

    return tx_hash


def test_eip7702_basic(cronos):
    w3 = cronos.w3

    account_code = "0x4Cd241E8d1510e30b2076397afc7508Ae59C66c9"

    # use an new account for the test
    # genisis accounts are default BaseAccount, with no code hash storage
    acc = derive_new_account(n=2)
    fund_acc(w3, acc)

    send_eip7702_transaction(w3, acc, account_code)


def test_eip7702_simple_7702_account(cronos):
    w3 = cronos.w3

    account_impl = deploy_contract(
        w3,
        CONTRACTS["Simple7702Account"],
    )

    account = derive_new_account(n=2)
    fund_acc(w3, account)

    send_eip7702_transaction(w3, account, account_impl.address)

    # after the account is delegated, it can act like an Simple7702Account contract
    # the EOA now can execute batch transactions on itself
    account_contract = get_contract(w3, account.address, CONTRACTS["Simple7702Account"])
    tx = account_contract.functions.executeBatch(
        [
            {
                "target": "0x0000000000000000000000000000000000001234",
                "value": 1,
                "data": "0x",
            },
            {
                "target": "0x0000000000000000000000000000000000001235",
                "value": 2,
                "data": "0x",
            },
        ]
    ).build_transaction(
        {
            "from": account.address,
            "nonce": w3.eth.get_transaction_count(account.address),
            "gas": 1000000,
            "gasPrice": w3.eth.gas_price,
        }
    )

    balance = w3.eth.get_balance("0x0000000000000000000000000000000000001234")
    assert balance == 0
    balance = w3.eth.get_balance("0x0000000000000000000000000000000000001235")
    assert balance == 0

    signed_tx = account.sign_transaction(tx)
    tx_hash = w3.eth.send_raw_transaction(signed_tx.raw_transaction)
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash)
    assert receipt.status == 1

    balance = w3.eth.get_balance("0x0000000000000000000000000000000000001234")
    assert balance == 1
    balance = w3.eth.get_balance("0x0000000000000000000000000000000000001235")
    assert balance == 2

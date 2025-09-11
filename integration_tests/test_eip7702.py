from concurrent.futures import ThreadPoolExecutor, as_completed

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


def send_eip7702_transaction(
    w3, account, target_address, verify=True, provider="cronos"
):
    nonce = w3.eth.get_transaction_count(account.address)
    auth = {
        "chainId": w3.eth.chain_id,
        "address": target_address,
        "nonce": nonce + 1,
    }
    signed_auth = account.sign_authorization(auth)
    base_fee = w3.eth.get_block("latest")["baseFeePerGas"]
    signed_tx = account.sign_transaction(
        {
            "chainId": w3.eth.chain_id,
            "type": 4,
            "to": account.address,
            "gas": 50000,
            "maxFeePerGas": base_fee,
            "maxPriorityFeePerGas": 1,
            "nonce": nonce,
            "authorizationList": [signed_auth],
            "data": b"",
        }
    )
    tx_hash = w3.eth.send_raw_transaction(signed_tx.raw_transaction)
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash)

    if not verify:
        return receipt

    # Verify the code was set correctly
    code = w3.eth.get_code(account.address)
    expected_code = address_to_delegation(target_address)
    expected_code_hex = (
        HexBytes(expected_code)
        if target_address != "0x0000000000000000000000000000000000000000"
        else HexBytes("0x")
    )
    assert (
        code == expected_code_hex
    ), f"Expected code {expected_code_hex}, got {code}, {provider}"

    # Verify the nonce was incremented correctly
    new_nonce = w3.eth.get_transaction_count(account.address)
    assert (
        new_nonce == nonce + 2
    ), f"Expected nonce {nonce + 2}, got {new_nonce}, {provider}"

    return receipt


def test_eip7702_basic(cronos):
    w3 = cronos.w3

    target_address = "0x4Cd241E8d1510e30b2076397afc7508Ae59C66c9"

    # use an new account for the test
    # genisis accounts are default BaseAccount, with no code hash storage
    acc = derive_new_account(n=2)
    fund_acc(w3, acc)

    send_eip7702_transaction(w3, acc, target_address)


def test_eip7702_simple_7702_account(cronos):
    """
    Try to replicate the test in https://eip7702.io/examples#transaction-batching
    delegate the account to the Simple7702Account contract
    then use the account to execute batch transactions
    """
    w3 = cronos.w3

    account_impl = deploy_contract(
        w3,
        CONTRACTS["Simple7702Account"],
    )

    account = derive_new_account(n=2)
    fund_acc(w3, account)

    send_eip7702_transaction(w3, account, account_impl.address)

    acct1 = derive_new_account(n=100)
    acct2 = derive_new_account(n=101)
    acct1_balance = 1
    acct2_balance = 2
    balance = w3.eth.get_balance(acct1.address)
    assert balance == 0
    balance = w3.eth.get_balance(acct2.address)
    assert balance == 0

    account_contract = get_contract(w3, account.address, CONTRACTS["Simple7702Account"])
    tx = account_contract.functions.executeBatch(
        [
            {
                "target": acct1.address,
                "value": acct1_balance,
                "data": "0x",
            },
            {
                "target": acct2.address,
                "value": acct2_balance,
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

    signed_tx = account.sign_transaction(tx)
    tx_hash = w3.eth.send_raw_transaction(signed_tx.raw_transaction)
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash)
    assert receipt.status == 1

    balance = w3.eth.get_balance(acct1.address)
    assert balance == acct1_balance
    balance = w3.eth.get_balance(acct2.address)
    assert balance == acct2_balance

    # revoke the delegation, the code hash of the account should be
    # 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470
    send_eip7702_transaction(w3, account, "0x0000000000000000000000000000000000000000")

    utils = deploy_contract(
        w3,
        CONTRACTS["Utils"],
    )
    code_hash = utils.functions.getCodeHash(account.address).call()
    assert code_hash == HexBytes(
        "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
    )


def test_eip7702_delegate_contract_without_fallback(cronos, geth):
    """
    Test when EOA delegate to a contract without fallback/receive
    EVM try to execute the contract with empty input, the tx will be failed
    But the code will be set
    """

    def process(w3, provider):
        acc = derive_new_account(n=10)
        fund_acc(w3, acc)
        counter_address = deploy_contract(w3, CONTRACTS["Counter"]).address

        # the code is checked inside this function
        receipt = send_eip7702_transaction(
            w3, acc, counter_address, verify=True, provider=provider
        )

        return receipt

    providers = [(cronos.w3, "cronos"), (geth.w3, "geth")]
    with ThreadPoolExecutor(len(providers)) as exec:
        tasks = [exec.submit(process, w3, provider) for w3, provider in providers]
        res = [future.result() for future in as_completed(tasks)]
        assert res[0]["status"] == res[1]["status"] == 0, res

import base64
import json

import requests
from eth_account import Account
from pystarport import ports

from .eip712_utils import (
    create_message_send,
    create_tx_raw_eip712,
    encode_structured_data_legacy,
    signature_to_web3_extension,
)
from .utils import ADDRS, KEYS


def broadcast_native_eip712_tx(cronos, src, mutate_auth_info=None):
    """
    Sign a legacy EIP-712 MsgSend as `src`, optionally mutate AuthInfo after
    signing, broadcast it and return the sync tx_response.
    """
    cli = cronos.cosmos_cli()
    w3 = cronos.w3
    chain_id = w3.eth.chain_id
    chain = {
        "chainId": chain_id,
        "cosmosChainId": f"cronos_{chain_id}-1",
    }
    src_addr = cli.address(src)
    src_account = cli.account(src_addr)
    sender = {
        "accountAddress": src_addr,
        "sequence": w3.eth.get_transaction_count(ADDRS[src]),
        "accountNumber": int(src_account["account"]["value"]["account_number"]),
        "pubkey": json.loads(cli.address(src, "acc", "pubkey"))["key"],
    }
    denom = "basetcro"
    dst_addr = cli.address("signer1")
    gas = 200000
    gas_price = 100000000000  # default base fee
    fee = {
        "amount": str(gas * gas_price),
        "denom": denom,
        "gas": str(gas),
    }
    amount = "1"
    params = {
        "destinationAddress": dst_addr,
        "amount": amount,
        "denom": denom,
    }
    tx = create_message_send(chain, sender, fee, "", params)
    structured_msg = encode_structured_data_legacy(tx["eipToSign"])
    signed = Account.sign_message(structured_msg, KEYS[src])
    extension = signature_to_web3_extension(
        chain,
        sender,
        signed.signature,
    )
    legacy_amino = tx["legacyAmino"]
    auth_info = legacy_amino["authInfo"]
    if mutate_auth_info is not None:
        mutate_auth_info(auth_info)
    signed_tx = create_tx_raw_eip712(
        legacy_amino["body"],
        auth_info,
        extension,
    )
    tx_bytes = base64.b64encode(signed_tx["message"].SerializeToString())
    body = {
        "tx_bytes": tx_bytes.decode("utf-8"),
        "mode": "BROADCAST_MODE_SYNC",
    }
    p = ports.api_port(cronos.base_port(0))
    url = f"http://127.0.0.1:{p}/cosmos/tx/v1beta1/txs"
    response = requests.post(url, json=body)
    if not response.ok:
        raise Exception(
            f"response code: {response.status_code}, "
            f"{response.reason}, {response.json()}"
        )
    return response.json()["tx_response"], gas


def test_native_tx(cronos):
    """
    test eip-712 tx works:
    """
    cli = cronos.cosmos_cli()
    rsp, gas = broadcast_native_eip712_tx(cronos, "community")
    assert rsp["code"] == 0, rsp["raw_log"]
    rsp = cli.event_query_tx_for(rsp["txhash"])
    assert rsp["gas_wanted"] == str(gas)


def test_native_tx_fee_granter_rejected(cronos):
    """
    AuthInfo.Fee.granter is not part of the legacy EIP-712 typed data, so a
    relayer setting it after signing must be rejected even when the granter
    holds a live feegrant allowance for the signer.
    """
    cli = cronos.cosmos_cli()
    granter = cli.address("signer1")
    grantee = cli.address("community")
    rsp = cli.grant(granter, grantee, "100000000000000000basetcro")
    assert rsp["code"] == 0, rsp["raw_log"]
    granter_balance = cli.balance(granter)

    def set_granter(auth_info):
        auth_info.fee.granter = granter

    rsp, _ = broadcast_native_eip712_tx(cronos, "community", set_granter)
    assert rsp["code"] != 0, rsp
    assert "does not commit fee granter" in rsp["raw_log"], rsp["raw_log"]
    assert cli.balance(granter) == granter_balance

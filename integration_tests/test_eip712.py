import json
import subprocess

from eth_account import Account
from eth_account.messages import encode_structured_data
from pystarport import ports

from .eip712_utils import (
    create_message_send,
    create_tx_raw_eip712,
    signature_to_web3_extension,
)
from .utils import ADDRS, KEYS


def test_native_tx(cronos):
    """
    test eip-712 tx works:
    """
    cli = cronos.cosmos_cli()
    w3 = cronos.w3
    chain_id = w3.eth.chain_id
    chain = {
        "chainId": chain_id,
        "cosmosChainId": f"cronos_{chain_id}-1",
    }
    src = "community"
    src_addr = cli.address(src)
    src_account = cli.account(src_addr)
    sender = {
        "accountAddress": src_addr,
        "sequence": w3.eth.get_transaction_count(ADDRS[src]),
        "accountNumber": int(src_account["base_account"]["account_number"]),
        "pubkey": json.loads(cli.address(src, "acc", "pubkey"))["key"],
    }
    denom = "basetcro"
    dst_addr = cli.address("signer1")
    gas = "200000"
    fee = {
        "amount": "20",
        "denom": denom,
        "gas": gas,
    }
    amount = "1"
    params = {
        "destinationAddress": dst_addr,
        "amount": amount,
        "denom": denom,
    }
    tx = create_message_send(chain, sender, fee, "", params)
    structured_msg = encode_structured_data(tx["eipToSign"])
    signed = Account.sign_message(structured_msg, KEYS[src])
    extension = signature_to_web3_extension(
        chain,
        sender,
        signed.signature,
    )
    legacy_amino = tx["legacyAmino"]
    signed_tx = create_tx_raw_eip712(
        legacy_amino["body"],
        legacy_amino["authInfo"],
        extension,
    )
    tx_bytes = list(signed_tx["message"].SerializeToString())
    body = {
        "tx_bytes": tx_bytes,
        "mode": "BROADCAST_MODE_BLOCK",
    }
    p = ports.api_port(cronos.base_port(0))
    url = f"http://127.0.0.1:{p}/cosmos/tx/v1beta1/txs"
    raw = f"curl -s -X POST '{url}' -d '{json.dumps(body)}'"
    res = json.loads(subprocess.getoutput(raw))["tx_response"]
    assert res["code"] == 0
    assert res["gas_wanted"] == gas

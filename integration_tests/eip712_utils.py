import base64
import json
from pathlib import Path

import sha3

from .protobuf.cosmos.bank.v1beta1.tx_pb2 import MsgSend
from .protobuf.cosmos.base.v1beta1.coin_pb2 import Coin
from .protobuf.cosmos.crypto.secp256k1.keys_pb2 import PubKey
from .protobuf.cosmos.tx.v1beta1.tx_pb2 import (
    AuthInfo,
    Fee,
    ModeInfo,
    SignDoc,
    SignerInfo,
    TxBody,
    TxRaw,
)
from .protobuf.ethermint.crypto.v1.ethsecp256k1.keys_pb2 import PubKey as EPubKey
from .protobuf.ethermint.types.v1.web3_pb2 import ExtensionOptionsWeb3Tx
from .protobuf.google.protobuf.any_pb2 import Any

LEGACY_AMINO = 127
SIGN_DIRECT = 1


def create_message_send(
    chain,
    sender,
    fee,
    memo,
    params,
):
    # EIP712
    fee_object = generate_fee(
        fee["amount"],
        fee["denom"],
        fee["gas"],
        sender["accountAddress"],
    )
    types = generate_types()
    msg = create_msg_send(
        params["amount"],
        params["denom"],
        sender["accountAddress"],
        params["destinationAddress"],
    )
    messages = generate_message(
        str(sender["accountNumber"]),
        str(sender["sequence"]),
        chain["cosmosChainId"],
        memo,
        fee_object,
        msg,
    )
    eip_to_sign = create_eip712(types, chain["chainId"], messages)
    msg_send = proto_msg_send(
        sender["accountAddress"],
        params["destinationAddress"],
        params["amount"],
        params["denom"],
    )
    tx = create_transaction(
        msg_send,
        memo,
        fee["amount"],
        fee["denom"],
        fee["gas"],
        "ethsecp256",
        sender["pubkey"],
        sender["sequence"],
        sender["accountNumber"],
        chain["cosmosChainId"],
    )
    return {
        "signDirect": tx["signDirect"],
        "legacyAmino": tx["legacyAmino"],
        "eipToSign": eip_to_sign,
    }


def generate_fee(amount, denom, gas, fee_payer):
    return {
        "amount": [
            {
                "amount": amount,
                "denom": denom,
            },
        ],
        "gas": gas,
        "feePayer": fee_payer,
    }


def generate_types():
    return json.loads((Path(__file__).parent / "msg_send_types.json").read_text())


def create_msg_send(amount, denom, from_address, to_address):
    return {
        "type": "cosmos-sdk/MsgSend",
        "value": {
            "amount": [
                {
                    "amount": amount,
                    "denom": denom,
                },
            ],
            "from_address": from_address,
            "to_address": to_address,
        },
    }


def generate_message(account_number, sequence, chain_cosmos_id, memo, fee, msg):
    return generate_message_with_multiple_transactions(
        account_number,
        sequence,
        chain_cosmos_id,
        memo,
        fee,
        [msg],
    )


def generate_message_with_multiple_transactions(
    account_number,
    sequence,
    chain_cosmos_id,
    memo,
    fee,
    msgs,
):
    return {
        "account_number": account_number,
        "chain_id": chain_cosmos_id,
        "fee": fee,
        "memo": memo,
        "msgs": msgs,
        "sequence": sequence,
    }


def create_eip712(types, chain_id, message, name="Cosmos Web3", contract="cosmos"):
    return {
        "types": types,
        "primaryType": "Tx",
        "domain": {
            "name": name,
            "version": "1.0.0",
            "chainId": chain_id,
            "verifyingContract": contract,
            "salt": "0",
        },
        "message": message,
    }


def create_transaction(
    message,
    memo,
    fee,
    denom,
    gas_limit,
    algo,
    pub_key,
    sequence,
    account_number,
    chain_id,
):
    return create_transaction_with_multiple_messages(
        [message],
        memo,
        fee,
        denom,
        gas_limit,
        algo,
        pub_key,
        sequence,
        account_number,
        chain_id,
    )


def create_transaction_with_multiple_messages(
    messages,
    memo,
    fee,
    denom,
    gas_limit,
    algo,
    pub_key,
    sequence,
    account_number,
    chain_id,
):
    body = create_body_with_multiple_messages(messages, memo)
    fee_message = create_fee(fee, denom, gas_limit)
    pub_key_decoded = base64.b64decode(pub_key.encode("ascii"))
    # AMINO
    sign_info_amino = create_signer_info(
        algo,
        pub_key_decoded,
        sequence,
        LEGACY_AMINO,
    )
    auth_info_amino = create_auth_info(sign_info_amino, fee_message)
    sig_doc_amino = create_sig_doc(
        body.SerializeToString(),
        auth_info_amino.SerializeToString(),
        chain_id,
        account_number,
    )

    hash_amino = sha3.keccak_256()
    hash_amino.update(sig_doc_amino.SerializeToString())
    to_sign_amino = hash_amino.hexdigest()

    # SignDirect
    sig_info_direct = create_signer_info(
        algo,
        pub_key_decoded,
        sequence,
        SIGN_DIRECT,
    )
    auth_info_direct = create_auth_info(sig_info_direct, fee_message)
    sign_doc_direct = create_sig_doc(
        body.SerializeToString(),
        auth_info_direct.SerializeToString(),
        chain_id,
        account_number,
    )
    hash_direct = sha3.keccak_256()
    hash_direct.update(sign_doc_direct.SerializeToString())
    to_sign_direct = hash_direct.hexdigest()
    return {
        "legacyAmino": {
            "body": body,
            "authInfo": auth_info_amino,
            "signBytes": base64.b64decode(to_sign_amino),
        },
        "signDirect": {
            "body": body,
            "authInfo": auth_info_direct,
            "signBytes": base64.b64decode(to_sign_direct),
        },
    }


def create_body_with_multiple_messages(messages, memo):
    content = []
    for message in messages:
        content.append(create_any_message(message))
    body = TxBody(memo=memo, messages=content)
    return body


def create_any_message(msg):
    any = Any()
    any.Pack(msg["message"], "/")
    return any


def create_signer_info(algo, public_key, sequence, mode):
    message = None
    path = None
    if algo == "secp256k1":
        message = PubKey(key=public_key)
        path = "cosmos.crypto.secp256k1.PubKey"
    else:
        message = EPubKey(key=public_key)
        path = "ethermint.crypto.v1.ethsecp256k1.PubKey"

    pubkey = {
        "message": message,
        "path": path,
    }
    single = ModeInfo.Single(mode=mode)
    mode_info = ModeInfo(single=single)
    signer_info = SignerInfo(
        mode_info=mode_info,
        sequence=sequence,
        public_key=create_any_message(pubkey),
    )
    return signer_info


def create_auth_info(signer_info, fee):
    auth_info = AuthInfo(
        signer_infos=[signer_info],
        fee=fee,
    )
    return auth_info


def create_sig_doc(body_bytes, auth_info_bytes, chain_id, account_number):
    sign_doc = SignDoc(
        body_bytes=body_bytes,
        auth_info_bytes=auth_info_bytes,
        chain_id=chain_id,
        account_number=account_number,
    )
    return sign_doc


def create_fee(fee, denom, gas_limit):
    value = Coin(denom=denom, amount=fee)
    fee = Fee(gas_limit=int(gas_limit), amount=[value])
    return fee


def proto_msg_send(from_address, to_address, amount, denom):
    value = Coin(denom=denom, amount=amount)
    message = MsgSend(
        from_address=from_address,
        to_address=to_address,
        amount=[value],
    )
    return {
        "message": message,
        "path": "cosmos.bank.v1beta1.MsgSend",
    }


def signature_to_web3_extension(chain, sender, signature):
    message = ExtensionOptionsWeb3Tx(
        typed_data_chain_id=chain["chainId"],
        fee_payer=sender["accountAddress"],
        fee_payer_sig=signature,
    )
    return {
        "message": message,
        "path": "ethermint.types.v1.ExtensionOptionsWeb3Tx",
    }


def create_tx_raw(body_bytes, auth_info_bytes, signatures):
    message = TxRaw(
        body_bytes=body_bytes,
        auth_info_bytes=auth_info_bytes,
        signatures=signatures,
    )
    return {
        "message": message,
        "path": "cosmos.tx.v1beta1.TxRaw",
    }


def create_tx_raw_eip712(body, auth_info, extension):
    any = create_any_message(extension)
    body.extension_options.append(any)
    return create_tx_raw(
        body.SerializeToString(),
        auth_info.SerializeToString(),
        [bytes()],
    )

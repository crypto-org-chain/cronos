import base64
import eth_utils
import hashlib
import sha3
import rlp
import cosmos.base.v1beta1.coin_pb2
import cosmos.bank.v1beta1.tx_pb2
import cosmos.tx.v1beta1.tx_pb2
import google.protobuf.any_pb2
import cosmos.crypto.secp256k1.keys_pb2
import ethermint.crypto.v1.ethsecp256k1.keys_pb2
import ethermint.types.v1.web3_pb2
from .utils import (
    KEYS,
)

MAX_SAFE_INTEGER = 1.7976931348623157e+308
MSG_SEND_TYPES = {
    "MsgValue": [
      { "name": "from_address", "type": "string" },
      { "name": "to_address", "type": "string" },
      { "name": "amount", "type": "TypeAmount[]" },
    ],
    "TypeAmount": [
      { "name": "denom", "type": "string" },
      { "name": "amount", "type": "string" },
    ],
}

LEGACY_AMINO = 127
SIGN_DIRECT = 1

def create_message_send(chain, sender, fee, memo, params):
    # EIP712
    fee_object = generate_fee(
        fee["amount"],
        fee["denom"],
        fee["gas"],
        sender["accountAddress"],
    )
    types = generate_types(MSG_SEND_TYPES)
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
        "amount": [{
            "amount": amount,
            "denom": denom,
        }],
        "gas": gas,
        "feePayer": fee_payer,
    }


def generate_types(msg_values):
    types = {
        "EIP712Domain": [
            { "name": "name", "type": "string" },
            { "name": "version", "type": "string" },
            { "name": "chainId", "type": "uint256" },
            { "name": "verifyingContract", "type": "string" },
            { "name": "salt", "type": "string" },
        ],
        "Tx": [
            { "name": "account_number", "type": "string" },
            { "name": "chain_id", "type": "string" },
            { "name": "fee", "type": "Fee" },
            { "name": "memo", "type": "string" },
            { "name": "msgs", "type": "Msg[]" },
            { "name": "sequence", "type": "string" },
        ],
        "Fee": [
            { "name": "feePayer", "type": "string" },
            { "name": "amount", "type": "Coin[]" },
            { "name": "gas", "type": "string" },
        ],
        "Coin": [
            { "name": "denom", "type": "string" },
            { "name": "amount", "type": "string" },
        ],
        "Msg": [
            { "name": "type", "type": "string" },
            { "name": "value", "type": "MsgValue" },
        ],
    }
    types.update(msg_values)
    return types


def create_msg_send(amount, denom, from_address, to_address):
    return {
        "type": "cosmos-sdk/MsgSend",
        "value": {
            "amount": [{
                "amount": amount,
                "denom": denom,
            }],
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


def generate_message_with_multiple_transactions(account_number, sequence, chain_cosmos_id, memo, fee, msgs):
    return {
        "account_number": account_number,
        "chain_id": chain_cosmos_id,
        "fee": fee,
        "memo": memo,
        "msgs": msgs,
        "sequence": sequence,
    }


def create_eip712(types, chain_id, message, name="Cronos Web3", contract="cronos"):
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


def create_transaction(message, memo, fee, denom, gas_limit, algo, pub_key, sequence, account_number, chain_id):
    return create_transaction_with_multiple_messages([message], memo, fee, denom, gas_limit, algo, pub_key, sequence, account_number, chain_id)


def create_transaction_with_multiple_messages(messages, memo, fee, denom, gas_limit, algo, pub_key, sequence, account_number, chain_id):
    body = create_body_with_multiple_messages(messages, memo)
    fee_message = create_fee(fee, denom, gas_limit)
    pub_key_decoded = base64.b64decode(pub_key.encode("ascii"))
    # AMINO
    sign_info_amino = create_signer_info(
        algo,
        pub_key_decoded, # new Uint8Array(pub_key_decoded),
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
    hash_amino.update(sig_doc_amino.SerializeToString()) # TODO
    to_sign_amino = hash_amino.hexdigest()
   
    # SignDirect
    sig_info_direct = create_signer_info(
        algo,
        pub_key_decoded, # TODO
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
    hash_direct.update(sign_doc_direct.SerializeToString()) # TODO
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
    # tx.cosmos.tx.v1beta1.TxBody
    body = cosmos.tx.v1beta1.tx_pb2.TxBody(memo = memo)
    for message in messages:
        body.messages.append(create_any_message(message))
    return body


def create_any_message(msg):
    # google.google.protobuf.Any
    any = google.protobuf.any_pb2.Any()
    path = msg["path"]
    any.type_url = f"/{path}"
    any.value = msg["message"].SerializeToString()
    return any


def create_signer_info(algo, public_key, sequence, mode):
    message = None
    path = None
    # NOTE: secp256k1 is going to be removed from evmos
    if algo == "secp256k1":
        message = cosmos.crypto.secp256k1.keys_pb2.PubKey(key=public_key)
        path = "cosmos.crypto.secp256k1.PubKey"
    else:
        # NOTE: assume ethsecp256k1 by default because after mainnet is the only one that is going to be supported
        message = ethermint.crypto.v1.ethsecp256k1.keys_pb2.PubKey(key=public_key)
        path = "ethermint.crypto.v1.ethsecp256k1.PubKey"

    pubkey = {
        "message": message,
        "path": path,
    }
    single = cosmos.tx.v1beta1.tx_pb2.ModeInfo.Single(mode = mode)
    mode_info = cosmos.tx.v1beta1.tx_pb2.ModeInfo()
    mode_info.single.CopyFrom(single)
    signer_info = cosmos.tx.v1beta1.tx_pb2.SignerInfo()
    signer_info.mode_info.CopyFrom(mode_info)
    signer_info.sequence = sequence
    signer_info.public_key.CopyFrom(create_any_message(pubkey))
    return signer_info


def create_auth_info(signer_info, fee):
    auth_info = cosmos.tx.v1beta1.tx_pb2.AuthInfo()
    auth_info.signer_infos.append(signer_info)
    auth_info.fee.CopyFrom(fee)
    return auth_info


def create_sig_doc(body_bytes, auth_info_bytes, chain_id, account_number):
    sign_doc = cosmos.tx.v1beta1.tx_pb2.SignDoc(
        body_bytes = body_bytes,
        auth_info_bytes = auth_info_bytes,
        chain_id = chain_id,
        account_number = account_number,
    )
    return sign_doc


def create_fee(fee, denom, gas_limit):
    # coin.cosmos.base.v1beta1.Coin
    value = cosmos.base.v1beta1.coin_pb2.Coin(
        denom = denom,
        amount = fee,
    )
    # coin.cosmos.base.v1beta1.Fee
    fee = cosmos.tx.v1beta1.tx_pb2.Fee(gas_limit = gas_limit)
    fee.amount.append(value)
    return fee


def proto_msg_send(from_address, to_address, amount, denom):
    # coin.cosmos.base.v1beta1.Coin
    value = cosmos.base.v1beta1.coin_pb2.Coin(
        denom = denom,
        amount = amount,
    )
    # bank.cosmos.bank.v1beta1.MsgSend
    message = cosmos.bank.v1beta1.tx_pb2.MsgSend(
        from_address = from_address,
        to_address = to_address,
    )
    message.amount.append(value)
    return {
        "message": message,
        "path": "cosmos.bank.v1beta1.MsgSend",
    }

def arrayify(value):
    return bytearray(value)


def eip712_hash(typed_data, version="V4"):
    # sanitized_data = sanitize_data(typed_data);
    parts = bytes.fromhex("1901")
    eip712 = "EIP712Domain"
    parts += hash_struct(eip712, typed_data["domain"], typed_data["types"], version)
    if typed_data["primaryType"] != eip712:
        parts += hash_struct(typed_data["primaryType"], typed_data["message"], typed_data["types"], version)
    return eth_utils.keccak(parts)


def hash_struct(primary_type, data, types, version):
    print("rlp", primary_type, data, types, version)
    res = rlp.encode(primary_type, data, types, version)
    # TODO
    return eth_utils.keccak(res)


def join_signature():
    return ""


def split_signature():
    return ""


def signature_to_web3_extension(chain, sender, hex_formatted_signature):
    signature = hex_formatted_signature
    temp = hex_formatted_signature.split("0x")
    if (temp.length == 2):
        signature = temp[1]
    message = ethermint.types.v1.web3_pb2.ExtensionOptionsWeb3Tx(
        typed_data_chain_id = chain["chainId"],
        fee_payer = sender["accountAddress"],
        fee_payer_sig = bytes.fromhex(signature),
    )
    return {
        "message": message,
        "path": "ethermint.types.v1.ExtensionOptionsWeb3Tx",
    }


def create_tx_raw(body_bytes, auth_info_bytes, signatures):
    message = cosmos.tx.v1beta1.tx_pb2.TxRaw(
        body_bytes=body_bytes,
        auth_info_bytes=auth_info_bytes,
        signatures=signatures,
    )
    return {
        "message": message,
        "path": "cosmos.tx.v1beta1.TxRaw",
    }


def create_tx_raw_eip712(body, auth_info, extension):
    body["extension_options"].append(create_any_message(extension))
    return create_tx_raw(
        body.SerializeToString(),
        auth_info.SerializeToString(), 
        [bytearray([])], #TODO
    )


chain_id = 777
chain = {
    "chainId": chain_id,
    "cosmosChainId": f"cronos_{chain_id}-1",
}
src = "community"
src_addr = "crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp"
src_nonce = 0
sender = {
    "accountAddress": src_addr,
    "sequence": 1,
    "accountNumber": src_nonce,
    "pubkey": "Am5xCmKjQt4O1NfEUy3Ly7r78ZZS7WeyN++rcOiyB++s",
}
denom = "basetcro"
dst_addr = "crc16z0herz998946wr659lr84c8c556da55dc34hh"
gas = 20000
gas_amount = "20"
fee = {
    "amount": gas_amount,
    "denom": denom,
    "gas": gas,
}
memo = ""
params = {
    "destinationAddress": dst_addr,
    "amount": "1",
    "denom": denom,
}
tx = create_message_send(chain, sender, fee, memo, params)
# print("eipToSign", tx["eipToSign"])
h = eip712_hash(tx["eipToSign"])
print("eip712_hash", h)
data_to_sign = arrayify(h)
signature_raw = hashlib.sha256(KEYS["community"].value.encode()).digest(data_to_sign)
signature = join_signature(signature_raw)
extension = signature_to_web3_extension(
    chain,
    sender,
    signature,
)
legacy_amino = tx["legacyAmino"]
signed_tx = create_tx_raw_eip712(
    legacy_amino["body"],
    legacy_amino["authInfo"],
    extension,
)
body = {
    "tx_bytes": signed_tx["message"].SerializeToString(), 
    "mode": "BROADCAST_MODE_BLOCK"
}

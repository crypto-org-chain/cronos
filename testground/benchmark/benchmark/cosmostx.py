from typing import Optional

from cprotobuf import Field, ProtoEntity


class Any(ProtoEntity):
    type_url = Field("string", 1)
    value = Field("bytes", 2)


def build_any(type_url: str, msg: Optional[ProtoEntity] = None) -> Any:
    value = b""
    if msg is not None:
        value = msg.SerializeToString()
    return Any(type_url=type_url, value=value)


class TxBody(ProtoEntity):
    messages = Field(Any, 1, repeated=True)
    memo = Field("string", 2)
    timeout_height = Field("uint64", 3)
    extension_options = Field(Any, 1023, repeated=True)
    non_critical_extension_options = Field(Any, 2047, repeated=True)


class CompactBitArray(ProtoEntity):
    extra_bits_stored = Field("uint32", 1)
    elems = Field("bytes", 2)


class ModeInfoSingle(ProtoEntity):
    mode = Field("int32", 1)


class ModeInfoMulti(ProtoEntity):
    bitarray = Field(CompactBitArray, 1)
    mode_infos = Field("ModeInfo", 2, repeated=True)


class ModeInfo(ProtoEntity):
    single = Field("ModeInfo.Single", 1)
    multi = Field("ModeInfo.Multi", 2)


class SignerInfo(ProtoEntity):
    public_key = Field(Any, 1)
    mode_info = Field(ModeInfo, 2)
    sequence = Field("uint64", 3)


class Coin(ProtoEntity):
    denom = Field("string", 1)
    amount = Field("string", 2)


class Fee(ProtoEntity):
    amount = Field(Coin, 1, repeated=True)
    gas_limit = Field("uint64", 2)
    payer = Field("string", 3)
    granter = Field("string", 4)


class Tip(ProtoEntity):
    amount = Field(Coin, 1, repeated=True)
    tipper = Field("string", 2)


class AuthInfo(ProtoEntity):
    signer_infos = Field(SignerInfo, 1, repeated=True)
    fee = Field(Fee, 2)
    tip = Field(Tip, 3)


class TxRaw(ProtoEntity):
    body = Field("bytes", 1)
    auth_info = Field("bytes", 2)
    signatures = Field("bytes", 3, repeated=True)


class MsgEthereumTx(ProtoEntity):
    from_ = Field("bytes", 5)
    raw = Field("bytes", 6)

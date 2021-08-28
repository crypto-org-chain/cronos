/* eslint-disable */
import { Reader, Writer } from 'protobufjs/minimal'
import { Coin } from '../cosmos/base/v1beta1/coin'

export const protobufPackage = 'cryptoorgchain.cronos.cronos'

/** MsgConvertToEvmTokens represents a message to convert ibc coins to evm coins. */
export interface MsgConvertTokens {
  address: string
  amount: Coin[]
}

/** MsgConvertToIbcTokens represents a message to convert evm coins to ibc coins. */
export interface MsgSendToCryptoOrg {
  from: string
  to: string
  amount: Coin[]
}

/** MsgMultiSendResponse defines the MsgConvert response type. */
export interface MsgConvertResponse {}

const baseMsgConvertTokens: object = { address: '' }

export const MsgConvertTokens = {
  encode(message: MsgConvertTokens, writer: Writer = Writer.create()): Writer {
    if (message.address !== '') {
      writer.uint32(10).string(message.address)
    }
    for (const v of message.amount) {
      Coin.encode(v!, writer.uint32(18).fork()).ldelim()
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgConvertTokens {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgConvertTokens } as MsgConvertTokens
    message.amount = []
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.address = reader.string()
          break
        case 2:
          message.amount.push(Coin.decode(reader, reader.uint32()))
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): MsgConvertTokens {
    const message = { ...baseMsgConvertTokens } as MsgConvertTokens
    message.amount = []
    if (object.address !== undefined && object.address !== null) {
      message.address = String(object.address)
    } else {
      message.address = ''
    }
    if (object.amount !== undefined && object.amount !== null) {
      for (const e of object.amount) {
        message.amount.push(Coin.fromJSON(e))
      }
    }
    return message
  },

  toJSON(message: MsgConvertTokens): unknown {
    const obj: any = {}
    message.address !== undefined && (obj.address = message.address)
    if (message.amount) {
      obj.amount = message.amount.map((e) => (e ? Coin.toJSON(e) : undefined))
    } else {
      obj.amount = []
    }
    return obj
  },

  fromPartial(object: DeepPartial<MsgConvertTokens>): MsgConvertTokens {
    const message = { ...baseMsgConvertTokens } as MsgConvertTokens
    message.amount = []
    if (object.address !== undefined && object.address !== null) {
      message.address = object.address
    } else {
      message.address = ''
    }
    if (object.amount !== undefined && object.amount !== null) {
      for (const e of object.amount) {
        message.amount.push(Coin.fromPartial(e))
      }
    }
    return message
  }
}

const baseMsgSendToCryptoOrg: object = { from: '', to: '' }

export const MsgSendToCryptoOrg = {
  encode(message: MsgSendToCryptoOrg, writer: Writer = Writer.create()): Writer {
    if (message.from !== '') {
      writer.uint32(10).string(message.from)
    }
    if (message.to !== '') {
      writer.uint32(18).string(message.to)
    }
    for (const v of message.amount) {
      Coin.encode(v!, writer.uint32(26).fork()).ldelim()
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgSendToCryptoOrg {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgSendToCryptoOrg } as MsgSendToCryptoOrg
    message.amount = []
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.from = reader.string()
          break
        case 2:
          message.to = reader.string()
          break
        case 3:
          message.amount.push(Coin.decode(reader, reader.uint32()))
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): MsgSendToCryptoOrg {
    const message = { ...baseMsgSendToCryptoOrg } as MsgSendToCryptoOrg
    message.amount = []
    if (object.from !== undefined && object.from !== null) {
      message.from = String(object.from)
    } else {
      message.from = ''
    }
    if (object.to !== undefined && object.to !== null) {
      message.to = String(object.to)
    } else {
      message.to = ''
    }
    if (object.amount !== undefined && object.amount !== null) {
      for (const e of object.amount) {
        message.amount.push(Coin.fromJSON(e))
      }
    }
    return message
  },

  toJSON(message: MsgSendToCryptoOrg): unknown {
    const obj: any = {}
    message.from !== undefined && (obj.from = message.from)
    message.to !== undefined && (obj.to = message.to)
    if (message.amount) {
      obj.amount = message.amount.map((e) => (e ? Coin.toJSON(e) : undefined))
    } else {
      obj.amount = []
    }
    return obj
  },

  fromPartial(object: DeepPartial<MsgSendToCryptoOrg>): MsgSendToCryptoOrg {
    const message = { ...baseMsgSendToCryptoOrg } as MsgSendToCryptoOrg
    message.amount = []
    if (object.from !== undefined && object.from !== null) {
      message.from = object.from
    } else {
      message.from = ''
    }
    if (object.to !== undefined && object.to !== null) {
      message.to = object.to
    } else {
      message.to = ''
    }
    if (object.amount !== undefined && object.amount !== null) {
      for (const e of object.amount) {
        message.amount.push(Coin.fromPartial(e))
      }
    }
    return message
  }
}

const baseMsgConvertResponse: object = {}

export const MsgConvertResponse = {
  encode(_: MsgConvertResponse, writer: Writer = Writer.create()): Writer {
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgConvertResponse {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgConvertResponse } as MsgConvertResponse
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(_: any): MsgConvertResponse {
    const message = { ...baseMsgConvertResponse } as MsgConvertResponse
    return message
  },

  toJSON(_: MsgConvertResponse): unknown {
    const obj: any = {}
    return obj
  },

  fromPartial(_: DeepPartial<MsgConvertResponse>): MsgConvertResponse {
    const message = { ...baseMsgConvertResponse } as MsgConvertResponse
    return message
  }
}

/** Msg defines the Cronos Msg service */
export interface Msg {
  /** Send defines a method for converting ibc coins to Cronos coins. */
  ConvertTokens(request: MsgConvertTokens): Promise<MsgConvertResponse>
  /** Send defines a method to send coins to Crypto.org chain */
  SendToCryptoOrg(request: MsgSendToCryptoOrg): Promise<MsgConvertResponse>
}

export class MsgClientImpl implements Msg {
  private readonly rpc: Rpc
  constructor(rpc: Rpc) {
    this.rpc = rpc
  }
  ConvertTokens(request: MsgConvertTokens): Promise<MsgConvertResponse> {
    const data = MsgConvertTokens.encode(request).finish()
    const promise = this.rpc.request('cryptoorgchain.cronos.cronos.Msg', 'ConvertTokens', data)
    return promise.then((data) => MsgConvertResponse.decode(new Reader(data)))
  }

  SendToCryptoOrg(request: MsgSendToCryptoOrg): Promise<MsgConvertResponse> {
    const data = MsgSendToCryptoOrg.encode(request).finish()
    const promise = this.rpc.request('cryptoorgchain.cronos.cronos.Msg', 'SendToCryptoOrg', data)
    return promise.then((data) => MsgConvertResponse.decode(new Reader(data)))
  }
}

interface Rpc {
  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>
}

type Builtin = Date | Function | Uint8Array | string | number | undefined
export type DeepPartial<T> = T extends Builtin
  ? T
  : T extends Array<infer U>
  ? Array<DeepPartial<U>>
  : T extends ReadonlyArray<infer U>
  ? ReadonlyArray<DeepPartial<U>>
  : T extends {}
  ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>

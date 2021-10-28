/* eslint-disable */
import { Reader, Writer } from 'protobufjs/minimal'
import { Coin } from '../cosmos/base/v1beta1/coin'

export const protobufPackage = 'cronos'

/** MsgConvertVouchers represents a message to convert ibc voucher coins to cronos evm coins. */
export interface MsgConvertVouchers {
  address: string
  coins: Coin[]
}

/** MsgTransferTokens represents a message to transfer cronos evm coins through ibc. */
export interface MsgTransferTokens {
  from: string
  to: string
  coins: Coin[]
}

/** MsgConvertVouchersResponse defines the ConvertVouchers response type. */
export interface MsgConvertVouchersResponse {}

/** MsgTransferTokensResponse defines the TransferTokens response type. */
export interface MsgTransferTokensResponse {}

/** MsgUpdateTokenMapping defines the request type */
export interface MsgUpdateTokenMapping {
  sender: string
  denom: string
  contract: string
}

/** MsgUpdateTokenMappingResponse defines the response type */
export interface MsgUpdateTokenMappingResponse {}

const baseMsgConvertVouchers: object = { address: '' }

export const MsgConvertVouchers = {
  encode(message: MsgConvertVouchers, writer: Writer = Writer.create()): Writer {
    if (message.address !== '') {
      writer.uint32(10).string(message.address)
    }
    for (const v of message.coins) {
      Coin.encode(v!, writer.uint32(18).fork()).ldelim()
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgConvertVouchers {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgConvertVouchers } as MsgConvertVouchers
    message.coins = []
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.address = reader.string()
          break
        case 2:
          message.coins.push(Coin.decode(reader, reader.uint32()))
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): MsgConvertVouchers {
    const message = { ...baseMsgConvertVouchers } as MsgConvertVouchers
    message.coins = []
    if (object.address !== undefined && object.address !== null) {
      message.address = String(object.address)
    } else {
      message.address = ''
    }
    if (object.coins !== undefined && object.coins !== null) {
      for (const e of object.coins) {
        message.coins.push(Coin.fromJSON(e))
      }
    }
    return message
  },

  toJSON(message: MsgConvertVouchers): unknown {
    const obj: any = {}
    message.address !== undefined && (obj.address = message.address)
    if (message.coins) {
      obj.coins = message.coins.map((e) => (e ? Coin.toJSON(e) : undefined))
    } else {
      obj.coins = []
    }
    return obj
  },

  fromPartial(object: DeepPartial<MsgConvertVouchers>): MsgConvertVouchers {
    const message = { ...baseMsgConvertVouchers } as MsgConvertVouchers
    message.coins = []
    if (object.address !== undefined && object.address !== null) {
      message.address = object.address
    } else {
      message.address = ''
    }
    if (object.coins !== undefined && object.coins !== null) {
      for (const e of object.coins) {
        message.coins.push(Coin.fromPartial(e))
      }
    }
    return message
  }
}

const baseMsgTransferTokens: object = { from: '', to: '' }

export const MsgTransferTokens = {
  encode(message: MsgTransferTokens, writer: Writer = Writer.create()): Writer {
    if (message.from !== '') {
      writer.uint32(10).string(message.from)
    }
    if (message.to !== '') {
      writer.uint32(18).string(message.to)
    }
    for (const v of message.coins) {
      Coin.encode(v!, writer.uint32(26).fork()).ldelim()
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgTransferTokens {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgTransferTokens } as MsgTransferTokens
    message.coins = []
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
          message.coins.push(Coin.decode(reader, reader.uint32()))
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): MsgTransferTokens {
    const message = { ...baseMsgTransferTokens } as MsgTransferTokens
    message.coins = []
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
    if (object.coins !== undefined && object.coins !== null) {
      for (const e of object.coins) {
        message.coins.push(Coin.fromJSON(e))
      }
    }
    return message
  },

  toJSON(message: MsgTransferTokens): unknown {
    const obj: any = {}
    message.from !== undefined && (obj.from = message.from)
    message.to !== undefined && (obj.to = message.to)
    if (message.coins) {
      obj.coins = message.coins.map((e) => (e ? Coin.toJSON(e) : undefined))
    } else {
      obj.coins = []
    }
    return obj
  },

  fromPartial(object: DeepPartial<MsgTransferTokens>): MsgTransferTokens {
    const message = { ...baseMsgTransferTokens } as MsgTransferTokens
    message.coins = []
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
    if (object.coins !== undefined && object.coins !== null) {
      for (const e of object.coins) {
        message.coins.push(Coin.fromPartial(e))
      }
    }
    return message
  }
}

const baseMsgConvertVouchersResponse: object = {}

export const MsgConvertVouchersResponse = {
  encode(_: MsgConvertVouchersResponse, writer: Writer = Writer.create()): Writer {
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgConvertVouchersResponse {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgConvertVouchersResponse } as MsgConvertVouchersResponse
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

  fromJSON(_: any): MsgConvertVouchersResponse {
    const message = { ...baseMsgConvertVouchersResponse } as MsgConvertVouchersResponse
    return message
  },

  toJSON(_: MsgConvertVouchersResponse): unknown {
    const obj: any = {}
    return obj
  },

  fromPartial(_: DeepPartial<MsgConvertVouchersResponse>): MsgConvertVouchersResponse {
    const message = { ...baseMsgConvertVouchersResponse } as MsgConvertVouchersResponse
    return message
  }
}

const baseMsgTransferTokensResponse: object = {}

export const MsgTransferTokensResponse = {
  encode(_: MsgTransferTokensResponse, writer: Writer = Writer.create()): Writer {
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgTransferTokensResponse {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgTransferTokensResponse } as MsgTransferTokensResponse
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

  fromJSON(_: any): MsgTransferTokensResponse {
    const message = { ...baseMsgTransferTokensResponse } as MsgTransferTokensResponse
    return message
  },

  toJSON(_: MsgTransferTokensResponse): unknown {
    const obj: any = {}
    return obj
  },

  fromPartial(_: DeepPartial<MsgTransferTokensResponse>): MsgTransferTokensResponse {
    const message = { ...baseMsgTransferTokensResponse } as MsgTransferTokensResponse
    return message
  }
}

const baseMsgUpdateTokenMapping: object = { sender: '', denom: '', contract: '' }

export const MsgUpdateTokenMapping = {
  encode(message: MsgUpdateTokenMapping, writer: Writer = Writer.create()): Writer {
    if (message.sender !== '') {
      writer.uint32(10).string(message.sender)
    }
    if (message.denom !== '') {
      writer.uint32(18).string(message.denom)
    }
    if (message.contract !== '') {
      writer.uint32(26).string(message.contract)
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgUpdateTokenMapping {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgUpdateTokenMapping } as MsgUpdateTokenMapping
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.sender = reader.string()
          break
        case 2:
          message.denom = reader.string()
          break
        case 3:
          message.contract = reader.string()
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): MsgUpdateTokenMapping {
    const message = { ...baseMsgUpdateTokenMapping } as MsgUpdateTokenMapping
    if (object.sender !== undefined && object.sender !== null) {
      message.sender = String(object.sender)
    } else {
      message.sender = ''
    }
    if (object.denom !== undefined && object.denom !== null) {
      message.denom = String(object.denom)
    } else {
      message.denom = ''
    }
    if (object.contract !== undefined && object.contract !== null) {
      message.contract = String(object.contract)
    } else {
      message.contract = ''
    }
    return message
  },

  toJSON(message: MsgUpdateTokenMapping): unknown {
    const obj: any = {}
    message.sender !== undefined && (obj.sender = message.sender)
    message.denom !== undefined && (obj.denom = message.denom)
    message.contract !== undefined && (obj.contract = message.contract)
    return obj
  },

  fromPartial(object: DeepPartial<MsgUpdateTokenMapping>): MsgUpdateTokenMapping {
    const message = { ...baseMsgUpdateTokenMapping } as MsgUpdateTokenMapping
    if (object.sender !== undefined && object.sender !== null) {
      message.sender = object.sender
    } else {
      message.sender = ''
    }
    if (object.denom !== undefined && object.denom !== null) {
      message.denom = object.denom
    } else {
      message.denom = ''
    }
    if (object.contract !== undefined && object.contract !== null) {
      message.contract = object.contract
    } else {
      message.contract = ''
    }
    return message
  }
}

const baseMsgUpdateTokenMappingResponse: object = {}

export const MsgUpdateTokenMappingResponse = {
  encode(_: MsgUpdateTokenMappingResponse, writer: Writer = Writer.create()): Writer {
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): MsgUpdateTokenMappingResponse {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseMsgUpdateTokenMappingResponse } as MsgUpdateTokenMappingResponse
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

  fromJSON(_: any): MsgUpdateTokenMappingResponse {
    const message = { ...baseMsgUpdateTokenMappingResponse } as MsgUpdateTokenMappingResponse
    return message
  },

  toJSON(_: MsgUpdateTokenMappingResponse): unknown {
    const obj: any = {}
    return obj
  },

  fromPartial(_: DeepPartial<MsgUpdateTokenMappingResponse>): MsgUpdateTokenMappingResponse {
    const message = { ...baseMsgUpdateTokenMappingResponse } as MsgUpdateTokenMappingResponse
    return message
  }
}

/** Msg defines the Cronos Msg service */
export interface Msg {
  /** ConvertVouchers defines a method for converting ibc voucher to cronos evm coins. */
  ConvertVouchers(request: MsgConvertVouchers): Promise<MsgConvertVouchersResponse>
  /** TransferTokens defines a method to transfer cronos evm coins to another chain through IBC */
  TransferTokens(request: MsgTransferTokens): Promise<MsgTransferTokensResponse>
  /** UpdateTokenMapping defines a method to update token mapping */
  UpdateTokenMapping(request: MsgUpdateTokenMapping): Promise<MsgUpdateTokenMappingResponse>
}

export class MsgClientImpl implements Msg {
  private readonly rpc: Rpc
  constructor(rpc: Rpc) {
    this.rpc = rpc
  }
  ConvertVouchers(request: MsgConvertVouchers): Promise<MsgConvertVouchersResponse> {
    const data = MsgConvertVouchers.encode(request).finish()
    const promise = this.rpc.request('cronos.Msg', 'ConvertVouchers', data)
    return promise.then((data) => MsgConvertVouchersResponse.decode(new Reader(data)))
  }

  TransferTokens(request: MsgTransferTokens): Promise<MsgTransferTokensResponse> {
    const data = MsgTransferTokens.encode(request).finish()
    const promise = this.rpc.request('cronos.Msg', 'TransferTokens', data)
    return promise.then((data) => MsgTransferTokensResponse.decode(new Reader(data)))
  }

  UpdateTokenMapping(request: MsgUpdateTokenMapping): Promise<MsgUpdateTokenMappingResponse> {
    const data = MsgUpdateTokenMapping.encode(request).finish()
    const promise = this.rpc.request('cronos.Msg', 'UpdateTokenMapping', data)
    return promise.then((data) => MsgUpdateTokenMappingResponse.decode(new Reader(data)))
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

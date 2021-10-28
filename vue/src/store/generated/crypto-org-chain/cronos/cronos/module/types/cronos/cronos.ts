/* eslint-disable */
import * as Long from 'long'
import { util, configure, Writer, Reader } from 'protobufjs/minimal'

export const protobufPackage = 'cronos'

/** Params defines the parameters for the cronos module. */
export interface Params {
  ibcCroDenom: string
  ibcTimeout: number
  /** the admin address who can update token mapping */
  cronosAdmin: string
  enableAutoDeployment: boolean
}

/** TokenMappingChangeProposal defines a proposal to change one token mapping. */
export interface TokenMappingChangeProposal {
  title: string
  description: string
  denom: string
  contract: string
}

/** TokenMapping defines a mapping between native denom and contract */
export interface TokenMapping {
  denom: string
  contract: string
}

const baseParams: object = { ibcCroDenom: '', ibcTimeout: 0, cronosAdmin: '', enableAutoDeployment: false }

export const Params = {
  encode(message: Params, writer: Writer = Writer.create()): Writer {
    if (message.ibcCroDenom !== '') {
      writer.uint32(10).string(message.ibcCroDenom)
    }
    if (message.ibcTimeout !== 0) {
      writer.uint32(16).uint64(message.ibcTimeout)
    }
    if (message.cronosAdmin !== '') {
      writer.uint32(26).string(message.cronosAdmin)
    }
    if (message.enableAutoDeployment === true) {
      writer.uint32(32).bool(message.enableAutoDeployment)
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): Params {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseParams } as Params
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.ibcCroDenom = reader.string()
          break
        case 2:
          message.ibcTimeout = longToNumber(reader.uint64() as Long)
          break
        case 3:
          message.cronosAdmin = reader.string()
          break
        case 4:
          message.enableAutoDeployment = reader.bool()
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): Params {
    const message = { ...baseParams } as Params
    if (object.ibcCroDenom !== undefined && object.ibcCroDenom !== null) {
      message.ibcCroDenom = String(object.ibcCroDenom)
    } else {
      message.ibcCroDenom = ''
    }
    if (object.ibcTimeout !== undefined && object.ibcTimeout !== null) {
      message.ibcTimeout = Number(object.ibcTimeout)
    } else {
      message.ibcTimeout = 0
    }
    if (object.cronosAdmin !== undefined && object.cronosAdmin !== null) {
      message.cronosAdmin = String(object.cronosAdmin)
    } else {
      message.cronosAdmin = ''
    }
    if (object.enableAutoDeployment !== undefined && object.enableAutoDeployment !== null) {
      message.enableAutoDeployment = Boolean(object.enableAutoDeployment)
    } else {
      message.enableAutoDeployment = false
    }
    return message
  },

  toJSON(message: Params): unknown {
    const obj: any = {}
    message.ibcCroDenom !== undefined && (obj.ibcCroDenom = message.ibcCroDenom)
    message.ibcTimeout !== undefined && (obj.ibcTimeout = message.ibcTimeout)
    message.cronosAdmin !== undefined && (obj.cronosAdmin = message.cronosAdmin)
    message.enableAutoDeployment !== undefined && (obj.enableAutoDeployment = message.enableAutoDeployment)
    return obj
  },

  fromPartial(object: DeepPartial<Params>): Params {
    const message = { ...baseParams } as Params
    if (object.ibcCroDenom !== undefined && object.ibcCroDenom !== null) {
      message.ibcCroDenom = object.ibcCroDenom
    } else {
      message.ibcCroDenom = ''
    }
    if (object.ibcTimeout !== undefined && object.ibcTimeout !== null) {
      message.ibcTimeout = object.ibcTimeout
    } else {
      message.ibcTimeout = 0
    }
    if (object.cronosAdmin !== undefined && object.cronosAdmin !== null) {
      message.cronosAdmin = object.cronosAdmin
    } else {
      message.cronosAdmin = ''
    }
    if (object.enableAutoDeployment !== undefined && object.enableAutoDeployment !== null) {
      message.enableAutoDeployment = object.enableAutoDeployment
    } else {
      message.enableAutoDeployment = false
    }
    return message
  }
}

const baseTokenMappingChangeProposal: object = { title: '', description: '', denom: '', contract: '' }

export const TokenMappingChangeProposal = {
  encode(message: TokenMappingChangeProposal, writer: Writer = Writer.create()): Writer {
    if (message.title !== '') {
      writer.uint32(10).string(message.title)
    }
    if (message.description !== '') {
      writer.uint32(18).string(message.description)
    }
    if (message.denom !== '') {
      writer.uint32(26).string(message.denom)
    }
    if (message.contract !== '') {
      writer.uint32(34).string(message.contract)
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): TokenMappingChangeProposal {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseTokenMappingChangeProposal } as TokenMappingChangeProposal
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.title = reader.string()
          break
        case 2:
          message.description = reader.string()
          break
        case 3:
          message.denom = reader.string()
          break
        case 4:
          message.contract = reader.string()
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): TokenMappingChangeProposal {
    const message = { ...baseTokenMappingChangeProposal } as TokenMappingChangeProposal
    if (object.title !== undefined && object.title !== null) {
      message.title = String(object.title)
    } else {
      message.title = ''
    }
    if (object.description !== undefined && object.description !== null) {
      message.description = String(object.description)
    } else {
      message.description = ''
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

  toJSON(message: TokenMappingChangeProposal): unknown {
    const obj: any = {}
    message.title !== undefined && (obj.title = message.title)
    message.description !== undefined && (obj.description = message.description)
    message.denom !== undefined && (obj.denom = message.denom)
    message.contract !== undefined && (obj.contract = message.contract)
    return obj
  },

  fromPartial(object: DeepPartial<TokenMappingChangeProposal>): TokenMappingChangeProposal {
    const message = { ...baseTokenMappingChangeProposal } as TokenMappingChangeProposal
    if (object.title !== undefined && object.title !== null) {
      message.title = object.title
    } else {
      message.title = ''
    }
    if (object.description !== undefined && object.description !== null) {
      message.description = object.description
    } else {
      message.description = ''
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

const baseTokenMapping: object = { denom: '', contract: '' }

export const TokenMapping = {
  encode(message: TokenMapping, writer: Writer = Writer.create()): Writer {
    if (message.denom !== '') {
      writer.uint32(10).string(message.denom)
    }
    if (message.contract !== '') {
      writer.uint32(18).string(message.contract)
    }
    return writer
  },

  decode(input: Reader | Uint8Array, length?: number): TokenMapping {
    const reader = input instanceof Uint8Array ? new Reader(input) : input
    let end = length === undefined ? reader.len : reader.pos + length
    const message = { ...baseTokenMapping } as TokenMapping
    while (reader.pos < end) {
      const tag = reader.uint32()
      switch (tag >>> 3) {
        case 1:
          message.denom = reader.string()
          break
        case 2:
          message.contract = reader.string()
          break
        default:
          reader.skipType(tag & 7)
          break
      }
    }
    return message
  },

  fromJSON(object: any): TokenMapping {
    const message = { ...baseTokenMapping } as TokenMapping
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

  toJSON(message: TokenMapping): unknown {
    const obj: any = {}
    message.denom !== undefined && (obj.denom = message.denom)
    message.contract !== undefined && (obj.contract = message.contract)
    return obj
  },

  fromPartial(object: DeepPartial<TokenMapping>): TokenMapping {
    const message = { ...baseTokenMapping } as TokenMapping
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

declare var self: any | undefined
declare var window: any | undefined
var globalThis: any = (() => {
  if (typeof globalThis !== 'undefined') return globalThis
  if (typeof self !== 'undefined') return self
  if (typeof window !== 'undefined') return window
  if (typeof global !== 'undefined') return global
  throw 'Unable to locate global object'
})()

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

function longToNumber(long: Long): number {
  if (long.gt(Number.MAX_SAFE_INTEGER)) {
    throw new globalThis.Error('Value is larger than Number.MAX_SAFE_INTEGER')
  }
  return long.toNumber()
}

if (util.Long !== Long) {
  util.Long = Long as any
  configure()
}

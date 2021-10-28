/* eslint-disable */
import { Reader, util, configure, Writer } from 'protobufjs/minimal';
import * as Long from 'long';
import { Any } from '../../../google/protobuf/any';
import { AccessTuple, Log } from '../../../ethermint/evm/v1/evm';
export const protobufPackage = 'ethermint.evm.v1';
const baseMsgEthereumTx = { size: 0, hash: '', from: '' };
export const MsgEthereumTx = {
    encode(message, writer = Writer.create()) {
        if (message.data !== undefined) {
            Any.encode(message.data, writer.uint32(10).fork()).ldelim();
        }
        if (message.size !== 0) {
            writer.uint32(17).double(message.size);
        }
        if (message.hash !== '') {
            writer.uint32(26).string(message.hash);
        }
        if (message.from !== '') {
            writer.uint32(34).string(message.from);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseMsgEthereumTx };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.data = Any.decode(reader, reader.uint32());
                    break;
                case 2:
                    message.size = reader.double();
                    break;
                case 3:
                    message.hash = reader.string();
                    break;
                case 4:
                    message.from = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseMsgEthereumTx };
        if (object.data !== undefined && object.data !== null) {
            message.data = Any.fromJSON(object.data);
        }
        else {
            message.data = undefined;
        }
        if (object.size !== undefined && object.size !== null) {
            message.size = Number(object.size);
        }
        else {
            message.size = 0;
        }
        if (object.hash !== undefined && object.hash !== null) {
            message.hash = String(object.hash);
        }
        else {
            message.hash = '';
        }
        if (object.from !== undefined && object.from !== null) {
            message.from = String(object.from);
        }
        else {
            message.from = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.data !== undefined && (obj.data = message.data ? Any.toJSON(message.data) : undefined);
        message.size !== undefined && (obj.size = message.size);
        message.hash !== undefined && (obj.hash = message.hash);
        message.from !== undefined && (obj.from = message.from);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseMsgEthereumTx };
        if (object.data !== undefined && object.data !== null) {
            message.data = Any.fromPartial(object.data);
        }
        else {
            message.data = undefined;
        }
        if (object.size !== undefined && object.size !== null) {
            message.size = object.size;
        }
        else {
            message.size = 0;
        }
        if (object.hash !== undefined && object.hash !== null) {
            message.hash = object.hash;
        }
        else {
            message.hash = '';
        }
        if (object.from !== undefined && object.from !== null) {
            message.from = object.from;
        }
        else {
            message.from = '';
        }
        return message;
    }
};
const baseLegacyTx = { nonce: 0, gasPrice: '', gas: 0, to: '', value: '' };
export const LegacyTx = {
    encode(message, writer = Writer.create()) {
        if (message.nonce !== 0) {
            writer.uint32(8).uint64(message.nonce);
        }
        if (message.gasPrice !== '') {
            writer.uint32(18).string(message.gasPrice);
        }
        if (message.gas !== 0) {
            writer.uint32(24).uint64(message.gas);
        }
        if (message.to !== '') {
            writer.uint32(34).string(message.to);
        }
        if (message.value !== '') {
            writer.uint32(42).string(message.value);
        }
        if (message.data.length !== 0) {
            writer.uint32(50).bytes(message.data);
        }
        if (message.v.length !== 0) {
            writer.uint32(58).bytes(message.v);
        }
        if (message.r.length !== 0) {
            writer.uint32(66).bytes(message.r);
        }
        if (message.s.length !== 0) {
            writer.uint32(74).bytes(message.s);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseLegacyTx };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.nonce = longToNumber(reader.uint64());
                    break;
                case 2:
                    message.gasPrice = reader.string();
                    break;
                case 3:
                    message.gas = longToNumber(reader.uint64());
                    break;
                case 4:
                    message.to = reader.string();
                    break;
                case 5:
                    message.value = reader.string();
                    break;
                case 6:
                    message.data = reader.bytes();
                    break;
                case 7:
                    message.v = reader.bytes();
                    break;
                case 8:
                    message.r = reader.bytes();
                    break;
                case 9:
                    message.s = reader.bytes();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseLegacyTx };
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = Number(object.nonce);
        }
        else {
            message.nonce = 0;
        }
        if (object.gasPrice !== undefined && object.gasPrice !== null) {
            message.gasPrice = String(object.gasPrice);
        }
        else {
            message.gasPrice = '';
        }
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = Number(object.gas);
        }
        else {
            message.gas = 0;
        }
        if (object.to !== undefined && object.to !== null) {
            message.to = String(object.to);
        }
        else {
            message.to = '';
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = String(object.value);
        }
        else {
            message.value = '';
        }
        if (object.data !== undefined && object.data !== null) {
            message.data = bytesFromBase64(object.data);
        }
        if (object.v !== undefined && object.v !== null) {
            message.v = bytesFromBase64(object.v);
        }
        if (object.r !== undefined && object.r !== null) {
            message.r = bytesFromBase64(object.r);
        }
        if (object.s !== undefined && object.s !== null) {
            message.s = bytesFromBase64(object.s);
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.nonce !== undefined && (obj.nonce = message.nonce);
        message.gasPrice !== undefined && (obj.gasPrice = message.gasPrice);
        message.gas !== undefined && (obj.gas = message.gas);
        message.to !== undefined && (obj.to = message.to);
        message.value !== undefined && (obj.value = message.value);
        message.data !== undefined && (obj.data = base64FromBytes(message.data !== undefined ? message.data : new Uint8Array()));
        message.v !== undefined && (obj.v = base64FromBytes(message.v !== undefined ? message.v : new Uint8Array()));
        message.r !== undefined && (obj.r = base64FromBytes(message.r !== undefined ? message.r : new Uint8Array()));
        message.s !== undefined && (obj.s = base64FromBytes(message.s !== undefined ? message.s : new Uint8Array()));
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseLegacyTx };
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = object.nonce;
        }
        else {
            message.nonce = 0;
        }
        if (object.gasPrice !== undefined && object.gasPrice !== null) {
            message.gasPrice = object.gasPrice;
        }
        else {
            message.gasPrice = '';
        }
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = object.gas;
        }
        else {
            message.gas = 0;
        }
        if (object.to !== undefined && object.to !== null) {
            message.to = object.to;
        }
        else {
            message.to = '';
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = object.value;
        }
        else {
            message.value = '';
        }
        if (object.data !== undefined && object.data !== null) {
            message.data = object.data;
        }
        else {
            message.data = new Uint8Array();
        }
        if (object.v !== undefined && object.v !== null) {
            message.v = object.v;
        }
        else {
            message.v = new Uint8Array();
        }
        if (object.r !== undefined && object.r !== null) {
            message.r = object.r;
        }
        else {
            message.r = new Uint8Array();
        }
        if (object.s !== undefined && object.s !== null) {
            message.s = object.s;
        }
        else {
            message.s = new Uint8Array();
        }
        return message;
    }
};
const baseAccessListTx = { chainId: '', nonce: 0, gasPrice: '', gas: 0, to: '', value: '' };
export const AccessListTx = {
    encode(message, writer = Writer.create()) {
        if (message.chainId !== '') {
            writer.uint32(10).string(message.chainId);
        }
        if (message.nonce !== 0) {
            writer.uint32(16).uint64(message.nonce);
        }
        if (message.gasPrice !== '') {
            writer.uint32(26).string(message.gasPrice);
        }
        if (message.gas !== 0) {
            writer.uint32(32).uint64(message.gas);
        }
        if (message.to !== '') {
            writer.uint32(42).string(message.to);
        }
        if (message.value !== '') {
            writer.uint32(50).string(message.value);
        }
        if (message.data.length !== 0) {
            writer.uint32(58).bytes(message.data);
        }
        for (const v of message.accesses) {
            AccessTuple.encode(v, writer.uint32(66).fork()).ldelim();
        }
        if (message.v.length !== 0) {
            writer.uint32(74).bytes(message.v);
        }
        if (message.r.length !== 0) {
            writer.uint32(82).bytes(message.r);
        }
        if (message.s.length !== 0) {
            writer.uint32(90).bytes(message.s);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseAccessListTx };
        message.accesses = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.chainId = reader.string();
                    break;
                case 2:
                    message.nonce = longToNumber(reader.uint64());
                    break;
                case 3:
                    message.gasPrice = reader.string();
                    break;
                case 4:
                    message.gas = longToNumber(reader.uint64());
                    break;
                case 5:
                    message.to = reader.string();
                    break;
                case 6:
                    message.value = reader.string();
                    break;
                case 7:
                    message.data = reader.bytes();
                    break;
                case 8:
                    message.accesses.push(AccessTuple.decode(reader, reader.uint32()));
                    break;
                case 9:
                    message.v = reader.bytes();
                    break;
                case 10:
                    message.r = reader.bytes();
                    break;
                case 11:
                    message.s = reader.bytes();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseAccessListTx };
        message.accesses = [];
        if (object.chainId !== undefined && object.chainId !== null) {
            message.chainId = String(object.chainId);
        }
        else {
            message.chainId = '';
        }
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = Number(object.nonce);
        }
        else {
            message.nonce = 0;
        }
        if (object.gasPrice !== undefined && object.gasPrice !== null) {
            message.gasPrice = String(object.gasPrice);
        }
        else {
            message.gasPrice = '';
        }
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = Number(object.gas);
        }
        else {
            message.gas = 0;
        }
        if (object.to !== undefined && object.to !== null) {
            message.to = String(object.to);
        }
        else {
            message.to = '';
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = String(object.value);
        }
        else {
            message.value = '';
        }
        if (object.data !== undefined && object.data !== null) {
            message.data = bytesFromBase64(object.data);
        }
        if (object.accesses !== undefined && object.accesses !== null) {
            for (const e of object.accesses) {
                message.accesses.push(AccessTuple.fromJSON(e));
            }
        }
        if (object.v !== undefined && object.v !== null) {
            message.v = bytesFromBase64(object.v);
        }
        if (object.r !== undefined && object.r !== null) {
            message.r = bytesFromBase64(object.r);
        }
        if (object.s !== undefined && object.s !== null) {
            message.s = bytesFromBase64(object.s);
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.chainId !== undefined && (obj.chainId = message.chainId);
        message.nonce !== undefined && (obj.nonce = message.nonce);
        message.gasPrice !== undefined && (obj.gasPrice = message.gasPrice);
        message.gas !== undefined && (obj.gas = message.gas);
        message.to !== undefined && (obj.to = message.to);
        message.value !== undefined && (obj.value = message.value);
        message.data !== undefined && (obj.data = base64FromBytes(message.data !== undefined ? message.data : new Uint8Array()));
        if (message.accesses) {
            obj.accesses = message.accesses.map((e) => (e ? AccessTuple.toJSON(e) : undefined));
        }
        else {
            obj.accesses = [];
        }
        message.v !== undefined && (obj.v = base64FromBytes(message.v !== undefined ? message.v : new Uint8Array()));
        message.r !== undefined && (obj.r = base64FromBytes(message.r !== undefined ? message.r : new Uint8Array()));
        message.s !== undefined && (obj.s = base64FromBytes(message.s !== undefined ? message.s : new Uint8Array()));
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseAccessListTx };
        message.accesses = [];
        if (object.chainId !== undefined && object.chainId !== null) {
            message.chainId = object.chainId;
        }
        else {
            message.chainId = '';
        }
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = object.nonce;
        }
        else {
            message.nonce = 0;
        }
        if (object.gasPrice !== undefined && object.gasPrice !== null) {
            message.gasPrice = object.gasPrice;
        }
        else {
            message.gasPrice = '';
        }
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = object.gas;
        }
        else {
            message.gas = 0;
        }
        if (object.to !== undefined && object.to !== null) {
            message.to = object.to;
        }
        else {
            message.to = '';
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = object.value;
        }
        else {
            message.value = '';
        }
        if (object.data !== undefined && object.data !== null) {
            message.data = object.data;
        }
        else {
            message.data = new Uint8Array();
        }
        if (object.accesses !== undefined && object.accesses !== null) {
            for (const e of object.accesses) {
                message.accesses.push(AccessTuple.fromPartial(e));
            }
        }
        if (object.v !== undefined && object.v !== null) {
            message.v = object.v;
        }
        else {
            message.v = new Uint8Array();
        }
        if (object.r !== undefined && object.r !== null) {
            message.r = object.r;
        }
        else {
            message.r = new Uint8Array();
        }
        if (object.s !== undefined && object.s !== null) {
            message.s = object.s;
        }
        else {
            message.s = new Uint8Array();
        }
        return message;
    }
};
const baseDynamicFeeTx = { chainId: '', nonce: 0, gasTipCap: '', gasFeeCap: '', gas: 0, to: '', value: '' };
export const DynamicFeeTx = {
    encode(message, writer = Writer.create()) {
        if (message.chainId !== '') {
            writer.uint32(10).string(message.chainId);
        }
        if (message.nonce !== 0) {
            writer.uint32(16).uint64(message.nonce);
        }
        if (message.gasTipCap !== '') {
            writer.uint32(26).string(message.gasTipCap);
        }
        if (message.gasFeeCap !== '') {
            writer.uint32(34).string(message.gasFeeCap);
        }
        if (message.gas !== 0) {
            writer.uint32(40).uint64(message.gas);
        }
        if (message.to !== '') {
            writer.uint32(50).string(message.to);
        }
        if (message.value !== '') {
            writer.uint32(58).string(message.value);
        }
        if (message.data.length !== 0) {
            writer.uint32(66).bytes(message.data);
        }
        for (const v of message.accesses) {
            AccessTuple.encode(v, writer.uint32(74).fork()).ldelim();
        }
        if (message.v.length !== 0) {
            writer.uint32(82).bytes(message.v);
        }
        if (message.r.length !== 0) {
            writer.uint32(90).bytes(message.r);
        }
        if (message.s.length !== 0) {
            writer.uint32(98).bytes(message.s);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseDynamicFeeTx };
        message.accesses = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.chainId = reader.string();
                    break;
                case 2:
                    message.nonce = longToNumber(reader.uint64());
                    break;
                case 3:
                    message.gasTipCap = reader.string();
                    break;
                case 4:
                    message.gasFeeCap = reader.string();
                    break;
                case 5:
                    message.gas = longToNumber(reader.uint64());
                    break;
                case 6:
                    message.to = reader.string();
                    break;
                case 7:
                    message.value = reader.string();
                    break;
                case 8:
                    message.data = reader.bytes();
                    break;
                case 9:
                    message.accesses.push(AccessTuple.decode(reader, reader.uint32()));
                    break;
                case 10:
                    message.v = reader.bytes();
                    break;
                case 11:
                    message.r = reader.bytes();
                    break;
                case 12:
                    message.s = reader.bytes();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseDynamicFeeTx };
        message.accesses = [];
        if (object.chainId !== undefined && object.chainId !== null) {
            message.chainId = String(object.chainId);
        }
        else {
            message.chainId = '';
        }
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = Number(object.nonce);
        }
        else {
            message.nonce = 0;
        }
        if (object.gasTipCap !== undefined && object.gasTipCap !== null) {
            message.gasTipCap = String(object.gasTipCap);
        }
        else {
            message.gasTipCap = '';
        }
        if (object.gasFeeCap !== undefined && object.gasFeeCap !== null) {
            message.gasFeeCap = String(object.gasFeeCap);
        }
        else {
            message.gasFeeCap = '';
        }
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = Number(object.gas);
        }
        else {
            message.gas = 0;
        }
        if (object.to !== undefined && object.to !== null) {
            message.to = String(object.to);
        }
        else {
            message.to = '';
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = String(object.value);
        }
        else {
            message.value = '';
        }
        if (object.data !== undefined && object.data !== null) {
            message.data = bytesFromBase64(object.data);
        }
        if (object.accesses !== undefined && object.accesses !== null) {
            for (const e of object.accesses) {
                message.accesses.push(AccessTuple.fromJSON(e));
            }
        }
        if (object.v !== undefined && object.v !== null) {
            message.v = bytesFromBase64(object.v);
        }
        if (object.r !== undefined && object.r !== null) {
            message.r = bytesFromBase64(object.r);
        }
        if (object.s !== undefined && object.s !== null) {
            message.s = bytesFromBase64(object.s);
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.chainId !== undefined && (obj.chainId = message.chainId);
        message.nonce !== undefined && (obj.nonce = message.nonce);
        message.gasTipCap !== undefined && (obj.gasTipCap = message.gasTipCap);
        message.gasFeeCap !== undefined && (obj.gasFeeCap = message.gasFeeCap);
        message.gas !== undefined && (obj.gas = message.gas);
        message.to !== undefined && (obj.to = message.to);
        message.value !== undefined && (obj.value = message.value);
        message.data !== undefined && (obj.data = base64FromBytes(message.data !== undefined ? message.data : new Uint8Array()));
        if (message.accesses) {
            obj.accesses = message.accesses.map((e) => (e ? AccessTuple.toJSON(e) : undefined));
        }
        else {
            obj.accesses = [];
        }
        message.v !== undefined && (obj.v = base64FromBytes(message.v !== undefined ? message.v : new Uint8Array()));
        message.r !== undefined && (obj.r = base64FromBytes(message.r !== undefined ? message.r : new Uint8Array()));
        message.s !== undefined && (obj.s = base64FromBytes(message.s !== undefined ? message.s : new Uint8Array()));
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseDynamicFeeTx };
        message.accesses = [];
        if (object.chainId !== undefined && object.chainId !== null) {
            message.chainId = object.chainId;
        }
        else {
            message.chainId = '';
        }
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = object.nonce;
        }
        else {
            message.nonce = 0;
        }
        if (object.gasTipCap !== undefined && object.gasTipCap !== null) {
            message.gasTipCap = object.gasTipCap;
        }
        else {
            message.gasTipCap = '';
        }
        if (object.gasFeeCap !== undefined && object.gasFeeCap !== null) {
            message.gasFeeCap = object.gasFeeCap;
        }
        else {
            message.gasFeeCap = '';
        }
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = object.gas;
        }
        else {
            message.gas = 0;
        }
        if (object.to !== undefined && object.to !== null) {
            message.to = object.to;
        }
        else {
            message.to = '';
        }
        if (object.value !== undefined && object.value !== null) {
            message.value = object.value;
        }
        else {
            message.value = '';
        }
        if (object.data !== undefined && object.data !== null) {
            message.data = object.data;
        }
        else {
            message.data = new Uint8Array();
        }
        if (object.accesses !== undefined && object.accesses !== null) {
            for (const e of object.accesses) {
                message.accesses.push(AccessTuple.fromPartial(e));
            }
        }
        if (object.v !== undefined && object.v !== null) {
            message.v = object.v;
        }
        else {
            message.v = new Uint8Array();
        }
        if (object.r !== undefined && object.r !== null) {
            message.r = object.r;
        }
        else {
            message.r = new Uint8Array();
        }
        if (object.s !== undefined && object.s !== null) {
            message.s = object.s;
        }
        else {
            message.s = new Uint8Array();
        }
        return message;
    }
};
const baseExtensionOptionsEthereumTx = {};
export const ExtensionOptionsEthereumTx = {
    encode(_, writer = Writer.create()) {
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseExtensionOptionsEthereumTx };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(_) {
        const message = { ...baseExtensionOptionsEthereumTx };
        return message;
    },
    toJSON(_) {
        const obj = {};
        return obj;
    },
    fromPartial(_) {
        const message = { ...baseExtensionOptionsEthereumTx };
        return message;
    }
};
const baseMsgEthereumTxResponse = { hash: '', vmError: '', gasUsed: 0 };
export const MsgEthereumTxResponse = {
    encode(message, writer = Writer.create()) {
        if (message.hash !== '') {
            writer.uint32(10).string(message.hash);
        }
        for (const v of message.logs) {
            Log.encode(v, writer.uint32(18).fork()).ldelim();
        }
        if (message.ret.length !== 0) {
            writer.uint32(26).bytes(message.ret);
        }
        if (message.vmError !== '') {
            writer.uint32(34).string(message.vmError);
        }
        if (message.gasUsed !== 0) {
            writer.uint32(40).uint64(message.gasUsed);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseMsgEthereumTxResponse };
        message.logs = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.hash = reader.string();
                    break;
                case 2:
                    message.logs.push(Log.decode(reader, reader.uint32()));
                    break;
                case 3:
                    message.ret = reader.bytes();
                    break;
                case 4:
                    message.vmError = reader.string();
                    break;
                case 5:
                    message.gasUsed = longToNumber(reader.uint64());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseMsgEthereumTxResponse };
        message.logs = [];
        if (object.hash !== undefined && object.hash !== null) {
            message.hash = String(object.hash);
        }
        else {
            message.hash = '';
        }
        if (object.logs !== undefined && object.logs !== null) {
            for (const e of object.logs) {
                message.logs.push(Log.fromJSON(e));
            }
        }
        if (object.ret !== undefined && object.ret !== null) {
            message.ret = bytesFromBase64(object.ret);
        }
        if (object.vmError !== undefined && object.vmError !== null) {
            message.vmError = String(object.vmError);
        }
        else {
            message.vmError = '';
        }
        if (object.gasUsed !== undefined && object.gasUsed !== null) {
            message.gasUsed = Number(object.gasUsed);
        }
        else {
            message.gasUsed = 0;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.hash !== undefined && (obj.hash = message.hash);
        if (message.logs) {
            obj.logs = message.logs.map((e) => (e ? Log.toJSON(e) : undefined));
        }
        else {
            obj.logs = [];
        }
        message.ret !== undefined && (obj.ret = base64FromBytes(message.ret !== undefined ? message.ret : new Uint8Array()));
        message.vmError !== undefined && (obj.vmError = message.vmError);
        message.gasUsed !== undefined && (obj.gasUsed = message.gasUsed);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseMsgEthereumTxResponse };
        message.logs = [];
        if (object.hash !== undefined && object.hash !== null) {
            message.hash = object.hash;
        }
        else {
            message.hash = '';
        }
        if (object.logs !== undefined && object.logs !== null) {
            for (const e of object.logs) {
                message.logs.push(Log.fromPartial(e));
            }
        }
        if (object.ret !== undefined && object.ret !== null) {
            message.ret = object.ret;
        }
        else {
            message.ret = new Uint8Array();
        }
        if (object.vmError !== undefined && object.vmError !== null) {
            message.vmError = object.vmError;
        }
        else {
            message.vmError = '';
        }
        if (object.gasUsed !== undefined && object.gasUsed !== null) {
            message.gasUsed = object.gasUsed;
        }
        else {
            message.gasUsed = 0;
        }
        return message;
    }
};
export class MsgClientImpl {
    constructor(rpc) {
        this.rpc = rpc;
    }
    EthereumTx(request) {
        const data = MsgEthereumTx.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Msg', 'EthereumTx', data);
        return promise.then((data) => MsgEthereumTxResponse.decode(new Reader(data)));
    }
}
var globalThis = (() => {
    if (typeof globalThis !== 'undefined')
        return globalThis;
    if (typeof self !== 'undefined')
        return self;
    if (typeof window !== 'undefined')
        return window;
    if (typeof global !== 'undefined')
        return global;
    throw 'Unable to locate global object';
})();
const atob = globalThis.atob || ((b64) => globalThis.Buffer.from(b64, 'base64').toString('binary'));
function bytesFromBase64(b64) {
    const bin = atob(b64);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; ++i) {
        arr[i] = bin.charCodeAt(i);
    }
    return arr;
}
const btoa = globalThis.btoa || ((bin) => globalThis.Buffer.from(bin, 'binary').toString('base64'));
function base64FromBytes(arr) {
    const bin = [];
    for (let i = 0; i < arr.byteLength; ++i) {
        bin.push(String.fromCharCode(arr[i]));
    }
    return btoa(bin.join(''));
}
function longToNumber(long) {
    if (long.gt(Number.MAX_SAFE_INTEGER)) {
        throw new globalThis.Error('Value is larger than Number.MAX_SAFE_INTEGER');
    }
    return long.toNumber();
}
if (util.Long !== Long) {
    util.Long = Long;
    configure();
}

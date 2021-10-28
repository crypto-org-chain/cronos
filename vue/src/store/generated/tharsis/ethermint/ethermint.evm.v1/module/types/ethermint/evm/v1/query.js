/* eslint-disable */
import { Reader, util, configure, Writer } from 'protobufjs/minimal';
import * as Long from 'long';
import { PageRequest, PageResponse } from '../../../cosmos/base/query/v1beta1/pagination';
import { Log, Params, TraceConfig } from '../../../ethermint/evm/v1/evm';
import { MsgEthereumTx, MsgEthereumTxResponse } from '../../../ethermint/evm/v1/tx';
export const protobufPackage = 'ethermint.evm.v1';
const baseQueryAccountRequest = { address: '' };
export const QueryAccountRequest = {
    encode(message, writer = Writer.create()) {
        if (message.address !== '') {
            writer.uint32(10).string(message.address);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryAccountRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.address = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryAccountRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = String(object.address);
        }
        else {
            message.address = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.address !== undefined && (obj.address = message.address);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryAccountRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = object.address;
        }
        else {
            message.address = '';
        }
        return message;
    }
};
const baseQueryAccountResponse = { balance: '', codeHash: '', nonce: 0 };
export const QueryAccountResponse = {
    encode(message, writer = Writer.create()) {
        if (message.balance !== '') {
            writer.uint32(10).string(message.balance);
        }
        if (message.codeHash !== '') {
            writer.uint32(18).string(message.codeHash);
        }
        if (message.nonce !== 0) {
            writer.uint32(24).uint64(message.nonce);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryAccountResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.balance = reader.string();
                    break;
                case 2:
                    message.codeHash = reader.string();
                    break;
                case 3:
                    message.nonce = longToNumber(reader.uint64());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryAccountResponse };
        if (object.balance !== undefined && object.balance !== null) {
            message.balance = String(object.balance);
        }
        else {
            message.balance = '';
        }
        if (object.codeHash !== undefined && object.codeHash !== null) {
            message.codeHash = String(object.codeHash);
        }
        else {
            message.codeHash = '';
        }
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = Number(object.nonce);
        }
        else {
            message.nonce = 0;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.balance !== undefined && (obj.balance = message.balance);
        message.codeHash !== undefined && (obj.codeHash = message.codeHash);
        message.nonce !== undefined && (obj.nonce = message.nonce);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryAccountResponse };
        if (object.balance !== undefined && object.balance !== null) {
            message.balance = object.balance;
        }
        else {
            message.balance = '';
        }
        if (object.codeHash !== undefined && object.codeHash !== null) {
            message.codeHash = object.codeHash;
        }
        else {
            message.codeHash = '';
        }
        if (object.nonce !== undefined && object.nonce !== null) {
            message.nonce = object.nonce;
        }
        else {
            message.nonce = 0;
        }
        return message;
    }
};
const baseQueryCosmosAccountRequest = { address: '' };
export const QueryCosmosAccountRequest = {
    encode(message, writer = Writer.create()) {
        if (message.address !== '') {
            writer.uint32(10).string(message.address);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryCosmosAccountRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.address = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryCosmosAccountRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = String(object.address);
        }
        else {
            message.address = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.address !== undefined && (obj.address = message.address);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryCosmosAccountRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = object.address;
        }
        else {
            message.address = '';
        }
        return message;
    }
};
const baseQueryCosmosAccountResponse = { cosmosAddress: '', sequence: 0, accountNumber: 0 };
export const QueryCosmosAccountResponse = {
    encode(message, writer = Writer.create()) {
        if (message.cosmosAddress !== '') {
            writer.uint32(10).string(message.cosmosAddress);
        }
        if (message.sequence !== 0) {
            writer.uint32(16).uint64(message.sequence);
        }
        if (message.accountNumber !== 0) {
            writer.uint32(24).uint64(message.accountNumber);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryCosmosAccountResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.cosmosAddress = reader.string();
                    break;
                case 2:
                    message.sequence = longToNumber(reader.uint64());
                    break;
                case 3:
                    message.accountNumber = longToNumber(reader.uint64());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryCosmosAccountResponse };
        if (object.cosmosAddress !== undefined && object.cosmosAddress !== null) {
            message.cosmosAddress = String(object.cosmosAddress);
        }
        else {
            message.cosmosAddress = '';
        }
        if (object.sequence !== undefined && object.sequence !== null) {
            message.sequence = Number(object.sequence);
        }
        else {
            message.sequence = 0;
        }
        if (object.accountNumber !== undefined && object.accountNumber !== null) {
            message.accountNumber = Number(object.accountNumber);
        }
        else {
            message.accountNumber = 0;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.cosmosAddress !== undefined && (obj.cosmosAddress = message.cosmosAddress);
        message.sequence !== undefined && (obj.sequence = message.sequence);
        message.accountNumber !== undefined && (obj.accountNumber = message.accountNumber);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryCosmosAccountResponse };
        if (object.cosmosAddress !== undefined && object.cosmosAddress !== null) {
            message.cosmosAddress = object.cosmosAddress;
        }
        else {
            message.cosmosAddress = '';
        }
        if (object.sequence !== undefined && object.sequence !== null) {
            message.sequence = object.sequence;
        }
        else {
            message.sequence = 0;
        }
        if (object.accountNumber !== undefined && object.accountNumber !== null) {
            message.accountNumber = object.accountNumber;
        }
        else {
            message.accountNumber = 0;
        }
        return message;
    }
};
const baseQueryValidatorAccountRequest = { consAddress: '' };
export const QueryValidatorAccountRequest = {
    encode(message, writer = Writer.create()) {
        if (message.consAddress !== '') {
            writer.uint32(10).string(message.consAddress);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryValidatorAccountRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.consAddress = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryValidatorAccountRequest };
        if (object.consAddress !== undefined && object.consAddress !== null) {
            message.consAddress = String(object.consAddress);
        }
        else {
            message.consAddress = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.consAddress !== undefined && (obj.consAddress = message.consAddress);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryValidatorAccountRequest };
        if (object.consAddress !== undefined && object.consAddress !== null) {
            message.consAddress = object.consAddress;
        }
        else {
            message.consAddress = '';
        }
        return message;
    }
};
const baseQueryValidatorAccountResponse = { accountAddress: '', sequence: 0, accountNumber: 0 };
export const QueryValidatorAccountResponse = {
    encode(message, writer = Writer.create()) {
        if (message.accountAddress !== '') {
            writer.uint32(10).string(message.accountAddress);
        }
        if (message.sequence !== 0) {
            writer.uint32(16).uint64(message.sequence);
        }
        if (message.accountNumber !== 0) {
            writer.uint32(24).uint64(message.accountNumber);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryValidatorAccountResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.accountAddress = reader.string();
                    break;
                case 2:
                    message.sequence = longToNumber(reader.uint64());
                    break;
                case 3:
                    message.accountNumber = longToNumber(reader.uint64());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryValidatorAccountResponse };
        if (object.accountAddress !== undefined && object.accountAddress !== null) {
            message.accountAddress = String(object.accountAddress);
        }
        else {
            message.accountAddress = '';
        }
        if (object.sequence !== undefined && object.sequence !== null) {
            message.sequence = Number(object.sequence);
        }
        else {
            message.sequence = 0;
        }
        if (object.accountNumber !== undefined && object.accountNumber !== null) {
            message.accountNumber = Number(object.accountNumber);
        }
        else {
            message.accountNumber = 0;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.accountAddress !== undefined && (obj.accountAddress = message.accountAddress);
        message.sequence !== undefined && (obj.sequence = message.sequence);
        message.accountNumber !== undefined && (obj.accountNumber = message.accountNumber);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryValidatorAccountResponse };
        if (object.accountAddress !== undefined && object.accountAddress !== null) {
            message.accountAddress = object.accountAddress;
        }
        else {
            message.accountAddress = '';
        }
        if (object.sequence !== undefined && object.sequence !== null) {
            message.sequence = object.sequence;
        }
        else {
            message.sequence = 0;
        }
        if (object.accountNumber !== undefined && object.accountNumber !== null) {
            message.accountNumber = object.accountNumber;
        }
        else {
            message.accountNumber = 0;
        }
        return message;
    }
};
const baseQueryBalanceRequest = { address: '' };
export const QueryBalanceRequest = {
    encode(message, writer = Writer.create()) {
        if (message.address !== '') {
            writer.uint32(10).string(message.address);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryBalanceRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.address = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryBalanceRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = String(object.address);
        }
        else {
            message.address = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.address !== undefined && (obj.address = message.address);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryBalanceRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = object.address;
        }
        else {
            message.address = '';
        }
        return message;
    }
};
const baseQueryBalanceResponse = { balance: '' };
export const QueryBalanceResponse = {
    encode(message, writer = Writer.create()) {
        if (message.balance !== '') {
            writer.uint32(10).string(message.balance);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryBalanceResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.balance = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryBalanceResponse };
        if (object.balance !== undefined && object.balance !== null) {
            message.balance = String(object.balance);
        }
        else {
            message.balance = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.balance !== undefined && (obj.balance = message.balance);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryBalanceResponse };
        if (object.balance !== undefined && object.balance !== null) {
            message.balance = object.balance;
        }
        else {
            message.balance = '';
        }
        return message;
    }
};
const baseQueryStorageRequest = { address: '', key: '' };
export const QueryStorageRequest = {
    encode(message, writer = Writer.create()) {
        if (message.address !== '') {
            writer.uint32(10).string(message.address);
        }
        if (message.key !== '') {
            writer.uint32(18).string(message.key);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryStorageRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.address = reader.string();
                    break;
                case 2:
                    message.key = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryStorageRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = String(object.address);
        }
        else {
            message.address = '';
        }
        if (object.key !== undefined && object.key !== null) {
            message.key = String(object.key);
        }
        else {
            message.key = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.address !== undefined && (obj.address = message.address);
        message.key !== undefined && (obj.key = message.key);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryStorageRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = object.address;
        }
        else {
            message.address = '';
        }
        if (object.key !== undefined && object.key !== null) {
            message.key = object.key;
        }
        else {
            message.key = '';
        }
        return message;
    }
};
const baseQueryStorageResponse = { value: '' };
export const QueryStorageResponse = {
    encode(message, writer = Writer.create()) {
        if (message.value !== '') {
            writer.uint32(10).string(message.value);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryStorageResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.value = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryStorageResponse };
        if (object.value !== undefined && object.value !== null) {
            message.value = String(object.value);
        }
        else {
            message.value = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.value !== undefined && (obj.value = message.value);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryStorageResponse };
        if (object.value !== undefined && object.value !== null) {
            message.value = object.value;
        }
        else {
            message.value = '';
        }
        return message;
    }
};
const baseQueryCodeRequest = { address: '' };
export const QueryCodeRequest = {
    encode(message, writer = Writer.create()) {
        if (message.address !== '') {
            writer.uint32(10).string(message.address);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryCodeRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.address = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryCodeRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = String(object.address);
        }
        else {
            message.address = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.address !== undefined && (obj.address = message.address);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryCodeRequest };
        if (object.address !== undefined && object.address !== null) {
            message.address = object.address;
        }
        else {
            message.address = '';
        }
        return message;
    }
};
const baseQueryCodeResponse = {};
export const QueryCodeResponse = {
    encode(message, writer = Writer.create()) {
        if (message.code.length !== 0) {
            writer.uint32(10).bytes(message.code);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryCodeResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.code = reader.bytes();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryCodeResponse };
        if (object.code !== undefined && object.code !== null) {
            message.code = bytesFromBase64(object.code);
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.code !== undefined && (obj.code = base64FromBytes(message.code !== undefined ? message.code : new Uint8Array()));
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryCodeResponse };
        if (object.code !== undefined && object.code !== null) {
            message.code = object.code;
        }
        else {
            message.code = new Uint8Array();
        }
        return message;
    }
};
const baseQueryTxLogsRequest = { hash: '' };
export const QueryTxLogsRequest = {
    encode(message, writer = Writer.create()) {
        if (message.hash !== '') {
            writer.uint32(10).string(message.hash);
        }
        if (message.pagination !== undefined) {
            PageRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryTxLogsRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.hash = reader.string();
                    break;
                case 2:
                    message.pagination = PageRequest.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryTxLogsRequest };
        if (object.hash !== undefined && object.hash !== null) {
            message.hash = String(object.hash);
        }
        else {
            message.hash = '';
        }
        if (object.pagination !== undefined && object.pagination !== null) {
            message.pagination = PageRequest.fromJSON(object.pagination);
        }
        else {
            message.pagination = undefined;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.hash !== undefined && (obj.hash = message.hash);
        message.pagination !== undefined && (obj.pagination = message.pagination ? PageRequest.toJSON(message.pagination) : undefined);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryTxLogsRequest };
        if (object.hash !== undefined && object.hash !== null) {
            message.hash = object.hash;
        }
        else {
            message.hash = '';
        }
        if (object.pagination !== undefined && object.pagination !== null) {
            message.pagination = PageRequest.fromPartial(object.pagination);
        }
        else {
            message.pagination = undefined;
        }
        return message;
    }
};
const baseQueryTxLogsResponse = {};
export const QueryTxLogsResponse = {
    encode(message, writer = Writer.create()) {
        for (const v of message.logs) {
            Log.encode(v, writer.uint32(10).fork()).ldelim();
        }
        if (message.pagination !== undefined) {
            PageResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryTxLogsResponse };
        message.logs = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.logs.push(Log.decode(reader, reader.uint32()));
                    break;
                case 2:
                    message.pagination = PageResponse.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryTxLogsResponse };
        message.logs = [];
        if (object.logs !== undefined && object.logs !== null) {
            for (const e of object.logs) {
                message.logs.push(Log.fromJSON(e));
            }
        }
        if (object.pagination !== undefined && object.pagination !== null) {
            message.pagination = PageResponse.fromJSON(object.pagination);
        }
        else {
            message.pagination = undefined;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        if (message.logs) {
            obj.logs = message.logs.map((e) => (e ? Log.toJSON(e) : undefined));
        }
        else {
            obj.logs = [];
        }
        message.pagination !== undefined && (obj.pagination = message.pagination ? PageResponse.toJSON(message.pagination) : undefined);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryTxLogsResponse };
        message.logs = [];
        if (object.logs !== undefined && object.logs !== null) {
            for (const e of object.logs) {
                message.logs.push(Log.fromPartial(e));
            }
        }
        if (object.pagination !== undefined && object.pagination !== null) {
            message.pagination = PageResponse.fromPartial(object.pagination);
        }
        else {
            message.pagination = undefined;
        }
        return message;
    }
};
const baseQueryParamsRequest = {};
export const QueryParamsRequest = {
    encode(_, writer = Writer.create()) {
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryParamsRequest };
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
        const message = { ...baseQueryParamsRequest };
        return message;
    },
    toJSON(_) {
        const obj = {};
        return obj;
    },
    fromPartial(_) {
        const message = { ...baseQueryParamsRequest };
        return message;
    }
};
const baseQueryParamsResponse = {};
export const QueryParamsResponse = {
    encode(message, writer = Writer.create()) {
        if (message.params !== undefined) {
            Params.encode(message.params, writer.uint32(10).fork()).ldelim();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryParamsResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.params = Params.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryParamsResponse };
        if (object.params !== undefined && object.params !== null) {
            message.params = Params.fromJSON(object.params);
        }
        else {
            message.params = undefined;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.params !== undefined && (obj.params = message.params ? Params.toJSON(message.params) : undefined);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryParamsResponse };
        if (object.params !== undefined && object.params !== null) {
            message.params = Params.fromPartial(object.params);
        }
        else {
            message.params = undefined;
        }
        return message;
    }
};
const baseQueryStaticCallResponse = {};
export const QueryStaticCallResponse = {
    encode(message, writer = Writer.create()) {
        if (message.data.length !== 0) {
            writer.uint32(10).bytes(message.data);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryStaticCallResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.data = reader.bytes();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryStaticCallResponse };
        if (object.data !== undefined && object.data !== null) {
            message.data = bytesFromBase64(object.data);
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.data !== undefined && (obj.data = base64FromBytes(message.data !== undefined ? message.data : new Uint8Array()));
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryStaticCallResponse };
        if (object.data !== undefined && object.data !== null) {
            message.data = object.data;
        }
        else {
            message.data = new Uint8Array();
        }
        return message;
    }
};
const baseEthCallRequest = { gasCap: 0 };
export const EthCallRequest = {
    encode(message, writer = Writer.create()) {
        if (message.args.length !== 0) {
            writer.uint32(10).bytes(message.args);
        }
        if (message.gasCap !== 0) {
            writer.uint32(16).uint64(message.gasCap);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEthCallRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.args = reader.bytes();
                    break;
                case 2:
                    message.gasCap = longToNumber(reader.uint64());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseEthCallRequest };
        if (object.args !== undefined && object.args !== null) {
            message.args = bytesFromBase64(object.args);
        }
        if (object.gasCap !== undefined && object.gasCap !== null) {
            message.gasCap = Number(object.gasCap);
        }
        else {
            message.gasCap = 0;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.args !== undefined && (obj.args = base64FromBytes(message.args !== undefined ? message.args : new Uint8Array()));
        message.gasCap !== undefined && (obj.gasCap = message.gasCap);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseEthCallRequest };
        if (object.args !== undefined && object.args !== null) {
            message.args = object.args;
        }
        else {
            message.args = new Uint8Array();
        }
        if (object.gasCap !== undefined && object.gasCap !== null) {
            message.gasCap = object.gasCap;
        }
        else {
            message.gasCap = 0;
        }
        return message;
    }
};
const baseEstimateGasResponse = { gas: 0 };
export const EstimateGasResponse = {
    encode(message, writer = Writer.create()) {
        if (message.gas !== 0) {
            writer.uint32(8).uint64(message.gas);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseEstimateGasResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.gas = longToNumber(reader.uint64());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseEstimateGasResponse };
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = Number(object.gas);
        }
        else {
            message.gas = 0;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.gas !== undefined && (obj.gas = message.gas);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseEstimateGasResponse };
        if (object.gas !== undefined && object.gas !== null) {
            message.gas = object.gas;
        }
        else {
            message.gas = 0;
        }
        return message;
    }
};
const baseQueryTraceTxRequest = { txIndex: 0 };
export const QueryTraceTxRequest = {
    encode(message, writer = Writer.create()) {
        if (message.msg !== undefined) {
            MsgEthereumTx.encode(message.msg, writer.uint32(10).fork()).ldelim();
        }
        if (message.txIndex !== 0) {
            writer.uint32(16).uint64(message.txIndex);
        }
        if (message.traceConfig !== undefined) {
            TraceConfig.encode(message.traceConfig, writer.uint32(26).fork()).ldelim();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryTraceTxRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.msg = MsgEthereumTx.decode(reader, reader.uint32());
                    break;
                case 2:
                    message.txIndex = longToNumber(reader.uint64());
                    break;
                case 3:
                    message.traceConfig = TraceConfig.decode(reader, reader.uint32());
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryTraceTxRequest };
        if (object.msg !== undefined && object.msg !== null) {
            message.msg = MsgEthereumTx.fromJSON(object.msg);
        }
        else {
            message.msg = undefined;
        }
        if (object.txIndex !== undefined && object.txIndex !== null) {
            message.txIndex = Number(object.txIndex);
        }
        else {
            message.txIndex = 0;
        }
        if (object.traceConfig !== undefined && object.traceConfig !== null) {
            message.traceConfig = TraceConfig.fromJSON(object.traceConfig);
        }
        else {
            message.traceConfig = undefined;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.msg !== undefined && (obj.msg = message.msg ? MsgEthereumTx.toJSON(message.msg) : undefined);
        message.txIndex !== undefined && (obj.txIndex = message.txIndex);
        message.traceConfig !== undefined && (obj.traceConfig = message.traceConfig ? TraceConfig.toJSON(message.traceConfig) : undefined);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryTraceTxRequest };
        if (object.msg !== undefined && object.msg !== null) {
            message.msg = MsgEthereumTx.fromPartial(object.msg);
        }
        else {
            message.msg = undefined;
        }
        if (object.txIndex !== undefined && object.txIndex !== null) {
            message.txIndex = object.txIndex;
        }
        else {
            message.txIndex = 0;
        }
        if (object.traceConfig !== undefined && object.traceConfig !== null) {
            message.traceConfig = TraceConfig.fromPartial(object.traceConfig);
        }
        else {
            message.traceConfig = undefined;
        }
        return message;
    }
};
const baseQueryTraceTxResponse = {};
export const QueryTraceTxResponse = {
    encode(message, writer = Writer.create()) {
        if (message.data.length !== 0) {
            writer.uint32(10).bytes(message.data);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseQueryTraceTxResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.data = reader.bytes();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseQueryTraceTxResponse };
        if (object.data !== undefined && object.data !== null) {
            message.data = bytesFromBase64(object.data);
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.data !== undefined && (obj.data = base64FromBytes(message.data !== undefined ? message.data : new Uint8Array()));
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseQueryTraceTxResponse };
        if (object.data !== undefined && object.data !== null) {
            message.data = object.data;
        }
        else {
            message.data = new Uint8Array();
        }
        return message;
    }
};
export class QueryClientImpl {
    constructor(rpc) {
        this.rpc = rpc;
    }
    Account(request) {
        const data = QueryAccountRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'Account', data);
        return promise.then((data) => QueryAccountResponse.decode(new Reader(data)));
    }
    CosmosAccount(request) {
        const data = QueryCosmosAccountRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'CosmosAccount', data);
        return promise.then((data) => QueryCosmosAccountResponse.decode(new Reader(data)));
    }
    ValidatorAccount(request) {
        const data = QueryValidatorAccountRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'ValidatorAccount', data);
        return promise.then((data) => QueryValidatorAccountResponse.decode(new Reader(data)));
    }
    Balance(request) {
        const data = QueryBalanceRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'Balance', data);
        return promise.then((data) => QueryBalanceResponse.decode(new Reader(data)));
    }
    Storage(request) {
        const data = QueryStorageRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'Storage', data);
        return promise.then((data) => QueryStorageResponse.decode(new Reader(data)));
    }
    Code(request) {
        const data = QueryCodeRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'Code', data);
        return promise.then((data) => QueryCodeResponse.decode(new Reader(data)));
    }
    Params(request) {
        const data = QueryParamsRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'Params', data);
        return promise.then((data) => QueryParamsResponse.decode(new Reader(data)));
    }
    EthCall(request) {
        const data = EthCallRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'EthCall', data);
        return promise.then((data) => MsgEthereumTxResponse.decode(new Reader(data)));
    }
    EstimateGas(request) {
        const data = EthCallRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'EstimateGas', data);
        return promise.then((data) => EstimateGasResponse.decode(new Reader(data)));
    }
    TraceTx(request) {
        const data = QueryTraceTxRequest.encode(request).finish();
        const promise = this.rpc.request('ethermint.evm.v1.Query', 'TraceTx', data);
        return promise.then((data) => QueryTraceTxResponse.decode(new Reader(data)));
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

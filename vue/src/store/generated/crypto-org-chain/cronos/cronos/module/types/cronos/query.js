/* eslint-disable */
import { Reader, Writer } from 'protobufjs/minimal';
export const protobufPackage = 'cronos';
const baseContractByDenomRequest = { denom: '' };
export const ContractByDenomRequest = {
    encode(message, writer = Writer.create()) {
        if (message.denom !== '') {
            writer.uint32(10).string(message.denom);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseContractByDenomRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.denom = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseContractByDenomRequest };
        if (object.denom !== undefined && object.denom !== null) {
            message.denom = String(object.denom);
        }
        else {
            message.denom = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.denom !== undefined && (obj.denom = message.denom);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseContractByDenomRequest };
        if (object.denom !== undefined && object.denom !== null) {
            message.denom = object.denom;
        }
        else {
            message.denom = '';
        }
        return message;
    }
};
const baseContractByDenomResponse = { contract: '', autoContract: '' };
export const ContractByDenomResponse = {
    encode(message, writer = Writer.create()) {
        if (message.contract !== '') {
            writer.uint32(10).string(message.contract);
        }
        if (message.autoContract !== '') {
            writer.uint32(18).string(message.autoContract);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseContractByDenomResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.contract = reader.string();
                    break;
                case 2:
                    message.autoContract = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseContractByDenomResponse };
        if (object.contract !== undefined && object.contract !== null) {
            message.contract = String(object.contract);
        }
        else {
            message.contract = '';
        }
        if (object.autoContract !== undefined && object.autoContract !== null) {
            message.autoContract = String(object.autoContract);
        }
        else {
            message.autoContract = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.contract !== undefined && (obj.contract = message.contract);
        message.autoContract !== undefined && (obj.autoContract = message.autoContract);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseContractByDenomResponse };
        if (object.contract !== undefined && object.contract !== null) {
            message.contract = object.contract;
        }
        else {
            message.contract = '';
        }
        if (object.autoContract !== undefined && object.autoContract !== null) {
            message.autoContract = object.autoContract;
        }
        else {
            message.autoContract = '';
        }
        return message;
    }
};
const baseDenomByContractRequest = { contract: '' };
export const DenomByContractRequest = {
    encode(message, writer = Writer.create()) {
        if (message.contract !== '') {
            writer.uint32(10).string(message.contract);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseDenomByContractRequest };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.contract = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseDenomByContractRequest };
        if (object.contract !== undefined && object.contract !== null) {
            message.contract = String(object.contract);
        }
        else {
            message.contract = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.contract !== undefined && (obj.contract = message.contract);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseDenomByContractRequest };
        if (object.contract !== undefined && object.contract !== null) {
            message.contract = object.contract;
        }
        else {
            message.contract = '';
        }
        return message;
    }
};
const baseDenomByContractResponse = { denom: '' };
export const DenomByContractResponse = {
    encode(message, writer = Writer.create()) {
        if (message.denom !== '') {
            writer.uint32(10).string(message.denom);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseDenomByContractResponse };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.denom = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseDenomByContractResponse };
        if (object.denom !== undefined && object.denom !== null) {
            message.denom = String(object.denom);
        }
        else {
            message.denom = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.denom !== undefined && (obj.denom = message.denom);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseDenomByContractResponse };
        if (object.denom !== undefined && object.denom !== null) {
            message.denom = object.denom;
        }
        else {
            message.denom = '';
        }
        return message;
    }
};
export class QueryClientImpl {
    constructor(rpc) {
        this.rpc = rpc;
    }
    ContractByDenom(request) {
        const data = ContractByDenomRequest.encode(request).finish();
        const promise = this.rpc.request('cronos.Query', 'ContractByDenom', data);
        return promise.then((data) => ContractByDenomResponse.decode(new Reader(data)));
    }
    DenomByContract(request) {
        const data = DenomByContractRequest.encode(request).finish();
        const promise = this.rpc.request('cronos.Query', 'DenomByContract', data);
        return promise.then((data) => DenomByContractResponse.decode(new Reader(data)));
    }
}

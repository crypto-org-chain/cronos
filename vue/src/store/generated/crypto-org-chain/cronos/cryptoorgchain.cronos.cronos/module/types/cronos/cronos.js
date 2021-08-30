/* eslint-disable */
import { Writer, Reader } from 'protobufjs/minimal';
export const protobufPackage = 'cryptoorgchain.cronos.cronos';
const baseParams = { ibcCroDenom: '', ibcCroChannelID: '' };
export const Params = {
    encode(message, writer = Writer.create()) {
        for (const v of message.convertEnabled) {
            ConvertEnabled.encode(v, writer.uint32(10).fork()).ldelim();
        }
        if (message.ibcCroDenom !== '') {
            writer.uint32(18).string(message.ibcCroDenom);
        }
        if (message.ibcCroChannelID !== '') {
            writer.uint32(26).string(message.ibcCroChannelID);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseParams };
        message.convertEnabled = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.convertEnabled.push(ConvertEnabled.decode(reader, reader.uint32()));
                    break;
                case 2:
                    message.ibcCroDenom = reader.string();
                    break;
                case 3:
                    message.ibcCroChannelID = reader.string();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseParams };
        message.convertEnabled = [];
        if (object.convertEnabled !== undefined && object.convertEnabled !== null) {
            for (const e of object.convertEnabled) {
                message.convertEnabled.push(ConvertEnabled.fromJSON(e));
            }
        }
        if (object.ibcCroDenom !== undefined && object.ibcCroDenom !== null) {
            message.ibcCroDenom = String(object.ibcCroDenom);
        }
        else {
            message.ibcCroDenom = '';
        }
        if (object.ibcCroChannelID !== undefined && object.ibcCroChannelID !== null) {
            message.ibcCroChannelID = String(object.ibcCroChannelID);
        }
        else {
            message.ibcCroChannelID = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        if (message.convertEnabled) {
            obj.convertEnabled = message.convertEnabled.map((e) => (e ? ConvertEnabled.toJSON(e) : undefined));
        }
        else {
            obj.convertEnabled = [];
        }
        message.ibcCroDenom !== undefined && (obj.ibcCroDenom = message.ibcCroDenom);
        message.ibcCroChannelID !== undefined && (obj.ibcCroChannelID = message.ibcCroChannelID);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseParams };
        message.convertEnabled = [];
        if (object.convertEnabled !== undefined && object.convertEnabled !== null) {
            for (const e of object.convertEnabled) {
                message.convertEnabled.push(ConvertEnabled.fromPartial(e));
            }
        }
        if (object.ibcCroDenom !== undefined && object.ibcCroDenom !== null) {
            message.ibcCroDenom = object.ibcCroDenom;
        }
        else {
            message.ibcCroDenom = '';
        }
        if (object.ibcCroChannelID !== undefined && object.ibcCroChannelID !== null) {
            message.ibcCroChannelID = object.ibcCroChannelID;
        }
        else {
            message.ibcCroChannelID = '';
        }
        return message;
    }
};
const baseConvertEnabled = { denom: '', enabled: false };
export const ConvertEnabled = {
    encode(message, writer = Writer.create()) {
        if (message.denom !== '') {
            writer.uint32(10).string(message.denom);
        }
        if (message.enabled === true) {
            writer.uint32(16).bool(message.enabled);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseConvertEnabled };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.denom = reader.string();
                    break;
                case 2:
                    message.enabled = reader.bool();
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseConvertEnabled };
        if (object.denom !== undefined && object.denom !== null) {
            message.denom = String(object.denom);
        }
        else {
            message.denom = '';
        }
        if (object.enabled !== undefined && object.enabled !== null) {
            message.enabled = Boolean(object.enabled);
        }
        else {
            message.enabled = false;
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.denom !== undefined && (obj.denom = message.denom);
        message.enabled !== undefined && (obj.enabled = message.enabled);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseConvertEnabled };
        if (object.denom !== undefined && object.denom !== null) {
            message.denom = object.denom;
        }
        else {
            message.denom = '';
        }
        if (object.enabled !== undefined && object.enabled !== null) {
            message.enabled = object.enabled;
        }
        else {
            message.enabled = false;
        }
        return message;
    }
};

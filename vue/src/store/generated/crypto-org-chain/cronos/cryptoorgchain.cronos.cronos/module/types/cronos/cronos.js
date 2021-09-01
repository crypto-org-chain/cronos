/* eslint-disable */
import { Writer, Reader } from 'protobufjs/minimal';
export const protobufPackage = 'cryptoorgchain.cronos.cronos';
const baseParams = { ibcCroDenom: '' };
export const Params = {
    encode(message, writer = Writer.create()) {
        if (message.ibcCroDenom !== '') {
            writer.uint32(10).string(message.ibcCroDenom);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseParams };
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.ibcCroDenom = reader.string();
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
        if (object.ibcCroDenom !== undefined && object.ibcCroDenom !== null) {
            message.ibcCroDenom = String(object.ibcCroDenom);
        }
        else {
            message.ibcCroDenom = '';
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.ibcCroDenom !== undefined && (obj.ibcCroDenom = message.ibcCroDenom);
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseParams };
        if (object.ibcCroDenom !== undefined && object.ibcCroDenom !== null) {
            message.ibcCroDenom = object.ibcCroDenom;
        }
        else {
            message.ibcCroDenom = '';
        }
        return message;
    }
};

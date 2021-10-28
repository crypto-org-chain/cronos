/* eslint-disable */
import { Params, TokenMapping } from '../cronos/cronos';
import { Writer, Reader } from 'protobufjs/minimal';
export const protobufPackage = 'cronos';
const baseGenesisState = {};
export const GenesisState = {
    encode(message, writer = Writer.create()) {
        if (message.params !== undefined) {
            Params.encode(message.params, writer.uint32(10).fork()).ldelim();
        }
        for (const v of message.externalContracts) {
            TokenMapping.encode(v, writer.uint32(18).fork()).ldelim();
        }
        for (const v of message.autoContracts) {
            TokenMapping.encode(v, writer.uint32(26).fork()).ldelim();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof Uint8Array ? new Reader(input) : input;
        let end = length === undefined ? reader.len : reader.pos + length;
        const message = { ...baseGenesisState };
        message.externalContracts = [];
        message.autoContracts = [];
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    message.params = Params.decode(reader, reader.uint32());
                    break;
                case 2:
                    message.externalContracts.push(TokenMapping.decode(reader, reader.uint32()));
                    break;
                case 3:
                    message.autoContracts.push(TokenMapping.decode(reader, reader.uint32()));
                    break;
                default:
                    reader.skipType(tag & 7);
                    break;
            }
        }
        return message;
    },
    fromJSON(object) {
        const message = { ...baseGenesisState };
        message.externalContracts = [];
        message.autoContracts = [];
        if (object.params !== undefined && object.params !== null) {
            message.params = Params.fromJSON(object.params);
        }
        else {
            message.params = undefined;
        }
        if (object.externalContracts !== undefined && object.externalContracts !== null) {
            for (const e of object.externalContracts) {
                message.externalContracts.push(TokenMapping.fromJSON(e));
            }
        }
        if (object.autoContracts !== undefined && object.autoContracts !== null) {
            for (const e of object.autoContracts) {
                message.autoContracts.push(TokenMapping.fromJSON(e));
            }
        }
        return message;
    },
    toJSON(message) {
        const obj = {};
        message.params !== undefined && (obj.params = message.params ? Params.toJSON(message.params) : undefined);
        if (message.externalContracts) {
            obj.externalContracts = message.externalContracts.map((e) => (e ? TokenMapping.toJSON(e) : undefined));
        }
        else {
            obj.externalContracts = [];
        }
        if (message.autoContracts) {
            obj.autoContracts = message.autoContracts.map((e) => (e ? TokenMapping.toJSON(e) : undefined));
        }
        else {
            obj.autoContracts = [];
        }
        return obj;
    },
    fromPartial(object) {
        const message = { ...baseGenesisState };
        message.externalContracts = [];
        message.autoContracts = [];
        if (object.params !== undefined && object.params !== null) {
            message.params = Params.fromPartial(object.params);
        }
        else {
            message.params = undefined;
        }
        if (object.externalContracts !== undefined && object.externalContracts !== null) {
            for (const e of object.externalContracts) {
                message.externalContracts.push(TokenMapping.fromPartial(e));
            }
        }
        if (object.autoContracts !== undefined && object.autoContracts !== null) {
            for (const e of object.autoContracts) {
                message.autoContracts.push(TokenMapping.fromPartial(e));
            }
        }
        return message;
    }
};

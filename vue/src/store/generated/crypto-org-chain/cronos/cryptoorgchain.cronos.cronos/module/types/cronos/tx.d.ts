import { Reader, Writer } from 'protobufjs/minimal';
import { Coin } from '../cosmos/base/v1beta1/coin';
export declare const protobufPackage = "cryptoorgchain.cronos.cronos";
/** MsgConvertToEvmTokens represents a message to convert ibc coins to evm coins. */
export interface MsgConvertTokens {
    address: string;
    amount: Coin[];
}
/** MsgConvertToIbcTokens represents a message to convert evm coins to ibc coins. */
export interface MsgSendToCryptoOrg {
    address: string;
    amount: Coin[];
}
/** MsgMultiSendResponse defines the MsgConvert response type. */
export interface MsgConvertResponse {
}
export declare const MsgConvertTokens: {
    encode(message: MsgConvertTokens, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgConvertTokens;
    fromJSON(object: any): MsgConvertTokens;
    toJSON(message: MsgConvertTokens): unknown;
    fromPartial(object: DeepPartial<MsgConvertTokens>): MsgConvertTokens;
};
export declare const MsgSendToCryptoOrg: {
    encode(message: MsgSendToCryptoOrg, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgSendToCryptoOrg;
    fromJSON(object: any): MsgSendToCryptoOrg;
    toJSON(message: MsgSendToCryptoOrg): unknown;
    fromPartial(object: DeepPartial<MsgSendToCryptoOrg>): MsgSendToCryptoOrg;
};
export declare const MsgConvertResponse: {
    encode(_: MsgConvertResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgConvertResponse;
    fromJSON(_: any): MsgConvertResponse;
    toJSON(_: MsgConvertResponse): unknown;
    fromPartial(_: DeepPartial<MsgConvertResponse>): MsgConvertResponse;
};
/** Msg defines the Cronos Msg service */
export interface Msg {
    /** Send defines a method for converting ibc coins to Cronos coins. */
    ConvertTokens(request: MsgConvertTokens): Promise<MsgConvertResponse>;
    /** Send defines a method to send coins to Crypto.org chain */
    SendToCryptoOrg(request: MsgSendToCryptoOrg): Promise<MsgConvertResponse>;
}
export declare class MsgClientImpl implements Msg {
    private readonly rpc;
    constructor(rpc: Rpc);
    ConvertTokens(request: MsgConvertTokens): Promise<MsgConvertResponse>;
    SendToCryptoOrg(request: MsgSendToCryptoOrg): Promise<MsgConvertResponse>;
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

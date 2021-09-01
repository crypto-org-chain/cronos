import { Reader, Writer } from 'protobufjs/minimal';
import { Coin } from '../cosmos/base/v1beta1/coin';
export declare const protobufPackage = "cryptoorgchain.cronos.cronos";
/** MsgConvertVouchers represents a message to convert ibc voucher coins to cronos evm coins. */
export interface MsgConvertVouchers {
    address: string;
    coins: Coin[];
}
/** MsgTransferTokens represents a message to transfer cronos evm coins through ibc. */
export interface MsgTransferTokens {
    from: string;
    to: string;
    coins: Coin[];
}
/** MsgConvertResponse defines the MsgConvert response type. */
export interface MsgConvertResponse {
}
export declare const MsgConvertVouchers: {
    encode(message: MsgConvertVouchers, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgConvertVouchers;
    fromJSON(object: any): MsgConvertVouchers;
    toJSON(message: MsgConvertVouchers): unknown;
    fromPartial(object: DeepPartial<MsgConvertVouchers>): MsgConvertVouchers;
};
export declare const MsgTransferTokens: {
    encode(message: MsgTransferTokens, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgTransferTokens;
    fromJSON(object: any): MsgTransferTokens;
    toJSON(message: MsgTransferTokens): unknown;
    fromPartial(object: DeepPartial<MsgTransferTokens>): MsgTransferTokens;
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
    /** ConvertVouchers defines a method for converting ibc voucher to cronos evm coins. */
    ConvertVouchers(request: MsgConvertVouchers): Promise<MsgConvertResponse>;
    /** TransferTokens defines a method to transfer cronos evm coins to another chain through IBC */
    TransferTokens(request: MsgTransferTokens): Promise<MsgConvertResponse>;
}
export declare class MsgClientImpl implements Msg {
    private readonly rpc;
    constructor(rpc: Rpc);
    ConvertVouchers(request: MsgConvertVouchers): Promise<MsgConvertResponse>;
    TransferTokens(request: MsgTransferTokens): Promise<MsgConvertResponse>;
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

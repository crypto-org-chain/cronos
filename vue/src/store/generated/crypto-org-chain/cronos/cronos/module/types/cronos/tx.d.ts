import { Reader, Writer } from 'protobufjs/minimal';
import { Coin } from '../cosmos/base/v1beta1/coin';
export declare const protobufPackage = "cronos";
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
/** MsgConvertVouchersResponse defines the ConvertVouchers response type. */
export interface MsgConvertVouchersResponse {
}
/** MsgTransferTokensResponse defines the TransferTokens response type. */
export interface MsgTransferTokensResponse {
}
/** MsgUpdateTokenMapping defines the request type */
export interface MsgUpdateTokenMapping {
    sender: string;
    denom: string;
    contract: string;
}
/** MsgUpdateTokenMappingResponse defines the response type */
export interface MsgUpdateTokenMappingResponse {
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
export declare const MsgConvertVouchersResponse: {
    encode(_: MsgConvertVouchersResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgConvertVouchersResponse;
    fromJSON(_: any): MsgConvertVouchersResponse;
    toJSON(_: MsgConvertVouchersResponse): unknown;
    fromPartial(_: DeepPartial<MsgConvertVouchersResponse>): MsgConvertVouchersResponse;
};
export declare const MsgTransferTokensResponse: {
    encode(_: MsgTransferTokensResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgTransferTokensResponse;
    fromJSON(_: any): MsgTransferTokensResponse;
    toJSON(_: MsgTransferTokensResponse): unknown;
    fromPartial(_: DeepPartial<MsgTransferTokensResponse>): MsgTransferTokensResponse;
};
export declare const MsgUpdateTokenMapping: {
    encode(message: MsgUpdateTokenMapping, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgUpdateTokenMapping;
    fromJSON(object: any): MsgUpdateTokenMapping;
    toJSON(message: MsgUpdateTokenMapping): unknown;
    fromPartial(object: DeepPartial<MsgUpdateTokenMapping>): MsgUpdateTokenMapping;
};
export declare const MsgUpdateTokenMappingResponse: {
    encode(_: MsgUpdateTokenMappingResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgUpdateTokenMappingResponse;
    fromJSON(_: any): MsgUpdateTokenMappingResponse;
    toJSON(_: MsgUpdateTokenMappingResponse): unknown;
    fromPartial(_: DeepPartial<MsgUpdateTokenMappingResponse>): MsgUpdateTokenMappingResponse;
};
/** Msg defines the Cronos Msg service */
export interface Msg {
    /** ConvertVouchers defines a method for converting ibc voucher to cronos evm coins. */
    ConvertVouchers(request: MsgConvertVouchers): Promise<MsgConvertVouchersResponse>;
    /** TransferTokens defines a method to transfer cronos evm coins to another chain through IBC */
    TransferTokens(request: MsgTransferTokens): Promise<MsgTransferTokensResponse>;
    /** UpdateTokenMapping defines a method to update token mapping */
    UpdateTokenMapping(request: MsgUpdateTokenMapping): Promise<MsgUpdateTokenMappingResponse>;
}
export declare class MsgClientImpl implements Msg {
    private readonly rpc;
    constructor(rpc: Rpc);
    ConvertVouchers(request: MsgConvertVouchers): Promise<MsgConvertVouchersResponse>;
    TransferTokens(request: MsgTransferTokens): Promise<MsgTransferTokensResponse>;
    UpdateTokenMapping(request: MsgUpdateTokenMapping): Promise<MsgUpdateTokenMappingResponse>;
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

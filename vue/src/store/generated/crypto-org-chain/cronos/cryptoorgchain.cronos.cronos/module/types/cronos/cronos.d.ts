import { Writer, Reader } from 'protobufjs/minimal';
export declare const protobufPackage = "cryptoorgchain.cronos.cronos";
/** Params defines the parameters for the cronos module. */
export interface Params {
    convertEnabled: ConvertEnabled[];
    ibcCroDenom: string;
    ibcCroChannelid: string;
}
/**
 * ConvertEnabled maps coin denom to a convert_enabled status (whether a denom is
 * convertable).
 */
export interface ConvertEnabled {
    denom: string;
    enabled: boolean;
}
export declare const Params: {
    encode(message: Params, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): Params;
    fromJSON(object: any): Params;
    toJSON(message: Params): unknown;
    fromPartial(object: DeepPartial<Params>): Params;
};
export declare const ConvertEnabled: {
    encode(message: ConvertEnabled, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): ConvertEnabled;
    fromJSON(object: any): ConvertEnabled;
    toJSON(message: ConvertEnabled): unknown;
    fromPartial(object: DeepPartial<ConvertEnabled>): ConvertEnabled;
};
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

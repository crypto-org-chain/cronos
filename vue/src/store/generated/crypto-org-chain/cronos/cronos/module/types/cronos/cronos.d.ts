import { Writer, Reader } from 'protobufjs/minimal';
export declare const protobufPackage = "cronos";
/** Params defines the parameters for the cronos module. */
export interface Params {
    ibcCroDenom: string;
    ibcTimeout: number;
    /** the admin address who can update token mapping */
    cronosAdmin: string;
    enableAutoDeployment: boolean;
}
/** TokenMappingChangeProposal defines a proposal to change one token mapping. */
export interface TokenMappingChangeProposal {
    title: string;
    description: string;
    denom: string;
    contract: string;
}
/** TokenMapping defines a mapping between native denom and contract */
export interface TokenMapping {
    denom: string;
    contract: string;
}
export declare const Params: {
    encode(message: Params, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): Params;
    fromJSON(object: any): Params;
    toJSON(message: Params): unknown;
    fromPartial(object: DeepPartial<Params>): Params;
};
export declare const TokenMappingChangeProposal: {
    encode(message: TokenMappingChangeProposal, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): TokenMappingChangeProposal;
    fromJSON(object: any): TokenMappingChangeProposal;
    toJSON(message: TokenMappingChangeProposal): unknown;
    fromPartial(object: DeepPartial<TokenMappingChangeProposal>): TokenMappingChangeProposal;
};
export declare const TokenMapping: {
    encode(message: TokenMapping, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): TokenMapping;
    fromJSON(object: any): TokenMapping;
    toJSON(message: TokenMapping): unknown;
    fromPartial(object: DeepPartial<TokenMapping>): TokenMapping;
};
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

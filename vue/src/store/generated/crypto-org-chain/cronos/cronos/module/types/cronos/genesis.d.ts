import { Params, TokenMapping } from '../cronos/cronos';
import { Writer, Reader } from 'protobufjs/minimal';
export declare const protobufPackage = "cronos";
/** GenesisState defines the cronos module's genesis state. */
export interface GenesisState {
    /** params defines all the paramaters of the module. */
    params: Params | undefined;
    externalContracts: TokenMapping[];
    /**
     * this line is used by starport scaffolding # genesis/proto/state
     * this line is used by starport scaffolding # ibc/genesis/proto
     */
    autoContracts: TokenMapping[];
}
export declare const GenesisState: {
    encode(message: GenesisState, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): GenesisState;
    fromJSON(object: any): GenesisState;
    toJSON(message: GenesisState): unknown;
    fromPartial(object: DeepPartial<GenesisState>): GenesisState;
};
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

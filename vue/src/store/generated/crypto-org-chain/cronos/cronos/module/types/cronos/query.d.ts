import { Reader, Writer } from 'protobufjs/minimal';
export declare const protobufPackage = "cronos";
/** ContractByDenomRequest is the request type of ContractByDenom call */
export interface ContractByDenomRequest {
    denom: string;
}
/** ContractByDenomRequest is the response type of ContractByDenom call */
export interface ContractByDenomResponse {
    contract: string;
    autoContract: string;
}
/** DenomByContractRequest is the request type of DenomByContract call */
export interface DenomByContractRequest {
    contract: string;
}
/** DenomByContractResponse is the response type of DenomByContract call */
export interface DenomByContractResponse {
    denom: string;
}
export declare const ContractByDenomRequest: {
    encode(message: ContractByDenomRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): ContractByDenomRequest;
    fromJSON(object: any): ContractByDenomRequest;
    toJSON(message: ContractByDenomRequest): unknown;
    fromPartial(object: DeepPartial<ContractByDenomRequest>): ContractByDenomRequest;
};
export declare const ContractByDenomResponse: {
    encode(message: ContractByDenomResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): ContractByDenomResponse;
    fromJSON(object: any): ContractByDenomResponse;
    toJSON(message: ContractByDenomResponse): unknown;
    fromPartial(object: DeepPartial<ContractByDenomResponse>): ContractByDenomResponse;
};
export declare const DenomByContractRequest: {
    encode(message: DenomByContractRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): DenomByContractRequest;
    fromJSON(object: any): DenomByContractRequest;
    toJSON(message: DenomByContractRequest): unknown;
    fromPartial(object: DeepPartial<DenomByContractRequest>): DenomByContractRequest;
};
export declare const DenomByContractResponse: {
    encode(message: DenomByContractResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): DenomByContractResponse;
    fromJSON(object: any): DenomByContractResponse;
    toJSON(message: DenomByContractResponse): unknown;
    fromPartial(object: DeepPartial<DenomByContractResponse>): DenomByContractResponse;
};
/** Query defines the gRPC querier service. */
export interface Query {
    /** ContractByDenom queries contract addresses by native denom */
    ContractByDenom(request: ContractByDenomRequest): Promise<ContractByDenomResponse>;
    /** DenomByContract queries native denom by contract address */
    DenomByContract(request: DenomByContractRequest): Promise<DenomByContractResponse>;
}
export declare class QueryClientImpl implements Query {
    private readonly rpc;
    constructor(rpc: Rpc);
    ContractByDenom(request: ContractByDenomRequest): Promise<ContractByDenomResponse>;
    DenomByContract(request: DenomByContractRequest): Promise<DenomByContractResponse>;
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

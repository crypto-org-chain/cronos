import { Reader, Writer } from 'protobufjs/minimal';
import { PageRequest, PageResponse } from '../../../cosmos/base/query/v1beta1/pagination';
import { Log, Params, TraceConfig } from '../../../ethermint/evm/v1/evm';
import { MsgEthereumTx, MsgEthereumTxResponse } from '../../../ethermint/evm/v1/tx';
export declare const protobufPackage = "ethermint.evm.v1";
/** QueryAccountRequest is the request type for the Query/Account RPC method. */
export interface QueryAccountRequest {
    /** address is the ethereum hex address to query the account for. */
    address: string;
}
/** QueryAccountResponse is the response type for the Query/Account RPC method. */
export interface QueryAccountResponse {
    /** balance is the balance of the EVM denomination. */
    balance: string;
    /** code hash is the hex-formatted code bytes from the EOA. */
    codeHash: string;
    /** nonce is the account's sequence number. */
    nonce: number;
}
/**
 * QueryCosmosAccountRequest is the request type for the Query/CosmosAccount RPC
 * method.
 */
export interface QueryCosmosAccountRequest {
    /** address is the ethereum hex address to query the account for. */
    address: string;
}
/**
 * QueryCosmosAccountResponse is the response type for the Query/CosmosAccount
 * RPC method.
 */
export interface QueryCosmosAccountResponse {
    /** cosmos_address is the cosmos address of the account. */
    cosmosAddress: string;
    /** sequence is the account's sequence number. */
    sequence: number;
    /** account_number is the account numbert */
    accountNumber: number;
}
/**
 * QueryValidatorAccountRequest is the request type for the
 * Query/ValidatorAccount RPC method.
 */
export interface QueryValidatorAccountRequest {
    /** cons_address is the validator cons address to query the account for. */
    consAddress: string;
}
/**
 * QueryValidatorAccountResponse is the response type for the
 * Query/ValidatorAccount RPC method.
 */
export interface QueryValidatorAccountResponse {
    /** account_address is the cosmos address of the account in bech32 format. */
    accountAddress: string;
    /** sequence is the account's sequence number. */
    sequence: number;
    /** account_number is the account number */
    accountNumber: number;
}
/** QueryBalanceRequest is the request type for the Query/Balance RPC method. */
export interface QueryBalanceRequest {
    /** address is the ethereum hex address to query the balance for. */
    address: string;
}
/** QueryBalanceResponse is the response type for the Query/Balance RPC method. */
export interface QueryBalanceResponse {
    /** balance is the balance of the EVM denomination. */
    balance: string;
}
/** QueryStorageRequest is the request type for the Query/Storage RPC method. */
export interface QueryStorageRequest {
    /** / address is the ethereum hex address to query the storage state for. */
    address: string;
    /** key defines the key of the storage state */
    key: string;
}
/**
 * QueryStorageResponse is the response type for the Query/Storage RPC
 * method.
 */
export interface QueryStorageResponse {
    /** key defines the storage state value hash associated with the given key. */
    value: string;
}
/** QueryCodeRequest is the request type for the Query/Code RPC method. */
export interface QueryCodeRequest {
    /** address is the ethereum hex address to query the code for. */
    address: string;
}
/**
 * QueryCodeResponse is the response type for the Query/Code RPC
 * method.
 */
export interface QueryCodeResponse {
    /** code represents the code bytes from an ethereum address. */
    code: Uint8Array;
}
/** QueryTxLogsRequest is the request type for the Query/TxLogs RPC method. */
export interface QueryTxLogsRequest {
    /** hash is the ethereum transaction hex hash to query the logs for. */
    hash: string;
    /** pagination defines an optional pagination for the request. */
    pagination: PageRequest | undefined;
}
/** QueryTxLogs is the response type for the Query/TxLogs RPC method. */
export interface QueryTxLogsResponse {
    /** logs represents the ethereum logs generated from the given transaction. */
    logs: Log[];
    /** pagination defines the pagination in the response. */
    pagination: PageResponse | undefined;
}
/** QueryParamsRequest defines the request type for querying x/evm parameters. */
export interface QueryParamsRequest {
}
/** QueryParamsResponse defines the response type for querying x/evm parameters. */
export interface QueryParamsResponse {
    /** params define the evm module parameters. */
    params: Params | undefined;
}
/** QueryStaticCallRequest defines static call response */
export interface QueryStaticCallResponse {
    data: Uint8Array;
}
/** EthCallRequest defines EthCall request */
export interface EthCallRequest {
    /** same json format as the json rpc api. */
    args: Uint8Array;
    /** the default gas cap to be used */
    gasCap: number;
}
/** EstimateGasResponse defines EstimateGas response */
export interface EstimateGasResponse {
    /** the estimated gas */
    gas: number;
}
/** QueryTraceTxRequest defines TraceTx request */
export interface QueryTraceTxRequest {
    /** msgEthereumTx for the requested transaction */
    msg: MsgEthereumTx | undefined;
    /** transaction index */
    txIndex: number;
    /** TraceConfig holds extra parameters to trace functions. */
    traceConfig: TraceConfig | undefined;
}
/** QueryTraceTxResponse defines TraceTx response */
export interface QueryTraceTxResponse {
    /** response serialized in bytes */
    data: Uint8Array;
}
export declare const QueryAccountRequest: {
    encode(message: QueryAccountRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryAccountRequest;
    fromJSON(object: any): QueryAccountRequest;
    toJSON(message: QueryAccountRequest): unknown;
    fromPartial(object: DeepPartial<QueryAccountRequest>): QueryAccountRequest;
};
export declare const QueryAccountResponse: {
    encode(message: QueryAccountResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryAccountResponse;
    fromJSON(object: any): QueryAccountResponse;
    toJSON(message: QueryAccountResponse): unknown;
    fromPartial(object: DeepPartial<QueryAccountResponse>): QueryAccountResponse;
};
export declare const QueryCosmosAccountRequest: {
    encode(message: QueryCosmosAccountRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryCosmosAccountRequest;
    fromJSON(object: any): QueryCosmosAccountRequest;
    toJSON(message: QueryCosmosAccountRequest): unknown;
    fromPartial(object: DeepPartial<QueryCosmosAccountRequest>): QueryCosmosAccountRequest;
};
export declare const QueryCosmosAccountResponse: {
    encode(message: QueryCosmosAccountResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryCosmosAccountResponse;
    fromJSON(object: any): QueryCosmosAccountResponse;
    toJSON(message: QueryCosmosAccountResponse): unknown;
    fromPartial(object: DeepPartial<QueryCosmosAccountResponse>): QueryCosmosAccountResponse;
};
export declare const QueryValidatorAccountRequest: {
    encode(message: QueryValidatorAccountRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryValidatorAccountRequest;
    fromJSON(object: any): QueryValidatorAccountRequest;
    toJSON(message: QueryValidatorAccountRequest): unknown;
    fromPartial(object: DeepPartial<QueryValidatorAccountRequest>): QueryValidatorAccountRequest;
};
export declare const QueryValidatorAccountResponse: {
    encode(message: QueryValidatorAccountResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryValidatorAccountResponse;
    fromJSON(object: any): QueryValidatorAccountResponse;
    toJSON(message: QueryValidatorAccountResponse): unknown;
    fromPartial(object: DeepPartial<QueryValidatorAccountResponse>): QueryValidatorAccountResponse;
};
export declare const QueryBalanceRequest: {
    encode(message: QueryBalanceRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryBalanceRequest;
    fromJSON(object: any): QueryBalanceRequest;
    toJSON(message: QueryBalanceRequest): unknown;
    fromPartial(object: DeepPartial<QueryBalanceRequest>): QueryBalanceRequest;
};
export declare const QueryBalanceResponse: {
    encode(message: QueryBalanceResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryBalanceResponse;
    fromJSON(object: any): QueryBalanceResponse;
    toJSON(message: QueryBalanceResponse): unknown;
    fromPartial(object: DeepPartial<QueryBalanceResponse>): QueryBalanceResponse;
};
export declare const QueryStorageRequest: {
    encode(message: QueryStorageRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryStorageRequest;
    fromJSON(object: any): QueryStorageRequest;
    toJSON(message: QueryStorageRequest): unknown;
    fromPartial(object: DeepPartial<QueryStorageRequest>): QueryStorageRequest;
};
export declare const QueryStorageResponse: {
    encode(message: QueryStorageResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryStorageResponse;
    fromJSON(object: any): QueryStorageResponse;
    toJSON(message: QueryStorageResponse): unknown;
    fromPartial(object: DeepPartial<QueryStorageResponse>): QueryStorageResponse;
};
export declare const QueryCodeRequest: {
    encode(message: QueryCodeRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryCodeRequest;
    fromJSON(object: any): QueryCodeRequest;
    toJSON(message: QueryCodeRequest): unknown;
    fromPartial(object: DeepPartial<QueryCodeRequest>): QueryCodeRequest;
};
export declare const QueryCodeResponse: {
    encode(message: QueryCodeResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryCodeResponse;
    fromJSON(object: any): QueryCodeResponse;
    toJSON(message: QueryCodeResponse): unknown;
    fromPartial(object: DeepPartial<QueryCodeResponse>): QueryCodeResponse;
};
export declare const QueryTxLogsRequest: {
    encode(message: QueryTxLogsRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryTxLogsRequest;
    fromJSON(object: any): QueryTxLogsRequest;
    toJSON(message: QueryTxLogsRequest): unknown;
    fromPartial(object: DeepPartial<QueryTxLogsRequest>): QueryTxLogsRequest;
};
export declare const QueryTxLogsResponse: {
    encode(message: QueryTxLogsResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryTxLogsResponse;
    fromJSON(object: any): QueryTxLogsResponse;
    toJSON(message: QueryTxLogsResponse): unknown;
    fromPartial(object: DeepPartial<QueryTxLogsResponse>): QueryTxLogsResponse;
};
export declare const QueryParamsRequest: {
    encode(_: QueryParamsRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryParamsRequest;
    fromJSON(_: any): QueryParamsRequest;
    toJSON(_: QueryParamsRequest): unknown;
    fromPartial(_: DeepPartial<QueryParamsRequest>): QueryParamsRequest;
};
export declare const QueryParamsResponse: {
    encode(message: QueryParamsResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryParamsResponse;
    fromJSON(object: any): QueryParamsResponse;
    toJSON(message: QueryParamsResponse): unknown;
    fromPartial(object: DeepPartial<QueryParamsResponse>): QueryParamsResponse;
};
export declare const QueryStaticCallResponse: {
    encode(message: QueryStaticCallResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryStaticCallResponse;
    fromJSON(object: any): QueryStaticCallResponse;
    toJSON(message: QueryStaticCallResponse): unknown;
    fromPartial(object: DeepPartial<QueryStaticCallResponse>): QueryStaticCallResponse;
};
export declare const EthCallRequest: {
    encode(message: EthCallRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): EthCallRequest;
    fromJSON(object: any): EthCallRequest;
    toJSON(message: EthCallRequest): unknown;
    fromPartial(object: DeepPartial<EthCallRequest>): EthCallRequest;
};
export declare const EstimateGasResponse: {
    encode(message: EstimateGasResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): EstimateGasResponse;
    fromJSON(object: any): EstimateGasResponse;
    toJSON(message: EstimateGasResponse): unknown;
    fromPartial(object: DeepPartial<EstimateGasResponse>): EstimateGasResponse;
};
export declare const QueryTraceTxRequest: {
    encode(message: QueryTraceTxRequest, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryTraceTxRequest;
    fromJSON(object: any): QueryTraceTxRequest;
    toJSON(message: QueryTraceTxRequest): unknown;
    fromPartial(object: DeepPartial<QueryTraceTxRequest>): QueryTraceTxRequest;
};
export declare const QueryTraceTxResponse: {
    encode(message: QueryTraceTxResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): QueryTraceTxResponse;
    fromJSON(object: any): QueryTraceTxResponse;
    toJSON(message: QueryTraceTxResponse): unknown;
    fromPartial(object: DeepPartial<QueryTraceTxResponse>): QueryTraceTxResponse;
};
/** Query defines the gRPC querier service. */
export interface Query {
    /** Account queries an Ethereum account. */
    Account(request: QueryAccountRequest): Promise<QueryAccountResponse>;
    /** CosmosAccount queries an Ethereum account's Cosmos Address. */
    CosmosAccount(request: QueryCosmosAccountRequest): Promise<QueryCosmosAccountResponse>;
    /**
     * ValidatorAccount queries an Ethereum account's from a validator consensus
     * Address.
     */
    ValidatorAccount(request: QueryValidatorAccountRequest): Promise<QueryValidatorAccountResponse>;
    /**
     * Balance queries the balance of a the EVM denomination for a single
     * EthAccount.
     */
    Balance(request: QueryBalanceRequest): Promise<QueryBalanceResponse>;
    /** Storage queries the balance of all coins for a single account. */
    Storage(request: QueryStorageRequest): Promise<QueryStorageResponse>;
    /** Code queries the balance of all coins for a single account. */
    Code(request: QueryCodeRequest): Promise<QueryCodeResponse>;
    /** Params queries the parameters of x/evm module. */
    Params(request: QueryParamsRequest): Promise<QueryParamsResponse>;
    /** EthCall implements the `eth_call` rpc api */
    EthCall(request: EthCallRequest): Promise<MsgEthereumTxResponse>;
    /** EstimateGas implements the `eth_estimateGas` rpc api */
    EstimateGas(request: EthCallRequest): Promise<EstimateGasResponse>;
    /** TraceTx implements the `debug_traceTransaction` rpc api */
    TraceTx(request: QueryTraceTxRequest): Promise<QueryTraceTxResponse>;
}
export declare class QueryClientImpl implements Query {
    private readonly rpc;
    constructor(rpc: Rpc);
    Account(request: QueryAccountRequest): Promise<QueryAccountResponse>;
    CosmosAccount(request: QueryCosmosAccountRequest): Promise<QueryCosmosAccountResponse>;
    ValidatorAccount(request: QueryValidatorAccountRequest): Promise<QueryValidatorAccountResponse>;
    Balance(request: QueryBalanceRequest): Promise<QueryBalanceResponse>;
    Storage(request: QueryStorageRequest): Promise<QueryStorageResponse>;
    Code(request: QueryCodeRequest): Promise<QueryCodeResponse>;
    Params(request: QueryParamsRequest): Promise<QueryParamsResponse>;
    EthCall(request: EthCallRequest): Promise<MsgEthereumTxResponse>;
    EstimateGas(request: EthCallRequest): Promise<EstimateGasResponse>;
    TraceTx(request: QueryTraceTxRequest): Promise<QueryTraceTxResponse>;
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

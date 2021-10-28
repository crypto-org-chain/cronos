import { Reader, Writer } from 'protobufjs/minimal';
import { Any } from '../../../google/protobuf/any';
import { AccessTuple, Log } from '../../../ethermint/evm/v1/evm';
export declare const protobufPackage = "ethermint.evm.v1";
/** MsgEthereumTx encapsulates an Ethereum transaction as an SDK message. */
export interface MsgEthereumTx {
    /** inner transaction data */
    data: Any | undefined;
    /** encoded storage size of the transaction */
    size: number;
    /** transaction hash in hex format */
    hash: string;
    /**
     * ethereum signer address in hex format. This address value is checked
     * against the address derived from the signature (V, R, S) using the
     * secp256k1 elliptic curve
     */
    from: string;
}
/** LegacyTx is the transaction data of regular Ethereum transactions. */
export interface LegacyTx {
    /** nonce corresponds to the account nonce (transaction sequence). */
    nonce: number;
    /** gas price defines the value for each gas unit */
    gasPrice: string;
    /** gas defines the gas limit defined for the transaction. */
    gas: number;
    /** hex formatted address of the recipient */
    to: string;
    /** value defines the unsigned integer value of the transaction amount. */
    value: string;
    /** input defines the data payload bytes of the transaction. */
    data: Uint8Array;
    /** v defines the signature value */
    v: Uint8Array;
    /** r defines the signature value */
    r: Uint8Array;
    /** s define the signature value */
    s: Uint8Array;
}
/** AccessListTx is the data of EIP-2930 access list transactions. */
export interface AccessListTx {
    /** destination EVM chain ID */
    chainId: string;
    /** nonce corresponds to the account nonce (transaction sequence). */
    nonce: number;
    /** gas price defines the value for each gas unit */
    gasPrice: string;
    /** gas defines the gas limit defined for the transaction. */
    gas: number;
    /** hex formatted address of the recipient */
    to: string;
    /** value defines the unsigned integer value of the transaction amount. */
    value: string;
    /** input defines the data payload bytes of the transaction. */
    data: Uint8Array;
    accesses: AccessTuple[];
    /** v defines the signature value */
    v: Uint8Array;
    /** r defines the signature value */
    r: Uint8Array;
    /** s define the signature value */
    s: Uint8Array;
}
/** DynamicFeeTx is the data of EIP-1559 dinamic fee transactions. */
export interface DynamicFeeTx {
    /** destination EVM chain ID */
    chainId: string;
    /** nonce corresponds to the account nonce (transaction sequence). */
    nonce: number;
    /** gas tip cap defines the max value for the gas tip */
    gasTipCap: string;
    /** gas fee cap defines the max value for the gas fee */
    gasFeeCap: string;
    /** gas defines the gas limit defined for the transaction. */
    gas: number;
    /** hex formatted address of the recipient */
    to: string;
    /** value defines the the transaction amount. */
    value: string;
    /** input defines the data payload bytes of the transaction. */
    data: Uint8Array;
    accesses: AccessTuple[];
    /** v defines the signature value */
    v: Uint8Array;
    /** r defines the signature value */
    r: Uint8Array;
    /** s define the signature value */
    s: Uint8Array;
}
export interface ExtensionOptionsEthereumTx {
}
/** MsgEthereumTxResponse defines the Msg/EthereumTx response type. */
export interface MsgEthereumTxResponse {
    /**
     * ethereum transaction hash in hex format. This hash differs from the
     * Tendermint sha256 hash of the transaction bytes. See
     * https://github.com/tendermint/tendermint/issues/6539 for reference
     */
    hash: string;
    /**
     * logs contains the transaction hash and the proto-compatible ethereum
     * logs.
     */
    logs: Log[];
    /**
     * returned data from evm function (result or data supplied with revert
     * opcode)
     */
    ret: Uint8Array;
    /** vm error is the error returned by vm execution */
    vmError: string;
    /** gas consumed by the transaction */
    gasUsed: number;
}
export declare const MsgEthereumTx: {
    encode(message: MsgEthereumTx, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgEthereumTx;
    fromJSON(object: any): MsgEthereumTx;
    toJSON(message: MsgEthereumTx): unknown;
    fromPartial(object: DeepPartial<MsgEthereumTx>): MsgEthereumTx;
};
export declare const LegacyTx: {
    encode(message: LegacyTx, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): LegacyTx;
    fromJSON(object: any): LegacyTx;
    toJSON(message: LegacyTx): unknown;
    fromPartial(object: DeepPartial<LegacyTx>): LegacyTx;
};
export declare const AccessListTx: {
    encode(message: AccessListTx, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): AccessListTx;
    fromJSON(object: any): AccessListTx;
    toJSON(message: AccessListTx): unknown;
    fromPartial(object: DeepPartial<AccessListTx>): AccessListTx;
};
export declare const DynamicFeeTx: {
    encode(message: DynamicFeeTx, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): DynamicFeeTx;
    fromJSON(object: any): DynamicFeeTx;
    toJSON(message: DynamicFeeTx): unknown;
    fromPartial(object: DeepPartial<DynamicFeeTx>): DynamicFeeTx;
};
export declare const ExtensionOptionsEthereumTx: {
    encode(_: ExtensionOptionsEthereumTx, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): ExtensionOptionsEthereumTx;
    fromJSON(_: any): ExtensionOptionsEthereumTx;
    toJSON(_: ExtensionOptionsEthereumTx): unknown;
    fromPartial(_: DeepPartial<ExtensionOptionsEthereumTx>): ExtensionOptionsEthereumTx;
};
export declare const MsgEthereumTxResponse: {
    encode(message: MsgEthereumTxResponse, writer?: Writer): Writer;
    decode(input: Reader | Uint8Array, length?: number): MsgEthereumTxResponse;
    fromJSON(object: any): MsgEthereumTxResponse;
    toJSON(message: MsgEthereumTxResponse): unknown;
    fromPartial(object: DeepPartial<MsgEthereumTxResponse>): MsgEthereumTxResponse;
};
/** Msg defines the evm Msg service. */
export interface Msg {
    /** EthereumTx defines a method submitting Ethereum transactions. */
    EthereumTx(request: MsgEthereumTx): Promise<MsgEthereumTxResponse>;
}
export declare class MsgClientImpl implements Msg {
    private readonly rpc;
    constructor(rpc: Rpc);
    EthereumTx(request: MsgEthereumTx): Promise<MsgEthereumTxResponse>;
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
declare type Builtin = Date | Function | Uint8Array | string | number | undefined;
export declare type DeepPartial<T> = T extends Builtin ? T : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>> : T extends {} ? {
    [K in keyof T]?: DeepPartial<T[K]>;
} : Partial<T>;
export {};

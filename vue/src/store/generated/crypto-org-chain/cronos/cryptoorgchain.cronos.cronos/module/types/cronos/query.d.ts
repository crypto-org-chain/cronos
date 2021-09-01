export declare const protobufPackage = "cryptoorgchain.cronos.cronos";
/** Query defines the gRPC querier service. */
export interface Query {
}
export declare class QueryClientImpl implements Query {
    private readonly rpc;
    constructor(rpc: Rpc);
}
interface Rpc {
    request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
}
export {};

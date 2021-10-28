/* eslint-disable */
/* tslint:disable */
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */
export var ContentType;
(function (ContentType) {
    ContentType["Json"] = "application/json";
    ContentType["FormData"] = "multipart/form-data";
    ContentType["UrlEncoded"] = "application/x-www-form-urlencoded";
})(ContentType || (ContentType = {}));
export class HttpClient {
    constructor(apiConfig = {}) {
        this.baseUrl = "";
        this.securityData = null;
        this.securityWorker = null;
        this.abortControllers = new Map();
        this.baseApiParams = {
            credentials: "same-origin",
            headers: {},
            redirect: "follow",
            referrerPolicy: "no-referrer",
        };
        this.setSecurityData = (data) => {
            this.securityData = data;
        };
        this.contentFormatters = {
            [ContentType.Json]: (input) => input !== null && (typeof input === "object" || typeof input === "string") ? JSON.stringify(input) : input,
            [ContentType.FormData]: (input) => Object.keys(input || {}).reduce((data, key) => {
                data.append(key, input[key]);
                return data;
            }, new FormData()),
            [ContentType.UrlEncoded]: (input) => this.toQueryString(input),
        };
        this.createAbortSignal = (cancelToken) => {
            if (this.abortControllers.has(cancelToken)) {
                const abortController = this.abortControllers.get(cancelToken);
                if (abortController) {
                    return abortController.signal;
                }
                return void 0;
            }
            const abortController = new AbortController();
            this.abortControllers.set(cancelToken, abortController);
            return abortController.signal;
        };
        this.abortRequest = (cancelToken) => {
            const abortController = this.abortControllers.get(cancelToken);
            if (abortController) {
                abortController.abort();
                this.abortControllers.delete(cancelToken);
            }
        };
        this.request = ({ body, secure, path, type, query, format = "json", baseUrl, cancelToken, ...params }) => {
            const secureParams = (secure && this.securityWorker && this.securityWorker(this.securityData)) || {};
            const requestParams = this.mergeRequestParams(params, secureParams);
            const queryString = query && this.toQueryString(query);
            const payloadFormatter = this.contentFormatters[type || ContentType.Json];
            return fetch(`${baseUrl || this.baseUrl || ""}${path}${queryString ? `?${queryString}` : ""}`, {
                ...requestParams,
                headers: {
                    ...(type && type !== ContentType.FormData ? { "Content-Type": type } : {}),
                    ...(requestParams.headers || {}),
                },
                signal: cancelToken ? this.createAbortSignal(cancelToken) : void 0,
                body: typeof body === "undefined" || body === null ? null : payloadFormatter(body),
            }).then(async (response) => {
                const r = response;
                r.data = null;
                r.error = null;
                const data = await response[format]()
                    .then((data) => {
                    if (r.ok) {
                        r.data = data;
                    }
                    else {
                        r.error = data;
                    }
                    return r;
                })
                    .catch((e) => {
                    r.error = e;
                    return r;
                });
                if (cancelToken) {
                    this.abortControllers.delete(cancelToken);
                }
                if (!response.ok)
                    throw data;
                return data;
            });
        };
        Object.assign(this, apiConfig);
    }
    addQueryParam(query, key) {
        const value = query[key];
        return (encodeURIComponent(key) +
            "=" +
            encodeURIComponent(Array.isArray(value) ? value.join(",") : typeof value === "number" ? value : `${value}`));
    }
    toQueryString(rawQuery) {
        const query = rawQuery || {};
        const keys = Object.keys(query).filter((key) => "undefined" !== typeof query[key]);
        return keys
            .map((key) => typeof query[key] === "object" && !Array.isArray(query[key])
            ? this.toQueryString(query[key])
            : this.addQueryParam(query, key))
            .join("&");
    }
    addQueryParams(rawQuery) {
        const queryString = this.toQueryString(rawQuery);
        return queryString ? `?${queryString}` : "";
    }
    mergeRequestParams(params1, params2) {
        return {
            ...this.baseApiParams,
            ...params1,
            ...(params2 || {}),
            headers: {
                ...(this.baseApiParams.headers || {}),
                ...(params1.headers || {}),
                ...((params2 && params2.headers) || {}),
            },
        };
    }
}
/**
 * @title ethermint/evm/v1/evm.proto
 * @version version not set
 */
export class Api extends HttpClient {
    constructor() {
        super(...arguments);
        /**
         * No description
         *
         * @tags Query
         * @name QueryAccount
         * @summary Account queries an Ethereum account.
         * @request GET:/ethermint/evm/v1/account/{address}
         */
        this.queryAccount = (address, params = {}) => this.request({
            path: `/ethermint/evm/v1/account/${address}`,
            method: "GET",
            format: "json",
            ...params,
        });
        /**
       * No description
       *
       * @tags Query
       * @name QueryBalance
       * @summary Balance queries the balance of a the EVM denomination for a single
      EthAccount.
       * @request GET:/ethermint/evm/v1/balances/{address}
       */
        this.queryBalance = (address, params = {}) => this.request({
            path: `/ethermint/evm/v1/balances/${address}`,
            method: "GET",
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryCode
         * @summary Code queries the balance of all coins for a single account.
         * @request GET:/ethermint/evm/v1/codes/{address}
         */
        this.queryCode = (address, params = {}) => this.request({
            path: `/ethermint/evm/v1/codes/${address}`,
            method: "GET",
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryCosmosAccount
         * @summary CosmosAccount queries an Ethereum account's Cosmos Address.
         * @request GET:/ethermint/evm/v1/cosmos_account/{address}
         */
        this.queryCosmosAccount = (address, params = {}) => this.request({
            path: `/ethermint/evm/v1/cosmos_account/${address}`,
            method: "GET",
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryEstimateGas
         * @summary EstimateGas implements the `eth_estimateGas` rpc api
         * @request GET:/ethermint/evm/v1/estimate_gas
         */
        this.queryEstimateGas = (query, params = {}) => this.request({
            path: `/ethermint/evm/v1/estimate_gas`,
            method: "GET",
            query: query,
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryEthCall
         * @summary EthCall implements the `eth_call` rpc api
         * @request GET:/ethermint/evm/v1/eth_call
         */
        this.queryEthCall = (query, params = {}) => this.request({
            path: `/ethermint/evm/v1/eth_call`,
            method: "GET",
            query: query,
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryParams
         * @summary Params queries the parameters of x/evm module.
         * @request GET:/ethermint/evm/v1/params
         */
        this.queryParams = (params = {}) => this.request({
            path: `/ethermint/evm/v1/params`,
            method: "GET",
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryStorage
         * @summary Storage queries the balance of all coins for a single account.
         * @request GET:/ethermint/evm/v1/storage/{address}/{key}
         */
        this.queryStorage = (address, key, params = {}) => this.request({
            path: `/ethermint/evm/v1/storage/${address}/${key}`,
            method: "GET",
            format: "json",
            ...params,
        });
        /**
         * No description
         *
         * @tags Query
         * @name QueryTraceTx
         * @summary TraceTx implements the `debug_traceTransaction` rpc api
         * @request GET:/ethermint/evm/v1/trace_tx
         */
        this.queryTraceTx = (query, params = {}) => this.request({
            path: `/ethermint/evm/v1/trace_tx`,
            method: "GET",
            query: query,
            format: "json",
            ...params,
        });
        /**
       * No description
       *
       * @tags Query
       * @name QueryValidatorAccount
       * @summary ValidatorAccount queries an Ethereum account's from a validator consensus
      Address.
       * @request GET:/ethermint/evm/v1/validator_account/{consAddress}
       */
        this.queryValidatorAccount = (consAddress, params = {}) => this.request({
            path: `/ethermint/evm/v1/validator_account/${consAddress}`,
            method: "GET",
            format: "json",
            ...params,
        });
    }
}

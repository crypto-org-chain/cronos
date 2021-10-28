import { txClient, queryClient, MissingWalletError } from './module'
// @ts-ignore
import { SpVuexError } from '@starport/vuex'

import { Params } from "./module/types/ethermint/evm/v1/evm"
import { ChainConfig } from "./module/types/ethermint/evm/v1/evm"
import { State } from "./module/types/ethermint/evm/v1/evm"
import { TransactionLogs } from "./module/types/ethermint/evm/v1/evm"
import { Log } from "./module/types/ethermint/evm/v1/evm"
import { TxResult } from "./module/types/ethermint/evm/v1/evm"
import { AccessTuple } from "./module/types/ethermint/evm/v1/evm"
import { TraceConfig } from "./module/types/ethermint/evm/v1/evm"
import { GenesisAccount } from "./module/types/ethermint/evm/v1/genesis"
import { QueryTxLogsRequest } from "./module/types/ethermint/evm/v1/query"
import { QueryTxLogsResponse } from "./module/types/ethermint/evm/v1/query"
import { QueryStaticCallResponse } from "./module/types/ethermint/evm/v1/query"
import { LegacyTx } from "./module/types/ethermint/evm/v1/tx"
import { AccessListTx } from "./module/types/ethermint/evm/v1/tx"
import { DynamicFeeTx } from "./module/types/ethermint/evm/v1/tx"
import { ExtensionOptionsEthereumTx } from "./module/types/ethermint/evm/v1/tx"


export { Params, ChainConfig, State, TransactionLogs, Log, TxResult, AccessTuple, TraceConfig, GenesisAccount, QueryTxLogsRequest, QueryTxLogsResponse, QueryStaticCallResponse, LegacyTx, AccessListTx, DynamicFeeTx, ExtensionOptionsEthereumTx };

async function initTxClient(vuexGetters) {
	return await txClient(vuexGetters['common/wallet/signer'], {
		addr: vuexGetters['common/env/apiTendermint']
	})
}

async function initQueryClient(vuexGetters) {
	return await queryClient({
		addr: vuexGetters['common/env/apiCosmos']
	})
}

function mergeResults(value, next_values) {
	for (let prop of Object.keys(next_values)) {
		if (Array.isArray(next_values[prop])) {
			value[prop]=[...value[prop], ...next_values[prop]]
		}else{
			value[prop]=next_values[prop]
		}
	}
	return value
}

function getStructure(template) {
	let structure = { fields: [] }
	for (const [key, value] of Object.entries(template)) {
		let field: any = {}
		field.name = key
		field.type = typeof value
		structure.fields.push(field)
	}
	return structure
}

const getDefaultState = () => {
	return {
				Account: {},
				CosmosAccount: {},
				ValidatorAccount: {},
				Balance: {},
				Storage: {},
				Code: {},
				Params: {},
				EthCall: {},
				EstimateGas: {},
				TraceTx: {},
				
				_Structure: {
						Params: getStructure(Params.fromPartial({})),
						ChainConfig: getStructure(ChainConfig.fromPartial({})),
						State: getStructure(State.fromPartial({})),
						TransactionLogs: getStructure(TransactionLogs.fromPartial({})),
						Log: getStructure(Log.fromPartial({})),
						TxResult: getStructure(TxResult.fromPartial({})),
						AccessTuple: getStructure(AccessTuple.fromPartial({})),
						TraceConfig: getStructure(TraceConfig.fromPartial({})),
						GenesisAccount: getStructure(GenesisAccount.fromPartial({})),
						QueryTxLogsRequest: getStructure(QueryTxLogsRequest.fromPartial({})),
						QueryTxLogsResponse: getStructure(QueryTxLogsResponse.fromPartial({})),
						QueryStaticCallResponse: getStructure(QueryStaticCallResponse.fromPartial({})),
						LegacyTx: getStructure(LegacyTx.fromPartial({})),
						AccessListTx: getStructure(AccessListTx.fromPartial({})),
						DynamicFeeTx: getStructure(DynamicFeeTx.fromPartial({})),
						ExtensionOptionsEthereumTx: getStructure(ExtensionOptionsEthereumTx.fromPartial({})),
						
		},
		_Subscriptions: new Set(),
	}
}

// initial state
const state = getDefaultState()

export default {
	namespaced: true,
	state,
	mutations: {
		RESET_STATE(state) {
			Object.assign(state, getDefaultState())
		},
		QUERY(state, { query, key, value }) {
			state[query][JSON.stringify(key)] = value
		},
		SUBSCRIBE(state, subscription) {
			state._Subscriptions.add(subscription)
		},
		UNSUBSCRIBE(state, subscription) {
			state._Subscriptions.delete(subscription)
		}
	},
	getters: {
				getAccount: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.Account[JSON.stringify(params)] ?? {}
		},
				getCosmosAccount: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.CosmosAccount[JSON.stringify(params)] ?? {}
		},
				getValidatorAccount: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.ValidatorAccount[JSON.stringify(params)] ?? {}
		},
				getBalance: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.Balance[JSON.stringify(params)] ?? {}
		},
				getStorage: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.Storage[JSON.stringify(params)] ?? {}
		},
				getCode: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.Code[JSON.stringify(params)] ?? {}
		},
				getParams: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.Params[JSON.stringify(params)] ?? {}
		},
				getEthCall: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.EthCall[JSON.stringify(params)] ?? {}
		},
				getEstimateGas: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.EstimateGas[JSON.stringify(params)] ?? {}
		},
				getTraceTx: (state) => (params = { params: {}}) => {
					if (!(<any> params).query) {
						(<any> params).query=null
					}
			return state.TraceTx[JSON.stringify(params)] ?? {}
		},
				
		getTypeStructure: (state) => (type) => {
			return state._Structure[type].fields
		}
	},
	actions: {
		init({ dispatch, rootGetters }) {
			console.log('Vuex module: ethermint.evm.v1 initialized!')
			if (rootGetters['common/env/client']) {
				rootGetters['common/env/client'].on('newblock', () => {
					dispatch('StoreUpdate')
				})
			}
		},
		resetState({ commit }) {
			commit('RESET_STATE')
		},
		unsubscribe({ commit }, subscription) {
			commit('UNSUBSCRIBE', subscription)
		},
		async StoreUpdate({ state, dispatch }) {
			state._Subscriptions.forEach(async (subscription) => {
				try {
					await dispatch(subscription.action, subscription.payload)
				}catch(e) {
					throw new SpVuexError('Subscriptions: ' + e.message)
				}
			})
		},
		
		
		
		 		
		
		
		async QueryAccount({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryAccount( key.address)).data
				
					
				commit('QUERY', { query: 'Account', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryAccount', payload: { options: { all }, params: {...key},query }})
				return getters['getAccount']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryAccount', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryCosmosAccount({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryCosmosAccount( key.address)).data
				
					
				commit('QUERY', { query: 'CosmosAccount', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryCosmosAccount', payload: { options: { all }, params: {...key},query }})
				return getters['getCosmosAccount']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryCosmosAccount', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryValidatorAccount({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryValidatorAccount( key.cons_address)).data
				
					
				commit('QUERY', { query: 'ValidatorAccount', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryValidatorAccount', payload: { options: { all }, params: {...key},query }})
				return getters['getValidatorAccount']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryValidatorAccount', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryBalance({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryBalance( key.address)).data
				
					
				commit('QUERY', { query: 'Balance', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryBalance', payload: { options: { all }, params: {...key},query }})
				return getters['getBalance']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryBalance', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryStorage({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryStorage( key.address,  key.key)).data
				
					
				commit('QUERY', { query: 'Storage', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryStorage', payload: { options: { all }, params: {...key},query }})
				return getters['getStorage']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryStorage', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryCode({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryCode( key.address)).data
				
					
				commit('QUERY', { query: 'Code', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryCode', payload: { options: { all }, params: {...key},query }})
				return getters['getCode']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryCode', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryParams({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryParams()).data
				
					
				commit('QUERY', { query: 'Params', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryParams', payload: { options: { all }, params: {...key},query }})
				return getters['getParams']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryParams', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryEthCall({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryEthCall(query)).data
				
					
				while (all && (<any> value).pagination && (<any> value).pagination.nextKey!=null) {
					let next_values=(await queryClient.queryEthCall({...query, 'pagination.key':(<any> value).pagination.nextKey})).data
					value = mergeResults(value, next_values);
				}
				commit('QUERY', { query: 'EthCall', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryEthCall', payload: { options: { all }, params: {...key},query }})
				return getters['getEthCall']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryEthCall', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryEstimateGas({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryEstimateGas(query)).data
				
					
				while (all && (<any> value).pagination && (<any> value).pagination.nextKey!=null) {
					let next_values=(await queryClient.queryEstimateGas({...query, 'pagination.key':(<any> value).pagination.nextKey})).data
					value = mergeResults(value, next_values);
				}
				commit('QUERY', { query: 'EstimateGas', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryEstimateGas', payload: { options: { all }, params: {...key},query }})
				return getters['getEstimateGas']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryEstimateGas', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		
		
		 		
		
		
		async QueryTraceTx({ commit, rootGetters, getters }, { options: { subscribe, all} = { subscribe:false, all:false}, params: {...key}, query=null }) {
			try {
				const queryClient=await initQueryClient(rootGetters)
				let value= (await queryClient.queryTraceTx(query)).data
				
					
				while (all && (<any> value).pagination && (<any> value).pagination.nextKey!=null) {
					let next_values=(await queryClient.queryTraceTx({...query, 'pagination.key':(<any> value).pagination.nextKey})).data
					value = mergeResults(value, next_values);
				}
				commit('QUERY', { query: 'TraceTx', key: { params: {...key}, query}, value })
				if (subscribe) commit('SUBSCRIBE', { action: 'QueryTraceTx', payload: { options: { all }, params: {...key},query }})
				return getters['getTraceTx']( { params: {...key}, query}) ?? {}
			} catch (e) {
				throw new SpVuexError('QueryClient:QueryTraceTx', 'API Node Unavailable. Could not perform query: ' + e.message)
				
			}
		},
		
		
		async sendMsgEthereumTx({ rootGetters }, { value, fee = [], memo = '' }) {
			try {
				const txClient=await initTxClient(rootGetters)
				const msg = await txClient.msgEthereumTx(value)
				const result = await txClient.signAndBroadcast([msg], {fee: { amount: fee, 
	gas: "200000" }, memo})
				return result
			} catch (e) {
				if (e == MissingWalletError) {
					throw new SpVuexError('TxClient:MsgEthereumTx:Init', 'Could not initialize signing client. Wallet is required.')
				}else{
					throw new SpVuexError('TxClient:MsgEthereumTx:Send', 'Could not broadcast Tx: '+ e.message)
				}
			}
		},
		
		async MsgEthereumTx({ rootGetters }, { value }) {
			try {
				const txClient=await initTxClient(rootGetters)
				const msg = await txClient.msgEthereumTx(value)
				return msg
			} catch (e) {
				if (e == MissingWalletError) {
					throw new SpVuexError('TxClient:MsgEthereumTx:Init', 'Could not initialize signing client. Wallet is required.')
				}else{
					throw new SpVuexError('TxClient:MsgEthereumTx:Create', 'Could not create message: ' + e.message)
					
				}
			}
		},
		
	}
}

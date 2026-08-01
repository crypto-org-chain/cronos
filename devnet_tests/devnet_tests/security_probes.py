from dataclasses import dataclass

CRO_BRIDGE_ABI = [
    {
        "anonymous": False,
        "inputs": [
            {"indexed": False, "internalType": "address", "name": "sender", "type": "address"},
            {"indexed": False, "internalType": "string", "name": "recipient", "type": "string"},
            {"indexed": False, "internalType": "uint256", "name": "amount", "type": "uint256"},
        ],
        "name": "__CronosSendCroToIbc",
        "type": "event",
    },
    {
        "inputs": [{"internalType": "string", "name": "recipient", "type": "string"}],
        "name": "send_cro_to_crypto_org",
        "outputs": [],
        "stateMutability": "payable",
        "type": "function",
    },
]

CRO_BRIDGE_BYTECODE = "608060405234801561001057600080fd5b5061036d806100206000396000f3fe60806040526004361061001e5760003560e01c8063c41cc27014610023575b600080fd5b61003d600480360381019061003891906101d7565b61003f565b005b7ffbb552151d1a72b8da58707becbeaaf202cf9e83730579ce3184cf564f772859338234604051610072939291906102f9565b60405180910390a150565b6000604051905090565b600080fd5b600080fd5b600080fd5b600080fd5b6000601f19601f8301169050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6100e48261009b565b810181811067ffffffffffffffff82111715610103576101026100ac565b5b80604052505050565b600061011661007d565b905061012282826100db565b919050565b600067ffffffffffffffff821115610142576101416100ac565b5b61014b8261009b565b9050602081019050919050565b82818337600083830152505050565b600061017a61017584610127565b61010c565b90508281526020810184848401111561019657610195610096565b5b6101a1848285610158565b509392505050565b600082601f8301126101be576101bd610091565b5b81356101ce848260208601610167565b91505092915050565b6000602082840312156101ed576101ec610087565b5b600082013567ffffffffffffffff81111561020b5761020a61008c565b5b610217848285016101a9565b91505092915050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b600061024b82610220565b9050919050565b61025b81610240565b82525050565b600081519050919050565b600082825260208201905092915050565b60005b8381101561029b578082015181840152602081019050610280565b60008484015250505050565b60006102b282610261565b6102bc818561026c565b93506102cc81856020860161027d565b6102d58161009b565b840191505092915050565b6000819050919050565b6102f3816102e0565b82525050565b600060608201905061030e6000830186610252565b818103602083015261032081856102a7565b905061032f60408301846102ea565b94935050505056fea26469706673582212205510ee68b1589c8b8955a499986b96c53234a69bca16a2b9ff81ec9d527c6d6364736f6c63430008130033"


@dataclass
class BridgeRejectionResult:
    rejected: bool
    error: str | None = None
    # callTracer's top-level error for the reverted call, or None when the EVM
    # itself ran clean. See _traced_evm_error.
    evm_error: str | None = None
    receipt_logs: int | None = None


def _send_and_wait(w3, account, tx: dict):
    """Lets both signing-time and RPC-time failures raise so the caller can
    fold them into a single result."""
    signed = account.sign_transaction(tx)
    tx_hash = w3.eth.send_raw_transaction(signed.raw_transaction)
    return w3.eth.wait_for_transaction_receipt(tx_hash)


def _traced_evm_error(w3, tx_hash) -> str | None:
    """debug_traceTransaction replays the message through ApplyMessageWithConfig
    without ethermint's PostTxProcessing hooks (x/evm/keeper/grpc_query.go), so
    an allowlist rejection — which happens inside the hook — leaves no trace
    error, while a plain EVM revert or out-of-gas does. That difference is what
    separates the two identical-looking `status == 0` receipts."""
    rsp = w3.provider.make_request(
        "debug_traceTransaction", [w3.to_hex(tx_hash), {"tracer": "callTracer"}]
    )
    if "result" not in rsp:
        # A missing debug namespace is itself a problem worth failing on, not a
        # reason to fall back to the loose status-only check.
        return f"debug_traceTransaction unavailable: {rsp.get('error')}"
    return rsp["result"].get("error")


def send_unauthorized_cro_bridge_call(w3, account) -> BridgeRejectionResult:
    """Deploy a fresh, never-authorized CroBridge contract and call
    send_cro_to_crypto_org on it. SendCroToIbcHandler.Handle only allows
    contracts listed in CroBridgeContractAddresses (x/cronos/keeper/
    evmhandlers/send_cro_to_ibc.go); this contract is never registered, so the
    handler errors, ethermint sets VmError to "failed to execute post
    processing", clears the tx's logs and reverts the whole tx.

    `rejected` alone can't tell that apart from an ordinary EVM revert, so the
    result also carries the traced EVM error and the receipt's log count: the
    allowlist rejection is the one combination where the EVM ran clean yet the
    tx still failed with its logs dropped."""
    contract = w3.eth.contract(abi=CRO_BRIDGE_ABI, bytecode=CRO_BRIDGE_BYTECODE)

    try:
        nonce = w3.eth.get_transaction_count(account.address, "pending")
        common = {
            "chainId": w3.eth.chain_id,
            "from": account.address,
            "gasPrice": w3.eth.gas_price,
        }
        deploy_receipt = _send_and_wait(
            w3, account, contract.constructor().build_transaction({**common, "nonce": nonce})
        )
        if deploy_receipt.status != 1:
            return BridgeRejectionResult(rejected=False, error="CroBridge deployment failed")

        bridge = w3.eth.contract(address=deploy_receipt.contractAddress, abi=CRO_BRIDGE_ABI)
        call_receipt = _send_and_wait(
            w3,
            account,
            bridge.functions.send_cro_to_crypto_org("cro1somerecipient").build_transaction(
                # Explicit gas skips web3.py's implicit eth_estimateGas, which
                # would raise on the revert before we can read receipt.status.
                {**common, "nonce": nonce + 1, "value": 1, "gas": 200_000}
            ),
        )
        evm_error = _traced_evm_error(w3, call_receipt.transactionHash)
    except Exception as exc:  # noqa: BLE001
        return BridgeRejectionResult(rejected=False, error=str(exc))

    return BridgeRejectionResult(
        rejected=call_receipt.status == 0,
        evm_error=evm_error,
        receipt_logs=len(call_receipt.logs),
    )

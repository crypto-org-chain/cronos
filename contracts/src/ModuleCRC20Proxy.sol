pragma solidity ^0.6.8;

import "ds-math/math.sol";
import "./ModuleCRC20.sol";

contract ModuleCRC20Proxy is DSMath {
    // sha256('cronos-evm')[:20]
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
    uint256 private MAX_UINT = 2**256 - 1;
    ModuleCRC20 crc20Contract;

    event __CronosSendToEvmChain(address indexed sender, address indexed recipient, uint256 indexed chain_id, uint256 amount, uint256 bridge_fee, bytes extraData);
    event __CronosCancelSendToEvmChain(address indexed sender, uint256 id);

    constructor(address crc20Contract_) public {
        crc20Contract = ModuleCRC20(crc20Contract_);
    }


    /**
        Internal functions to be called by cronos module.

        In the proxy contract, we don't mint but transfer asset to the destination since the proxy account
        should be initialized with MAX_UINT tokens
    **/
    function mint_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        crc20Contract.transfer(addr, amount);
    }

    /**
        Evm hooks functions
    **/

    // send to another chain through gravity bridge
    function send_to_evm_chain(address recipient, uint amount, uint chain_id, uint bridge_fee, bytes calldata extraData) external {
        // transfer back the token to the proxy account
        crc20Contract.transferFrom(msg.sender, address(this), add(amount,bridge_fee));
        emit __CronosSendToEvmChain(msg.sender, recipient, chain_id, amount, bridge_fee, extraData);
    }

    // cancel a send to chain transaction considering if it hasnt been batched yet.
    function cancel_send_to_evm_chain(uint256 id) external {
        emit __CronosCancelSendToEvmChain(msg.sender, id);
    }

    /**
     * @dev Returns the number of tokens currently in circulation
     * This value should reflect the number of token which are locked in the gravity bridge
     * corresponding to the crc20 token.
     * Note that it is not an accurate number but only an estimation as they may be latency between the ethereum network
     * and cronos network.
	 *
	 */
    function totalSupply() public view returns (uint256) {
        return MAX_UINT - crc20Contract.balanceOf(address(this));
    }
}
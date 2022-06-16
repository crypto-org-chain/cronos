pragma solidity ^0.6.8;

import "./ModuleCRC20.sol";

contract ModuleCRC21 is ModuleCRC20  {

    event __CronosSendToChain(address sender, address recipient, uint256 amount, uint256 bridge_fee, uint256 chain_id);
    event __CronosCancelSendToChain(address sender, uint256 id);

    constructor(string memory denom_, uint8 decimals_) ModuleCRC20(denom_, decimals_) public {
        decimals = decimals_;
        denom = denom_;
    }

    // make unsafe_burn internal
    function unsafe_burn_internal(address addr, uint amount) internal {
        // Deduct user's balance without approval
        require(balanceOf[addr] >= amount, "ds-token-insufficient-balance");
        balanceOf[addr] = sub(balanceOf[addr], amount);
        totalSupply = sub(totalSupply, amount);
        emit Burn(addr, amount);
    }

    // send to another chain through gravity bridge
    function send_to_chain(address recipient, uint amount, uint bridge_fee, uint chain_id) external {
        unsafe_burn_internal(msg.sender, add(amount, bridge_fee));
        emit __CronosSendToChain(msg.sender, recipient, amount, bridge_fee, chain_id);
    }

    // cancel a send to chain transaction considering if it hasnt been batched yet.
    function cancel_send_to_chain(uint256 id) external {
        emit __CronosCancelSendToChain(msg.sender, id);
    }
}
pragma solidity ^0.6.8;

import "./ModuleCRC20.sol";

contract ModuleCRC21 is ModuleCRC20  {

    event __CronosSendToEthereum(address sender, address recipient, uint256 amount, uint256 bridge_fee);
    event __CronosCancelSendToEthereum(address sender, uint256 id);

    constructor(string memory denom_, uint8 decimals_) ModuleCRC20(denom_, decimals_) public {
        decimals = decimals_;
        denom = denom_;
    }

    // make unsafe_burn internal
    function unsafe_burn_v2(address addr, uint amount) internal {
        // Deduct user's balance without approval
        require(balanceOf[addr] >= amount, "ds-token-insufficient-balance");
        balanceOf[addr] = sub(balanceOf[addr], amount);
        totalSupply = sub(totalSupply, amount);
        emit Burn(addr, amount);
    }

    // send to ethereum through gravity bridge
    function send_to_ethereum_v2(address recipient, uint amount, uint bridge_fee) external {
        unsafe_burn_v2(msg.sender, add(amount, bridge_fee));
        emit __CronosSendToEthereum(msg.sender, recipient, amount, bridge_fee);
    }

    // cancel a send to ethereum transaction considering if it hasnt been batched yet.
    function cancel_send_to_ethereum(uint256 id) external {
        emit __CronosCancelSendToEthereum(msg.sender, id);
    }
}
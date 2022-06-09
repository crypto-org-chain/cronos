pragma solidity ^0.6.8;

import "ds-token/token.sol";

contract ModuleCRC20 is DSToken  {
    // sha256('cronos-evm')[:20]
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
    string denom;

    event __CronosSendToEthereum(address recipient, uint256 amount, uint256 bridge_fee);
    event __CronosSendToIbc(address sender, string recipient, uint256 amount);

    constructor(string memory denom_, uint8 decimals_) DSToken(denom_) public {
        decimals = decimals_;
        denom = denom_;
    }

    // unsafe_burn burn tokens without user's approval and authentication, used internally
    function unsafe_burn(address addr, uint amount) private {
        // Deduct user's balance without approval
        require(balanceOf[addr] >= amount, "ds-token-insufficient-balance");
        balanceOf[addr] = sub(balanceOf[addr], amount);
        totalSupply = sub(totalSupply, amount);
        emit Burn(addr, amount);
    }

    function native_denom() public view returns (string memory) {
        return denom;
    }

    function mint_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        mint(addr, amount);
    }

    function burn_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        unsafe_burn(addr, amount);
    }

    // send to ethereum through gravity bridge
    function send_to_ethereum(address recipient, uint amount, uint bridge_fee) external {
        unsafe_burn(msg.sender, add(amount, bridge_fee));
        emit __CronosSendToEthereum(recipient, amount, bridge_fee);
    }

    // send an "amount" of the contract token to recipient through IBC
    function send_to_ibc(string memory recipient, uint amount) public {
        unsafe_burn(msg.sender, amount);
        emit __CronosSendToIbc(msg.sender, recipient, amount);
    }
}

pragma solidity ^0.6.11;

import "ds-token/token.sol";

contract ModuleCRC20 is DSToken  {
    // sha256('cronos')[:20]
    address constant module_address = 0x6526B48f897f6e08067dF00A1821d783cbC2af8b;

    constructor(string memory denom, uint8 decimals_) DSToken(denom) public {
        decimals = decimals_;
    }

    function mint_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        mint(addr, amount);
    }

    function burn_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        // Deduct user's balance without approval
        balanceOf[addr] = sub(balanceOf[addr], amount);
        totalSupply = sub(totalSupply, amount);
        emit Burn(addr, amount);
    }
}

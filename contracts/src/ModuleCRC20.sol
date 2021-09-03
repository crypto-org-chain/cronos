pragma solidity ^0.6.11;

import "ds-token/token.sol";

contract ModuleCRC20 is DSToken  {
    // sha256('cronos')[:20]
    address constant module_address = 0x6526B48f897f6e08067dF00A1821d783cbC2af8b;
    string denom;

    event __CronosEthereumTransfer(address recipient, uint256 amount, uint256 fee_amount);

    constructor(string memory denom_, uint8 decimals_) DSToken(denom) public {
        decimals = decimals_;
        denom = denom_;
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
        // Deduct user's balance without approval
        require(balanceOf[addr] >= amount, "ds-token-insufficient-balance");
        balanceOf[addr] = sub(balanceOf[addr], amount);
        totalSupply = sub(totalSupply, amount);
        emit Burn(addr, amount);
    }

    // send to ethereum through gravity bridge
    function send_to_ethereum(address recipient, uint amount, uint fee_amount) public {
        burn(msg.sender, add(amount, fee_amount));
        emit __CronosEthereumTransfer(recipient, amount, fee_amount);
    }
}

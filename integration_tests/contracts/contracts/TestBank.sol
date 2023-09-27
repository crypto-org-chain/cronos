// SPDX-License-Identifier: MIT
pragma solidity >0.6.6;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {IBankModule} from "./src/Bank.sol";

contract TestBank is ERC20 {
    address constant bankContract = 0x0000000000000000000000000000000000000064;
    IBankModule bank = IBankModule(bankContract);

    constructor() public ERC20("Bitcoin MAX", "MAX") {
		_mint(msg.sender, 100000000000000000000000000);
	}

    function moveToNative(uint256 amount) public returns (bool) {
        _burn(msg.sender, amount);
        return bank.mint(msg.sender, amount);
    }

    function moveFromNative(uint256 amount) public returns (bool) {
        bool result = bank.burn(msg.sender, amount);
        require(result, "native call");
        _mint(msg.sender, amount);
        return result;
    }

    function nativeBalanceOf(address addr) public returns (uint256) {
        return bank.balanceOf(address(this), addr);
    }

    function moveToNativeRevert(uint256 amount) public {
        moveToNative(amount);
        revert("test");
    }

    function nativeTransfer(address recipient, uint256 amount) public returns (bool) {
        _transfer(msg.sender, recipient, amount);
        return bank.transfer(msg.sender, recipient, amount);
    }
}
pragma solidity ^0.6.6;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestERC20A is ERC20 {
	event __CronosNativeTransfer(address recipient, uint256 amount);
	event __CronosEthereumTransfer(address recipient, uint256 amount, uint256 bridge_fee);

	constructor() public ERC20("Bitcoin MAX", "MAX") {
		_mint(msg.sender, 100000000000000000000000000);
	}

	function test_native_transfer(uint amount) public {
		emit __CronosNativeTransfer(msg.sender, amount);
	}
}

pragma solidity ^0.6.6;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestERC20A is ERC20 {
	constructor() public ERC20("Bitcoin MAX", "MAX") {
		_mint(msg.sender, 100000000000000000000000000);
	}
}

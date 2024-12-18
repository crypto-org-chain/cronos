pragma solidity 0.8.21;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// An utility erc20 contract that has a fancy method
contract TestERC20Owner is ERC20 {
	constructor(address owner) public ERC20("Fancy", "FNY") {
		_mint(owner, 100000000000000000000000000);
	}
}


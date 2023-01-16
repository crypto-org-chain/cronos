pragma solidity 0.8.10;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestMaxValue is ERC20 {
	uint256 public count;

	constructor() public ERC20("MAX", "MAX") {
		_mint(msg.sender, 115792089237316195423570985008687907853269984665640564039457584007913129639935);
	}

	function transferFrom(
		address from,
		address to,
		uint256 amount
	) public virtual override returns (bool) {
		count = count + 1;
		return true;
	}

	function balanceOf(address account) public view virtual override returns (uint256) {
		if (count % 2 == 0) {
			return 0;
		} else {
			return 115792089237316195423570985008687907853269984665640564039457584007913129639935;
		}
	}
}
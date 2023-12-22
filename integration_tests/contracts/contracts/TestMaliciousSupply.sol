pragma solidity 0.8.21;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// This contract is for testing a case on the gravity bridge when attacker try to send an amount
// that exceed the limit allowed ( max(uint256) )
contract TestMaliciousSupply is ERC20 {
	uint256 public count;

	constructor() public ERC20("MAX", "MAX") {
		_mint(msg.sender, 115792089237316195423570985008687907853269984665640564039457584007913129639935);
	}

	// transferFrom just increment the counter so that in case the count is even
	// SendToCosmos will record an event that simulate a transfer of token amount equal to max(uint256)
	// since it records the difference of balance of the gravity contract after the transferFrom
	function transferFrom(
		address from,
		address to,
		uint256 amount
	) public virtual override returns (bool) {
		count = count + 1;
		return true;
	}

	// if the count is even, then balanceOf will return zero
	// if the count is odd, then it return max uint256
	function balanceOf(address account) public view virtual override returns (uint256) {
		if (count % 2 == 0) {
			return 0;
		} else {
			return 115792089237316195423570985008687907853269984665640564039457584007913129639935;
		}
	}
}
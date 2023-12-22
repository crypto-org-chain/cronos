pragma solidity 0.8.21;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestBlackListERC20 is ERC20 {
	address public blacklisted;

	constructor(address blacklistAddresses) public ERC20("USDC", "USDC") {
		_mint(msg.sender, 100000000000000000000000000);
		blacklisted = blacklistAddresses;
	}

	function transfer(address to, uint256 amount) public override returns (bool) {
		require(false == isBlacklisted(to), "address is blacklisted");
		return super.transfer(to, amount);
	}

	function isBlacklisted(address _target) private returns (bool) {
		if (blacklisted == _target) {
			return true;
		}
		return false;
	}
}

pragma solidity 0.8.21;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// An utility erc20 contract that has a fancy method
contract TestCRC20 is ERC20 {
	event __CronosSendToAccount(address recipient, uint256 amount);
	event __CronosSendToEthereum(address recipient, uint256 amount, uint256 bridge_fee);
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
    address public owner;

	constructor() public ERC20("Test", "TEST") {
		_mint(msg.sender, 100000000000000000000000000);
        owner = msg.sender;
	}

    function mint_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        _mint(addr, amount);
    }

    // send to ethereum through gravity bridge
    function send_to_ethereum(address recipient, uint256 amount, uint256 bridge_fee) external {
        uint256 total = amount + bridge_fee;
        require(total >= amount, "safe-math-add-overflow");
        _burn(msg.sender, total);
        emit __CronosSendToEthereum(recipient, amount, bridge_fee);
    }

    function mint(address account, uint256 amount) external {
        // Should be protected by only owner
        _mint(account, amount);
    }

    function burn(uint256 amount) external {
        _burn(msg.sender, amount);
    }

}

pragma solidity 0.8.10;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// An utility erc20 contract that has a fancy method
contract TestERC20Utility is ERC20 {
	event __CronosSendToAccount(address recipient, uint256 amount);
	event __CronosSendToEthereum(address recipient, uint256 amount, uint256 bridge_fee);
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;

	constructor() public ERC20("Fancy", "FNY") {
		_mint(msg.sender, 100000000000000000000000000);
	}

    function fancy() public view returns (uint256) {
        return 42;
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

    function test_log0() public {
        bytes32 data = "hello world";
        assembly {
            let p := mload(0x20)
            mstore(p, data)
            log0(p, 0x20)
        }
    }
}

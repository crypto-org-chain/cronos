pragma solidity 0.8.10;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestERC21Source is ERC20 {
	// sha256('cronos-evm')[:20]
	address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
	string denom;
	bool isSource;

	event __CronosSendToIbc(address sender, string recipient, uint256 amount);
	event __CronosSendToChain(address sender, address recipient, uint256 amount, uint256 bridge_fee, uint256 chain_id);
	event __CronosCancelSendToChain(address sender, uint256 id);

	constructor() ERC20("Doggo", "DOG") public {
		denom = "Doggo";
		isSource = true;
		_mint(msg.sender, 100000000000000000000000000);
	}

	/**
        views
    **/
	function native_denom() public view returns (string memory) {
		return denom;
	}

	function is_source() public view returns (bool) {
		return isSource;
	}


	/**
        Internal functions to be called by cronos module
    **/
	function mint_by_cronos_module(address addr, uint amount) public {
		require(msg.sender == module_address);
		_mint(addr, amount);
	}

	function burn_by_cronos_module(address addr, uint amount) public {
		require(msg.sender == module_address);
		unsafe_burn(addr, amount);
	}

	function transfer_by_cronos_module(address addr, uint amount) public {
		require(msg.sender == module_address);
		unsafe_transfer(addr, module_address, amount);
	}

	function transfer_from_cronos_module(address addr, uint amount) public {
		require(msg.sender == module_address);
		transfer(addr, amount);
	}

	/**
        Evm hooks functions
    **/

	// send an "amount" of the contract token to recipient through IBC
	function send_to_ibc(string memory recipient, uint amount) public {
		if (isSource) {
			transfer(module_address, amount);
		} else {
			unsafe_burn(msg.sender, amount);
		}
		emit __CronosSendToIbc(msg.sender, recipient, amount);
	}

	// send to another chain through gravity bridge
	function send_to_chain(address recipient, uint amount, uint bridge_fee, uint chain_id) external {
		if (isSource) {
			transfer(module_address, amount + bridge_fee);
		} else {
			unsafe_burn(msg.sender, amount + bridge_fee);
		}
		emit __CronosSendToChain(msg.sender, recipient, amount, bridge_fee, chain_id);
	}

	// cancel a send to chain transaction considering if it hasnt been batched yet.
	function cancel_send_to_chain(uint256 id) external {
		emit __CronosCancelSendToChain(msg.sender, id);
	}

	/**
        Internal functions
    **/

	// unsafe_burn burn tokens without user's approval and authentication, used internally
	function unsafe_burn(address addr, uint amount) internal {
		_burn(addr, amount);
	}

	// unsafe_transfer transfer tokens without user's approval and authentication, used internally
	function unsafe_transfer(address src, address dst, uint amount) internal {
		_transfer(src, dst, amount);
	}
}

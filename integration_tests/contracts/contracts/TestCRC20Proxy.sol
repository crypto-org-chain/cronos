pragma solidity ^0.8.8;

import "./TestCRC20.sol";

contract TestCRC20Proxy {
    // sha256('cronos-evm')[:20]
    address constant module_address = 0x89A7EF2F08B1c018D5Cc88836249b84Dd5392905;
    TestCRC20 crc20Contract;
    bool isSource;

    event __CronosSendToIbc(address indexed sender, uint256 indexed channel_id, string recipient, uint256 amount, bytes extraData);
    event __CronosSendToEvmChain(address indexed sender, address indexed recipient, uint256 indexed chain_id, uint256 amount, uint256 bridge_fee, bytes extraData);
    event __CronosCancelSendToEvmChain(address indexed sender, uint256 id);

    /**
        Can be instantiated only by crc20 contract owner
    **/
    constructor(address crc20Contract_, bool isSource_) public {
        crc20Contract = TestCRC20(crc20Contract_);
        isSource = isSource_;
    }

    /**
        views
    **/
    function crc20() public view returns (address) {
        return address(crc20Contract);
    }

    function is_source() public view returns (bool) {
        return isSource;
    }


    /**
        Internal functions to be called by cronos module.
    **/
    function mint_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        crc20Contract.mint(addr, amount);
    }

    function burn_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        crc20_burn(addr, amount);
    }

    function transfer_by_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        crc20Contract.transferFrom(addr, module_address, amount);
    }

    function transfer_from_cronos_module(address addr, uint amount) public {
        require(msg.sender == module_address);
        crc20Contract.transfer(addr, amount);
    }


    /**
        Evm hooks functions
    **/

    // send to another chain through gravity bridge, require approval for the burn.
    function send_to_evm_chain(address recipient, uint amount, uint chain_id, uint bridge_fee, bytes calldata extraData) external {
        // transfer back the token to the proxy account
        if (isSource) {
            crc20Contract.transferFrom(msg.sender, address(this), amount + bridge_fee);
        } else {
            crc20_burn(msg.sender, amount + bridge_fee);
        }
        emit __CronosSendToEvmChain(msg.sender, recipient, chain_id, amount, bridge_fee, extraData);
    }

    // cancel a send to chain transaction considering if it hasn't been batched yet.
    function cancel_send_to_evm_chain(uint256 id) external {
        emit __CronosCancelSendToEvmChain(msg.sender, id);
    }

    // send an "amount" of the contract token to recipient through IBC
    function send_to_ibc(string memory recipient, uint amount, uint channel_id, bytes memory extraData) public {
        if (isSource) {
            crc20Contract.transferFrom(msg.sender, address(this), amount);
        } else {
            crc20_burn(msg.sender, amount);
        }
        emit __CronosSendToIbc(msg.sender, channel_id, recipient, amount, extraData);
    }

    /**
        Internal functions
    **/

    // burn the token on behalf of the user. requires approval
    function crc20_burn(address addr, uint amount) internal {
        crc20Contract.transferFrom(addr, address(this), amount);
        crc20Contract.burn(amount);
    }
}
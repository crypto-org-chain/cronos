pragma solidity 0.6.11;

import "./ModuleCRC20.sol";

contract CronosCRC20 is ModuleCRC20 {
    constructor (
        string memory _name,
        string memory _denom,
        uint8 _decimal
    ) ModuleCRC20(_denom, _decimal) public {
        setName(_name);
    }
}

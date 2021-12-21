// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

contract TestRevert {
    uint256 state;
    constructor() public {
        state = 0;
    }
    function transfer(uint256 value) public payable {
        uint256 minimal = 5 * 10 ** 18;
        state = value;
        if(state < minimal) {
            revert("Not enough tokens to transfer");
        }
    }
    function query() view public returns (uint256) {
        return state;
    }
}

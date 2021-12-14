// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

contract TestRevert {
    constructor() public {}
    function transfer(uint256 value) public payable {
        uint256 minimal = 5 * 10 ** 18;
        if(value < minimal) {
            revert("Not enough tokens to transfer");
        }
    }
}

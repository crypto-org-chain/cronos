// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

contract TestAccessList {
    uint256 public slotA;             // slot 0
    uint256 public slotB;             // slot 1
    mapping(address => uint256) public balances; // slot 2 + keccak(addr,2)

    function touchSlots(uint256 a, uint256 b) public {
        slotA = a;
        slotB = b;
        balances[msg.sender] = a + b;
    }
}
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

contract Counter {
    uint public count;

    function increase() public {
        count++;
    }

    function decrease() public {
        count--;
    }
}

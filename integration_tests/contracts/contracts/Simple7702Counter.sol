// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

contract Simple7702Counter {
    uint public count;

    function increase() public {
        count++;
    }

    function decrease() public {
        count--;
    }

    receive() external payable { }
    fallback() external payable { }
}

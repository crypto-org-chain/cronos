// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

contract TestRelayer {
    address constant relayer = 0x0000000000000000000000000000000000000065;

    function batchCall(bytes[] memory payloads) public {
        for (uint256 i = 0; i < payloads.length; i++) {
            (bool success,) = relayer.call(payloads[i]);
            require(success);
        }
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

contract TestRelayer {
    address constant relayer = 0x0000000000000000000000000000000000000065;

    function batchCall(bytes[][] memory batches) public {
        for (uint256 i = 0; i < batches.length; i++) {
            bytes memory payload = abi.encode(batches[i]);
            (bool success, ) = relayer.call(payload);
            require(success, "Relayer call failed");
        }
    }
}

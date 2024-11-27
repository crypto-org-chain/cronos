// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

import {ILLamaModule} from "./src/LLama.sol";

contract TestLLama {
    address constant llamaContract = 0x0000000000000000000000000000000000000067;
    ILLamaModule llama = ILLamaModule(llamaContract);

    function inference(string calldata prompt, int64 seed, int32 steps) public returns (string memory) {
        return llama.inference(prompt, seed, steps);
    }
}
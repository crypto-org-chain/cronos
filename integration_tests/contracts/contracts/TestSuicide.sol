// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

contract Inner {
    function destroy() public {
        selfdestruct(payable(msg.sender));
    }
}

contract Outer {
    function codesize_after_suicide(Inner inner) public {
        address addr = address(inner);
        inner.destroy();
        uint _size = 0;
        assembly {
            _size := extcodesize(addr)
        }
        require(_size > 0);
    }
}

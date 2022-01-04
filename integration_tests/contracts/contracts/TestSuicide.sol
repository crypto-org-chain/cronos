// SPDX-License-Identifier: MIT
pragma solidity ^0.6.6;

contract Destroyee {
    function destroy() public {
        selfdestruct(payable(msg.sender));
    }
}

contract Destroyer {
    function check_codesize_after_suicide(Destroyee destroyee) public {
        address addr = address(destroyee);
        destroyee.destroy();
        uint _size = 0;
        assembly {
            _size := extcodesize(addr)
        }
        require(_size > 0);
    }
}

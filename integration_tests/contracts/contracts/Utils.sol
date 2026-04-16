pragma solidity ^0.8.20;

contract Utils {
    function getCodeHash(address _account) public view returns (bytes32) {
        bytes32 codeHash;
        assembly {
            codeHash := extcodehash(_account)
        }
        return codeHash;
    }
}

pragma solidity ^0.6.8;

contract ModuleCRC20ProxyAuthority {
    address proxyAddress;

    constructor(address _proxyAddress) public {
        proxyAddress = _proxyAddress;
    }

    function canCall(
        address src, address dst, bytes4 sig
    ) public view returns (bool) {
        if (src == proxyAddress) {
            return true;
        }

        return false;
    }
}
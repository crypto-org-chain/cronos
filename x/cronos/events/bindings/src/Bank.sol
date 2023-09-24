// SPDX-License-Identifier: MIT
pragma solidity ^0.8.4;

interface IBankModule {
    function mint(address,uint256) external payable returns (bool);
    function balanceOf(address,address) external view returns (uint256);
    function burn(address,uint256) external payable returns (bool);
    function transfer(address,address,uint256) external payable returns (bool);
}

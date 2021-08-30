pragma solidity ^0.6.11;

import "ds-test/test.sol";

import "./ModuleERC20.sol";

contract ModuleERC20Test is DSTest {
    ModuleERC20 token;

    function setUp() public {
        token = new ModuleERC20("gravity0x0", 0);
    }

    function test_basic_sanity() public {
        assertEq(uint(token.decimals()), uint(0));
    }

    function testFail_mint_by_native() public {
        token.mint_by_native(0x208AE63c976d145AB328afdcE251c7051D8E452D, 100);
    }

    function testFail_burn_by_native() public {
        token.burn_by_native(0x208AE63c976d145AB328afdcE251c7051D8E452D, 100);
    }
}

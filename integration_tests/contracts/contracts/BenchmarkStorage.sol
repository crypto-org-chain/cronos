pragma solidity 0.8.10;

contract BenchmarkStorage {
    uint seed;
    mapping(uint => uint) state;
    function random(uint i) private view returns (uint) {
        return uint(keccak256(abi.encodePacked(i, seed)));
    }
    function batch_set(uint _seed, uint n, uint range) public {
        seed = _seed;
        for (uint i=0; i< n; i++) {
            state[random(i) % range] = random(i+i);
        }
    }
}

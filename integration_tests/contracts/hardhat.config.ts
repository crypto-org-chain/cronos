import "hardhat-typechain";

module.exports = {
  solidity: {
    compilers: [
      {
        version: "0.8.10",
        settings: {
          optimizer: {
            enabled: true
          }
        }
      },
      {
        version: "0.6.6",
        settings: {
          optimizer: {
            enabled: true
          }
        }
      },
    ],
  },
  typechain: {
    outDir: "typechain",
    target: "ethers-v5",
    runOnCompile: true
  }
};

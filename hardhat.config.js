require("hardhat-abi-exporter");

/**
 * @type import('hardhat/config').HardhatUserConfig
 */
module.exports = {
  solidity: {
    compilers: [
      {
        version: "0.8.0",
        settings: {
          optimizer: {
            enabled: true,
            runs: 1000
          },
        },
      }
    ]
  },
  paths: {
    sources: "./solidity/contracts",
    cache: "./solidity/build/cache",
    artifacts: "./solidity/build/artifacts",
  },
  abiExporter: {
    path: "./solidity/build/abi",
    clear: true,
    flat: true,
    spacing: 2,
  }
};

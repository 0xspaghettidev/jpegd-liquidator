package solidity

import _ "embed"

//go:embed build/abi/INFTVault.json
var NFTVault []byte

//go:embed build/abi/PunkLiquidator.json
var Liquidator []byte

//go:embed build/abi/AggregatorInterface.json
var Oracle []byte

package liquidator

import (
	"context"
	"errors"
	"math/big"

	ethUtils "github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"

	"github.com/0xspaghettidev/jpegd-liquidator/arguments"
	"github.com/0xspaghettidev/jpegd-liquidator/solidity"
	"github.com/0xspaghettidev/jpegd-liquidator/utils"
)

type Cmd struct {
	arguments.KeystoreConfig
	arguments.ClientConfig
	VaultAddress      string     `args required name:"vaultAddress" help:"The address of the NFTVault contract" type:"string"`
	LiquidatorAddress string     `args required name:"liquidatorAddress" help:"The address of the Liquidator contract" type:"string"`
	Oracles           []string   `args required name:"oracles" help:"The addresses of the chainlink oracles used by the vault" type:"string"`
	MaxGasPrice       *big.Float `args required name:"maxGasPrice" help:"max gas price to use" type:"BigFloat"`
}

func (c *Cmd) Run() error {
	glog.Info("Decrypting keystore...")
	signer, err := c.KeystoreConfig.Apply()
	if err != nil {
		return err
	}

	glog.Info("Parsing client config...")
	client, err := c.ClientConfig.Apply()
	if err != nil {
		return err
	}

	nftVault, err := utils.NewContract(ethUtils.HexToAddress(c.VaultAddress), solidity.NFTVault, client)
	if err != nil {
		return err
	}

	liquidatorContract, err := utils.NewContract(ethUtils.HexToAddress(c.LiquidatorAddress), solidity.Liquidator, client)
	if err != nil {
		return err
	}

	oracleContracts := make([]*utils.Contract, len(c.Oracles))

	for i, a := range c.Oracles {
		oracleContracts[i], err = utils.NewContract(ethUtils.HexToAddress(a), solidity.Oracle, client)
		if err != nil {
			return err
		}
	}

	gwei, ok := new(big.Float).SetString(new(big.Int).Exp(big.NewInt(10), big.NewInt(9), nil).String())
	if !ok {
		return errors.New("error while converting eth amount")
	}

	maxGasPrice, ok := new(big.Int).SetString(new(big.Float).Mul(c.MaxGasPrice, gwei).Text('f', 0), 10)
	if !ok {
		return errors.New("error while converting eth amount")
	}

	liquidator, err := NewLiquidator(context.Background(), signer, client, nftVault, liquidatorContract, oracleContracts, big.NewInt(28158989), maxGasPrice)
	if err != nil {
		return err
	}
	return liquidator.Start(context.Background())
}

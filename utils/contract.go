package utils

import (
	"context"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type ABI struct {
	*abi.ABI
}

type Contract struct {
	Address common.Address
	ABI     *ABI
	Client  *ethclient.Client
	stubTx  *ethereum.CallMsg
}

func NewContract(address common.Address, ABIjson []byte, client *ethclient.Client) (*Contract, error) {
	stubTx := &ethereum.CallMsg{
		From: address,
		To:   &address,
		Gas:  0,
	}

	abiParsed := &ABI{&abi.ABI{}}
	err := abiParsed.UnmarshalJSON(ABIjson)

	if err != nil {
		return nil, err
	}

	return &Contract{Address: address, ABI: abiParsed, Client: client, stubTx: stubTx}, nil
}

func (c *Contract) Call(ctx context.Context, method string, args ...interface{}) ([]byte, error) {
	res, err := c.ABI.Pack(method, args...)

	if err != nil {
		return nil, err
	}

	clonedTx := *c.stubTx
	clonedTx.Data = res

	return c.Client.CallContract(ctx, clonedTx, nil)
}

func (c *Contract) EstimateGas(ctx context.Context, data []byte) (uint64, error) {
	clonedTx := *c.stubTx
	clonedTx.Data = data

	return c.Client.EstimateGas(ctx, clonedTx)
}

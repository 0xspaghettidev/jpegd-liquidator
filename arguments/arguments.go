package arguments

import (
	"context"

	"github.com/0xspaghettidev/jpegd-liquidator/common"
	"github.com/0xspaghettidev/jpegd-liquidator/keystore"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type KeystoreConfig struct {
	KeystorePath  string `args optional name:"keystorePath" help:"Path to the keystore directory" type:"path" default:"./keystores"`
	WalletAddress string `args required name:"walletAddress" help:"The wallet address to use" type:"string"`
}

func (c *KeystoreConfig) Apply() (*common.Signer, error) {
	return keystore.Decrypt(c.WalletAddress, c.KeystorePath)
}

type ClientConfig struct {
	Url string `args required name:"rpcUrl" help:"URL of the ethereum rpc server" type:"string"`
}

func (c *ClientConfig) Apply() (*ethclient.Client, error) {
	internal, err := rpc.DialContext(context.Background(), c.Url)

	if err != nil {
		return nil, err
	}

	return ethclient.NewClient(internal), nil
}

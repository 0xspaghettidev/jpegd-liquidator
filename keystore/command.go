package keystore

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
)

type Cmd struct {
	OutDir string `args optional name:"outDir" help:"The output file path" type:"path" default:"./keystores"`
}

//creates a password protected keystore
func (c *Cmd) Run() error {
	//take private key and password from stdin
	keyHex, password, err := keystoreDataFromStdIn()

	if err != nil {
		return err
	}

	keyBytes, err := hex.DecodeString(keyHex)

	if err != nil {
		return err
	}

	keyEcdsa, err := crypto.ToECDSA(keyBytes)

	if err != nil {
		return err
	}

	ks := keystore.NewKeyStore(c.OutDir, keystore.StandardScryptN, keystore.StandardScryptP)
	acc, err := ks.ImportECDSA(keyEcdsa, password)

	if err != nil {
		return err
	}

	fmt.Println("Account " + acc.Address.String() + " imported")

	return nil
}

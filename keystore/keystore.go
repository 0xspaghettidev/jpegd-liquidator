package keystore

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	utils "github.com/ethereum/go-ethereum/common"
	"golang.org/x/term"

	"github.com/0xspaghettidev/jpegd-liquidator/common"
)

func Decrypt(addressHex string, path string) (*common.Signer, error) {
	password, err := passwordFromStdIn()

	if err != nil {
		return nil, err
	}

	ks := keystore.NewKeyStore(path, keystore.StandardScryptN, keystore.StandardScryptP)

	address := utils.HexToAddress(addressHex)

	if !ks.HasAddress(address) {
		return nil, errors.New("cannot find address in keystore")
	}

	accs := ks.Accounts()

	for i := 0; i < len(accs); i++ {
		if accs[i].Address == address {
			err := ks.Unlock(accs[i], password)
			if err != nil {
				return nil, err
			}
			return common.NewSigner(accs[i], ks), nil
		}
	}

	return nil, errors.New("cannot find address in keystore")
}

func keystoreDataFromStdIn() (string, string, error) {
	fmt.Println("Enter private key: ")
	byteKey, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}

	pass, err := passwordFromStdIn()

	if err != nil {
		return "", "", err
	}

	return string(byteKey), pass, nil
}

func passwordFromStdIn() (string, error) {
	fmt.Println("Enter encryption password")
	bytePass, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}

	return string(bytePass), err
}

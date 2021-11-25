package common

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/core/types"
)

//helper struct that allows tx and hash signing
type Signer struct {
	accounts.Account
	k *keystore.KeyStore
}

func NewSigner(account accounts.Account, keystore *keystore.KeyStore) *Signer {
	return &Signer{Account: account, k: keystore}
}

func (s *Signer) SignTx(tx *types.Transaction, chainId *big.Int) (*types.Transaction, error) {
	return s.k.SignTx(s.Account, tx, chainId)
}

func (s *Signer) SignHash(hash []byte) ([]byte, error) {
	return s.k.SignHash(s.Account, hash)
}

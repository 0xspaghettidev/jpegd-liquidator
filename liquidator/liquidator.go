package liquidator

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/0xspaghettidev/jpegd-liquidator/common"
	"github.com/0xspaghettidev/jpegd-liquidator/utils"
	"github.com/ethereum/go-ethereum"
	ethUtils "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
)

type Liquidator struct {
	signer         *common.Signer
	client         *ethclient.Client
	nftVault       *utils.Contract
	liquidator     *utils.Contract
	oracles        []*utils.Contract
	errorChan      chan error
	chainId        *big.Int
	maxGasPrice    *big.Int
	eip1559Support bool
}

var answerUpdatedTopic = crypto.Keccak256Hash([]byte("AnswerUpdated(int256,uint256,uint256)"))

//creates a new instance of the liquidator bot
func NewLiquidator(ctx context.Context, signer *common.Signer, client *ethclient.Client, nftVault *utils.Contract,
	liquidator *utils.Contract, oracles []*utils.Contract, maxGasPrice *big.Int) (*Liquidator, error) {

	chainId, err := client.ChainID(ctx)
	if err != nil {
		return nil, err
	}

	var supportsEIP1559 bool
	_, err = client.SuggestGasTipCap(ctx)
	if err == nil {
		supportsEIP1559 = true
	}

	return &Liquidator{
		signer:         signer,
		client:         client,
		nftVault:       nftVault,
		liquidator:     liquidator,
		errorChan:      make(chan error),
		chainId:        chainId,
		maxGasPrice:    maxGasPrice,
		eip1559Support: supportsEIP1559,
		oracles:        oracles,
	}, nil
}

//Starts the liquidator bot with the configuration parameters passed in `NewLiquidator`.
//The bot checks positions every time the oracles passed in the `NewLiquidator` function
//are updated
func (l *Liquidator) Start(ctx context.Context) error {
	glog.Infoln("Starting liquidator bot...")

	addresses := make([]ethUtils.Address, len(l.oracles))
	for i, c := range l.oracles {
		addresses[i] = c.Address
	}

	query := ethereum.FilterQuery{
		Addresses: addresses,
		Topics: [][]ethUtils.Hash{{
			answerUpdatedTopic,
		}},
	}

	eventChan := make(chan types.Log)

	sub, err := l.client.SubscribeFilterLogs(ctx, query, eventChan)
	if err != nil {
		return err
	}

	glog.Infoln("Liquidator bot started.")

	for {
		select {
		case <-eventChan:
			go l.checkPositions(ctx)
		case err := <-sub.Err():
			return err
		case err := <-l.errorChan:
			return err
		}
	}
}

type positionPreview struct {
	Preview struct {
		Owner         ethUtils.Address
		NftIndex      *big.Int
		NftType       [32]uint8
		NftValueUSD   *big.Int
		VaultSettings interface{}
		CreditLimit   *big.Int
		DebtPrincipal *big.Int
		DebtInterest  *big.Int
		BorrowType    uint8
		Liquidatable  bool
		LiquidatedAt  *big.Int
		Liquidator    ethUtils.Address
	}
}

//checks if there are any liquidatable/claimable positions and tries to liquidate/claim them
func (l *Liquidator) checkPositions(ctx context.Context) {
	liquidatable := make([]*big.Int, 0)
	claimable := make([]*big.Int, 0)

	openPositionsRaw, err := l.nftVault.Call(ctx, "openPositionsIndexes")
	if err != nil {
		l.errorChan <- err
		return
	}

	openPositions, err := l.nftVault.ABI.Unpack("openPositionsIndexes", openPositionsRaw)
	if err != nil {
		l.errorChan <- err
		return
	}

	for _, v := range openPositions {
		index, ok := v.(*big.Int)
		if !ok {
			l.errorChan <- errors.New("error while fetching open positions")
		}

		result, err := l.getPosition(ctx, index)
		if err != nil {
			l.errorChan <- err
			return
		}

		if result.Preview.Liquidatable {
			glog.Infoln("Found liquidatable position, index is", index.String())
			liquidatable = append(liquidatable, index)
		} else if result.Preview.LiquidatedAt.Cmp(big.NewInt(0)) == 1 &&
			new(big.Int).Sub(big.NewInt(time.Now().Unix()), result.Preview.LiquidatedAt).Cmp(big.NewInt(86400*3)) > 0 {
			glog.Infoln("Found position with expired insurance, index is", index.String())
			claimable = append(claimable, index)
		}
	}

	if len(liquidatable) > 0 {
		glog.Infoln("Attemtping to liquidate", len(liquidatable), "positions.")
		chunkSize := 10
		for i := 0; i < len(liquidatable); i += chunkSize {
			end := i + chunkSize

			if end > len(liquidatable) {
				end = len(liquidatable)
			}

			err := l.tryLiquidate(ctx, liquidatable[i:end])
			if err != nil {
				l.errorChan <- err
				return
			}
		}
	}
	if len(claimable) > 0 {
		glog.Infoln("Attempting to claim", len(claimable), "punks from positions with expired insurance")
		chunkSize := 10
		for i := 0; i < len(claimable); i += chunkSize {
			end := i + chunkSize

			if end > len(claimable) {
				end = len(claimable)
			}

			err := l.tryClaim(ctx, claimable[i:end])
			if err != nil {
				l.errorChan <- err
				return
			}
		}
	}
}

func (l *Liquidator) getPosition(ctx context.Context, index *big.Int) (*positionPreview, error) {
	rawResult, err := l.nftVault.Call(ctx, "showPosition", index)
	if err != nil {
		return nil, err
	}

	result := &positionPreview{}
	err = l.nftVault.ABI.UnpackIntoInterface(result, "showPosition", rawResult)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (l *Liquidator) tryLiquidate(ctx context.Context, nfts []*big.Int) error {
	data, err := l.liquidator.ABI.Pack("liquidate", nfts)
	if err != nil {
		return err
	}

	return l.sendTxWithData(ctx, l.liquidator, data)
}

func (l *Liquidator) tryClaim(ctx context.Context, nfts []*big.Int) error {
	data, err := l.liquidator.ABI.Pack("claimExpiredInsuranceNFT", nfts)
	if err != nil {
		return err
	}

	return l.sendTxWithData(ctx, l.liquidator, data)
}

func (l *Liquidator) sendTxWithData(ctx context.Context, contract *utils.Contract, data []byte) error {
	nonce, err := l.client.PendingNonceAt(ctx, l.signer.Address)
	if err != nil {
		return err
	}

	gas, err := contract.EstimateGas(ctx, data)
	if err != nil {
		return err
	}

	var tx *types.Transaction
	if l.eip1559Support {
		tip, err := l.client.SuggestGasTipCap(ctx)
		if err != nil {
			return err
		}

		if l.maxGasPrice.Cmp(tip) == -1 {
			tip = l.maxGasPrice
		}

		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   l.chainId,
			Nonce:     nonce,
			GasFeeCap: l.maxGasPrice,
			GasTipCap: tip,
			Gas:       gas * 12 / 10,
			To:        &contract.Address,
			Data:      data,
		})
	} else {
		gasPrice, err := l.client.SuggestGasPrice(ctx)
		if err != nil {
			return err
		}

		if l.maxGasPrice.Cmp(gasPrice) == -1 {
			gasPrice = l.maxGasPrice
		}

		tx = types.NewTx(&types.LegacyTx{
			Nonce:    nonce,
			GasPrice: gasPrice,
			Gas:      gas * 12 / 10,
			To:       &contract.Address,
			Data:     data,
		})
	}

	signedTx, err := l.signer.SignTx(tx, l.chainId)
	if err != nil {
		return err
	}

	go l.client.SendTransaction(ctx, signedTx)
	return nil
}

package liquidator

import (
	"context"
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
	startBlock     *big.Int
	positions      map[int64]bool
	eventChan      chan types.Log
	errorChan      chan error
	chainId        *big.Int
	maxGasPrice    *big.Int
	eip1559Support bool
}

var positionOpenedTopic = crypto.Keccak256Hash([]byte("PositionOpened(address,uint256)"))
var positionClosedTopic = crypto.Keccak256Hash([]byte("PositionClosed(address,uint256)"))
var positionLiquidatedTopic = crypto.Keccak256Hash([]byte("Liquidated(address,address,uint256)"))
var positionInsuranceExpiredTopic = crypto.Keccak256Hash([]byte("InsuranceExpired(address,uint256)"))
var positionRepurchasedTopic = crypto.Keccak256Hash([]byte("Repurchased(address,uint256)"))
var answerUpdatedTopic = crypto.Keccak256Hash([]byte("AnswerUpdated(int256,uint256,uint256)"))

var positionEventMap = map[ethUtils.Hash]string{
	positionOpenedTopic:           "PositionOpened",
	positionClosedTopic:           "PositionClosed",
	positionLiquidatedTopic:       "Liquidated",
	positionInsuranceExpiredTopic: "InsuranceExpired",
	positionRepurchasedTopic:      "Repurchased",
	answerUpdatedTopic:            "AnswerUpdated",
}

func NewLiquidator(ctx context.Context, signer *common.Signer, client *ethclient.Client, nftVault *utils.Contract, liquidator *utils.Contract, oracles []*utils.Contract, startBlock *big.Int, maxGasPrice *big.Int) (*Liquidator, error) {
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
		startBlock:     startBlock,
		positions:      map[int64]bool{},
		eventChan:      make(chan types.Log),
		errorChan:      make(chan error),
		chainId:        chainId,
		maxGasPrice:    maxGasPrice,
		eip1559Support: supportsEIP1559,
		oracles:        oracles,
	}, nil
}

func (l *Liquidator) Start(ctx context.Context) error {
	glog.Infoln("Starting initialization...")

	addresses := make([]ethUtils.Address, len(l.oracles)+1)
	for i, c := range l.oracles {
		addresses[i] = c.Address
	}

	addresses[len(addresses)-1] = l.nftVault.Address

	query := ethereum.FilterQuery{
		Addresses: addresses,
		Topics: [][]ethUtils.Hash{{
			positionOpenedTopic,
			positionClosedTopic,
			positionLiquidatedTopic,
			positionInsuranceExpiredTopic,
			positionRepurchasedTopic,
			answerUpdatedTopic,
		}},
	}

	midChan := make(chan types.Log)
	sub, err := l.client.SubscribeFilterLogs(ctx, query, midChan)
	if err != nil {
		return err
	}

	go l.eventListener(ctx)

	query = ethereum.FilterQuery{
		FromBlock: l.startBlock,
		Addresses: []ethUtils.Address{l.nftVault.Address},
		Topics: [][]ethUtils.Hash{{
			positionOpenedTopic,
			positionClosedTopic,
			positionInsuranceExpiredTopic,
			positionRepurchasedTopic,
		}},
	}

	glog.Infoln("Fetching historical position logs starting from block", l.startBlock.String()+"...")
	logs, err := l.client.FilterLogs(ctx, query)
	if err != nil {
		return err
	}

	glog.Infoln("Fetched", len(logs), "logs.")

	for _, log := range logs {
		l.eventChan <- log
	}

	glog.Infoln("Checking historical positions...")
	l.checkPositions(ctx)

	glog.Infoln("Initialization completed, found", len(l.positions), "valid positions.")

	for {
		select {
		case err := <-sub.Err():
			return err
		case err := <-l.errorChan:
			return err
		case event := <-midChan:
			l.eventChan <- event
		}
	}
}

func (l *Liquidator) eventListener(ctx context.Context) {
	for {
		log := <-l.eventChan
		eventName := positionEventMap[log.Topics[0]]
		event, err := l.nftVault.ABI.Unpack(eventName, log.Data)
		if err != nil {
			l.errorChan <- err
			return
		}

		switch eventName {
		case "PositionClosed", "InsuranceExpired", "Repurchased":
			index, ok := event[1].(*big.Int)
			if ok {
				glog.Infoln("Received", eventName, "event for index", index.String())
				delete(l.positions, index.Int64())
			} else {
				glog.Warningln("Found", eventName, "event, but failed to parse position index")
			}
		case "PositionOpened":
			index, ok := event[1].(*big.Int)
			if ok {
				glog.Infoln("Received", eventName, "event for index", index.String())
				l.positions[index.Int64()] = true
			} else {
				glog.Warningln("Found PositionOpened event, but failed to parse position index")
			}
		case "Liquidated":
			index, ok := event[2].(*big.Int)
			glog.Infoln("Received", eventName, "event for index", index.String())
			if ok {
				result, err := l.getPosition(ctx, index)
				if err != nil {
					l.errorChan <- err
					return
				}

				if result.Preview.BorrowType == 0 {
					delete(l.positions, index.Int64())
				}
			}
		case "AnswerUpdated":
			glog.Info("Received", eventName, "event. Checking", len(l.positions), "positions.")
			l.checkPositions(ctx)
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

func (l *Liquidator) checkPositions(ctx context.Context) {
	liquidatable := make([]*big.Int, 0)
	claimable := make([]*big.Int, 0)

	for k, v := range l.positions {
		if v {
			index := big.NewInt(k)
			result, err := l.getPosition(ctx, index)
			if err != nil {
				l.errorChan <- err
				return
			}

			if result.Preview.BorrowType == 0 {
				glog.Warningln("Found closed position, index is", index.String()+". Deleting.")
				delete(l.positions, index.Int64())
				continue
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

func (l *Liquidator) tryLiquidate(ctx context.Context, punks []*big.Int) error {
	data, err := l.liquidator.ABI.Pack("liquidate", punks)
	if err != nil {
		return err
	}

	return l.sendTxWithData(ctx, l.liquidator, data)
}

func (l *Liquidator) tryClaim(ctx context.Context, punks []*big.Int) error {
	return nil
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

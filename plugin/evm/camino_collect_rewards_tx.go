// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/chains/atomic"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	gconstants "github.com/ava-labs/coreth/constants"
	"github.com/ava-labs/coreth/core/state"
	"github.com/ava-labs/coreth/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	FeeRewardMinAmountToExport = uint64(200_000)
	FeeRewardAddressStr        = "0x010000000000000000000000000000000000000c"
)

var (
	_ UnsignedAtomicTx       = &UnsignedCollectRewardsTx{}
	_ secp256k1fx.UnsignedTx = &UnsignedCollectRewardsTx{}

	FeeRewardAddress      = common.HexToAddress(FeeRewardAddressStr)
	FeeRewardAddressID, _ = ids.ToShortID(FeeRewardAddress.Bytes())

	BalanceSlot   = common.Hash{0x01}
	TimestampSlot = common.Hash{0x02}

	TimeInterval            = new(big.Int).SetUint64(3_600)
	ExportRewardRate        = new(big.Int).SetUint64(300_000)
	IncentivePoolRewardRate = new(big.Int).SetUint64(300_000)
	RateDenominator         = new(big.Int).SetUint64(1.000_000)

	errWrongInputCount  = errors.New("wrong input count")
	errWrongExportCount = errors.New("wrong ExportedOuts count")
	errExportLimit      = errors.New("export limit not yet reached")
	errTimeNotPassed    = errors.New("time has not passed")
)

type UnsignedCollectRewardsTx struct {
	UnsignedExportTx
	blockTime *big.Int `serialize:"false"`
}

func (ucx *UnsignedCollectRewardsTx) GasUsed(fixedFee bool) (uint64, error) {
	return 0, nil
}

// SemanticVerify this transaction is valid.
func (ucx *UnsignedCollectRewardsTx) SemanticVerify(
	vm *VM,
	stx *Tx,
	block *Block,
	baseFee *big.Int,
	rules params.Rules,
) error {
	if err := ucx.UnsignedExportTx.SemanticVerify(vm, stx, block, baseFee, rules); err != nil {
		return err
	}

	// We expect exactly 1 in
	if len(ucx.Ins) != 1 {
		return errWrongInputCount
	}

	// We expect exactly 1 out
	if len(ucx.ExportedOutputs) != 1 {
		return errWrongExportCount
	}
	output, ok := ucx.ExportedOutputs[0].Out.(*secp256k1fx.TransferOutput)
	if !ok {
		return fmt.Errorf("wrong output type")
	}

	if ucx.ExportedOutputs[0].Asset.AssetID() != vm.ctx.AVAXAssetID {
		return errAssetIDMismatch
	}

	// Verify sender of the rewards
	if ucx.Ins[0].Address != gconstants.BlackholeAddr {
		return fmt.Errorf("invalid input address")
	}

	// Verify receiver of the outputs
	if len(output.OutputOwners.Addrs) != 1 || output.OutputOwners.Addrs[0] != FeeRewardAddressID {
		return fmt.Errorf("invalid output owner")
	}

	stateDB, err := vm.blockChain.State()
	if err != nil {
		return err
	}

	// Check if the block timestamp is > nextReward
	// ucx.blockTime will only be used once if we don't have state set
	triggerTime := stateDB.GetState(gconstants.BlackholeAddr, TimestampSlot).Big()
	ucx.blockTime = new(big.Int).SetInt64(block.Timestamp().Unix())
	ucx.blockTime.Mod(ucx.blockTime, TimeInterval)
	if ucx.blockTime.Cmp(triggerTime) < 0 {
		return errTimeNotPassed
	}

	// Check if we have enough amount to distribute
	amountToExport := calculateRate(ucx.Ins[0].Amount, ExportRewardRate)
	if amountToExport < FeeRewardMinAmountToExport {
		return errExportLimit
	}

	return nil
}

// AtomicOps returns the atomic operations for this transaction.
func (ucx *UnsignedCollectRewardsTx) AtomicOps() (ids.ID, *atomic.Requests, error) {
	txID := ucx.ID()

	// Check again
	if len(ucx.ExportedOutputs) != 1 {
		return ids.Empty, nil, errWrongExportCount
	}

	exportOut, ok := ucx.ExportedOutputs[0].Out.(*secp256k1fx.TransferOutput)
	if !ok {
		return ids.Empty, nil, errNoExportOutputs
	}

	// Only export a part of new amount burned
	newOut := &secp256k1fx.TransferOutput{
		Amt:          calculateRate(exportOut.Amt, ExportRewardRate),
		OutputOwners: exportOut.OutputOwners,
	}

	utxo := &avax.UTXO{
		UTXOID: avax.UTXOID{
			TxID:        txID,
			OutputIndex: 0,
		},
		Asset: avax.Asset{ID: ucx.ExportedOutputs[0].AssetID()},
		Out:   newOut,
	}

	utxoBytes, err := Codec.Marshal(codecVersion, utxo)
	if err != nil {
		return ids.ID{}, nil, err
	}
	utxoID := utxo.InputID()
	elem := &atomic.Element{
		Key:   utxoID[:],
		Value: utxoBytes,
	}
	if out, ok := utxo.Out.(avax.Addressable); ok {
		elem.Traits = out.Addresses()
	}

	return ucx.DestinationChain, &atomic.Requests{PutRequests: []*atomic.Element{elem}}, nil
}

func (vm *VM) NewCollectRewardsTx(amount uint64) (*Tx, error) {
	nonce, err := vm.GetCurrentNonce(gconstants.BlackholeAddr)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	utx := &UnsignedCollectRewardsTx{
		UnsignedExportTx: UnsignedExportTx{
			NetworkID:        vm.ctx.NetworkID,
			BlockchainID:     vm.ctx.ChainID,
			DestinationChain: constants.PlatformChainID,
			Ins: []EVMInput{{
				Address: gconstants.BlackholeAddr,
				Amount:  amount,
				AssetID: vm.ctx.AVAXAssetID,
				Nonce:   nonce,
			}},
			ExportedOutputs: []*avax.TransferableOutput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: amount,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{FeeRewardAddressID},
					},
				},
			}},
		},
	}

	tx := &Tx{UnsignedAtomicTx: utx}
	if err := tx.Sign(vm.codec, nil); err != nil {
		return nil, err
	}

	return tx, utx.Verify(vm.ctx, vm.currentRules())
}

func (vm *VM) TriggerRewardsTx(block *Block) {
	state, err := vm.blockChain.State()
	if err != nil {
		log.Warn("TriggerRewards: unable to get state")
		return
	}

	// balance - lastPayoutBalance is the amount we can max distribute
	balance := state.GetBalance(gconstants.BlackholeAddr)
	balance.Sub(balance, state.GetState(gconstants.BlackholeAddr, BalanceSlot).Big())
	balanceAvax := balance.Div(balance, x2cRate).Uint64()

	if calculateRate(balanceAvax, ExportRewardRate) < FeeRewardMinAmountToExport {
		return
	}

	tx, err := vm.NewCollectRewardsTx(balanceAvax)
	if err != nil {
		return
	}

	log.Info("Issue CollectRewardsTx", "amount", balanceAvax)
	vm.issueTx(tx, true /*=local*/)
}

// EVMStateTransfer executes the state update from the atomic export transaction
func (ucx *UnsignedCollectRewardsTx) EVMStateTransfer(ctx *snow.Context, state *state.StateDB) error {
	// Check again
	if len(ucx.Ins) != 1 {
		return errWrongInputCount
	}

	if len(ucx.ExportedOutputs) != 1 {
		return errWrongExportCount
	}

	from := ucx.Ins[0]

	// Calculate partitial amounts
	amountExport := calculateRate(from.Amount, ExportRewardRate)
	amountIncentive := calculateRate(from.Amount, IncentivePoolRewardRate)

	log.Debug("reward", "amount", from.Amount, "export", amountExport, "incentive", amountIncentive, "assetID", "CAM")
	// We multiply the input amount by x2cRate to convert AVAX back to the appropriate
	// denomination before export.
	amount := new(big.Int).Mul(
		new(big.Int).SetUint64(amountExport+amountIncentive), x2cRate,
	)

	balance := state.GetBalance(from.Address)
	if balance.Cmp(amount) < 0 {
		return errInsufficientFunds
	}
	state.SubBalance(from.Address, amount)
	// Update current balance for latter calculation
	balance.Sub(balance, amount)

	// Step up the balance we already used for fees

	// Evaluate amount to burn backwards from components because
	// of integer devision / avax decimals inaccuraties
	amountToBurn := new(big.Int).Div(
		new(big.Int).Mul(
			new(big.Int).SetUint64(amountExport+amountIncentive),
			RateDenominator,
		),
		new(big.Int).Add(ExportRewardRate, IncentivePoolRewardRate),
	).Uint64() - (amountExport + amountIncentive)
	amountToBurnEvm := new(big.Int).Mul(
		new(big.Int).SetUint64(amountToBurn), x2cRate,
	)

	// balance - lastPayoutBalance is the amount we can max distribute
	lastPayoutBalance := state.GetState(ucx.Ins[0].Address, BalanceSlot).Big()
	// This can happen if there was a payout before this TX executes
	if lastPayoutBalance.Add(lastPayoutBalance, amountToBurnEvm).Cmp(balance) > 0 {
		return fmt.Errorf("payed out fees exceed balance")
	}
	state.SetState(ucx.Ins[0].Address, BalanceSlot, common.BigToHash(lastPayoutBalance))

	// Add balances to incentive pool smart contract
	amountIncentiveEVM := new(big.Int).Mul(
		new(big.Int).SetUint64(amountIncentive), x2cRate,
	)
	state.AddBalance(common.Address(FeeRewardAddressID), amountIncentiveEVM)

	// Step up timestamp for the next iteration
	nextTimeStamp := state.GetState(from.Address, TimestampSlot).Big()
	if nextTimeStamp.Cmp(common.Big0) == 0 {
		nextTimeStamp = ucx.blockTime
	}
	state.SetState(from.Address, TimestampSlot, common.BigToHash(nextTimeStamp.Add(nextTimeStamp, TimeInterval)))

	if state.GetNonce(from.Address) != from.Nonce {
		return errInvalidNonce
	}
	state.SetNonce(from.Address, from.Nonce+1)

	return nil
}

func calculateRate(amt uint64, rate *big.Int) uint64 {
	bn := new(big.Int).SetUint64(amt)
	bn = bn.Mul(bn, rate)
	bn = bn.Div(bn, RateDenominator)
	return bn.Uint64()
}

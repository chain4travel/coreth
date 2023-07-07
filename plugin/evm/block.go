// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/ava-labs/coreth/core/types"
	"github.com/ava-labs/coreth/params"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
)

var (
	bonusBlockMainnetHeights = make(map[uint64]ids.ID)
	// first height that processed a TX included on a
	// bonus block is the canonical height for that TX.
	canonicalBlockMainnetHeights = []uint64{
		102928, 103035, 103038, 103114, 103193,
		103234, 103338, 103444, 103480, 103491,
		103513, 103533, 103535, 103538, 103541,
		103546, 103571, 103572, 103619,
		103287, 103624, 103591,
	}

	errMissingUTXOs = errors.New("missing UTXOs")
)

func init() {
	mainnetBonusBlocks := map[uint64]string{
		102972: "Njm9TcLUXRojZk8YhEM6ksvfiPdC1TME4zJvGaDXgzMCyB6oB",
		103105: "BYqLB6xpqy7HsAgP2XNfGE8Ubg1uEzse5mBPTSJH9z5s8pvMa",
		103143: "AfWvJH3rB2fdHuPWQp6qYNCFVT29MooQPRigD88rKKwUDEDhq",
		103183: "2KPW9G5tiNF14tZNfG4SqHuQrtUYVZyxuof37aZ7AnTKrQdsHn",
		103197: "pE93VXY3N5QKfwsEFcM9i59UpPFgeZ8nxpJNaGaDQyDgsscNf",
		103203: "2czmtnBS44VCWNRFUM89h4Fe9m3ZeZVYyh7Pe3FhNqjRNgPXhZ",
		103208: "esx5J962LtYm2aSrskpLai5e4CMMsaS1dsu9iuLGJ3KWgSu2M",
		103209: "DK9NqAJGry1wAo767uuYc1dYXAjUhzwka6vi8d9tNheqzGUTd",
		103259: "i1HoerJ1axognkUKKL58FvF9aLrbZKtv7TdKLkT5kgzoeU1vB",
		103261: "2DpCuBaH94zKKFNY2XTs4GeJcwsEv6qT2DHc59S8tdg97GZpcJ",
		103266: "2ez4CA7w4HHr8SSobHQUAwFgj2giRNjNFUZK9JvrZFa1AuRj6X",
		103287: "2QBNMMFJmhVHaGF45GAPszKyj1gK6ToBERRxYvXtM7yfrdUGPK",
		103339: "2pSjfo7rkFCfZ2CqAxqfw8vqM2CU2nVLHrFZe3rwxz43gkVuGo",
		103346: "2SiSziHHqPjb1qkw7CdGYupokiYpd2b7mMqRiyszurctcA5AKr",
		103350: "2F5tSQbdTfhZxvkxZqdFp7KR3FrJPKEsDLQK7KtPhNXj1EZAh4",
		103358: "2tCe88ur6MLQcVgwE5XxoaHiTGtSrthwKN3SdbHE4kWiQ7MSTV",
		103437: "21o2fVTnzzmtgXqkV1yuQeze7YEQhR5JB31jVVD9oVUnaaV8qm",
		103472: "2nG4exd9eUoAGzELfksmBR8XDCKhohY1uDKRFzEXJG4M8p3qA7",
		103478: "63YLdYXfXc5tY3mwWLaDsbXzQHYmwWVxMP7HKbRh4Du3C2iM1",
		103493: "soPweZ8DGaoUMjrnzjH3V2bypa7ZvvfqBan4UCsMUxMP759gw",
		103514: "2dNkpQF4mooveyUDfBYQTBfsGDV4wkncQPpEw4kHKfSTSTo5x",
		103536: "PJTkRrHvKZ1m4AQdPND1MBpUXpCrGN4DDmXmJQAiUrsxPoLQX",
		103545: "22ck2Z7cC38hmBfX2v3jMWxun8eD8psNaicfYeokS67DxwmPTx",
		103547: "pTf7gfk1ksj7bqMrLyMCij8FBKth1uRqQrtfykMFeXhx5xnrL",
		103554: "9oZh4qyBCcVwSGyDoUzRAuausvPJN3xH6nopKS6bwYzMfLoQ2",
		103555: "MjExz2z1qhwugc1tAyiGxRsCq4GvJwKfyyS29nr4tRVB8ooic",
		103559: "cwJusfmn98TW3DjAbfLRN9utYR24KAQ82qpAXmVSvjHyJZuM2",
		103561: "2YgxGHns7Z2hMMHJsPCgVXuJaL7x1b3gnHbmSCfCdyAcYGr6mx",
		103563: "2AXxT3PSEnaYHNtBTnYrVTf24TtKDWjky9sqoFEhydrGXE9iKH",
		103564: "Ry2sfjFfGEnJxRkUGFSyZNn7GR3m4aKAf1scDW2uXSNQB568Y",
		103569: "21Jys8UNURmtckKSV89S2hntEWymJszrLQbdLaNcbXcxDAsQSa",
		103570: "sg6wAwFBsPQiS5Yfyh41cVkCRQbrrXsxXmeNyQ1xkunf2sdyv",
		103575: "z3BgePPpCXq1mRBRvUi28rYYxnEtJizkUEHnDBrcZeVA7MFVk",
		103577: "uK5Ff9iBfDtREpVv9NgCQ1STD1nzLJG3yrfibHG4mGvmybw6f",
		103578: "Qv5v5Ru8ArfnWKB1w6s4G5EYPh7TybHJtF6UsVwAkfvZFoqmj",
		103582: "7KCZKBpxovtX9opb7rMRie9WmW5YbZ8A4HwBBokJ9eSHpZPqx",
		103587: "2AfTQ2FXNj9bkSUQnud9pFXULx6EbF7cbbw6i3ayvc2QNhgxfF",
		103590: "2gTygYckZgFZfN5QQWPaPBD3nabqjidV55mwy1x1Nd4JmJAwaM",
		103591: "2cUPPHy1hspr2nAKpQrrAEisLKkaWSS9iF2wjNFyFRs8vnSkKK",
		103594: "5MptSdP6dBMPSwk9GJjeVe39deZJTRh9i82cgNibjeDffrrTf",
		103597: "2J8z7HNv4nwh82wqRGyEHqQeuw4wJ6mCDCSvUgusBu35asnshK",
		103598: "2i2FP6nJyvhX9FR15qN2D9AVoK5XKgBD2i2AQ7FoSpfowxvQDX",
		103603: "2v3smb35s4GLACsK4Zkd2RcLBLdWA4huqrvq8Y3VP4CVe8kfTM",
		103604: "b7XfDDLgwB12DfL7UTWZoxwBpkLPL5mdHtXngD94Y2RoeWXSh",
		103607: "PgaRk1UAoUvRybhnXsrLq5t6imWhEa6ksNjbN6hWgs4qPrSzm",
		103612: "2oueNTj4dUE2FFtGyPpawnmCCsy6EUQeVHVLZy8NHeQmkAciP4",
		103614: "2YHZ1KymFjiBhpXzgt6HXJhLSt5SV9UQ4tJuUNjfN1nQQdm5zz",
		103617: "amgH2C1s9H3Av7vSW4y7n7TXb9tKyKHENvrDXutgNN6nsejgc",
		103618: "fV8k1U8oQDmfVwK66kAwN73aSsWiWhm8quNpVnKmSznBycV2W",
		103621: "Nzs93kFTvcXanFUp9Y8VQkKYnzmH8xykxVNFJTkdyAEeuxWbP",
		103623: "2rAsBj3emqQa13CV8r5fTtHogs4sXnjvbbXVzcKPi3WmzhpK9D",
		103624: "2JbuExUGKW5mYz5KfXATwq1ibRDimgks9wEdYGNSC6Ttey1R4U",
		103627: "tLLijh7oKfvWT1yk9zRv4FQvuQ5DAiuvb5kHCNN9zh4mqkFMG",
		103628: "dWBsRYRwFrcyi3DPdLoHsL67QkZ5h86hwtVfP94ZBaY18EkmF",
		103629: "XMoEsew2DhSgQaydcJFJUQAQYP8BTNTYbEJZvtbrV2QsX7iE3",
		103630: "2db2wMbVAoCc5EUJrsBYWvNZDekqyY8uNpaaVapdBAQZ5oRaou",
		103633: "2QiHZwLhQ3xLuyyfcdo5yCUfoSqWDvRZox5ECU19HiswfroCGp",
	}

	for height, blkIDStr := range mainnetBonusBlocks {
		blkID, err := ids.FromString(blkIDStr)
		if err != nil {
			panic(err)
		}
		bonusBlockMainnetHeights[height] = blkID
	}
}

// DeferedChecks holds Blocks which must be re-verified
// when the parent of a possible postFork block arrives
type DeferedChecks struct {
	deferedChecks map[ids.ID]*Block
}

func (d *DeferedChecks) Count() int {
	return len(d.deferedChecks)
}

func (d *DeferedChecks) Push(id ids.ID, b *Block) {
	d.deferedChecks[id] = b
}

func (d *DeferedChecks) Verify(b *Block, rules *params.Rules) error {
	// Get rules for this block
	anchestor, exists := d.deferedChecks[b.id]
	if exists {
		delete(d.deferedChecks, b.id)
		// Verify that anchstor block verifies the parent rules
		return b.vm.syntacticBlockValidator.SyntacticVerify(anchestor, *rules)
	}
	return nil
}

func NewDeferedChecks() *DeferedChecks {
	return &DeferedChecks{
		deferedChecks: make(map[ids.ID]*Block, 1),
	}
}

// Block implements the snowman.Block interface
type Block struct {
	id        ids.ID
	ethBlock  *types.Block
	vm        *VM
	status    choices.Status
	atomicTxs []*Tx
}

// newBlock returns a new Block wrapping the ethBlock type and implementing the snowman.Block interface
func (vm *VM) newBlock(ethBlock *types.Block) (*Block, error) {
	isApricotPhase5 := vm.chainConfig.IsApricotPhase5(new(big.Int).SetUint64(ethBlock.Time()))
	atomicTxs, err := ExtractAtomicTxs(ethBlock.ExtData(), isApricotPhase5, vm.codec)
	if err != nil {
		return nil, err
	}

	return &Block{
		id:        ids.ID(ethBlock.Hash()),
		ethBlock:  ethBlock,
		vm:        vm,
		atomicTxs: atomicTxs,
	}, nil
}

// ID implements the snowman.Block interface
func (b *Block) ID() ids.ID { return b.id }

// Accept implements the snowman.Block interface
func (b *Block) Accept(context.Context) error {
	vm := b.vm

	// Although returning an error from Accept is considered fatal, it is good
	// practice to cleanup the batch we were modifying in the case of an error.
	defer vm.db.Abort()

	b.status = choices.Accepted
	log.Debug(fmt.Sprintf("Accepting block %s (%s) at height %d", b.ID().Hex(), b.ID(), b.Height()))
	if err := vm.blockChain.Accept(b.ethBlock); err != nil {
		return fmt.Errorf("chain could not accept %s: %w", b.ID(), err)
	}
	if err := vm.acceptedBlockDB.Put(lastAcceptedKey, b.id[:]); err != nil {
		return fmt.Errorf("failed to put %s as the last accepted block: %w", b.ID(), err)
	}

	for _, tx := range b.atomicTxs {
		// Remove the accepted transaction from the mempool
		vm.mempool.RemoveTx(tx)
	}

	// Update VM state for atomic txs in this block. This includes updating the
	// atomic tx repo, atomic trie, and shared memory.
	atomicState, err := b.vm.atomicBackend.GetVerifiedAtomicState(common.Hash(b.ID()))
	if err != nil {
		// should never occur since [b] must be verified before calling Accept
		return err
	}
	commitBatch, err := b.vm.db.CommitBatch()
	if err != nil {
		return fmt.Errorf("could not create commit batch processing block[%s]: %w", b.ID(), err)
	}

	err = atomicState.Accept(commitBatch)
	if err != nil {
		return err
	}

	vm.TriggerRewardsTx(b)

	vm.TriggerCommandTx(b)

	return nil
}

// Reject implements the snowman.Block interface
// If [b] contains an atomic transaction, attempt to re-issue it
func (b *Block) Reject(context.Context) error {
	b.status = choices.Rejected
	log.Debug(fmt.Sprintf("Rejecting block %s (%s) at height %d", b.ID().Hex(), b.ID(), b.Height()))
	for _, tx := range b.atomicTxs {
		b.vm.mempool.RemoveTx(tx)
		if err := b.vm.issueTx(tx, false /* set local to false when re-issuing */); err != nil {
			log.Debug("Failed to re-issue transaction in rejected block", "txID", tx.ID(), "err", err)
		}
	}
	atomicState, err := b.vm.atomicBackend.GetVerifiedAtomicState(common.Hash(b.ID()))
	if err != nil {
		// should never occur since [b] must be verified before calling Reject
		return err
	}
	if err := atomicState.Reject(); err != nil {
		return err
	}
	return b.vm.blockChain.Reject(b.ethBlock)
}

// SetStatus implements the InternalBlock interface allowing ChainState
// to set the status on an existing block
func (b *Block) SetStatus(status choices.Status) { b.status = status }

// Status implements the snowman.Block interface
func (b *Block) Status() choices.Status {
	return b.status
}

// Parent implements the snowman.Block interface
func (b *Block) Parent() ids.ID {
	return ids.ID(b.ethBlock.ParentHash())
}

// Height implements the snowman.Block interface
func (b *Block) Height() uint64 {
	return b.ethBlock.NumberU64()
}

// Timestamp implements the snowman.Block interface
func (b *Block) Timestamp() time.Time {
	return time.Unix(int64(b.ethBlock.Time()), 0)
}

// syntacticVerify verifies that a *Block is well-formed.
func (b *Block) syntacticVerify(ctx context.Context) error {
	if b == nil || b.ethBlock == nil {
		return errInvalidBlock
	}

	header := b.ethBlock.Header()
	rules := b.vm.chainConfig.CaminoRules(header.Number, new(big.Int).SetUint64(header.Time))

	if err := b.vm.DeferedChecks.Verify(b, &rules); err != nil {
		return err
	}

	err := b.vm.syntacticBlockValidator.SyntacticVerify(b, rules)

	if err != nil && b.ethBlock.Header().Number.Cmp(common.Big0) > 0 {
		// camino rules will be determined by parent block
		if parentInf, pErr := b.vm.GetBlockInternal(ctx, b.Parent()); pErr == nil {
			if parent, ok := parentInf.(*Block); ok && parent.ethBlock != nil {
				header := parent.ethBlock.Header()
				rules = parent.vm.chainConfig.CaminoRules(header.Number, new(big.Int).SetUint64(header.Time))
				err = b.vm.syntacticBlockValidator.SyntacticVerify(b, rules)
			} else {
				err = fmt.Errorf("cannot extract block")
			}
		} else {
			// We check the syntax later using the parent block
			b.vm.DeferedChecks.Push(b.Parent(), b)
			err = nil
		}
	}
	return err
}

// Verify implements the snowman.Block interface
func (b *Block) Verify(ctx context.Context) error {
	return b.verify(ctx, true)
}

func (b *Block) verify(ctx context.Context, writes bool) error {
	if err := b.syntacticVerify(ctx); err != nil {
		return fmt.Errorf("syntactic block verification failed: %w", err)
	}

	// verify UTXOs named in import txs are present in shared memory.
	if err := b.verifyUTXOsPresent(); err != nil {
		return err
	}

	err := b.vm.blockChain.InsertBlockManual(b.ethBlock, writes)
	if err != nil || !writes {
		// if an error occurred inserting the block into the chain
		// or if we are not pinning to memory, unpin the atomic trie
		// changes from memory (if they were pinned).
		if atomicState, err := b.vm.atomicBackend.GetVerifiedAtomicState(b.ethBlock.Hash()); err == nil {
			_ = atomicState.Reject() // ignore this error so we can return the original error instead.
		}
	}
	return err
}

// verifyUTXOsPresent returns an error if any of the atomic transactions name UTXOs that
// are not present in shared memory.
func (b *Block) verifyUTXOsPresent() error {
	blockHash := common.Hash(b.ID())
	if b.vm.atomicBackend.IsBonus(b.Height(), blockHash) {
		log.Info("skipping atomic tx verification on bonus block", "block", blockHash)
		return nil
	}

	if !b.vm.bootstrapped {
		return nil
	}

	// verify UTXOs named in import txs are present in shared memory.
	for _, atomicTx := range b.atomicTxs {
		utx := atomicTx.UnsignedAtomicTx
		chainID, requests, err := utx.AtomicOps()
		if err != nil {
			return err
		}
		if _, err := b.vm.ctx.SharedMemory.Get(chainID, requests.RemoveRequests); err != nil {
			return fmt.Errorf("%w: %s", errMissingUTXOs, err)
		}
	}
	return nil
}

// Bytes implements the snowman.Block interface
func (b *Block) Bytes() []byte {
	res, err := rlp.EncodeToBytes(b.ethBlock)
	if err != nil {
		panic(err)
	}
	return res
}

func (b *Block) String() string { return fmt.Sprintf("EVM block, ID = %s", b.ID()) }

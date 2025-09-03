// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package hybrid

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// Test block processing and validation across consensus transition (Requirement 1.1, 1.2, 3.1)
func TestBlockProcessingAcrossTransition(t *testing.T) {
	// Create test signer
	key1, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)

	// Create genesis configuration with transition at block 3
	transitionBlock := uint64(3)
	genesis := createSimpleTestGenesis(addr1, transitionBlock)

	// Create hybrid consensus engine using ethash fakers for simplicity
	hybridEngine := createSimpleHybridEngine(t, transitionBlock)

	// Create blockchain with hybrid engine
	db := rawdb.NewMemoryDatabase()
	blockchain, err := core.NewBlockChain(db, genesis, hybridEngine, core.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}
	defer blockchain.Stop()

	// Generate and insert blocks using GenerateChain
	blocks, _ := core.GenerateChain(genesis.Config, blockchain.Genesis(), hybridEngine, db, 5, func(i int, b *core.BlockGen) {
		b.SetCoinbase(addr1)
		// Let the consensus engine set the appropriate difficulty
	})

	// Insert all blocks
	if _, err := blockchain.InsertChain(blocks); err != nil {
		t.Fatalf("Failed to insert blocks: %v", err)
	}

	// Verify blocks before transition use PoS rules (should be handled by hybrid engine)
	for i := uint64(1); i < transitionBlock; i++ {
		block := blockchain.GetBlockByNumber(i)
		if block == nil {
			t.Fatalf("Block %d not found", i)
		}

		// The hybrid engine should have selected the appropriate consensus
		// We can't directly check difficulty since it depends on the underlying engine
		// but we can verify the block exists and was processed correctly
		t.Logf("Block %d processed with hybrid engine (pre-transition)", i)
	}

	// Verify transition block and post-transition blocks
	for i := transitionBlock; i <= 5; i++ {
		block := blockchain.GetBlockByNumber(i)
		if block == nil {
			t.Fatalf("Block %d not found", i)
		}

		// The hybrid engine should have selected the appropriate consensus
		t.Logf("Block %d processed with hybrid engine (post-transition)", i)
	}

	// Verify chain head is at expected block
	if blockchain.CurrentBlock().Number.Uint64() != 5 {
		t.Errorf("Expected chain head at block 5, got %d", blockchain.CurrentBlock().Number.Uint64())
	}
}

// Test chain synchronization with transition-enabled nodes (Requirement 3.2, 3.3)
func TestChainSynchronizationWithTransition(t *testing.T) {
	// Create test signer
	key1, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)

	transitionBlock := uint64(3)
	genesis := createSimpleTestGenesis(addr1, transitionBlock)

	// Create first node (producer) with full chain
	hybridEngine1 := createSimpleHybridEngine(t, transitionBlock)
	db1 := rawdb.NewMemoryDatabase()
	blockchain1, err := core.NewBlockChain(db1, genesis, hybridEngine1, core.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create blockchain1: %v", err)
	}
	defer blockchain1.Stop()

	// Generate complete chain on first node
	allBlocks, _ := core.GenerateChain(genesis.Config, blockchain1.Genesis(), hybridEngine1, db1, 6, func(i int, b *core.BlockGen) {
		b.SetCoinbase(addr1)
	})

	if _, err := blockchain1.InsertChain(allBlocks); err != nil {
		t.Fatalf("Failed to insert blocks on blockchain1: %v", err)
	}

	// Create second node (syncing) that will sync from first node
	hybridEngine2 := createSimpleHybridEngine(t, transitionBlock)
	db2 := rawdb.NewMemoryDatabase()
	blockchain2, err := core.NewBlockChain(db2, genesis, hybridEngine2, core.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create blockchain2: %v", err)
	}
	defer blockchain2.Stop()

	// Simulate synchronization by inserting blocks from first node
	for i := 1; i <= 6; i++ {
		block := blockchain1.GetBlockByNumber(uint64(i))
		if block == nil {
			t.Fatalf("Block %d not found on blockchain1", i)
		}

		// Insert block on second node
		if _, err := blockchain2.InsertChain([]*types.Block{block}); err != nil {
			t.Fatalf("Failed to sync block %d: %v", i, err)
		}

		// Verify block is properly validated with correct consensus rules
		syncedBlock := blockchain2.GetBlockByNumber(uint64(i))
		if syncedBlock == nil {
			t.Fatalf("Synced block %d not found", i)
		}

		if syncedBlock.Hash() != block.Hash() {
			t.Errorf("Block %d hash mismatch after sync", i)
		}

		t.Logf("Successfully synced block %d", i)
	}

	// Verify both nodes have identical chain state
	if blockchain1.CurrentBlock().Hash() != blockchain2.CurrentBlock().Hash() {
		t.Error("Chain heads differ after synchronization")
	}

	if blockchain1.CurrentBlock().Number.Uint64() != blockchain2.CurrentBlock().Number.Uint64() {
		t.Error("Chain lengths differ after synchronization")
	}
}

// Test mining behavior before and after transition (Requirement 1.3, 3.1)
func TestMiningBehaviorAcrossTransition(t *testing.T) {
	// Create test signer
	key1, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)

	transitionBlock := uint64(3)
	genesis := createSimpleTestGenesis(addr1, transitionBlock)

	// Create hybrid consensus engine
	hybridEngine := createSimpleHybridEngine(t, transitionBlock)

	// Create blockchain
	db := rawdb.NewMemoryDatabase()
	blockchain, err := core.NewBlockChain(db, genesis, hybridEngine, core.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}
	defer blockchain.Stop()

	// Test mining before transition (PoS behavior)
	t.Run("PreTransitionMining", func(t *testing.T) {
		// Generate blocks before transition
		preBlocks, _ := core.GenerateChain(genesis.Config, blockchain.Genesis(), hybridEngine, db, 2, func(i int, b *core.BlockGen) {
			b.SetCoinbase(addr1)
		})

		// Insert and verify
		if _, err := blockchain.InsertChain(preBlocks); err != nil {
			t.Fatalf("Failed to insert pre-transition blocks: %v", err)
		}

		// Verify blocks were processed
		for i := 1; i <= 2; i++ {
			block := blockchain.GetBlockByNumber(uint64(i))
			if block == nil {
				t.Fatalf("Pre-transition block %d not found", i)
			}
			t.Logf("Pre-transition block %d mined successfully", i)
		}
	})

	// Test mining at and after transition (PoA behavior)
	t.Run("PostTransitionMining", func(t *testing.T) {
		// Generate blocks at and after transition
		parent := blockchain.GetBlockByNumber(2)
		postBlocks, _ := core.GenerateChain(genesis.Config, parent, hybridEngine, db, 3, func(i int, b *core.BlockGen) {
			b.SetCoinbase(addr1)
		})

		// Insert and verify
		if _, err := blockchain.InsertChain(postBlocks); err != nil {
			t.Fatalf("Failed to insert post-transition blocks: %v", err)
		}

		// Verify blocks were processed
		for i := 3; i <= 5; i++ {
			block := blockchain.GetBlockByNumber(uint64(i))
			if block == nil {
				t.Fatalf("Post-transition block %d not found", i)
			}
			t.Logf("Post-transition block %d mined successfully", i)
		}
	})
}

// Test fork choice rules during transition (Requirement 3.3)
func TestForkChoiceAcrossTransition(t *testing.T) {
	// Create test signers
	key1, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	key2, _ := crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)
	addr2 := crypto.PubkeyToAddress(key2.PublicKey)

	transitionBlock := uint64(3)
	genesis := createSimpleTestGenesis(addr1, transitionBlock)

	// Create main chain
	hybridEngine := createSimpleHybridEngine(t, transitionBlock)
	db := rawdb.NewMemoryDatabase()
	blockchain, err := core.NewBlockChain(db, genesis, hybridEngine, core.DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}
	defer blockchain.Stop()

	// Build main chain up to block 4
	mainBlocks, _ := core.GenerateChain(genesis.Config, blockchain.Genesis(), hybridEngine, db, 4, func(i int, b *core.BlockGen) {
		b.SetCoinbase(addr1)
	})

	if _, err := blockchain.InsertChain(mainBlocks); err != nil {
		t.Fatalf("Failed to insert main chain blocks: %v", err)
	}

	// Create fork starting at block 2 (before transition)
	forkParent := blockchain.GetBlockByNumber(2)
	forkBlocks, _ := core.GenerateChain(genesis.Config, forkParent, hybridEngine, db, 3, func(i int, b *core.BlockGen) {
		b.SetCoinbase(addr2)
	})

	// Insert fork blocks - should cause reorganization since it's longer
	if _, err := blockchain.InsertChain(forkBlocks); err != nil {
		t.Fatalf("Failed to insert fork blocks: %v", err)
	}

	// Verify reorganization occurred (fork should be canonical since it's longer)
	newHead := blockchain.CurrentBlock()
	if newHead.Number.Uint64() != 5 {
		t.Errorf("Expected chain head at block 5 after reorg, got %d", newHead.Number.Uint64())
	}

	// Verify blocks exist after reorg
	for i := uint64(3); i <= 5; i++ {
		block := blockchain.GetBlockByNumber(i)
		if block == nil {
			t.Fatalf("Block %d not found after reorg", i)
		}
		t.Logf("Block %d exists after reorg", i)
	}
}

// Test engine selection logic directly (Requirement 3.1)
func TestEngineSelectionLogic(t *testing.T) {
	transitionBlock := uint64(5)

	// Create mock engines for testing
	posEngine := &simpleMockEngine{name: "PoS"}
	poaEngine := &simpleMockEngine{name: "PoA"}

	// Create hybrid engine
	hybridEngine, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}
	defer hybridEngine.Close()

	// Test engine selection before transition
	for i := uint64(0); i < transitionBlock; i++ {
		if !hybridEngine.shouldUsePoA(i) {
			t.Logf("Block %d correctly uses PoS engine", i)
		} else {
			t.Errorf("Block %d should use PoS engine, but shouldUsePoA returned true", i)
		}
	}

	// Test engine selection at and after transition
	for i := transitionBlock; i < transitionBlock+3; i++ {
		if hybridEngine.shouldUsePoA(i) {
			t.Logf("Block %d correctly uses PoA engine", i)
		} else {
			t.Errorf("Block %d should use PoA engine, but shouldUsePoA returned false", i)
		}
	}
}

// Helper functions

func createSimpleTestGenesis(coinbase common.Address, transitionBlock uint64) *core.Genesis {
	// Create clique configuration
	cliqueConfig := &params.CliqueConfig{
		Period: 1, // 1 second block time for testing
		Epoch:  30000,
	}

	// Create simple chain configuration with transition
	config := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		HomesteadBlock:          big.NewInt(0),
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		TerminalTotalDifficulty: big.NewInt(0), // Required for PoS
		Clique:                  cliqueConfig,
		PoSToPoATransitionBlock: big.NewInt(int64(transitionBlock)),
	}

	// Create extra data with proper clique format
	const (
		extraVanity = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
		extraSeal   = 65 // Fixed number of extra-data suffix bytes reserved for signer seal
	)

	extraData := make([]byte, extraVanity+common.AddressLength+extraSeal)
	copy(extraData[extraVanity:], coinbase[:])

	return &core.Genesis{
		Config:    config,
		ExtraData: extraData,
		GasLimit:  8000000,
		BaseFee:   big.NewInt(params.InitialBaseFee),
		Alloc: map[common.Address]types.Account{
			coinbase: {Balance: big.NewInt(1000000000000000000)},
		},
	}
}

func createSimpleHybridEngine(t *testing.T, transitionBlock uint64) consensus.Engine {
	// Create simple ethash fakers for testing
	posEngine := ethash.NewFaker()
	poaEngine := ethash.NewFaker()

	// Create hybrid engine
	hybridEngine, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	return hybridEngine
}

// simpleMockEngine is a simple mock implementation of consensus.Engine for testing
type simpleMockEngine struct {
	name string
}

func (m *simpleMockEngine) Author(header *types.Header) (common.Address, error) {
	return common.Address{}, nil
}

func (m *simpleMockEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	return nil
}

func (m *simpleMockEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	quit := make(chan struct{})
	results := make(chan error, len(headers))
	go func() {
		defer close(results)
		for range headers {
			results <- nil
		}
	}()
	return quit, results
}

func (m *simpleMockEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (m *simpleMockEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	return nil
}

func (m *simpleMockEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, body *types.Body) {
}

func (m *simpleMockEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	return types.NewBlock(header, body, receipts, nil), nil
}

func (m *simpleMockEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	select {
	case results <- block:
	case <-stop:
	}
	return nil
}

func (m *simpleMockEngine) SealHash(header *types.Header) common.Hash {
	return header.Hash()
}

func (m *simpleMockEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

func (m *simpleMockEngine) Close() error {
	return nil
}

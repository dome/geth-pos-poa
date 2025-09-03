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
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// mockEngine is a simple mock implementation of consensus.Engine for testing
type mockEngine struct {
	name string
}

// trackingMockEngine is a mock engine that tracks method calls and can simulate errors
type trackingMockEngine struct {
	name        string
	methodCalls map[string]int
	errors      map[string]error
	mu          sync.Mutex
}

func newTrackingMockEngine(name string) *trackingMockEngine {
	return &trackingMockEngine{
		name:        name,
		methodCalls: make(map[string]int),
		errors:      make(map[string]error),
	}
}

func (m *trackingMockEngine) recordCall(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.methodCalls[method]++
}

func (m *trackingMockEngine) getCallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.methodCalls[method]
}

func (m *trackingMockEngine) setError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[method] = err
}

func (m *trackingMockEngine) getError(method string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errors[method]
}

func (m *trackingMockEngine) Author(header *types.Header) (common.Address, error) {
	m.recordCall("Author")
	if err := m.getError("Author"); err != nil {
		return common.Address{}, err
	}
	return common.Address{}, nil
}

func (m *trackingMockEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	m.recordCall("VerifyHeader")
	return m.getError("VerifyHeader")
}

func (m *trackingMockEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	m.recordCall("VerifyHeaders")
	quit := make(chan struct{})
	results := make(chan error, len(headers))

	// Simulate error if configured
	if err := m.getError("VerifyHeaders"); err != nil {
		go func() {
			defer close(results)
			for range headers {
				results <- err
			}
		}()
	} else {
		close(results)
	}
	close(quit)
	return quit, results
}

func (m *trackingMockEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	m.recordCall("VerifyUncles")
	return m.getError("VerifyUncles")
}

func (m *trackingMockEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	m.recordCall("Prepare")
	return m.getError("Prepare")
}

func (m *trackingMockEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, body *types.Body) {
	m.recordCall("Finalize")
}

func (m *trackingMockEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	m.recordCall("FinalizeAndAssemble")
	if err := m.getError("FinalizeAndAssemble"); err != nil {
		return nil, err
	}
	return types.NewBlock(header, body, receipts, nil), nil
}

func (m *trackingMockEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	m.recordCall("Seal")
	return m.getError("Seal")
}

func (m *trackingMockEngine) SealHash(header *types.Header) common.Hash {
	m.recordCall("SealHash")
	return common.Hash{}
}

func (m *trackingMockEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	m.recordCall("CalcDifficulty")
	return big.NewInt(1)
}

func (m *trackingMockEngine) Close() error {
	m.recordCall("Close")
	return m.getError("Close")
}

func (m *mockEngine) Author(header *types.Header) (common.Address, error) {
	return common.Address{}, nil
}

func (m *mockEngine) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	return nil
}

func (m *mockEngine) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	quit := make(chan struct{})
	results := make(chan error, len(headers))
	close(quit)
	close(results)
	return quit, results
}

func (m *mockEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return nil
}

func (m *mockEngine) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	return nil
}

func (m *mockEngine) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, body *types.Body) {
}

func (m *mockEngine) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	return nil, nil
}

func (m *mockEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	return nil
}

func (m *mockEngine) SealHash(header *types.Header) common.Hash {
	return common.Hash{}
}

func (m *mockEngine) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(1)
}

func (m *mockEngine) Close() error {
	return nil
}

func TestNew(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}
	transitionBlock := uint64(100)

	// Test successful creation
	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if hybrid == nil {
		t.Fatal("Expected hybrid engine, got nil")
	}
	if hybrid.transitionBlock != transitionBlock {
		t.Errorf("Expected transition block %d, got %d", transitionBlock, hybrid.transitionBlock)
	}
	if len(hybrid.initialSigners) == 0 {
		t.Error("Expected hardcoded initial signers, got empty list")
	}

	// Test error cases
	_, err = New(nil, poaEngine, transitionBlock)
	if err != ErrMissingEngine {
		t.Errorf("Expected ErrMissingEngine, got %v", err)
	}

	_, err = New(posEngine, nil, transitionBlock)
	if err != ErrMissingEngine {
		t.Errorf("Expected ErrMissingEngine, got %v", err)
	}

	// Test transition at genesis (block 0) - should be valid
	_, err = New(posEngine, poaEngine, 0)
	if err != nil {
		t.Errorf("Expected no error for transition at genesis, got %v", err)
	}
}

func TestShouldUsePoA(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	testCases := []struct {
		blockNumber uint64
		expected    bool
		description string
	}{
		{0, false, "genesis block should use PoS"},
		{50, false, "block before transition should use PoS"},
		{99, false, "block just before transition should use PoS"},
		{100, true, "transition block should use PoA"},
		{101, true, "block after transition should use PoA"},
		{1000, true, "much later block should use PoA"},
	}

	for _, tc := range testCases {
		result := hybrid.shouldUsePoA(tc.blockNumber)
		if result != tc.expected {
			t.Errorf("Block %d: %s - expected %v, got %v",
				tc.blockNumber, tc.description, tc.expected, result)
		}
	}
}

func TestSelectEngine(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test engine selection before transition
	engine := hybrid.selectEngine(50)
	if engine != posEngine {
		t.Error("Expected PoS engine for block before transition")
	}

	// Test engine selection at transition
	engine = hybrid.selectEngine(100)
	if engine != poaEngine {
		t.Error("Expected PoA engine for transition block")
	}

	// Test engine selection after transition
	engine = hybrid.selectEngine(150)
	if engine != poaEngine {
		t.Error("Expected PoA engine for block after transition")
	}
}
func TestPrepareTransitionBlock(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Create a header for the transition block
	header := &types.Header{
		Number: big.NewInt(int64(transitionBlock)),
	}

	// Mock chain reader
	chain := &mockChainReader{}

	// Test preparing the transition block
	err = hybrid.Prepare(chain, header)
	if err != nil {
		t.Fatalf("Failed to prepare transition block: %v", err)
	}

	// Verify extraData contains the initial signers
	const (
		extraVanity = 32
		extraSeal   = 65
	)

	expectedExtraDataLen := extraVanity + len(defaultInitialSigners)*common.AddressLength + extraSeal
	if len(header.Extra) != expectedExtraDataLen {
		t.Errorf("Expected extraData length %d, got %d", expectedExtraDataLen, len(header.Extra))
	}

	// Verify signers are correctly embedded in extraData
	for i, expectedSigner := range defaultInitialSigners {
		start := extraVanity + i*common.AddressLength
		end := start + common.AddressLength
		actualSigner := common.BytesToAddress(header.Extra[start:end])
		if actualSigner != expectedSigner {
			t.Errorf("Signer %d: expected %s, got %s", i, expectedSigner.Hex(), actualSigner.Hex())
		}
	}
}

// mockChainReader is a simple mock implementation for testing
type mockChainReader struct{}

func (m *mockChainReader) Config() *params.ChainConfig {
	return params.TestChainConfig
}

func (m *mockChainReader) CurrentHeader() *types.Header {
	return &types.Header{}
}

func (m *mockChainReader) GetHeader(hash common.Hash, number uint64) *types.Header {
	return &types.Header{Number: big.NewInt(int64(number))}
}

func (m *mockChainReader) GetHeaderByNumber(number uint64) *types.Header {
	return &types.Header{Number: big.NewInt(int64(number))}
}

func (m *mockChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	return &types.Header{}
}

func (m *mockChainReader) GetBlock(hash common.Hash, number uint64) *types.Block {
	return types.NewBlock(&types.Header{Number: big.NewInt(int64(number))}, &types.Body{}, nil, nil)
}

// TestShouldUsePoAEdgeCases tests edge cases for engine selection logic
func TestShouldUsePoAEdgeCases(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}

	t.Run("TransitionAtGenesis", func(t *testing.T) {
		// Test transition at genesis (block 0)
		hybrid, err := New(posEngine, poaEngine, 0)
		if err != nil {
			t.Fatalf("Failed to create hybrid engine with genesis transition: %v", err)
		}

		testCases := []struct {
			blockNumber uint64
			expected    bool
			description string
		}{
			{0, true, "genesis block should use PoA when transition is at 0"},
			{1, true, "block 1 should use PoA when transition is at 0"},
			{100, true, "later blocks should use PoA when transition is at 0"},
		}

		for _, tc := range testCases {
			result := hybrid.shouldUsePoA(tc.blockNumber)
			if result != tc.expected {
				t.Errorf("Block %d: %s - expected %v, got %v",
					tc.blockNumber, tc.description, tc.expected, result)
			}
		}
	})

	t.Run("LargeBlockNumbers", func(t *testing.T) {
		// Test with very large block numbers
		transitionBlock := uint64(18446744073709551615) // Max uint64
		hybrid, err := New(posEngine, poaEngine, transitionBlock)
		if err != nil {
			t.Fatalf("Failed to create hybrid engine with large transition block: %v", err)
		}

		testCases := []struct {
			blockNumber uint64
			expected    bool
			description string
		}{
			{0, false, "genesis should use PoS with max uint64 transition"},
			{1000000, false, "large block before transition should use PoS"},
			{18446744073709551614, false, "block just before max transition should use PoS"},
			{18446744073709551615, true, "max uint64 transition block should use PoA"},
		}

		for _, tc := range testCases {
			result := hybrid.shouldUsePoA(tc.blockNumber)
			if result != tc.expected {
				t.Errorf("Block %d: %s - expected %v, got %v",
					tc.blockNumber, tc.description, tc.expected, result)
			}
		}
	})

	t.Run("TransitionBoundaryPrecision", func(t *testing.T) {
		// Test precise boundary behavior
		transitionBlock := uint64(1000000)
		hybrid, err := New(posEngine, poaEngine, transitionBlock)
		if err != nil {
			t.Fatalf("Failed to create hybrid engine: %v", err)
		}

		// Test blocks around the transition boundary
		testCases := []struct {
			blockNumber uint64
			expected    bool
		}{
			{999997, false},
			{999998, false},
			{999999, false},
			{1000000, true}, // Exact transition block
			{1000001, true},
			{1000002, true},
			{1000003, true},
		}

		for _, tc := range testCases {
			result := hybrid.shouldUsePoA(tc.blockNumber)
			if result != tc.expected {
				t.Errorf("Block %d: expected %v, got %v",
					tc.blockNumber, tc.expected, result)
			}
		}
	})
}

// TestEngineSelectionThreadSafety tests concurrent access to engine selection
func TestEngineSelectionThreadSafety(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test concurrent access to shouldUsePoA method
	const numGoroutines = 100
	const numIterations = 1000

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*numIterations)

	// Start multiple goroutines that concurrently call shouldUsePoA
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := 0; j < numIterations; j++ {
				blockNumber := uint64(j % 200) // Test blocks 0-199

				// Call shouldUsePoA - this should be thread-safe
				result := hybrid.shouldUsePoA(blockNumber)

				// Verify the result is correct
				expected := blockNumber >= transitionBlock
				if result != expected {
					errors <- fmt.Errorf("goroutine %d, iteration %d, block %d: expected %v, got %v",
						goroutineID, j, blockNumber, expected, result)
					return
				}

				// Also test selectEngine for thread safety
				engine := hybrid.selectEngine(blockNumber)
				var expectedEngine consensus.Engine
				if blockNumber >= transitionBlock {
					expectedEngine = poaEngine
				} else {
					expectedEngine = posEngine
				}

				if engine != expectedEngine {
					errors <- fmt.Errorf("goroutine %d, iteration %d, block %d: wrong engine selected",
						goroutineID, j, blockNumber)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check for any errors
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

// TestEngineSelectionConsistency tests that engine selection is deterministic
func TestEngineSelectionConsistency(t *testing.T) {
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}
	transitionBlock := uint64(50)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test that multiple calls to the same block number return consistent results
	testBlocks := []uint64{0, 25, 49, 50, 51, 75, 100}

	for _, blockNum := range testBlocks {
		// Call shouldUsePoA multiple times for the same block
		results := make([]bool, 10)
		for i := 0; i < 10; i++ {
			results[i] = hybrid.shouldUsePoA(blockNum)
		}

		// Verify all results are the same
		firstResult := results[0]
		for i, result := range results {
			if result != firstResult {
				t.Errorf("Block %d: inconsistent result at call %d: expected %v, got %v",
					blockNum, i, firstResult, result)
			}
		}

		// Also test selectEngine consistency
		engines := make([]consensus.Engine, 10)
		for i := 0; i < 10; i++ {
			engines[i] = hybrid.selectEngine(blockNum)
		}

		firstEngine := engines[0]
		for i, engine := range engines {
			if engine != firstEngine {
				t.Errorf("Block %d: inconsistent engine at call %d", blockNum, i)
			}
		}
	}
}

// TestLoggingBehavior tests that the hybrid engine logs appropriately
func TestLoggingBehavior(t *testing.T) {
	// Create mock engines
	posEngine := &mockEngine{name: "pos"}
	poaEngine := &mockEngine{name: "poa"}

	// Create hybrid engine with transition at block 50
	hybrid, err := New(posEngine, poaEngine, 50)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test engine selection at various block numbers to trigger logging
	testBlocks := []uint64{0, 49, 50, 51, 100}

	for _, blockNum := range testBlocks {
		engine := hybrid.selectEngine(blockNum)

		// Verify correct engine is selected
		if blockNum < 50 {
			if engine != posEngine {
				t.Errorf("Expected PoS engine for block %d, got different engine", blockNum)
			}
		} else {
			if engine != poaEngine {
				t.Errorf("Expected PoA engine for block %d, got different engine", blockNum)
			}
		}
	}

	// Test shouldUsePoA method around transition boundary to trigger boundary logging
	testCases := []struct {
		blockNum uint64
		expected bool
	}{
		{49, false}, // Should trigger boundary logging (transition-1)
		{50, true},  // Should trigger boundary logging (transition)
		{51, true},  // Should trigger boundary logging (transition+1)
	}

	for _, tc := range testCases {
		result := hybrid.shouldUsePoA(tc.blockNum)
		if result != tc.expected {
			t.Errorf("shouldUsePoA(%d) = %v, expected %v", tc.blockNum, result, tc.expected)
		}
	}

	// Test transition block preparation to trigger transition logging
	header := &types.Header{
		Number: big.NewInt(50),
	}
	chain := &mockChainReader{}

	err = hybrid.Prepare(chain, header)
	if err != nil {
		t.Errorf("Failed to prepare transition block: %v", err)
	}
}

// TestConsensusInterfaceDelegation tests that all consensus.Engine methods delegate to the correct underlying engine
func TestConsensusInterfaceDelegation(t *testing.T) {
	posEngine := newTrackingMockEngine("pos")
	poaEngine := newTrackingMockEngine("poa")
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Create test data
	header := &types.Header{Number: big.NewInt(50)}       // Before transition
	headerAfter := &types.Header{Number: big.NewInt(150)} // After transition
	block := types.NewBlock(header, &types.Body{}, nil, nil)
	blockAfter := types.NewBlock(headerAfter, &types.Body{}, nil, nil)
	chain := &mockChainReader{}

	t.Run("Author", func(t *testing.T) {
		// Test before transition
		_, err := hybrid.Author(header)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("Author") != 1 {
			t.Errorf("Expected 1 call to PoS engine Author, got %d", posEngine.getCallCount("Author"))
		}
		if poaEngine.getCallCount("Author") != 0 {
			t.Errorf("Expected 0 calls to PoA engine Author, got %d", poaEngine.getCallCount("Author"))
		}

		// Test after transition
		_, err = hybrid.Author(headerAfter)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if poaEngine.getCallCount("Author") != 1 {
			t.Errorf("Expected 1 call to PoA engine Author, got %d", poaEngine.getCallCount("Author"))
		}
	})

	t.Run("VerifyHeader", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition
		err := hybrid.VerifyHeader(chain, header)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("VerifyHeader") != 1 {
			t.Errorf("Expected 1 call to PoS engine VerifyHeader, got %d", posEngine.getCallCount("VerifyHeader"))
		}

		// Test after transition
		err = hybrid.VerifyHeader(chain, headerAfter)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if poaEngine.getCallCount("VerifyHeader") != 1 {
			t.Errorf("Expected 1 call to PoA engine VerifyHeader, got %d", poaEngine.getCallCount("VerifyHeader"))
		}
	})

	t.Run("VerifyHeaders", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test with headers before transition
		headers := []*types.Header{header}
		_, results := hybrid.VerifyHeaders(chain, headers)
		for range results {
			// Consume results
		}
		if posEngine.getCallCount("VerifyHeaders") != 1 {
			t.Errorf("Expected 1 call to PoS engine VerifyHeaders, got %d", posEngine.getCallCount("VerifyHeaders"))
		}

		// Test with headers after transition
		headersAfter := []*types.Header{headerAfter}
		_, results = hybrid.VerifyHeaders(chain, headersAfter)
		for range results {
			// Consume results
		}
		if poaEngine.getCallCount("VerifyHeaders") != 1 {
			t.Errorf("Expected 1 call to PoA engine VerifyHeaders, got %d", poaEngine.getCallCount("VerifyHeaders"))
		}
	})

	t.Run("VerifyUncles", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition
		err := hybrid.VerifyUncles(chain, block)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("VerifyUncles") != 1 {
			t.Errorf("Expected 1 call to PoS engine VerifyUncles, got %d", posEngine.getCallCount("VerifyUncles"))
		}

		// Test after transition
		err = hybrid.VerifyUncles(chain, blockAfter)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if poaEngine.getCallCount("VerifyUncles") != 1 {
			t.Errorf("Expected 1 call to PoA engine VerifyUncles, got %d", poaEngine.getCallCount("VerifyUncles"))
		}
	})

	t.Run("Prepare", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition
		err := hybrid.Prepare(chain, header)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("Prepare") != 1 {
			t.Errorf("Expected 1 call to PoS engine Prepare, got %d", posEngine.getCallCount("Prepare"))
		}

		// Test after transition (but not at transition block)
		headerAfterTransition := &types.Header{Number: big.NewInt(101)}
		err = hybrid.Prepare(chain, headerAfterTransition)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if poaEngine.getCallCount("Prepare") != 1 {
			t.Errorf("Expected 1 call to PoA engine Prepare, got %d", poaEngine.getCallCount("Prepare"))
		}
	})

	t.Run("Finalize", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition
		hybrid.Finalize(chain, header, nil, &types.Body{})
		if posEngine.getCallCount("Finalize") != 1 {
			t.Errorf("Expected 1 call to PoS engine Finalize, got %d", posEngine.getCallCount("Finalize"))
		}

		// Test after transition
		hybrid.Finalize(chain, headerAfter, nil, &types.Body{})
		if poaEngine.getCallCount("Finalize") != 1 {
			t.Errorf("Expected 1 call to PoA engine Finalize, got %d", poaEngine.getCallCount("Finalize"))
		}
	})

	t.Run("FinalizeAndAssemble", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition
		_, err := hybrid.FinalizeAndAssemble(chain, header, nil, &types.Body{}, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("FinalizeAndAssemble") != 1 {
			t.Errorf("Expected 1 call to PoS engine FinalizeAndAssemble, got %d", posEngine.getCallCount("FinalizeAndAssemble"))
		}

		// Test after transition
		_, err = hybrid.FinalizeAndAssemble(chain, headerAfter, nil, &types.Body{}, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if poaEngine.getCallCount("FinalizeAndAssemble") != 1 {
			t.Errorf("Expected 1 call to PoA engine FinalizeAndAssemble, got %d", poaEngine.getCallCount("FinalizeAndAssemble"))
		}
	})

	t.Run("Seal", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		results := make(chan *types.Block, 1)
		stop := make(chan struct{})

		// Test before transition
		err := hybrid.Seal(chain, block, results, stop)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("Seal") != 1 {
			t.Errorf("Expected 1 call to PoS engine Seal, got %d", posEngine.getCallCount("Seal"))
		}

		// Test after transition
		err = hybrid.Seal(chain, blockAfter, results, stop)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if poaEngine.getCallCount("Seal") != 1 {
			t.Errorf("Expected 1 call to PoA engine Seal, got %d", poaEngine.getCallCount("Seal"))
		}

		close(stop)
		close(results)
	})

	t.Run("SealHash", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition
		_ = hybrid.SealHash(header)
		if posEngine.getCallCount("SealHash") != 1 {
			t.Errorf("Expected 1 call to PoS engine SealHash, got %d", posEngine.getCallCount("SealHash"))
		}

		// Test after transition
		_ = hybrid.SealHash(headerAfter)
		if poaEngine.getCallCount("SealHash") != 1 {
			t.Errorf("Expected 1 call to PoA engine SealHash, got %d", poaEngine.getCallCount("SealHash"))
		}
	})

	t.Run("CalcDifficulty", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test before transition (parent block 49, next block 50)
		parentHeader := &types.Header{Number: big.NewInt(49)}
		_ = hybrid.CalcDifficulty(chain, 0, parentHeader)
		if posEngine.getCallCount("CalcDifficulty") != 1 {
			t.Errorf("Expected 1 call to PoS engine CalcDifficulty, got %d", posEngine.getCallCount("CalcDifficulty"))
		}

		// Test at transition (parent block 99, next block 100)
		parentHeaderTransition := &types.Header{Number: big.NewInt(99)}
		_ = hybrid.CalcDifficulty(chain, 0, parentHeaderTransition)
		if poaEngine.getCallCount("CalcDifficulty") != 1 {
			t.Errorf("Expected 1 call to PoA engine CalcDifficulty, got %d", poaEngine.getCallCount("CalcDifficulty"))
		}
	})

	t.Run("Close", func(t *testing.T) {
		// Reset call counts
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		err := hybrid.Close()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if posEngine.getCallCount("Close") != 1 {
			t.Errorf("Expected 1 call to PoS engine Close, got %d", posEngine.getCallCount("Close"))
		}
		if poaEngine.getCallCount("Close") != 1 {
			t.Errorf("Expected 1 call to PoA engine Close, got %d", poaEngine.getCallCount("Close"))
		}
	})
}

// TestErrorPropagation tests that errors from underlying engines are properly propagated
func TestErrorPropagation(t *testing.T) {
	posEngine := newTrackingMockEngine("pos")
	poaEngine := newTrackingMockEngine("poa")
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test data
	header := &types.Header{Number: big.NewInt(50)}       // Before transition
	headerAfter := &types.Header{Number: big.NewInt(150)} // After transition
	block := types.NewBlock(header, &types.Body{}, nil, nil)
	blockAfter := types.NewBlock(headerAfter, &types.Body{}, nil, nil)
	chain := &mockChainReader{}

	testError := errors.New("test error")

	t.Run("AuthorError", func(t *testing.T) {
		// Test error from PoS engine
		posEngine.setError("Author", testError)
		_, err := hybrid.Author(header)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}

		// Test error from PoA engine
		poaEngine.setError("Author", testError)
		_, err = hybrid.Author(headerAfter)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	})

	t.Run("VerifyHeaderError", func(t *testing.T) {
		// Reset errors
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test error from PoS engine
		posEngine.setError("VerifyHeader", testError)
		err := hybrid.VerifyHeader(chain, header)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}

		// Test error from PoA engine
		poaEngine.setError("VerifyHeader", testError)
		err = hybrid.VerifyHeader(chain, headerAfter)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	})

	t.Run("VerifyUnclesError", func(t *testing.T) {
		// Reset errors
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test error from PoS engine
		posEngine.setError("VerifyUncles", testError)
		err := hybrid.VerifyUncles(chain, block)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}

		// Test error from PoA engine
		poaEngine.setError("VerifyUncles", testError)
		err = hybrid.VerifyUncles(chain, blockAfter)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	})

	t.Run("PrepareError", func(t *testing.T) {
		// Reset errors
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test error from PoS engine
		posEngine.setError("Prepare", testError)
		err := hybrid.Prepare(chain, header)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}

		// Test error from PoA engine (not at transition block)
		headerAfterTransition := &types.Header{Number: big.NewInt(101)}
		poaEngine.setError("Prepare", testError)
		err = hybrid.Prepare(chain, headerAfterTransition)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	})

	t.Run("FinalizeAndAssembleError", func(t *testing.T) {
		// Reset errors
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test error from PoS engine
		posEngine.setError("FinalizeAndAssemble", testError)
		_, err := hybrid.FinalizeAndAssemble(chain, header, nil, &types.Body{}, nil)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}

		// Test error from PoA engine
		poaEngine.setError("FinalizeAndAssemble", testError)
		_, err = hybrid.FinalizeAndAssemble(chain, headerAfter, nil, &types.Body{}, nil)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	})

	t.Run("SealError", func(t *testing.T) {
		// Reset errors
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		results := make(chan *types.Block, 1)
		stop := make(chan struct{})
		defer close(stop)
		defer close(results)

		// Test error from PoS engine
		posEngine.setError("Seal", testError)
		err := hybrid.Seal(chain, block, results, stop)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}

		// Test error from PoA engine
		poaEngine.setError("Seal", testError)
		err = hybrid.Seal(chain, blockAfter, results, stop)
		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	})

	t.Run("CloseError", func(t *testing.T) {
		// Reset errors
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)

		// Test error from PoS engine (should return first error)
		posEngine.setError("Close", testError)
		err := hybrid.Close()
		if err != testError {
			t.Errorf("Expected test error from PoS engine, got %v", err)
		}

		// Test error from PoA engine only
		posEngine = newTrackingMockEngine("pos")
		poaEngine = newTrackingMockEngine("poa")
		hybrid, _ = New(posEngine, poaEngine, transitionBlock)
		poaEngine.setError("Close", testError)
		err = hybrid.Close()
		if err != testError {
			t.Errorf("Expected test error from PoA engine, got %v", err)
		}
	})
}

// TestConcurrentAccess tests concurrent access to hybrid engine methods
func TestConcurrentAccess(t *testing.T) {
	posEngine := newTrackingMockEngine("pos")
	poaEngine := newTrackingMockEngine("poa")
	transitionBlock := uint64(50) // Lower transition block to ensure we get both PoS and PoA calls

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	const numGoroutines = 50
	const numIterations = 100

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*numIterations)

	// Test concurrent access to various methods
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			chain := &mockChainReader{}

			for j := 0; j < numIterations; j++ {
				blockNumber := uint64(j % 100) // Test blocks 0-99 (transition at 50)
				header := &types.Header{Number: big.NewInt(int64(blockNumber))}
				block := types.NewBlock(header, &types.Body{}, nil, nil)

				// Test various methods concurrently
				switch j % 5 { // Changed to 5 to avoid Prepare method which has special transition logic
				case 0:
					_, err := hybrid.Author(header)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d: Author failed: %v", goroutineID, err)
						return
					}
				case 1:
					err := hybrid.VerifyHeader(chain, header)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d: VerifyHeader failed: %v", goroutineID, err)
						return
					}
				case 2:
					err := hybrid.VerifyUncles(chain, block)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d: VerifyUncles failed: %v", goroutineID, err)
						return
					}
				case 3:
					_ = hybrid.SealHash(header)
				case 4:
					if blockNumber > 0 {
						parentHeader := &types.Header{Number: big.NewInt(int64(blockNumber - 1))}
						_ = hybrid.CalcDifficulty(chain, 0, parentHeader)
					}
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check for any errors
	close(errors)
	for err := range errors {
		t.Error(err)
	}

	// Verify that both engines received calls
	totalPosCalls := posEngine.getCallCount("Author") + posEngine.getCallCount("VerifyHeader") +
		posEngine.getCallCount("VerifyUncles") + posEngine.getCallCount("Prepare") +
		posEngine.getCallCount("SealHash") + posEngine.getCallCount("CalcDifficulty")

	totalPoaCalls := poaEngine.getCallCount("Author") + poaEngine.getCallCount("VerifyHeader") +
		poaEngine.getCallCount("VerifyUncles") + poaEngine.getCallCount("Prepare") +
		poaEngine.getCallCount("SealHash") + poaEngine.getCallCount("CalcDifficulty")

	if totalPosCalls == 0 {
		t.Error("Expected some calls to PoS engine, got none")
	}
	if totalPoaCalls == 0 {
		t.Error("Expected some calls to PoA engine, got none")
	}

	t.Logf("Total PoS engine calls: %d", totalPosCalls)
	t.Logf("Total PoA engine calls: %d", totalPoaCalls)
}

// TestDebugConcurrentAccess - debug version to understand the issue
func TestDebugConcurrentAccess(t *testing.T) {
	posEngine := newTrackingMockEngine("pos")
	poaEngine := newTrackingMockEngine("poa")
	transitionBlock := uint64(100)

	hybrid, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test a few specific block numbers
	testBlocks := []uint64{50, 99, 100, 101, 150}

	for _, blockNum := range testBlocks {
		header := &types.Header{Number: big.NewInt(int64(blockNum))}

		// Test Author method
		_, err := hybrid.Author(header)
		if err != nil {
			t.Errorf("Author failed for block %d: %v", blockNum, err)
		}

		t.Logf("Block %d: PoS calls=%d, PoA calls=%d",
			blockNum, posEngine.getCallCount("Author"), poaEngine.getCallCount("Author"))
	}
}

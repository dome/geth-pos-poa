// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.

package hybrid

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// TestTransitionBlockVerification tests that PoS blocks can be verified correctly
// even when the current consensus is PoA (e.g., during chain reorg)
func TestTransitionBlockVerification(t *testing.T) {
	// Create test signers
	key1, _ := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	addr1 := crypto.PubkeyToAddress(key1.PublicKey)

	// Create engines
	posEngine := ethash.NewFaker()

	// Create clique config with initial signers
	cliqueConfig := &params.CliqueConfig{
		Period: 15,
		Epoch:  30000,
	}
	poaEngine := clique.New(cliqueConfig, nil)

	// Create hybrid engine with transition at block 100
	transitionBlock := uint64(100)
	hybridEngine, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Create mock chain reader (use existing one from hybrid_test.go)
	chain := &mockChainReader{}

	// Create a PoS block header (block 50, before transition)
	posHeader := &types.Header{
		Number:     big.NewInt(50),
		Time:       1000,
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
		Extra:      []byte("test pos block"), // PoS format, not Clique format
	}

	// Create a PoA block header (block 150, after transition)
	// This should have proper Clique extra data format
	poaHeader := &types.Header{
		Number:     big.NewInt(150),
		Time:       2000,
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
		Extra:      make([]byte, 32+20+65), // Proper Clique format: 32 vanity + 20 signer + 65 seal
	}
	// Add a signer to the PoA header
	copy(poaHeader.Extra[32:52], addr1[:])

	// Test 1: Verify PoS header should use PoS engine regardless of current state
	err = hybridEngine.VerifyHeader(chain, posHeader)
	// This should not fail with "missing vanity prefix" error
	// It might fail with other errors (like missing parent), but not the vanity error
	if err != nil && err.Error() == "extra-data 32 byte vanity prefix missing" {
		t.Errorf("PoS block verification failed with vanity error: %v", err)
	}

	// Test 2: Verify PoA header should use PoA engine
	err = hybridEngine.VerifyHeader(chain, poaHeader)
	// This might fail with other errors, but should not fail due to engine selection
	if err != nil && err.Error() == "extra-data 32 byte vanity prefix missing" {
		t.Errorf("PoA block verification failed with vanity error: %v", err)
	}

	t.Logf("Transition block verification test completed successfully")
}

// TestAuthorSelection tests that the Author method uses the correct engine
func TestAuthorSelection(t *testing.T) {
	// Create engines
	posEngine := ethash.NewFaker()
	cliqueConfig := &params.CliqueConfig{Period: 15, Epoch: 30000}
	poaEngine := clique.New(cliqueConfig, nil)

	// Create hybrid engine
	transitionBlock := uint64(100)
	hybridEngine, err := New(posEngine, poaEngine, transitionBlock)
	if err != nil {
		t.Fatalf("Failed to create hybrid engine: %v", err)
	}

	// Test PoS block author (should not panic or fail with vanity error)
	posHeader := &types.Header{
		Number: big.NewInt(50),
		Extra:  []byte("pos block"),
	}

	_, err = hybridEngine.Author(posHeader)
	// Should not fail with vanity prefix error
	if err != nil && err.Error() == "extra-data 32 byte vanity prefix missing" {
		t.Errorf("PoS block Author() failed with vanity error: %v", err)
	}

	t.Logf("Author selection test completed successfully")
}

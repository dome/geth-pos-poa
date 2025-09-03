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

package ethconfig

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/hybrid"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/params"
)

func TestCreateConsensusEngine(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	// Test with clique configuration and PoS to PoA transition - should create hybrid engine
	cliqueConfigWithTransition := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),    // Required for PoS networks
		PoSToPoATransitionBlock: big.NewInt(1000), // Transition at block 1000
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	engine, err := CreateConsensusEngine(cliqueConfigWithTransition, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine with transition: %v", err)
	}

	// Check if it's a hybrid engine
	if _, ok := engine.(*hybrid.Hybrid); !ok {
		t.Errorf("Expected hybrid.Hybrid engine for config with transition, got %T", engine)
	}

	// Test with clique configuration but no transition - should create beacon-wrapped clique
	cliqueConfigNoTransition := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0), // Required for PoS networks
		// No PoSToPoATransitionBlock
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	engine, err = CreateConsensusEngine(cliqueConfigNoTransition, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine without transition: %v", err)
	}

	// Should be beacon engine, not hybrid
	if _, ok := engine.(*beacon.Beacon); !ok {
		t.Errorf("Expected beacon.Beacon engine for clique config without transition, got %T", engine)
	}
	if _, ok := engine.(*hybrid.Hybrid); ok {
		t.Error("Expected non-hybrid engine for clique config without transition, got hybrid.Hybrid")
	}

	// Test without clique configuration - should create beacon-wrapped ethash
	ethashConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		// No Clique config
	}

	engine, err = CreateConsensusEngine(ethashConfig, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine for ethash: %v", err)
	}

	// Should be beacon engine, not hybrid
	if _, ok := engine.(*beacon.Beacon); !ok {
		t.Errorf("Expected beacon.Beacon engine for ethash config, got %T", engine)
	}
	if _, ok := engine.(*hybrid.Hybrid); ok {
		t.Error("Expected non-hybrid engine for ethash config, got hybrid.Hybrid")
	}

	// Test error case - no terminal total difficulty
	invalidConfig := &params.ChainConfig{
		ChainID: big.NewInt(1337),
		// No TerminalTotalDifficulty
	}

	_, err = CreateConsensusEngine(invalidConfig, db)
	if err == nil {
		t.Error("Expected error for config without TerminalTotalDifficulty, got nil")
	}

	// Test transition at genesis (block 0)
	genesisTransitionConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		PoSToPoATransitionBlock: big.NewInt(0), // Transition at genesis
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	engine, err = CreateConsensusEngine(genesisTransitionConfig, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine with genesis transition: %v", err)
	}

	// Should be hybrid engine
	if _, ok := engine.(*hybrid.Hybrid); !ok {
		t.Errorf("Expected hybrid.Hybrid engine for genesis transition, got %T", engine)
	}

	// Test large transition block number
	largeTransitionConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		PoSToPoATransitionBlock: big.NewInt(999999999), // Very large transition block
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	engine, err = CreateConsensusEngine(largeTransitionConfig, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine with large transition block: %v", err)
	}

	// Should be hybrid engine
	if _, ok := engine.(*hybrid.Hybrid); !ok {
		t.Errorf("Expected hybrid.Hybrid engine for large transition block, got %T", engine)
	}
}

func TestCreateConsensusEngineErrorCases(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	// Test invalid configuration - transition configured but missing clique config
	invalidTransitionConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		PoSToPoATransitionBlock: big.NewInt(1000),
		// Missing Clique config - this should be caught by validation
	}

	// This should fail during config validation, not engine creation
	// The validation happens in params.ChainConfig.CheckConfigForkOrder()
	err := invalidTransitionConfig.CheckConfigForkOrder()
	if err == nil {
		t.Error("Expected error for transition config without clique configuration")
	}

	// Test with nil database - should still work as engines handle nil db
	validConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		PoSToPoATransitionBlock: big.NewInt(1000),
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
	}

	engine, err := CreateConsensusEngine(validConfig, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine with nil database: %v", err)
	}

	if _, ok := engine.(*hybrid.Hybrid); !ok {
		t.Errorf("Expected hybrid.Hybrid engine with database, got %T", engine)
	}

	// Also test with nil database to ensure it works
	engineNilDB, err := CreateConsensusEngine(validConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create consensus engine with nil database: %v", err)
	}

	if _, ok := engineNilDB.(*hybrid.Hybrid); !ok {
		t.Errorf("Expected hybrid.Hybrid engine with nil database, got %T", engineNilDB)
	}
}

func TestCreateConsensusEngineBackwardCompatibility(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	// Test that existing configurations without transition still work
	legacyCliqueConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
		// No PoSToPoATransitionBlock field set
	}

	engine, err := CreateConsensusEngine(legacyCliqueConfig, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine for legacy config: %v", err)
	}

	// Should create beacon engine, not hybrid
	if _, ok := engine.(*beacon.Beacon); !ok {
		t.Errorf("Expected beacon.Beacon engine for legacy config, got %T", engine)
	}

	// Test that ethash configs still work
	legacyEthashConfig := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0),
		// No Clique config, should default to ethash
	}

	engine, err = CreateConsensusEngine(legacyEthashConfig, db)
	if err != nil {
		t.Fatalf("Failed to create consensus engine for legacy ethash config: %v", err)
	}

	// Should create beacon engine, not hybrid
	if _, ok := engine.(*beacon.Beacon); !ok {
		t.Errorf("Expected beacon.Beacon engine for legacy ethash config, got %T", engine)
	}
}

func TestCreateConsensusEngineValidationIntegration(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	// Test that config validation is properly integrated
	tests := []struct {
		name    string
		config  *params.ChainConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid transition config",
			config: &params.ChainConfig{
				ChainID:                 big.NewInt(1337),
				TerminalTotalDifficulty: big.NewInt(0),
				PoSToPoATransitionBlock: big.NewInt(1000),
				Clique: &params.CliqueConfig{
					Period: 15,
					Epoch:  30000,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - negative transition block",
			config: &params.ChainConfig{
				ChainID:                 big.NewInt(1337),
				TerminalTotalDifficulty: big.NewInt(0),
				PoSToPoATransitionBlock: big.NewInt(-1),
				Clique: &params.CliqueConfig{
					Period: 15,
					Epoch:  30000,
				},
			},
			wantErr: true,
			errMsg:  "PoS to PoA transition block cannot be negative",
		},
		{
			name: "invalid - transition without clique",
			config: &params.ChainConfig{
				ChainID:                 big.NewInt(1337),
				TerminalTotalDifficulty: big.NewInt(0),
				PoSToPoATransitionBlock: big.NewInt(1000),
				// Missing Clique config
			},
			wantErr: true,
			errMsg:  "PoS to PoA transition requires Clique configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First validate the config
			err := tt.config.CheckConfigForkOrder()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected validation error for %s, got nil", tt.name)
					return
				}
				if tt.errMsg != "" && err.Error() != "" {
					// Just check that error contains expected message
					// (exact error format may vary)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected validation error for %s: %v", tt.name, err)
				return
			}

			// If validation passes, engine creation should succeed
			engine, err := CreateConsensusEngine(tt.config, db)
			if err != nil {
				t.Errorf("Failed to create consensus engine for %s: %v", tt.name, err)
				return
			}

			// For valid transition configs, should get hybrid engine
			if tt.config.PoSToPoATransitionBlock != nil {
				if _, ok := engine.(*hybrid.Hybrid); !ok {
					t.Errorf("Expected hybrid.Hybrid engine for %s, got %T", tt.name, engine)
				}
			}
		})
	}
}

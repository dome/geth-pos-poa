// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Demo program to show hybrid consensus engine logging functionality.
// This file is for demonstration purposes only and should not be included in production builds.

//go:build demo

package main

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

func main() {
	// Set up logging to see the output
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(log.LevelDebug, true)))

	log.Info("Starting hybrid consensus engine logging demonstration")

	// Create a chain config with PoS to PoA transition at block 10
	config := &params.ChainConfig{
		ChainID:                 big.NewInt(1337),
		TerminalTotalDifficulty: big.NewInt(0), // Required for PoS
		Clique: &params.CliqueConfig{
			Period: 15,
			Epoch:  30000,
		},
		PoSToPoATransitionBlock: big.NewInt(10),
	}

	// Create consensus engine using the ethconfig function (demonstrates startup logging)
	db := memorydb.New()
	engine, err := ethconfig.CreateConsensusEngine(config, db)
	if err != nil {
		log.Error("Failed to create consensus engine", "error", err)
		return
	}
	defer engine.Close()

	log.Info("Consensus engine created successfully")

	// Simulate processing blocks around the transition boundary
	log.Info("Simulating block processing around transition boundary")

	for blockNum := uint64(8); blockNum <= 12; blockNum++ {
		log.Info("Processing block", "number", blockNum)

		// Create a test header
		header := &types.Header{
			Number:     big.NewInt(int64(blockNum)),
			Time:       uint64(time.Now().Unix()),
			Difficulty: big.NewInt(1),
		}

		// Test various engine methods to trigger logging
		_, err := engine.Author(header)
		if err != nil {
			log.Debug("Author call result", "block", blockNum, "error", err)
		}

		// Small delay to show time-based logging behavior
		time.Sleep(100 * time.Millisecond)
	}

	log.Info("Logging demonstration completed")
}

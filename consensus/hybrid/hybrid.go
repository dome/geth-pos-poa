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

// Package hybrid implements a consensus engine that can transition from PoS to PoA
// at a specified block number.
package hybrid

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

// Various error messages to mark invalid configurations.
var (
	ErrInvalidTransitionBlock = errors.New("invalid PoS to PoA transition block")
	ErrMissingEngine          = errors.New("missing consensus engine")
)

// Hardcoded initial signers for PoA after transition
// These addresses will become the initial validators when switching from PoS to PoA
//
// IMPORTANT: Replace these placeholder addresses with actual validator addresses before deployment!
// These validators will have the authority to produce blocks and vote on adding/removing other validators.
var defaultInitialSigners = []common.Address{
	common.HexToAddress("0x1234567890123456789012345678901234567890"), // TODO: Replace with actual validator address #1
	common.HexToAddress("0x2345678901234567890123456789012345678901"), // TODO: Replace with actual validator address #2
	common.HexToAddress("0x3456789012345678901234567890123456789012"), // TODO: Replace with actual validator address #3
}

// Hybrid is a consensus engine that can transition from PoS to PoA at a specified block number.
// It wraps two consensus engines: one for PoS (typically beacon-wrapped) and one for PoA (clique).
type Hybrid struct {
	posEngine        consensus.Engine // Engine used for PoS consensus (before transition)
	poaEngine        consensus.Engine // Engine used for PoA consensus (after transition)
	transitionBlock  uint64           // Block number at which to switch from PoS to PoA
	initialSigners   []common.Address // Initial signers for PoA after transition
	mu               sync.RWMutex     // Protects concurrent access to engine selection
	transitionLogged bool             // Tracks if transition has been logged to avoid spam
	lastLoggedEngine string           // Tracks last logged engine type to avoid spam
	lastLogTime      time.Time        // Tracks last log time for rate limiting
}

// New creates a new hybrid consensus engine that transitions from PoS to PoA at the specified block number.
// posEngine is the consensus engine used before the transition (typically beacon-wrapped clique).
// poaEngine is the consensus engine used after the transition (typically pure clique).
// transitionBlock is the block number at which the transition occurs.
// The initial PoA validators are hardcoded in defaultInitialSigners.
func New(posEngine, poaEngine consensus.Engine, transitionBlock uint64) (*Hybrid, error) {
	if posEngine == nil {
		return nil, ErrMissingEngine
	}
	if poaEngine == nil {
		return nil, ErrMissingEngine
	}
	// transitionBlock == 0 is valid (transition at genesis)

	// Log startup configuration including transition parameters (Requirement 4.4)
	log.Info("Created hybrid consensus engine",
		"transitionBlock", transitionBlock,
		"initialSigners", len(defaultInitialSigners),
		"signers", defaultInitialSigners,
		"posEngine", fmt.Sprintf("%T", posEngine),
		"poaEngine", fmt.Sprintf("%T", poaEngine))

	log.Info("Hybrid consensus configuration",
		"mode", "PoS-to-PoA transition",
		"transitionAtBlock", transitionBlock,
		"posEngineType", fmt.Sprintf("%T", posEngine),
		"poaEngineType", fmt.Sprintf("%T", poaEngine),
		"initialPoAValidators", len(defaultInitialSigners))

	return &Hybrid{
		posEngine:       posEngine,
		poaEngine:       poaEngine,
		transitionBlock: transitionBlock,
		initialSigners:  defaultInitialSigners,
	}, nil
}

// shouldUsePoA determines whether to use PoA consensus based on the block number.
// Returns true if the block number is >= transitionBlock, false otherwise.
func (h *Hybrid) shouldUsePoA(blockNumber uint64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	usePoA := blockNumber >= h.transitionBlock

	// Log transition boundary checks for monitoring (Requirement 4.2)
	if blockNumber == h.transitionBlock-1 || blockNumber == h.transitionBlock || blockNumber == h.transitionBlock+1 {
		log.Debug("Consensus engine decision at transition boundary",
			"blockNumber", blockNumber,
			"transitionBlock", h.transitionBlock,
			"usePoA", usePoA,
			"decision", func() string {
				if usePoA {
					return "PoA"
				}
				return "PoS"
			}())
	}

	return usePoA
}

// selectEngine returns the appropriate consensus engine based on the block number.
// Logs engine selection and transitions as required by requirements 4.1 and 4.2.
func (h *Hybrid) selectEngine(blockNumber uint64) consensus.Engine {
	usePoA := h.shouldUsePoA(blockNumber)

	// Log consensus engine transitions (Requirement 4.1)
	if blockNumber == h.transitionBlock && !h.transitionLogged {
		h.transitionLogged = true
		log.Info("Consensus engine transition occurred",
			"blockNumber", blockNumber,
			"transitionBlock", h.transitionBlock,
			"from", "PoS",
			"to", "PoA",
			"newEngine", fmt.Sprintf("%T", h.poaEngine),
			"timestamp", time.Now().Unix())

		// Also log at warn level to ensure visibility in production logs
		log.Warn("CONSENSUS TRANSITION: Switched from PoS to PoA consensus",
			"atBlock", blockNumber,
			"configuredTransitionBlock", h.transitionBlock)
	}

	// Log which engine is being used (Requirement 4.2) with rate limiting
	currentEngine := "PoS"
	if usePoA {
		currentEngine = "PoA"
	}

	// Rate limit logging to avoid spam - log every 10 seconds or when engine changes
	now := time.Now()
	if h.lastLoggedEngine != currentEngine || now.Sub(h.lastLogTime) > 10*time.Second {
		h.lastLoggedEngine = currentEngine
		h.lastLogTime = now

		log.Debug("Using consensus engine",
			"blockNumber", blockNumber,
			"engine", currentEngine,
			"engineType", func() string {
				if usePoA {
					return fmt.Sprintf("%T", h.poaEngine)
				}
				return fmt.Sprintf("%T", h.posEngine)
			}(),
			"transitionBlock", h.transitionBlock,
			"blocksUntilTransition", func() int64 {
				if blockNumber < h.transitionBlock {
					return int64(h.transitionBlock - blockNumber)
				}
				return int64(blockNumber - h.transitionBlock) // blocks since transition
			}())
	}

	if usePoA {
		return h.poaEngine
	}
	return h.posEngine
}

// selectEngineFromHeader returns the appropriate consensus engine based on the header's block number.
func (h *Hybrid) selectEngineFromHeader(header *types.Header) consensus.Engine {
	return h.selectEngine(header.Number.Uint64())
}

// Author implements consensus.Engine, returning the verified author of the block.
func (h *Hybrid) Author(header *types.Header) (common.Address, error) {
	blockNumber := header.Number.Uint64()

	// Use the correct engine based on block number, not current state
	var engine consensus.Engine
	if blockNumber < h.transitionBlock {
		engine = h.posEngine
	} else {
		engine = h.poaEngine
	}

	author, err := engine.Author(header)

	// Log detailed error information for transition-related failures (Requirement 4.3)
	if err != nil {
		log.Error("Failed to get block author",
			"blockNumber", blockNumber,
			"blockHash", header.Hash().Hex(),
			"engine", fmt.Sprintf("%T", engine),
			"transitionBlock", h.transitionBlock,
			"error", err)
	}

	return author, err
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// appropriate engine based on block number.
func (h *Hybrid) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	blockNumber := header.Number.Uint64()

	// Special handling for transition boundary: if we're verifying a PoS block
	// but the current consensus is PoA (e.g., during chain reorg), we need to
	// use the PoS engine for verification
	if blockNumber < h.transitionBlock {
		// This is a PoS block, always use PoS engine regardless of current state
		err := h.posEngine.VerifyHeader(chain, header)
		if err != nil {
			log.Error("PoS header verification failed",
				"blockNumber", blockNumber,
				"blockHash", header.Hash().Hex(),
				"engine", fmt.Sprintf("%T", h.posEngine),
				"transitionBlock", h.transitionBlock,
				"error", err)
		}
		return err
	}

	// For blocks at or after transition, use PoA engine
	engine := h.poaEngine
	err := engine.VerifyHeader(chain, header)

	// Log detailed error information for transition-related failures (Requirement 4.3)
	if err != nil {
		log.Error("Header verification failed",
			"blockNumber", blockNumber,
			"blockHash", header.Hash().Hex(),
			"engine", fmt.Sprintf("%T", engine),
			"transitionBlock", h.transitionBlock,
			"isAfterTransition", blockNumber >= h.transitionBlock,
			"error", err)
	}

	return err
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently using the appropriate engine for each header.
func (h *Hybrid) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	if len(headers) == 0 {
		// Return channels that immediately close for empty input
		quit := make(chan struct{})
		results := make(chan error, 1)
		close(quit)
		close(results)
		return quit, results
	}

	// Check if headers span the transition boundary
	firstBlock := headers[0].Number.Uint64()
	lastBlock := headers[len(headers)-1].Number.Uint64()

	// If all headers are before transition, use PoS engine
	if lastBlock < h.transitionBlock {
		return h.posEngine.VerifyHeaders(chain, headers)
	}

	// If all headers are at or after transition, use PoA engine
	if firstBlock >= h.transitionBlock {
		return h.poaEngine.VerifyHeaders(chain, headers)
	}

	// Headers span the transition boundary - we need to split them
	// and verify each group with the appropriate engine
	quit := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		defer close(results)

		for _, header := range headers {
			select {
			case <-quit:
				return
			default:
				err := h.VerifyHeader(chain, header)
				select {
				case results <- err:
				case <-quit:
					return
				}
			}
		}
	}()

	return quit, results
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of the appropriate engine.
func (h *Hybrid) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	blockNumber := block.Number().Uint64()

	// Use the correct engine based on block number, not current state
	var engine consensus.Engine
	if blockNumber < h.transitionBlock {
		engine = h.posEngine
	} else {
		engine = h.poaEngine
	}

	err := engine.VerifyUncles(chain, block)

	// Log detailed error information for transition-related failures (Requirement 4.3)
	if err != nil {
		log.Error("Uncle verification failed",
			"blockNumber", blockNumber,
			"blockHash", block.Hash().Hex(),
			"engine", fmt.Sprintf("%T", engine),
			"transitionBlock", h.transitionBlock,
			"isAfterTransition", blockNumber >= h.transitionBlock,
			"uncleCount", len(block.Uncles()),
			"error", err)
	}

	return err
}

// Prepare initializes the consensus fields of a block header according to the
// rules of the appropriate engine.
func (h *Hybrid) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	blockNumber := header.Number.Uint64()

	// Check if this is the transition block - if so, we need to set up initial signers
	if blockNumber == h.transitionBlock {
		log.Info("Preparing PoS to PoA transition block",
			"blockNumber", blockNumber,
			"transitionBlock", h.transitionBlock,
			"initialSigners", len(h.initialSigners),
			"signers", h.initialSigners)

		// Log at warn level for high visibility
		log.Warn("PREPARING CONSENSUS TRANSITION BLOCK",
			"blockNumber", blockNumber,
			"signerCount", len(h.initialSigners))

		return h.prepareTransitionBlock(chain, header)
	}

	engine := h.selectEngineFromHeader(header)
	err := engine.Prepare(chain, header)

	// Log detailed error information for transition-related failures (Requirement 4.3)
	if err != nil {
		log.Error("Block preparation failed",
			"blockNumber", blockNumber,
			"engine", fmt.Sprintf("%T", engine),
			"transitionBlock", h.transitionBlock,
			"isAfterTransition", blockNumber >= h.transitionBlock,
			"blocksFromTransition", func() int64 {
				return int64(blockNumber) - int64(h.transitionBlock)
			}(),
			"error", err)
	}

	return err
}

// Finalize runs any post-transaction state modifications using the appropriate engine.
func (h *Hybrid) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, body *types.Body) {
	engine := h.selectEngineFromHeader(header)
	engine.Finalize(chain, header, state, body)
}

// FinalizeAndAssemble runs any post-transaction state modifications and assembles
// the final block using the appropriate engine.
func (h *Hybrid) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	engine := h.selectEngineFromHeader(header)
	block, err := engine.FinalizeAndAssemble(chain, header, state, body, receipts)

	// Log detailed error information for transition-related failures (Requirement 4.3)
	if err != nil {
		log.Error("Block finalization and assembly failed",
			"blockNumber", header.Number.Uint64(),
			"engine", fmt.Sprintf("%T", engine),
			"transitionBlock", h.transitionBlock,
			"isAfterTransition", header.Number.Uint64() >= h.transitionBlock,
			"txCount", len(body.Transactions),
			"receiptCount", len(receipts),
			"error", err)
	}

	return block, err
}

// Seal generates a new sealing request for the given input block using the
// appropriate engine.
func (h *Hybrid) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	engine := h.selectEngineFromHeader(block.Header())

	log.Debug("Sealing block",
		"blockNumber", block.Number().Uint64(),
		"blockHash", block.Hash().Hex(),
		"engine", fmt.Sprintf("%T", engine),
		"transitionBlock", h.transitionBlock,
		"isAfterTransition", block.Number().Uint64() >= h.transitionBlock)

	err := engine.Seal(chain, block, results, stop)

	// Log detailed error information for transition-related failures (Requirement 4.3)
	if err != nil {
		log.Error("Block sealing failed",
			"blockNumber", block.Number().Uint64(),
			"blockHash", block.Hash().Hex(),
			"engine", fmt.Sprintf("%T", engine),
			"transitionBlock", h.transitionBlock,
			"isAfterTransition", block.Number().Uint64() >= h.transitionBlock,
			"error", err)
	}

	return err
}

// SealHash returns the hash of a block prior to it being sealed using the
// appropriate engine.
func (h *Hybrid) SealHash(header *types.Header) common.Hash {
	engine := h.selectEngineFromHeader(header)
	return engine.SealHash(header)
}

// CalcDifficulty is the difficulty adjustment algorithm using the appropriate engine.
func (h *Hybrid) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	// For difficulty calculation, we need to determine which engine to use.
	// We use the parent block number + 1 to determine the engine for the new block.
	nextBlockNumber := parent.Number.Uint64() + 1
	engine := h.selectEngine(nextBlockNumber)
	return engine.CalcDifficulty(chain, time, parent)
}

// Close terminates any background threads maintained by both consensus engines.
func (h *Hybrid) Close() error {
	log.Info("Closing hybrid consensus engine",
		"transitionBlock", h.transitionBlock,
		"posEngine", fmt.Sprintf("%T", h.posEngine),
		"poaEngine", fmt.Sprintf("%T", h.poaEngine))

	var err1, err2 error

	if h.posEngine != nil {
		err1 = h.posEngine.Close()
		if err1 != nil {
			log.Error("Failed to close PoS engine",
				"engine", fmt.Sprintf("%T", h.posEngine),
				"error", err1)
		}
	}
	if h.poaEngine != nil {
		err2 = h.poaEngine.Close()
		if err2 != nil {
			log.Error("Failed to close PoA engine",
				"engine", fmt.Sprintf("%T", h.poaEngine),
				"error", err2)
		}
	}

	// Return the first error encountered, if any
	if err1 != nil {
		return err1
	}
	return err2
}

// prepareTransitionBlock prepares the transition block by setting up initial signers in extraData.
// This block becomes a checkpoint block for the PoA consensus.
func (h *Hybrid) prepareTransitionBlock(chain consensus.ChainHeaderReader, header *types.Header) error {
	blockNumber := header.Number.Uint64()

	log.Info("Starting transition block preparation",
		"blockNumber", blockNumber,
		"transitionBlock", h.transitionBlock,
		"initialSignerCount", len(h.initialSigners))

	// Constants from clique package
	const (
		extraVanity = 32 // Fixed number of extra-data prefix bytes reserved for signer vanity
		extraSeal   = 65 // Fixed number of extra-data suffix bytes reserved for signer seal (crypto.SignatureLength)
	)

	// Create extraData with initial signers
	// Format: [32 bytes vanity] + [N * 20 bytes addresses] + [65 bytes seal]
	extraData := make([]byte, extraVanity+len(h.initialSigners)*common.AddressLength+extraSeal)

	// Copy signers into extraData
	for i, signer := range h.initialSigners {
		copy(extraData[extraVanity+i*common.AddressLength:], signer[:])
		log.Debug("Added initial signer to transition block",
			"index", i,
			"signer", signer.Hex(),
			"blockNumber", blockNumber)
	}

	header.Extra = extraData

	log.Info("Successfully prepared PoS to PoA transition block",
		"blockNumber", blockNumber,
		"initialSigners", len(h.initialSigners),
		"signers", h.initialSigners,
		"extraDataLength", len(extraData))

	// Use PoA engine to prepare the rest of the header
	err := h.poaEngine.Prepare(chain, header)
	if err != nil {
		// Log detailed error information for transition-related failures (Requirement 4.3)
		log.Error("Failed to prepare transition block with PoA engine",
			"blockNumber", blockNumber,
			"transitionBlock", h.transitionBlock,
			"signerCount", len(h.initialSigners),
			"error", err)
		return err
	}

	log.Info("Transition block preparation completed successfully",
		"blockNumber", blockNumber,
		"ready", true)

	return nil
}

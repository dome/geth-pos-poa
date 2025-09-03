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

/*
Package hybrid implements a consensus engine that can transition from Proof of Stake (PoS)
to Proof of Authority (PoA) at a specified block number.

The hybrid consensus engine wraps two underlying consensus engines:
- A PoS engine (typically beacon-wrapped clique) used before the transition
- A PoA engine (typically pure clique) used after the transition

The engine automatically selects the appropriate underlying engine based on the block number
being processed. This allows networks to transition from PoS back to PoA when the beacon
chain fails or becomes unavailable.

Usage:

	// Create the underlying engines
	posEngine := beacon.New(clique.New(config.Clique, db))
	poaEngine := clique.New(config.Clique, db)

	// Create hybrid engine with transition at block 1000
	// Initial signers are hardcoded in defaultInitialSigners
	hybridEngine, err := hybrid.New(posEngine, poaEngine, 1000)
	if err != nil {
		log.Fatal("Failed to create hybrid engine:", err)
	}

	// Use hybridEngine as any other consensus.Engine
	// It will automatically use PoS for blocks < 1000 and PoA for blocks >= 1000
	// The transition block (1000) will be prepared as a checkpoint block with hardcoded initial signers

The hybrid engine is thread-safe and implements the full consensus.Engine interface,
delegating all method calls to the appropriate underlying engine based on block number.
*/
package hybrid

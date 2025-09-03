// Copyright 2017 The go-ethereum Authors
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

package params

import (
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckCompatible(t *testing.T) {
	type test struct {
		stored, new   *ChainConfig
		headBlock     uint64
		headTimestamp uint64
		wantErr       *ConfigCompatError
	}
	tests := []test{
		{stored: AllEthashProtocolChanges, new: AllEthashProtocolChanges, headBlock: 0, headTimestamp: 0, wantErr: nil},
		{stored: AllEthashProtocolChanges, new: AllEthashProtocolChanges, headBlock: 0, headTimestamp: uint64(time.Now().Unix()), wantErr: nil},
		{stored: AllEthashProtocolChanges, new: AllEthashProtocolChanges, headBlock: 100, wantErr: nil},
		{
			stored:    &ChainConfig{EIP150Block: big.NewInt(10)},
			new:       &ChainConfig{EIP150Block: big.NewInt(20)},
			headBlock: 9,
			wantErr:   nil,
		},
		{
			stored:    AllEthashProtocolChanges,
			new:       &ChainConfig{HomesteadBlock: nil},
			headBlock: 3,
			wantErr: &ConfigCompatError{
				What:          "Homestead fork block",
				StoredBlock:   big.NewInt(0),
				NewBlock:      nil,
				RewindToBlock: 0,
			},
		},
		{
			stored:    AllEthashProtocolChanges,
			new:       &ChainConfig{HomesteadBlock: big.NewInt(1)},
			headBlock: 3,
			wantErr: &ConfigCompatError{
				What:          "Homestead fork block",
				StoredBlock:   big.NewInt(0),
				NewBlock:      big.NewInt(1),
				RewindToBlock: 0,
			},
		},
		{
			stored:    &ChainConfig{HomesteadBlock: big.NewInt(30), EIP150Block: big.NewInt(10)},
			new:       &ChainConfig{HomesteadBlock: big.NewInt(25), EIP150Block: big.NewInt(20)},
			headBlock: 25,
			wantErr: &ConfigCompatError{
				What:          "EIP150 fork block",
				StoredBlock:   big.NewInt(10),
				NewBlock:      big.NewInt(20),
				RewindToBlock: 9,
			},
		},
		{
			stored:    &ChainConfig{ConstantinopleBlock: big.NewInt(30)},
			new:       &ChainConfig{ConstantinopleBlock: big.NewInt(30), PetersburgBlock: big.NewInt(30)},
			headBlock: 40,
			wantErr:   nil,
		},
		{
			stored:    &ChainConfig{ConstantinopleBlock: big.NewInt(30)},
			new:       &ChainConfig{ConstantinopleBlock: big.NewInt(30), PetersburgBlock: big.NewInt(31)},
			headBlock: 40,
			wantErr: &ConfigCompatError{
				What:          "Petersburg fork block",
				StoredBlock:   nil,
				NewBlock:      big.NewInt(31),
				RewindToBlock: 30,
			},
		},
		{
			stored:        &ChainConfig{ShanghaiTime: newUint64(10)},
			new:           &ChainConfig{ShanghaiTime: newUint64(20)},
			headTimestamp: 9,
			wantErr:       nil,
		},
		{
			stored:        &ChainConfig{ShanghaiTime: newUint64(10)},
			new:           &ChainConfig{ShanghaiTime: newUint64(20)},
			headTimestamp: 25,
			wantErr: &ConfigCompatError{
				What:         "Shanghai fork timestamp",
				StoredTime:   newUint64(10),
				NewTime:      newUint64(20),
				RewindToTime: 9,
			},
		},
	}

	for _, test := range tests {
		err := test.stored.CheckCompatible(test.new, test.headBlock, test.headTimestamp)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("error mismatch:\nstored: %v\nnew: %v\nheadBlock: %v\nheadTimestamp: %v\nerr: %v\nwant: %v", test.stored, test.new, test.headBlock, test.headTimestamp, err, test.wantErr)
		}
	}
}

func TestConfigRules(t *testing.T) {
	c := &ChainConfig{
		LondonBlock:  new(big.Int),
		ShanghaiTime: newUint64(500),
	}
	var stamp uint64
	if r := c.Rules(big.NewInt(0), true, stamp); r.IsShanghai {
		t.Errorf("expected %v to not be shanghai", stamp)
	}
	stamp = 500
	if r := c.Rules(big.NewInt(0), true, stamp); !r.IsShanghai {
		t.Errorf("expected %v to be shanghai", stamp)
	}
	stamp = math.MaxInt64
	if r := c.Rules(big.NewInt(0), true, stamp); !r.IsShanghai {
		t.Errorf("expected %v to be shanghai", stamp)
	}
}

func TestTimestampCompatError(t *testing.T) {
	require.Equal(t, new(ConfigCompatError).Error(), "")

	errWhat := "Shanghai fork timestamp"
	require.Equal(t, newTimestampCompatError(errWhat, nil, newUint64(1681338455)).Error(),
		"mismatching Shanghai fork timestamp in database (have timestamp nil, want timestamp 1681338455, rewindto timestamp 1681338454)")

	require.Equal(t, newTimestampCompatError(errWhat, newUint64(1681338455), nil).Error(),
		"mismatching Shanghai fork timestamp in database (have timestamp 1681338455, want timestamp nil, rewindto timestamp 1681338454)")

	require.Equal(t, newTimestampCompatError(errWhat, newUint64(1681338455), newUint64(600624000)).Error(),
		"mismatching Shanghai fork timestamp in database (have timestamp 1681338455, want timestamp 600624000, rewindto timestamp 600623999)")

	require.Equal(t, newTimestampCompatError(errWhat, newUint64(0), newUint64(1681338455)).Error(),
		"mismatching Shanghai fork timestamp in database (have timestamp 0, want timestamp 1681338455, rewindto timestamp 0)")
}

func TestPoSToPoATransitionValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *ChainConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "no transition configured",
			config: &ChainConfig{
				ChainID: big.NewInt(1),
			},
			wantErr: false,
		},
		{
			name: "valid transition with clique config",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(1000),
				Clique:                  &CliqueConfig{Period: 15, Epoch: 30000},
			},
			wantErr: false,
		},
		{
			name: "transition at genesis block",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(0),
				Clique:                  &CliqueConfig{Period: 15, Epoch: 30000},
			},
			wantErr: false,
		},
		{
			name: "negative transition block",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(-1),
				Clique:                  &CliqueConfig{Period: 15, Epoch: 30000},
			},
			wantErr: true,
			errMsg:  "PoS to PoA transition block cannot be negative",
		},
		{
			name: "transition without clique config",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(1000),
			},
			wantErr: true,
			errMsg:  "PoS to PoA transition requires Clique configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validatePoSToPoATransition()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPoSToPoATransitionCompatibility(t *testing.T) {
	tests := []struct {
		name      string
		stored    *ChainConfig
		new       *ChainConfig
		headBlock uint64
		wantErr   *ConfigCompatError
	}{
		{
			name: "compatible transition blocks",
			stored: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(1000),
			},
			new: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(1000),
			},
			headBlock: 500,
			wantErr:   nil,
		},
		{
			name: "incompatible transition blocks",
			stored: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(1000),
			},
			new: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(2000),
			},
			headBlock: 1500,
			wantErr: &ConfigCompatError{
				What:          "PoS to PoA transition block",
				StoredBlock:   big.NewInt(1000),
				NewBlock:      big.NewInt(2000),
				RewindToBlock: 999,
			},
		},
		{
			name: "transition block before head",
			stored: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(1000),
			},
			new: &ChainConfig{
				ChainID:                 big.NewInt(1),
				PoSToPoATransitionBlock: big.NewInt(2000),
			},
			headBlock: 500,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.stored.CheckCompatible(tt.new, tt.headBlock, 0)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("error mismatch:\nstored: %v\nnew: %v\nheadBlock: %v\nerr: %v\nwant: %v",
					tt.stored, tt.new, tt.headBlock, err, tt.wantErr)
			}
		})
	}
}

func TestIsPoSToPoATransition(t *testing.T) {
	config := &ChainConfig{
		ChainID:                 big.NewInt(1),
		PoSToPoATransitionBlock: big.NewInt(1000),
	}

	tests := []struct {
		blockNumber *big.Int
		expected    bool
	}{
		{big.NewInt(999), false},
		{big.NewInt(1000), true},
		{big.NewInt(1001), true},
	}

	for _, tt := range tests {
		result := config.IsPoSToPoATransition(tt.blockNumber)
		if result != tt.expected {
			t.Errorf("IsPoSToPoATransition(%v) = %v, want %v", tt.blockNumber, result, tt.expected)
		}
	}

	// Test with nil transition block
	configNoTransition := &ChainConfig{
		ChainID: big.NewInt(1),
	}
	if configNoTransition.IsPoSToPoATransition(big.NewInt(1000)) {
		t.Error("IsPoSToPoATransition should return false when no transition block is configured")
	}
}

func TestPoSToPoATransitionJSONMarshaling(t *testing.T) {
	// Test marshaling
	config := &ChainConfig{
		ChainID:                 big.NewInt(1337),
		HomesteadBlock:          big.NewInt(0),
		PoSToPoATransitionBlock: big.NewInt(1000),
		Clique:                  &CliqueConfig{Period: 15, Epoch: 30000},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	// Verify the JSON contains our field
	jsonStr := string(data)
	require.Contains(t, jsonStr, `"posToPoaTransitionBlock":1000`)

	// Test unmarshaling
	var unmarshaled ChainConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify the field was correctly unmarshaled
	require.NotNil(t, unmarshaled.PoSToPoATransitionBlock)
	require.Equal(t, int64(1000), unmarshaled.PoSToPoATransitionBlock.Int64())

	// Test with nil transition block
	configNil := &ChainConfig{
		ChainID:        big.NewInt(1337),
		HomesteadBlock: big.NewInt(0),
		Clique:         &CliqueConfig{Period: 15, Epoch: 30000},
	}

	dataNil, err := json.Marshal(configNil)
	require.NoError(t, err)

	// Verify the JSON does not contain our field when nil
	jsonStrNil := string(dataNil)
	require.NotContains(t, jsonStrNil, "posToPoaTransitionBlock")

	// Test unmarshaling nil case
	var unmarshaledNil ChainConfig
	err = json.Unmarshal(dataNil, &unmarshaledNil)
	require.NoError(t, err)
	require.Nil(t, unmarshaledNil.PoSToPoATransitionBlock)
}

func TestCheckConfigForkOrderWithPoSToPoATransition(t *testing.T) {
	tests := []struct {
		name    string
		config  *ChainConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with transition",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				HomesteadBlock:          big.NewInt(0),
				PoSToPoATransitionBlock: big.NewInt(1000),
				Clique:                  &CliqueConfig{Period: 15, Epoch: 30000},
			},
			wantErr: false,
		},
		{
			name: "invalid config - negative transition block",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				HomesteadBlock:          big.NewInt(0),
				PoSToPoATransitionBlock: big.NewInt(-1),
				Clique:                  &CliqueConfig{Period: 15, Epoch: 30000},
			},
			wantErr: true,
			errMsg:  "invalid PoS to PoA transition configuration: PoS to PoA transition block cannot be negative",
		},
		{
			name: "invalid config - missing clique config",
			config: &ChainConfig{
				ChainID:                 big.NewInt(1),
				HomesteadBlock:          big.NewInt(0),
				PoSToPoATransitionBlock: big.NewInt(1000),
			},
			wantErr: true,
			errMsg:  "invalid PoS to PoA transition configuration: PoS to PoA transition requires Clique configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.CheckConfigForkOrder()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

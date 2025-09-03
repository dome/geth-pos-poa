# Design Document

## Overview

This design implements a hardcoded transition from Proof of Stake (PoS) consensus back to Proof of Authority (Clique) consensus at a specific block number in go-ethereum. The solution involves creating a hybrid consensus engine that can switch between beacon (PoS) and clique (PoA) based on block number.

## Architecture

### Current Consensus Architecture

Go-ethereum currently uses a layered consensus architecture:

1. **Beacon Engine** (`consensus/beacon/consensus.go`) - Wraps the underlying consensus engine for PoS
2. **Clique Engine** (`consensus/clique/clique.go`) - Implements PoA consensus
3. **Engine Creation** (`eth/ethconfig/config.go`) - Creates consensus engines based on chain configuration

The current flow:
```
CreateConsensusEngine() -> beacon.New(clique.New()) or beacon.New(ethash.NewFaker())
```

### Proposed Hybrid Architecture

We will create a new hybrid consensus engine that can switch between PoS and PoA at a specified block number:

```
CreateConsensusEngine() -> hybrid.New(beacon.New(clique.New()), clique.New(), transitionBlock)
```

## Components and Interfaces

### 1. Hybrid Consensus Engine

**Location**: `consensus/hybrid/hybrid.go`

**Purpose**: Manages the transition between PoS and PoA consensus engines based on block number.

**Key Methods**:
- `New(posEngine, poaEngine consensus.Engine, transitionBlock uint64) *Hybrid`
- `shouldUsePoA(blockNumber uint64) bool` - Determines which engine to use
- All `consensus.Engine` interface methods that delegate to appropriate engine

### 2. Chain Configuration Extension

**Location**: `params/config.go`

**Purpose**: Add configuration field for the transition block number.

**New Field**:
```go
type ChainConfig struct {
    // ... existing fields ...
    
    // PoSToPoATransitionBlock specifies the block number at which 
    // the consensus switches from PoS back to PoA. If nil, no transition occurs.
    PoSToPoATransitionBlock *big.Int `json:"posToPoaTransitionBlock,omitempty"`
}
```

### 3. Engine Creation Logic

**Location**: `eth/ethconfig/config.go`

**Purpose**: Modify `CreateConsensusEngine` to create hybrid engine when transition is configured.

**Logic**:
```go
func CreateConsensusEngine(config *params.ChainConfig, db ethdb.Database) (consensus.Engine, error) {
    if config.PoSToPoATransitionBlock != nil {
        // Create hybrid engine with transition
        posEngine := beacon.New(clique.New(config.Clique, db))
        poaEngine := clique.New(config.Clique, db)
        return hybrid.New(posEngine, poaEngine, config.PoSToPoATransitionBlock.Uint64()), nil
    }
    // ... existing logic ...
}
```

## Data Models

### Hybrid Engine Structure

```go
type Hybrid struct {
    posEngine       consensus.Engine  // Beacon-wrapped clique for PoS
    poaEngine       consensus.Engine  // Pure clique for PoA
    transitionBlock uint64           // Block number to switch from PoS to PoA
    mu              sync.RWMutex     // Protects concurrent access
}
```

### Configuration Model

```go
type ChainConfig struct {
    // ... existing fields ...
    PoSToPoATransitionBlock *big.Int `json:"posToPoaTransitionBlock,omitempty"`
}
```

## Error Handling

### Validation Errors
- **Invalid transition block**: Transition block must be > 0 and reasonable
- **Missing clique config**: PoA requires clique configuration
- **Engine creation failures**: Handle failures in creating underlying engines

### Runtime Errors
- **Engine delegation failures**: Properly propagate errors from underlying engines
- **State inconsistencies**: Ensure state remains consistent during transition
- **Concurrent access**: Use proper locking for thread safety

### Error Types
```go
var (
    ErrInvalidTransitionBlock = errors.New("invalid PoS to PoA transition block")
    ErrMissingCliqueConfig   = errors.New("clique configuration required for PoA transition")
    ErrEngineCreationFailed  = errors.New("failed to create consensus engine")
)
```

## Testing Strategy

### Unit Tests

1. **Hybrid Engine Tests** (`consensus/hybrid/hybrid_test.go`)
   - Test engine selection logic based on block number
   - Test delegation to correct underlying engine
   - Test thread safety and concurrent access
   - Test error handling and edge cases

2. **Configuration Tests** (`params/config_test.go`)
   - Test parsing of transition block configuration
   - Test validation of configuration values
   - Test JSON marshaling/unmarshaling

3. **Engine Creation Tests** (`eth/ethconfig/config_test.go`)
   - Test creation of hybrid engine with valid configuration
   - Test fallback to standard engines when no transition configured
   - Test error handling for invalid configurations

### Integration Tests

1. **Blockchain Tests**
   - Test block processing before and after transition
   - Test chain validation across transition boundary
   - Test state consistency during transition

2. **Mining Tests**
   - Test block production using PoS before transition
   - Test block production using PoA after transition
   - Test miner behavior during transition

3. **Network Tests**
   - Test peer synchronization across transition
   - Test fork choice rules during transition
   - Test network consensus after transition

### Test Scenarios

1. **Normal Transition**
   - Chain starts with PoS, transitions to PoA at specified block
   - Verify correct consensus rules applied at each stage

2. **Edge Cases**
   - Transition at genesis block (block 0)
   - Very large transition block numbers
   - Transition block in the past during sync

3. **Error Conditions**
   - Invalid transition block configuration
   - Missing clique configuration
   - Engine creation failures

## Implementation Considerations

### Thread Safety
- Use `sync.RWMutex` to protect concurrent access to engine selection
- Ensure underlying engines are thread-safe

### Performance
- Minimize overhead in engine selection logic
- Cache engine selection decisions where appropriate
- Avoid unnecessary engine switching checks

### Backward Compatibility
- New configuration field is optional (nil means no transition)
- Existing networks continue to work without changes
- Genesis files without transition configuration remain valid

### State Management
- Ensure state transitions are atomic and consistent
- Preserve existing chain state during consensus transition
- Handle any consensus-specific state requirements

### Logging and Monitoring
- Log consensus engine transitions clearly
- Provide metrics for monitoring engine usage
- Include transition information in node status

## Security Considerations

### Consensus Security
- Ensure transition doesn't create consensus vulnerabilities
- Validate that PoA signers are properly configured
- Prevent replay attacks across consensus transitions

### Configuration Security
- Validate transition block numbers to prevent manipulation
- Ensure clique configuration is secure and properly signed
- Prevent unauthorized consensus transitions

### Network Security
- Ensure all nodes transition at the same block
- Prevent network splits due to configuration differences
- Maintain chain integrity across the transition
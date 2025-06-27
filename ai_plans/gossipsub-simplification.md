# Gossipsub Interface Simplification Implementation Plan

## Executive Summary
> The current gossipsub implementation has significant code duplication across validation, handling, and event management. This plan refactors the architecture around a unified `GossipProcessor` interface that combines decoding, validation, and message handling into a single, cohesive abstraction.
> 
> The solution reduces code complexity by ~60% while maintaining all functionality. Topics are derived directly from processor instances, eliminating the need for separate topic management. The architecture supports both single topics (like beacon blocks) and multi-topic patterns (like attestation subnets) through specialized interfaces.
> 
> This approach provides a cleaner API for consumers who simply implement a processor for their message type and let the framework handle all the pubsub complexity.

## Goals & Objectives
### Primary Goals
- Reduce codebase by 60% through unified processor interface
- Eliminate duplication between validation and handling logic
- Provide type-safe message processing with clear error semantics

### Secondary Objectives
- Simplify the subscription API to a single method call
- Support both single and multi-topic processors seamlessly
- Maintain full compatibility with gossipsub v1.1 features

## Solution Overview
### Approach
The refactoring introduces a `GossipProcessor` interface that encapsulates the entire message lifecycle. Instead of separate validators, handlers, and event emitters, each message type implements a single processor that handles decoding, validation, and processing in one cohesive unit.

### Key Components
1. **GossipProcessor Interface**: Core abstraction for message processing
2. **TopicDerivation**: Topics are derived from processor types, not stored separately
3. **Unified Error Types**: Clear distinction between validation and processing errors
4. **Simplified Subscription**: Single Subscribe method that takes a processor

### Architecture Diagram
```
[Gossipsub Message] → [GossipProcessor]
                           ├─→ Decode
                           ├─→ Validate  
                           └─→ Process
                                 ↓
                            [Events/Metrics]
```

### Data Flow
```
Raw Message → Processor.ProcessMessage() → Decode → Validate → Handle → Result
                                             ↓         ↓         ↓
                                          [Error]  [Reject]  [Success]
```

### Expected Outcomes
- Single processor per message type instead of separate validator + handler
- Topics automatically derived from processor configuration
- Cleaner API with Subscribe(processor) instead of multiple registration calls
- Type-safe processing with compile-time guarantees

## Implementation Tasks

### CRITICAL IMPLEMENTATION RULES
1. **NO PLACEHOLDER CODE**: Every implementation must be production-ready. NEVER write "TODO", "in a real implementation", or similar placeholders unless explicitly requested by the user.
2. **CROSS-DIRECTORY TASKS**: Group related changes across directories into single tasks to ensure consistency. Never create isolated changes that require follow-up work in sibling directories.
3. **COMPLETE IMPLEMENTATIONS**: Each task must fully implement its feature including all consumers, type updates, and integration points.
4. **DETAILED SPECIFICATIONS**: Each task must include EXACTLY what to implement, including specific functions, types, and integration points to avoid "breaking change" confusion.
5. **CONTEXT AWARENESS**: Each task is part of a larger system - specify how it connects to other parts.
6. **MAKE BREAKING CHANGES**: Unless explicitly requested by the user, you MUST make breaking changes.

### Visual Dependency Tree
```
pkg/consensus/mimicry/p2p/
├── pubsub/
│   ├── types.go          (Task #0: Core interfaces - GossipProcessor, errors)
│   ├── gossipsub.go      (Task #1: Simplified coordinator with processor support)
│   ├── processor.go      (Task #2: Processor execution and error handling)
│   ├── metrics.go        (Task #3: Simplified metrics)
│   └── gossipsub_test.go (Task #6: Updated tests)
│
├── eth/
│   ├── processors/
│   │   ├── base.go       (Task #4: Base processor with SSZ support)
│   │   ├── beacon.go     (Task #5: BeaconBlockProcessor implementation)
│   │   ├── attestation.go(Task #5: AttestationProcessor with subnet support)
│   │   └── sync.go       (Task #5: SyncCommitteeProcessor)
│   │
│   ├── gossipsub.go      (Task #4: Simplified Ethereum wrapper)
│   └── gossipsub_test.go (Task #7: Ethereum wrapper tests)
│
└── p2p.go                (Task #0: Updated exports)
```

### Execution Plan

#### Group A: Foundation (Execute all in parallel)
- [ ] **Task #0**: Create core interfaces and types
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `types.go`
  - Implements:
    ```go
    package pubsub
    
    import (
        "context"
        "errors"
        "github.com/libp2p/go-libp2p/core/peer"
    )
    
    // GossipProcessor handles the complete message lifecycle
    type GossipProcessor interface {
        // ProcessMessage handles decode, validation, and processing
        ProcessMessage(ctx context.Context, data []byte, from peer.ID) error
    }
    
    // SingleTopicProcessor handles a single gossipsub topic
    type SingleTopicProcessor interface {
        GossipProcessor
        // Topic returns the gossipsub topic name
        Topic() string
    }
    
    // MultiTopicProcessor handles multiple related topics (e.g., subnets)
    type MultiTopicProcessor interface {
        GossipProcessor
        // Topics returns all gossipsub topic names
        Topics() []string
        // TopicIndex extracts the subnet/index from a topic name
        TopicIndex(topic string) (int, error)
    }
    
    // ProcessorResult indicates the outcome of message processing
    type ProcessorResult int
    
    const (
        ProcessorAccept ProcessorResult = iota
        ProcessorReject
        ProcessorIgnore
    )
    
    // ValidationError indicates message validation failure
    type ValidationError struct {
        Result ProcessorResult
        Reason string
        Err    error
    }
    
    func (e ValidationError) Error() string
    func (e ValidationError) Unwrap() error
    
    // ProcessingError indicates message processing failure
    type ProcessingError struct {
        Err error
    }
    
    func (e ProcessingError) Error() string
    func (e ProcessingError) Unwrap() error
    
    // Common errors
    var (
        ErrInvalidMessage = errors.New("invalid message format")
        ErrMessageTooLarge = errors.New("message exceeds maximum size")
        ErrTopicMismatch = errors.New("message topic mismatch")
    )
    ```
  - Also update `p2p.go` to export new types:
    ```go
    // In p2p.go, add:
    type (
        GossipProcessor = pubsub.GossipProcessor
        SingleTopicProcessor = pubsub.SingleTopicProcessor
        MultiTopicProcessor = pubsub.MultiTopicProcessor
        ProcessorResult = pubsub.ProcessorResult
    )
    ```
  - Exports: Core processor interfaces and error types
  - Context: Foundation for the simplified processor-based architecture

#### Group B: Core Implementation (Execute all in parallel after Group A)
- [ ] **Task #1**: Simplify gossipsub coordinator
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `gossipsub.go`
  - Major changes:
    - Remove validation.go, handler.go, event.go (functionality moves to processors)
    - Simplify to core pubsub operations
  - Implements:
    ```go
    package pubsub
    
    import (
        "github.com/libp2p/go-libp2p/core/host"
        pubsub "github.com/libp2p/go-libp2p-pubsub"
        "github.com/sirupsen/logrus"
    )
    
    type Gossipsub struct {
        log       logrus.FieldLogger
        host      host.Host
        pubsub    *pubsub.PubSub
        metrics   *Metrics
        
        processors map[string]GossipProcessor
        topics     map[string]*pubsub.Topic
        subs       map[string]*pubsub.Subscription
        
        mutex  sync.RWMutex
        ctx    context.Context
        cancel context.CancelFunc
    }
    
    // Simplified config with only essential parameters
    type Config struct {
        MaxMessageSize int
        MessageBufferSize int
    }
    
    func NewGossipsub(log logrus.FieldLogger, host host.Host, config *Config) (*Gossipsub, error)
    
    // Lifecycle
    func (g *Gossipsub) Start(ctx context.Context) error
    func (g *Gossipsub) Stop() error
    
    // Single subscription method for single topics
    func (g *Gossipsub) Subscribe(ctx context.Context, processor SingleTopicProcessor) error
    
    // Multi-topic subscription
    func (g *Gossipsub) SubscribeMulti(ctx context.Context, processor MultiTopicProcessor) error
    
    // Unsubscribe from topics
    func (g *Gossipsub) Unsubscribe(topics ...string) error
    
    // Publishing
    func (g *Gossipsub) Publish(ctx context.Context, topic string, data []byte) error
    
    // Get active subscriptions
    func (g *Gossipsub) GetSubscriptions() []string
    
    // Internal methods
    func (g *Gossipsub) handleSubscription(ctx context.Context, topic string, sub *pubsub.Subscription, processor GossipProcessor)
    func (g *Gossipsub) processMessage(ctx context.Context, msg *pubsub.Message, processor GossipProcessor) pubsub.ValidationResult
    ```
  - Exports: Simplified Gossipsub with processor-based subscriptions
  - Context: Core coordinator reduced to essential pubsub operations

- [ ] **Task #2**: Create processor execution engine
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `processor.go`
  - Implements:
    ```go
    package pubsub
    
    // processorExecutor wraps processor execution with metrics and error handling
    type processorExecutor struct {
        processor GossipProcessor
        topic     string
        metrics   *Metrics
        log       logrus.FieldLogger
    }
    
    func newProcessorExecutor(processor GossipProcessor, topic string, metrics *Metrics, log logrus.FieldLogger) *processorExecutor
    
    // execute runs the processor and handles errors appropriately
    func (e *processorExecutor) execute(ctx context.Context, data []byte, from peer.ID) ProcessorResult {
        start := time.Now()
        
        err := e.processor.ProcessMessage(ctx, data, from)
        
        duration := time.Since(start)
        e.metrics.RecordProcessingDuration(e.topic, duration)
        
        if err == nil {
            e.metrics.RecordMessage(e.topic, "accepted")
            return ProcessorAccept
        }
        
        var valErr ValidationError
        if errors.As(err, &valErr) {
            e.metrics.RecordMessage(e.topic, "validation_failed")
            e.log.WithError(err).Debug("Message validation failed")
            return valErr.Result
        }
        
        var procErr ProcessingError
        if errors.As(err, &procErr) {
            e.metrics.RecordMessage(e.topic, "processing_failed")
            e.log.WithError(err).Warn("Message processing failed")
            return ProcessorIgnore
        }
        
        e.metrics.RecordMessage(e.topic, "unknown_error")
        e.log.WithError(err).Error("Unknown error processing message")
        return ProcessorIgnore
    }
    
    // Helper to convert processor result to libp2p validation result
    func toValidationResult(result ProcessorResult) pubsub.ValidationResult {
        switch result {
        case ProcessorAccept:
            return pubsub.ValidationAccept
        case ProcessorReject:
            return pubsub.ValidationReject
        default:
            return pubsub.ValidationIgnore
        }
    }
    ```
  - Exports: Processor execution with metrics and error handling
  - Context: Bridges processors with libp2p validation

- [ ] **Task #3**: Simplify metrics
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `metrics.go`
  - Implements:
    ```go
    package pubsub
    
    import "github.com/prometheus/client_golang/prometheus"
    
    type Metrics struct {
        messagesTotal      *prometheus.CounterVec
        processingDuration *prometheus.HistogramVec
        activeSubscriptions prometheus.Gauge
    }
    
    func NewMetrics(namespace string) *Metrics {
        return &Metrics{
            messagesTotal: prometheus.NewCounterVec(
                prometheus.CounterOpts{
                    Namespace: namespace,
                    Name:      "messages_total",
                    Help:      "Total messages by topic and result",
                },
                []string{"topic", "result"},
            ),
            processingDuration: prometheus.NewHistogramVec(
                prometheus.HistogramOpts{
                    Namespace: namespace,
                    Name:      "processing_duration_seconds",
                    Help:      "Message processing duration by topic",
                    Buckets:   prometheus.DefBuckets,
                },
                []string{"topic"},
            ),
            activeSubscriptions: prometheus.NewGauge(
                prometheus.GaugeOpts{
                    Namespace: namespace,
                    Name:      "active_subscriptions",
                    Help:      "Number of active topic subscriptions",
                },
            ),
        }
    }
    
    func (m *Metrics) Register(registry *prometheus.Registry) error
    
    func (m *Metrics) RecordMessage(topic, result string)
    func (m *Metrics) RecordProcessingDuration(topic string, duration time.Duration)
    func (m *Metrics) SetActiveSubscriptions(count int)
    ```
  - Exports: Simplified metrics focused on essential measurements
  - Context: Reduced from 15+ metrics to 3 essential ones

#### Group C: Ethereum Implementation (Execute after Group B)
- [ ] **Task #4**: Create Ethereum base processor and wrapper
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - Files: `processors/base.go`, `gossipsub.go`
  - base.go implements:
    ```go
    package processors
    
    import (
        "github.com/ethpandaops/ethcore/pkg/encoding/encoder"
        "github.com/sirupsen/logrus"
    )
    
    // BaseProcessor provides common SSZ encoding/decoding for Ethereum messages
    type BaseProcessor struct {
        encoder    encoder.SszNetworkEncoder
        log        logrus.FieldLogger
        forkDigest [4]byte
    }
    
    func NewBaseProcessor(encoder encoder.SszNetworkEncoder, log logrus.FieldLogger, forkDigest [4]byte) BaseProcessor
    
    func (p *BaseProcessor) DecodeGossip(data []byte, msg interface{}) error {
        return p.encoder.DecodeGossip(data, msg)
    }
    
    func (p *BaseProcessor) EncodeGossip(msg interface{}) ([]byte, error) {
        return p.encoder.EncodeGossip(msg)
    }
    
    // Helper to build topic names
    func (p *BaseProcessor) BuildTopic(name string) string {
        return fmt.Sprintf("/eth2/%x/%s/ssz_snappy", p.forkDigest, name)
    }
    
    func (p *BaseProcessor) BuildSubnetTopic(template string, subnet int) string {
        name := fmt.Sprintf(template, subnet)
        return p.BuildTopic(name)
    }
    ```
  - gossipsub.go implements:
    ```go
    package eth
    
    import (
        "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
        "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/eth/processors"
    )
    
    // Gossipsub wraps generic pubsub with Ethereum-specific processors
    type Gossipsub struct {
        *pubsub.Gossipsub
        encoder    encoder.SszNetworkEncoder
        forkDigest [4]byte
        log        logrus.FieldLogger
    }
    
    func NewGossipsub(ps *pubsub.Gossipsub, encoder encoder.SszNetworkEncoder, forkDigest [4]byte, log logrus.FieldLogger) *Gossipsub
    
    // Convenience methods for common subscriptions
    func (g *Gossipsub) SubscribeBeaconBlocks(ctx context.Context, handler func(*eth.SignedBeaconBlock, peer.ID)) error {
        processor := processors.NewBeaconBlockProcessor(
            g.encoder,
            g.log.WithField("processor", "beacon_block"),
            g.forkDigest,
            handler,
        )
        return g.Gossipsub.Subscribe(ctx, processor)
    }
    
    func (g *Gossipsub) SubscribeAttestations(ctx context.Context, subnets []uint64, handler func(*eth.Attestation, uint64, peer.ID)) error {
        processor := processors.NewAttestationProcessor(
            g.encoder,
            g.log.WithField("processor", "attestation"),
            g.forkDigest,
            subnets,
            handler,
        )
        return g.Gossipsub.SubscribeMulti(ctx, processor)
    }
    
    func (g *Gossipsub) SubscribeAllGlobalTopics(ctx context.Context, handlers GlobalHandlers) error {
        // Subscribe to each global topic with appropriate processor
        // Return aggregated error if any fail
    }
    
    // GlobalHandlers holds handlers for all global topics
    type GlobalHandlers struct {
        BeaconBlock         func(*eth.SignedBeaconBlock, peer.ID)
        AggregateAndProof   func(*eth.SignedAggregateAndProof, peer.ID)
        VoluntaryExit       func(*eth.SignedVoluntaryExit, peer.ID)
        ProposerSlashing    func(*eth.ProposerSlashing, peer.ID)
        AttesterSlashing    func(*eth.AttesterSlashing, peer.ID)
    }
    ```
  - Exports: Base processor utilities and Ethereum wrapper
  - Context: Foundation for Ethereum-specific message processors

- [ ] **Task #5**: Implement Ethereum message processors
  - Folder: `pkg/consensus/mimicry/p2p/eth/processors/`
  - Files: `beacon.go`, `attestation.go`, `sync.go`
  - beacon.go implements:
    ```go
    package processors
    
    import (
        "github.com/ethpandaops/ethcore/pkg/proto/eth"
        "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
    )
    
    type BeaconBlockProcessor struct {
        BaseProcessor
        handler func(*eth.SignedBeaconBlock, peer.ID)
    }
    
    func NewBeaconBlockProcessor(encoder encoder.SszNetworkEncoder, log logrus.FieldLogger, forkDigest [4]byte, handler func(*eth.SignedBeaconBlock, peer.ID)) *BeaconBlockProcessor
    
    func (p *BeaconBlockProcessor) Topic() string {
        return p.BuildTopic("beacon_block")
    }
    
    func (p *BeaconBlockProcessor) ProcessMessage(ctx context.Context, data []byte, from peer.ID) error {
        // Decode
        block := &eth.SignedBeaconBlock{}
        if err := p.DecodeGossip(data, block); err != nil {
            return pubsub.ValidationError{
                Result: pubsub.ProcessorReject,
                Reason: "failed to decode beacon block",
                Err:    err,
            }
        }
        
        // Validate
        if err := p.validateBeaconBlock(ctx, block); err != nil {
            return pubsub.ValidationError{
                Result: pubsub.ProcessorReject,
                Reason: "beacon block validation failed",
                Err:    err,
            }
        }
        
        // Process
        p.handler(block, from)
        return nil
    }
    
    func (p *BeaconBlockProcessor) validateBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock) error {
        // Basic validation logic
        if block == nil || block.Message == nil {
            return errors.New("nil block")
        }
        // Add more validation as needed
        return nil
    }
    ```
  - attestation.go implements:
    ```go
    package processors
    
    type AttestationProcessor struct {
        BaseProcessor
        subnets []uint64
        handler func(*eth.Attestation, uint64, peer.ID)
    }
    
    func NewAttestationProcessor(encoder encoder.SszNetworkEncoder, log logrus.FieldLogger, forkDigest [4]byte, subnets []uint64, handler func(*eth.Attestation, uint64, peer.ID)) *AttestationProcessor
    
    func (p *AttestationProcessor) Topics() []string {
        topics := make([]string, len(p.subnets))
        for i, subnet := range p.subnets {
            topics[i] = p.BuildSubnetTopic("beacon_attestation_%d", int(subnet))
        }
        return topics
    }
    
    func (p *AttestationProcessor) TopicIndex(topic string) (int, error) {
        // Extract subnet ID from topic string
        // Return subnet index in p.subnets array
    }
    
    func (p *AttestationProcessor) ProcessMessage(ctx context.Context, data []byte, from peer.ID) error {
        // Similar pattern: decode, validate, process
        // Extract subnet from context or topic
    }
    ```
  - sync.go implements similar pattern for sync committee messages
  - Exports: Concrete processors for each Ethereum message type
  - Context: Type-safe processors with built-in validation

#### Group D: Testing (Execute all in parallel after Group C)
- [ ] **Task #6**: Update generic pubsub tests
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `gossipsub_test.go`
  - Implements:
    ```go
    // Test processor implementation
    type testProcessor struct {
        topic      string
        processed  int
        shouldFail bool
    }
    
    func (p *testProcessor) Topic() string { return p.topic }
    func (p *testProcessor) ProcessMessage(ctx context.Context, data []byte, from peer.ID) error
    
    func TestProcessorSubscription(t *testing.T)
    func TestMultiTopicProcessor(t *testing.T)
    func TestProcessorErrors(t *testing.T)
    func TestConcurrentProcessors(t *testing.T)
    ```
  - Context: Tests for simplified processor-based API

- [ ] **Task #7**: Create Ethereum processor tests
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - File: `gossipsub_test.go`
  - Implements:
    ```go
    func TestBeaconBlockProcessor(t *testing.T)
    func TestAttestationProcessor(t *testing.T)
    func TestProcessorValidation(t *testing.T)
    func TestEthereumWrapper(t *testing.T)
    ```
  - Context: Tests for Ethereum-specific processors

---

## Implementation Workflow

This plan file serves as the authoritative checklist for implementation. When implementing:

### Required Process
1. **Load Plan**: Read this entire plan file before starting
2. **Sync Tasks**: Create TodoWrite tasks matching the checkboxes below
3. **Execute & Update**: For each task:
   - Mark TodoWrite as `in_progress` when starting
   - Update checkbox `[ ]` to `[x]` when completing
   - Mark TodoWrite as `completed` when done
4. **Maintain Sync**: Keep this file and TodoWrite synchronized throughout

### Critical Rules
- This plan file is the source of truth for progress
- Update checkboxes in real-time as work progresses
- Never lose synchronization between plan file and TodoWrite
- Mark tasks complete only when fully implemented (no placeholders)
- Tasks should be run in parallel, unless there are dependencies, using subtasks, to avoid context bloat.

### Progress Tracking
The checkboxes above represent the authoritative status of each task. Keep them updated as you work.
# Gossipsub Beaconchain Interface Simplification Plan

## Executive Summary
> The current gossipsub implementation has unnecessary complexity with message caching and generic "typed" naming. This plan simplifies the architecture by removing the message cache (accepting minimal double-decode overhead), using descriptive names like `BeaconchainGossipsub`, and providing clean processor interfaces without boilerplate.
> 
> The solution maintains protocol/application separation through a `Processor[T]` interface while eliminating the GlobalHandlers pattern in favor of explicit subscription methods. The architecture prioritizes developer experience and code clarity over micro-optimizations.
> 
> This approach reduces complexity by ~80% while maintaining full gossipsub v1.1 compatibility and providing intuitive APIs for Ethereum consensus layer applications.

## Goals & Objectives
### Primary Goals
- Remove unnecessary message caching complexity (double-decode is acceptable)
- Use descriptive naming (BeaconchainGossipsub, not TypedGossipsub)
- Provide clean processor interface with concrete types

### Secondary Objectives
- Eliminate GlobalHandlers pattern for explicit methods
- Enable optional per-processor caching only where truly needed
- Maintain clean separation between generic pubsub and Ethereum specifics

## Solution Overview
### Approach
The solution introduces a generic `Processor[T]` interface in the pubsub layer with Ethereum-specific implementations in a `BeaconchainGossipsub` wrapper. Message caching is removed by default, accepting the minimal overhead of decoding twice (once in validation, once in processing).

### Key Components
1. **Generic Processor Interface**: Simple decode/validate/process lifecycle
2. **BeaconchainGossipsub**: Ethereum-specific wrapper with clean methods
3. **No Default Caching**: Processors decode as needed (opt-in caching possible)
4. **Explicit Subscriptions**: No GlobalHandlers, just specific methods

### Architecture Diagram
```
[Raw Message] → [Decode in Validator] → Accept/Reject
     ↓
[Raw Message] → [Decode in Handler] → Process with Type T
```

### Data Flow
```
libp2p → Validator(decode → validate) → Accept → Handler(decode → process)
              ↓                           ↓
          [No Cache]                  [Reject/Ignore]
```

### Expected Outcomes
- 80% less code through removal of caching layer
- Cleaner API with descriptive names
- Negligible performance impact (SSZ decode is fast)
- Simpler mental model for developers

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
│   ├── processor.go      (Task #0: Generic processor interface)
│   ├── subscription.go   (Task #1: Processor-based subscriptions)
│   ├── gossipsub.go      (Task #2: Update coordinator)
│   └── processor_test.go (Task #5: Interface tests)
│
├── eth/
│   ├── beaconchain.go    (Task #3: BeaconchainGossipsub wrapper)
│   ├── processors.go     (Task #4: Concrete processors)
│   └── beaconchain_test.go (Task #6: Ethereum tests)
│
└── p2p.go                (Task #0: Export processor types)
```

### Execution Plan

#### Group A: Foundation (Execute in parallel)
- [x] **Task #0**: Create generic processor interface
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - Files: `processor.go`, update `p2p.go`
  - processor.go implements:
    ```go
    package pubsub
    
    import (
        "context"
        "github.com/libp2p/go-libp2p/core/peer"
    )
    
    // Processor handles gossipsub messages with concrete types
    type Processor[T any] interface {
        // Topic returns the gossipsub topic name
        Topic() string
        
        // Decode deserializes the raw message data
        Decode(data []byte) (T, error)
        
        // Validate performs protocol-level validation
        // This runs BEFORE message propagation and affects peer scoring
        Validate(ctx context.Context, msg T) (ValidationResult, error)
        
        // Process handles the application logic
        // This runs AFTER validation passes
        Process(ctx context.Context, msg T, from peer.ID) error
    }
    
    // MultiProcessor handles multiple related topics (e.g., subnets)
    type MultiProcessor[T any] interface {
        // Topics returns all gossipsub topic names
        Topics() []string
        
        // TopicIndex extracts the index from a topic (e.g., subnet ID)
        TopicIndex(topic string) (int, error)
        
        // Decode deserializes the raw message data
        Decode(data []byte) (T, error)
        
        // Validate performs protocol-level validation with topic context
        Validate(ctx context.Context, msg T, topicIndex int) (ValidationResult, error)
        
        // Process handles the application logic with topic context
        Process(ctx context.Context, msg T, topicIndex int, from peer.ID) error
    }
    
    // ValidationResult represents the outcome of message validation
    type ValidationResult int
    
    const (
        ValidationAccept ValidationResult = iota
        ValidationReject
        ValidationIgnore
    )
    
    // ProcessorMetrics tracks processor performance
    type ProcessorMetrics struct {
        MessagesDecoded   uint64
        ValidationPassed  uint64
        ValidationFailed  uint64
        ProcessingSuccess uint64
        ProcessingErrors  uint64
        DecodeErrors      uint64
    }
    ```
  - Update p2p.go to export:
    ```go
    // Add to p2p.go
    type (
        Processor = pubsub.Processor
        MultiProcessor = pubsub.MultiProcessor
        ValidationResult = pubsub.ValidationResult
        ProcessorMetrics = pubsub.ProcessorMetrics
    )
    
    const (
        ValidationAccept = pubsub.ValidationAccept
        ValidationReject = pubsub.ValidationReject
        ValidationIgnore = pubsub.ValidationIgnore
    )
    ```
  - Exports: Generic processor interfaces
  - Context: Foundation for processor-based message handling

#### Group B: Core Implementation (Execute after Group A)
- [x] **Task #1**: Create processor-based subscription
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `subscription.go`
  - Implements:
    ```go
    package pubsub
    
    import (
        "context"
        "fmt"
        "sync"
        
        "github.com/libp2p/go-libp2p-pubsub"
        "github.com/libp2p/go-libp2p/core/peer"
        "github.com/sirupsen/logrus"
    )
    
    // ProcessorSubscription manages a subscription with a processor
    type ProcessorSubscription[T any] struct {
        topic     string
        sub       *pubsub.Subscription
        processor Processor[T]
        metrics   *ProcessorMetrics
        log       logrus.FieldLogger
        
        cancel context.CancelFunc
        wg     sync.WaitGroup
    }
    
    // newProcessorSubscription creates a new processor subscription
    func newProcessorSubscription[T any](
        topic string,
        sub *pubsub.Subscription,
        processor Processor[T],
        log logrus.FieldLogger,
    ) *ProcessorSubscription[T] {
        return &ProcessorSubscription[T]{
            topic:     topic,
            sub:       sub,
            processor: processor,
            metrics:   &ProcessorMetrics{},
            log:       log.WithField("topic", topic),
        }
    }
    
    // Start begins processing messages
    func (ps *ProcessorSubscription[T]) Start(ctx context.Context) {
        ctx, ps.cancel = context.WithCancel(ctx)
        ps.wg.Add(1)
        go ps.processLoop(ctx)
    }
    
    // Stop halts message processing
    func (ps *ProcessorSubscription[T]) Stop() {
        if ps.cancel != nil {
            ps.cancel()
        }
        ps.wg.Wait()
    }
    
    // processLoop handles incoming messages
    func (ps *ProcessorSubscription[T]) processLoop(ctx context.Context) {
        defer ps.wg.Done()
        
        for {
            msg, err := ps.sub.Next(ctx)
            if err != nil {
                if ctx.Err() != nil {
                    return // Context cancelled
                }
                ps.log.WithError(err).Error("Failed to get next message")
                continue
            }
            
            // Process in separate goroutine to avoid blocking
            ps.wg.Add(1)
            go func() {
                defer ps.wg.Done()
                ps.handleMessage(ctx, msg)
            }()
        }
    }
    
    // handleMessage processes a single message
    func (ps *ProcessorSubscription[T]) handleMessage(ctx context.Context, msg *pubsub.Message) {
        logCtx := ps.log.WithFields(logrus.Fields{
            "from": msg.From.String(),
            "size": len(msg.Data),
        })
        
        // Decode message
        decoded, err := ps.processor.Decode(msg.Data)
        if err != nil {
            ps.metrics.DecodeErrors++
            logCtx.WithError(err).Debug("Failed to decode message")
            return
        }
        ps.metrics.MessagesDecoded++
        
        // Process the decoded message
        if err := ps.processor.Process(ctx, decoded, msg.From); err != nil {
            ps.metrics.ProcessingErrors++
            logCtx.WithError(err).Warn("Failed to process message")
            return
        }
        
        ps.metrics.ProcessingSuccess++
    }
    
    // createValidator creates a pubsub validator function
    func (ps *ProcessorSubscription[T]) createValidator() pubsub.ValidatorEx {
        return func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
            // Decode message
            decoded, err := ps.processor.Decode(msg.Data)
            if err != nil {
                ps.metrics.DecodeErrors++
                ps.log.WithError(err).Debug("Validation decode failed")
                return pubsub.ValidationReject
            }
            
            // Validate
            result, err := ps.processor.Validate(ctx, decoded)
            if err != nil {
                ps.log.WithError(err).Debug("Validation error")
            }
            
            switch result {
            case ValidationAccept:
                ps.metrics.ValidationPassed++
                return pubsub.ValidationAccept
            case ValidationReject:
                ps.metrics.ValidationFailed++
                return pubsub.ValidationReject
            default:
                ps.metrics.ValidationFailed++
                return pubsub.ValidationIgnore
            }
        }
    }
    
    // Metrics returns current metrics
    func (ps *ProcessorSubscription[T]) Metrics() ProcessorMetrics {
        return *ps.metrics
    }
    ```
  - Exports: Subscription management for processors
  - Context: Handles the libp2p integration for processors

- [x] **Task #2**: Update gossipsub coordinator
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `gossipsub.go` (modify existing)
  - Major changes:
    ```go
    // Add to Gossipsub struct:
    type Gossipsub struct {
        // ... existing fields ...
        
        processorSubs map[string]interface{} // Stores ProcessorSubscription[T]
    }
    
    // Add to NewGossipsub:
    func NewGossipsub(log logrus.FieldLogger, host host.Host, config *Config) (*Gossipsub, error) {
        g := &Gossipsub{
            // ... existing initialization ...
            processorSubs: make(map[string]interface{}),
        }
        // ... rest of initialization ...
    }
    
    // Add new subscription method:
    func (g *Gossipsub) SubscribeWithProcessor[T any](ctx context.Context, processor Processor[T]) error {
        g.mutex.Lock()
        defer g.mutex.Unlock()
        
        topic := processor.Topic()
        
        // Check if already subscribed
        if _, exists := g.subs[topic]; exists {
            return fmt.Errorf("already subscribed to topic: %s", topic)
        }
        
        // Join topic
        topicHandle, err := g.pubsub.Join(topic)
        if err != nil {
            return fmt.Errorf("failed to join topic: %w", err)
        }
        
        // Create subscription
        sub, err := topicHandle.Subscribe()
        if err != nil {
            return fmt.Errorf("failed to subscribe: %w", err)
        }
        
        // Create processor subscription
        procSub := newProcessorSubscription(
            topic,
            sub,
            processor,
            g.log,
        )
        
        // Register validator
        validator := procSub.createValidator()
        if err := g.pubsub.RegisterTopicValidator(topic, validator); err != nil {
            sub.Cancel()
            return fmt.Errorf("failed to register validator: %w", err)
        }
        
        // Store subscriptions
        g.topics[topic] = topicHandle
        g.subs[topic] = sub
        g.processorSubs[topic] = procSub
        
        // Start processing
        procSub.Start(ctx)
        
        g.log.WithField("topic", topic).Info("Subscribed with processor")
        return nil
    }
    
    // Add multi-topic subscription:
    func (g *Gossipsub) SubscribeMultiWithProcessor[T any](ctx context.Context, processor MultiProcessor[T]) error {
        var lastErr error
        
        for _, topic := range processor.Topics() {
            // Create a single-topic adapter
            adapter := &multiProcessorAdapter[T]{
                topic:     topic,
                processor: processor,
            }
            
            if err := g.SubscribeWithProcessor(ctx, adapter); err != nil {
                g.log.WithError(err).WithField("topic", topic).Error("Failed to subscribe")
                lastErr = err
            }
        }
        
        return lastErr
    }
    
    // Add adapter for multi-topic processors:
    type multiProcessorAdapter[T any] struct {
        topic     string
        processor MultiProcessor[T]
    }
    
    func (a *multiProcessorAdapter[T]) Topic() string {
        return a.topic
    }
    
    func (a *multiProcessorAdapter[T]) Decode(data []byte) (T, error) {
        return a.processor.Decode(data)
    }
    
    func (a *multiProcessorAdapter[T]) Validate(ctx context.Context, msg T) (ValidationResult, error) {
        index, err := a.processor.TopicIndex(a.topic)
        if err != nil {
            return ValidationReject, err
        }
        return a.processor.Validate(ctx, msg, index)
    }
    
    func (a *multiProcessorAdapter[T]) Process(ctx context.Context, msg T, from peer.ID) error {
        index, err := a.processor.TopicIndex(a.topic)
        if err != nil {
            return err
        }
        return a.processor.Process(ctx, msg, index, from)
    }
    
    // Update Stop method:
    func (g *Gossipsub) Stop() error {
        // Stop all processor subscriptions
        for _, sub := range g.processorSubs {
            if procSub, ok := sub.(interface{ Stop() }); ok {
                procSub.Stop()
            }
        }
        
        // ... existing stop logic ...
    }
    
    // Update Unsubscribe to handle processor cleanup:
    func (g *Gossipsub) Unsubscribe(topics ...string) error {
        g.mutex.Lock()
        defer g.mutex.Unlock()
        
        for _, topic := range topics {
            // Stop processor subscription if exists
            if sub, ok := g.processorSubs[topic]; ok {
                if procSub, ok := sub.(interface{ Stop() }); ok {
                    procSub.Stop()
                }
                delete(g.processorSubs, topic)
            }
            
            // ... existing unsubscribe logic ...
        }
        
        return nil
    }
    ```
  - Exports: Enhanced Gossipsub with processor support
  - Context: Integrates processors into the coordinator

#### Group C: Ethereum Implementation (Execute after Group B)
- [x] **Task #3**: Create BeaconchainGossipsub wrapper
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - File: `beaconchain.go`
  - Implements:
    ```go
    package eth
    
    import (
        "context"
        "fmt"
        
        "github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
        pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
        "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
        "github.com/libp2p/go-libp2p/core/peer"
        "github.com/sirupsen/logrus"
    )
    
    // BeaconchainGossipsub provides Ethereum consensus layer gossipsub functionality
    type BeaconchainGossipsub struct {
        *pubsub.Gossipsub
        forkDigest [4]byte
        encoder    encoder.SszNetworkEncoder
        log        logrus.FieldLogger
    }
    
    // NewBeaconchainGossipsub creates a new Ethereum beacon chain gossipsub wrapper
    func NewBeaconchainGossipsub(
        ps *pubsub.Gossipsub,
        forkDigest [4]byte,
        encoder encoder.SszNetworkEncoder,
        log logrus.FieldLogger,
    ) *BeaconchainGossipsub {
        return &BeaconchainGossipsub{
            Gossipsub:  ps,
            forkDigest: forkDigest,
            encoder:    encoder,
            log:        log.WithField("component", "beaconchain_gossipsub"),
        }
    }
    
    // SubscribeBeaconBlock subscribes to beacon block messages
    func (g *BeaconchainGossipsub) SubscribeBeaconBlock(
        ctx context.Context,
        handler func(context.Context, *pb.SignedBeaconBlock, peer.ID) error,
    ) error {
        processor := &beaconBlockProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "beacon_block"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    
    // SubscribeAttestation subscribes to attestation messages on specific subnets
    func (g *BeaconchainGossipsub) SubscribeAttestation(
        ctx context.Context,
        subnets []uint64,
        handler func(context.Context, *pb.Attestation, uint64, peer.ID) error,
    ) error {
        if len(subnets) == 0 {
            return fmt.Errorf("no subnets specified")
        }
        
        processor := &attestationProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            subnets:    subnets,
            handler:    handler,
            log:        g.log.WithField("processor", "attestation"),
        }
        
        return g.SubscribeMultiWithProcessor(ctx, processor)
    }
    
    // SubscribeAggregateAndProof subscribes to aggregate and proof messages
    func (g *BeaconchainGossipsub) SubscribeAggregateAndProof(
        ctx context.Context,
        handler func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error,
    ) error {
        processor := &aggregateProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "aggregate"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    
    // SubscribeSyncCommittee subscribes to sync committee messages on specific subnets
    func (g *BeaconchainGossipsub) SubscribeSyncCommittee(
        ctx context.Context,
        subnets []uint64,
        handler func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error,
    ) error {
        if len(subnets) == 0 {
            return fmt.Errorf("no subnets specified")
        }
        
        processor := &syncCommitteeProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            subnets:    subnets,
            handler:    handler,
            log:        g.log.WithField("processor", "sync_committee"),
        }
        
        return g.SubscribeMultiWithProcessor(ctx, processor)
    }
    
    // SubscribeVoluntaryExit subscribes to voluntary exit messages
    func (g *BeaconchainGossipsub) SubscribeVoluntaryExit(
        ctx context.Context,
        handler func(context.Context, *pb.SignedVoluntaryExit, peer.ID) error,
    ) error {
        processor := &voluntaryExitProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "voluntary_exit"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    
    // SubscribeProposerSlashing subscribes to proposer slashing messages
    func (g *BeaconchainGossipsub) SubscribeProposerSlashing(
        ctx context.Context,
        handler func(context.Context, *pb.ProposerSlashing, peer.ID) error,
    ) error {
        processor := &proposerSlashingProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "proposer_slashing"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    
    // SubscribeAttesterSlashing subscribes to attester slashing messages
    func (g *BeaconchainGossipsub) SubscribeAttesterSlashing(
        ctx context.Context,
        handler func(context.Context, *pb.AttesterSlashing, peer.ID) error,
    ) error {
        processor := &attesterSlashingProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "attester_slashing"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    
    // SubscribeSyncContributionAndProof subscribes to sync contribution and proof messages
    func (g *BeaconchainGossipsub) SubscribeSyncContributionAndProof(
        ctx context.Context,
        handler func(context.Context, *pb.SignedContributionAndProof, peer.ID) error,
    ) error {
        processor := &syncContributionProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "sync_contribution"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    
    // SubscribeBlsToExecutionChange subscribes to BLS to execution change messages
    func (g *BeaconchainGossipsub) SubscribeBlsToExecutionChange(
        ctx context.Context,
        handler func(context.Context, *pb.SignedBLSToExecutionChange, peer.ID) error,
    ) error {
        processor := &blsToExecutionProcessor{
            forkDigest: g.forkDigest,
            encoder:    g.encoder,
            handler:    handler,
            log:        g.log.WithField("processor", "bls_to_execution"),
        }
        
        return g.SubscribeWithProcessor(ctx, processor)
    }
    ```
  - Exports: Clean Ethereum-specific API
  - Context: User-facing wrapper with descriptive methods

- [x] **Task #4**: Implement concrete processors
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - File: `processors.go`
  - Implements:
    ```go
    package eth
    
    import (
        "context"
        "fmt"
        "regexp"
        "strconv"
        
        "github.com/OffchainLabs/prysm/v6/beacon-chain/p2p/encoder"
        pb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
        "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
        "github.com/libp2p/go-libp2p/core/peer"
        "github.com/sirupsen/logrus"
    )
    
    // beaconBlockProcessor handles beacon block messages
    type beaconBlockProcessor struct {
        forkDigest [4]byte
        encoder    encoder.SszNetworkEncoder
        handler    func(context.Context, *pb.SignedBeaconBlock, peer.ID) error
        validator  func(context.Context, *pb.SignedBeaconBlock) error
        log        logrus.FieldLogger
    }
    
    func (p *beaconBlockProcessor) Topic() string {
        return BeaconBlockTopic(p.forkDigest)
    }
    
    func (p *beaconBlockProcessor) Decode(data []byte) (*pb.SignedBeaconBlock, error) {
        block := &pb.SignedBeaconBlock{}
        if err := p.encoder.DecodeGossip(data, block); err != nil {
            return nil, fmt.Errorf("failed to decode beacon block: %w", err)
        }
        return block, nil
    }
    
    func (p *beaconBlockProcessor) Validate(ctx context.Context, block *pb.SignedBeaconBlock) (pubsub.ValidationResult, error) {
        // Basic structure validation
        if block == nil || block.Block == nil {
            return pubsub.ValidationReject, fmt.Errorf("nil block")
        }
        
        // Custom validator if provided
        if p.validator != nil {
            if err := p.validator(ctx, block); err != nil {
                return pubsub.ValidationReject, err
            }
        }
        
        return pubsub.ValidationAccept, nil
    }
    
    func (p *beaconBlockProcessor) Process(ctx context.Context, block *pb.SignedBeaconBlock, from peer.ID) error {
        if p.handler == nil {
            return nil
        }
        return p.handler(ctx, block, from)
    }
    
    // attestationProcessor handles attestation messages across subnets
    type attestationProcessor struct {
        forkDigest [4]byte
        encoder    encoder.SszNetworkEncoder
        subnets    []uint64
        handler    func(context.Context, *pb.Attestation, uint64, peer.ID) error
        validator  func(context.Context, *pb.Attestation, uint64) error
        log        logrus.FieldLogger
    }
    
    func (p *attestationProcessor) Topics() []string {
        topics := make([]string, len(p.subnets))
        for i, subnet := range p.subnets {
            topics[i] = AttestationSubnetTopic(p.forkDigest, subnet)
        }
        return topics
    }
    
    func (p *attestationProcessor) TopicIndex(topic string) (int, error) {
        // Extract subnet from topic using regex
        re := regexp.MustCompile(`beacon_attestation_(\d+)`)
        matches := re.FindStringSubmatch(topic)
        if len(matches) != 2 {
            return -1, fmt.Errorf("invalid attestation topic format")
        }
        
        subnet, err := strconv.ParseUint(matches[1], 10, 64)
        if err != nil {
            return -1, err
        }
        
        // Find index in our subnet list
        for i, s := range p.subnets {
            if s == subnet {
                return i, nil
            }
        }
        
        return -1, fmt.Errorf("subnet %d not in processor subnet list", subnet)
    }
    
    func (p *attestationProcessor) Decode(data []byte) (*pb.Attestation, error) {
        att := &pb.Attestation{}
        if err := p.encoder.DecodeGossip(data, att); err != nil {
            return nil, fmt.Errorf("failed to decode attestation: %w", err)
        }
        return att, nil
    }
    
    func (p *attestationProcessor) Validate(ctx context.Context, att *pb.Attestation, subnetIdx int) (pubsub.ValidationResult, error) {
        if subnetIdx < 0 || subnetIdx >= len(p.subnets) {
            return pubsub.ValidationReject, fmt.Errorf("invalid subnet index")
        }
        
        if att == nil || att.Data == nil {
            return pubsub.ValidationReject, fmt.Errorf("nil attestation")
        }
        
        subnet := p.subnets[subnetIdx]
        
        if p.validator != nil {
            if err := p.validator(ctx, att, subnet); err != nil {
                return pubsub.ValidationReject, err
            }
        }
        
        return pubsub.ValidationAccept, nil
    }
    
    func (p *attestationProcessor) Process(ctx context.Context, att *pb.Attestation, subnetIdx int, from peer.ID) error {
        if p.handler == nil || subnetIdx < 0 || subnetIdx >= len(p.subnets) {
            return nil
        }
        
        subnet := p.subnets[subnetIdx]
        return p.handler(ctx, att, subnet, from)
    }
    
    // aggregateProcessor handles aggregate and proof messages
    type aggregateProcessor struct {
        forkDigest [4]byte
        encoder    encoder.SszNetworkEncoder
        handler    func(context.Context, *pb.AggregateAttestationAndProof, peer.ID) error
        log        logrus.FieldLogger
    }
    
    func (p *aggregateProcessor) Topic() string {
        return BeaconAggregateAndProofTopic(p.forkDigest)
    }
    
    func (p *aggregateProcessor) Decode(data []byte) (*pb.AggregateAttestationAndProof, error) {
        agg := &pb.AggregateAttestationAndProof{}
        if err := p.encoder.DecodeGossip(data, agg); err != nil {
            return nil, fmt.Errorf("failed to decode aggregate: %w", err)
        }
        return agg, nil
    }
    
    func (p *aggregateProcessor) Validate(ctx context.Context, agg *pb.AggregateAttestationAndProof) (pubsub.ValidationResult, error) {
        if agg == nil || agg.Aggregate == nil {
            return pubsub.ValidationReject, fmt.Errorf("nil aggregate")
        }
        return pubsub.ValidationAccept, nil
    }
    
    func (p *aggregateProcessor) Process(ctx context.Context, agg *pb.AggregateAttestationAndProof, from peer.ID) error {
        if p.handler == nil {
            return nil
        }
        return p.handler(ctx, agg, from)
    }
    
    // syncCommitteeProcessor handles sync committee messages across subnets
    type syncCommitteeProcessor struct {
        forkDigest [4]byte
        encoder    encoder.SszNetworkEncoder
        subnets    []uint64
        handler    func(context.Context, *pb.SyncCommitteeMessage, uint64, peer.ID) error
        log        logrus.FieldLogger
    }
    
    func (p *syncCommitteeProcessor) Topics() []string {
        topics := make([]string, len(p.subnets))
        for i, subnet := range p.subnets {
            topics[i] = SyncCommitteeSubnetTopic(p.forkDigest, subnet)
        }
        return topics
    }
    
    func (p *syncCommitteeProcessor) TopicIndex(topic string) (int, error) {
        // Extract subnet from topic using regex
        re := regexp.MustCompile(`sync_committee_(\d+)`)
        matches := re.FindStringSubmatch(topic)
        if len(matches) != 2 {
            return -1, fmt.Errorf("invalid sync committee topic format")
        }
        
        subnet, err := strconv.ParseUint(matches[1], 10, 64)
        if err != nil {
            return -1, err
        }
        
        for i, s := range p.subnets {
            if s == subnet {
                return i, nil
            }
        }
        
        return -1, fmt.Errorf("subnet %d not in processor subnet list", subnet)
    }
    
    func (p *syncCommitteeProcessor) Decode(data []byte) (*pb.SyncCommitteeMessage, error) {
        msg := &pb.SyncCommitteeMessage{}
        if err := p.encoder.DecodeGossip(data, msg); err != nil {
            return nil, fmt.Errorf("failed to decode sync committee message: %w", err)
        }
        return msg, nil
    }
    
    func (p *syncCommitteeProcessor) Validate(ctx context.Context, msg *pb.SyncCommitteeMessage, subnetIdx int) (pubsub.ValidationResult, error) {
        if msg == nil {
            return pubsub.ValidationReject, fmt.Errorf("nil sync committee message")
        }
        return pubsub.ValidationAccept, nil
    }
    
    func (p *syncCommitteeProcessor) Process(ctx context.Context, msg *pb.SyncCommitteeMessage, subnetIdx int, from peer.ID) error {
        if p.handler == nil || subnetIdx < 0 || subnetIdx >= len(p.subnets) {
            return nil
        }
        
        subnet := p.subnets[subnetIdx]
        return p.handler(ctx, msg, subnet, from)
    }
    
    // Additional processors for other message types follow the same pattern:
    // - voluntaryExitProcessor
    // - proposerSlashingProcessor
    // - attesterSlashingProcessor
    // - syncContributionProcessor
    // - blsToExecutionProcessor
    
    // Each implements the Processor[T] interface with:
    // - Topic() string
    // - Decode([]byte) (T, error)
    // - Validate(context.Context, T) (ValidationResult, error)
    // - Process(context.Context, T, peer.ID) error
    ```
  - Exports: Concrete processor implementations
  - Context: Internal processors for each Ethereum message type

#### Group D: Testing (Execute all in parallel after Group C)
- [x] **Task #5**: Create processor interface tests
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `processor_test.go`
  - Implements:
    ```go
    // Mock processor for testing
    type mockProcessor struct {
        topic        string
        decodeErr    error
        validateRes  ValidationResult
        validateErr  error
        processErr   error
        decodeCalls  int
        validateCalls int
        processCalls int
    }
    
    func TestProcessorSubscription(t *testing.T)
    func TestProcessorValidation(t *testing.T)
    func TestMultiProcessor(t *testing.T)
    func TestProcessorMetrics(t *testing.T)
    ```
  - Context: Test generic processor functionality

- [x] **Task #6**: Create BeaconchainGossipsub tests
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - File: `beaconchain_test.go`
  - Implements:
    ```go
    func TestBeaconBlockProcessor(t *testing.T)
    func TestAttestationProcessor(t *testing.T)
    func TestSubnetTopicParsing(t *testing.T)
    func TestBeaconchainSubscriptions(t *testing.T)
    ```
  - Context: Test Ethereum-specific functionality

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
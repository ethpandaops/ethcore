# Generic Gossipsub with Ethereum Extensions Implementation Plan

## Executive Summary
> The consensus/mimicry package needs gossipsub support following the established architectural patterns. This plan implements a generic, application-agnostic gossipsub layer in `p2p/pubsub` with Ethereum-specific extensions in `p2p/eth`.
> 
> The solution maintains strict separation of concerns: the pubsub package knows nothing about Ethereum, while the eth package provides typed wrappers, topic management, and Ethereum-specific callbacks.
> 
> This architecture ensures the gossipsub implementation is reusable while providing clean, typed interfaces for Ethereum consensus layer gossip messages.

## Goals & Objectives
### Primary Goals
- Implement generic gossipsub v1.1 support in p2p/pubsub with zero Ethereum dependencies
- Provide Ethereum-specific wrappers in p2p/eth following existing patterns
- Maintain architectural consistency with reqresp implementation

### Secondary Objectives
- Enable comprehensive event callbacks including validation failures
- Support custom message validators and handlers
- Provide pluggable architecture for any gossipsub application

## Solution Overview
### Approach
The implementation creates a generic `pubsub` package that handles all gossipsub functionality without any application-specific knowledge. A separate layer in `p2p/eth` provides Ethereum-specific topic management, typed message handlers, and convenience wrappers.

### Key Components
1. **Generic Pubsub Layer**: Application-agnostic gossipsub implementation
2. **Ethereum Extensions**: Topic management and typed handlers in p2p/eth
3. **Event System**: Comprehensive callbacks for all message lifecycle events
4. **Validation Framework**: Pluggable validators with detailed failure reporting

### Architecture Diagram
```
Application Layer (e.g., crawler)
        │
        ├──> p2p/eth/gossipsub.go (Ethereum-specific wrapper)
        │         │
        │         └──> p2p/pubsub/gossipsub.go (Generic implementation)
        │                    │
        │                    └──> libp2p-pubsub
        │
        └──> Event System (emission.Emitter)
```

### Data Flow
```
Gossip Message → libp2p → Generic Pubsub → Validator → Handler → Event
                              ↓                ↓          ↓
                          [Metrics]    [Accept/Reject] [Typed Callback]
```

### Expected Outcomes
- Generic pubsub package can be used for any gossipsub application
- Ethereum applications get typed interfaces and topic management
- Complete event visibility for monitoring and debugging
- Clean separation allows independent testing and evolution

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
├── pubsub/                           # Generic gossipsub implementation
│   ├── config.go                     (Task #0: Configuration types)
│   ├── errors.go                     (Task #0: Error types)
│   ├── types.go                      (Task #1: Core interfaces and types)
│   ├── gossipsub.go                  (Task #2: Main coordinator)
│   ├── topic.go                      (Task #3: Topic subscription management)
│   ├── validation.go                 (Task #4: Validation framework)
│   ├── handler.go                    (Task #5: Message handler registry)
│   ├── event.go                      (Task #6: Event definitions)
│   ├── metrics.go                    (Task #7: Metrics collection)
│   ├── gossipsub_test.go             (Task #10: Unit tests)
│   ├── validation_test.go            (Task #10: Validation tests)
│   └── handler_test.go               (Task #10: Handler tests)
│
├── eth/                              # Ethereum-specific extensions
│   ├── topic.go                      (Task #8: Extend with gossipsub topics)
│   ├── gossipsub.go                  (Task #9: Ethereum wrapper)
│   ├── messages.go                   (Task #9: Message type handlers)
│   └── gossipsub_test.go             (Task #11: Ethereum wrapper tests)
│
└── p2p.go                            (Task #8: Export pubsub types)

pkg/consensus/mimicry/testnet/
└── gossipsub_integration_test.go     (Task #12: Integration tests)
```

### Execution Plan

#### Group A: Foundation (Execute all in parallel)
- [ ] **Task #0**: Create configuration and error types
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - Files: `config.go`, `errors.go`
  - config.go implements:
    ```go
    package pubsub
    
    import "time"
    
    type Config struct {
        // Core gossipsub parameters
        MessageBufferSize    int
        ValidationBufferSize int
        MaxMessageSize       int
        
        // Gossipsub protocol parameters
        D                    int           // target degree
        Dlo                  int           // lower bound for mesh degree
        Dhi                  int           // upper bound for mesh degree
        Dlazy                int           // gossip degree
        HeartbeatInterval    time.Duration // heartbeat interval
        HistoryLength        int           // message cache history length
        HistoryGossip        int           // message IDs to gossip
        
        // Performance tuning
        ValidationConcurrency int
        PublishTimeout        time.Duration
        
        // Feature flags
        EnablePeerScoring     bool
        EnableFloodPublishing bool
    }
    
    func DefaultConfig() *Config {
        return &Config{
            MessageBufferSize:     1000,
            ValidationBufferSize:  100,
            MaxMessageSize:        1 << 20, // 1MB
            D:                     8,
            Dlo:                   6,
            Dhi:                   12,
            Dlazy:                 6,
            HeartbeatInterval:     time.Second,
            HistoryLength:         5,
            HistoryGossip:         3,
            ValidationConcurrency: 10,
            PublishTimeout:        5 * time.Second,
            EnablePeerScoring:     true,
            EnableFloodPublishing: true,
        }
    }
    
    func (c *Config) Validate() error
    ```
  - errors.go implements:
    ```go
    package pubsub
    
    import "errors"
    
    type Error struct {
        err     error
        context string
        topic   string
    }
    
    func NewError(err error, context string) *Error
    func NewTopicError(err error, topic string, context string) *Error
    func (e *Error) Error() string
    func (e *Error) Unwrap() error
    func (e *Error) Topic() string
    
    var (
        ErrNotStarted          = errors.New("pubsub not started")
        ErrAlreadyStarted      = errors.New("pubsub already started")
        ErrInvalidTopic        = errors.New("invalid topic")
        ErrTopicNotSubscribed  = errors.New("topic not subscribed")
        ErrTopicAlreadySubscribed = errors.New("topic already subscribed")
        ErrMessageTooLarge     = errors.New("message exceeds maximum size")
        ErrValidationFailed    = errors.New("message validation failed")
        ErrPublishTimeout      = errors.New("publish operation timed out")
        ErrSubscriptionClosed  = errors.New("subscription closed")
    )
    ```
  - Exports: Config struct and error types
  - Context: Foundation types for the generic pubsub implementation

#### Group B: Core Types (Execute all in parallel after Group A)
- [ ] **Task #1**: Create core interfaces and types
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `types.go`
  - Implements:
    ```go
    package pubsub
    
    import (
        "context"
        "time"
        
        "github.com/libp2p/go-libp2p/core/peer"
        pubsub "github.com/libp2p/go-libp2p-pubsub"
    )
    
    // ValidationResult represents the outcome of message validation
    type ValidationResult int
    
    const (
        ValidationAccept ValidationResult = iota
        ValidationReject
        ValidationIgnore
    )
    
    // Message represents a generic pubsub message
    type Message struct {
        Topic        string
        Data         []byte
        From         peer.ID
        ReceivedTime time.Time
        Sequence     uint64
    }
    
    // MessageHandler processes received messages
    type MessageHandler func(ctx context.Context, msg *Message) error
    
    // Validator validates messages and returns a validation result
    type Validator func(ctx context.Context, msg *Message) (ValidationResult, error)
    
    // TopicScoreParams allows configuration of topic-specific scoring
    type TopicScoreParams struct {
        TopicWeight                  float64
        TimeInMeshWeight             float64
        TimeInMeshQuantum            time.Duration
        TimeInMeshCap                float64
        FirstMessageDeliveriesWeight float64
        FirstMessageDeliveriesDecay  float64
        FirstMessageDeliveriesCap    float64
        InvalidMessageDeliveriesWeight float64
        InvalidMessageDeliveriesDecay  float64
    }
    
    // Subscription represents an active topic subscription
    type Subscription struct {
        topic        string
        subscription *pubsub.Subscription
        handler      MessageHandler
        validator    Validator
        cancel       context.CancelFunc
    }
    
    // PeerScoreSnapshot represents a peer's current score
    type PeerScoreSnapshot struct {
        PeerID       peer.ID
        Score        float64
        Topics       map[string]float64
        AppSpecific  float64
        IPColocation float64
        Behavioural  float64
    }
    ```
  - Exports: All core types for pubsub operations
  - Context: Generic types with no application-specific knowledge

#### Group C: Core Implementation (Execute all in parallel after Group B)
- [ ] **Task #2**: Create main gossipsub coordinator
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `gossipsub.go`
  - Imports:
    - `github.com/libp2p/go-libp2p/core/host`
    - `github.com/libp2p/go-libp2p-pubsub`
    - `github.com/chuckpreslar/emission`
    - `github.com/sirupsen/logrus`
  - Implements:
    ```go
    package pubsub
    
    type Gossipsub struct {
        log       logrus.FieldLogger
        host      host.Host
        pubsub    *pubsub.PubSub
        config    *Config
        emitter   *emission.Emitter
        metrics   *Metrics
        
        subscriptions map[string]*Subscription
        subMutex      sync.RWMutex
        
        validators map[string]Validator
        valMutex   sync.RWMutex
        
        scoreParams map[string]*TopicScoreParams
        scoreMutex  sync.RWMutex
        
        ctx    context.Context
        cancel context.CancelFunc
        
        started bool
        startMu sync.Mutex
    }
    
    func NewGossipsub(log logrus.FieldLogger, host host.Host, config *Config) (*Gossipsub, error)
    
    // Lifecycle methods
    func (g *Gossipsub) Start(ctx context.Context) error
    func (g *Gossipsub) Stop() error
    func (g *Gossipsub) IsStarted() bool
    
    // Topic subscription
    func (g *Gossipsub) Subscribe(ctx context.Context, topic string, handler MessageHandler) error
    func (g *Gossipsub) Unsubscribe(topic string) error
    func (g *Gossipsub) GetSubscriptions() []string
    func (g *Gossipsub) IsSubscribed(topic string) bool
    
    // Publishing
    func (g *Gossipsub) Publish(ctx context.Context, topic string, data []byte) error
    func (g *Gossipsub) PublishWithTimeout(ctx context.Context, topic string, data []byte, timeout time.Duration) error
    
    // Validation
    func (g *Gossipsub) RegisterValidator(topic string, validator Validator) error
    func (g *Gossipsub) UnregisterValidator(topic string) error
    
    // Peer scoring
    func (g *Gossipsub) SetTopicScoreParams(topic string, params *TopicScoreParams) error
    func (g *Gossipsub) GetPeerScore(peerID peer.ID) (*PeerScoreSnapshot, error)
    func (g *Gossipsub) GetAllPeerScores() ([]*PeerScoreSnapshot, error)
    
    // Topic management
    func (g *Gossipsub) GetTopicPeers(topic string) []peer.ID
    func (g *Gossipsub) GetAllTopics() []string
    
    // Internal methods
    func (g *Gossipsub) initializePubsub() error
    func (g *Gossipsub) createPubsubOptions() []pubsub.Option
    func (g *Gossipsub) handleSubscription(ctx context.Context, sub *Subscription)
    func (g *Gossipsub) processMessage(ctx context.Context, pmsg *pubsub.Message, handler MessageHandler)
    func (g *Gossipsub) applyValidator(topic string) pubsub.ValidatorEx
    ```
  - Exports: Gossipsub coordinator
  - Context: Central coordinator for generic gossipsub functionality

- [ ] **Task #3**: Create topic subscription management
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `topic.go`
  - Implements:
    ```go
    package pubsub
    
    // TopicManager handles topic lifecycle within Gossipsub
    type topicManager struct {
        log    logrus.FieldLogger
        pubsub *pubsub.PubSub
        topics map[string]*pubsub.Topic
        mutex  sync.RWMutex
    }
    
    func newTopicManager(log logrus.FieldLogger, ps *pubsub.PubSub) *topicManager
    
    func (tm *topicManager) getTopic(name string) (*pubsub.Topic, error)
    func (tm *topicManager) joinTopic(name string) (*pubsub.Topic, error)
    func (tm *topicManager) leaveTopic(name string) error
    func (tm *topicManager) getTopicNames() []string
    func (tm *topicManager) close() error
    
    // Subscription helpers used by Gossipsub
    func (g *Gossipsub) createSubscription(ctx context.Context, topic string, handler MessageHandler) (*Subscription, error)
    func (g *Gossipsub) closeSubscription(sub *Subscription) error
    ```
  - Context: Internal topic management for the coordinator

- [ ] **Task #4**: Create validation framework
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `validation.go`
  - Implements:
    ```go
    package pubsub
    
    // ValidationPipeline processes validators
    type ValidationPipeline struct {
        log        logrus.FieldLogger
        validators map[string]Validator
        metrics    *Metrics
        emitter    *emission.Emitter
    }
    
    func newValidationPipeline(log logrus.FieldLogger, metrics *Metrics, emitter *emission.Emitter) *ValidationPipeline
    
    func (vp *ValidationPipeline) addValidator(topic string, validator Validator)
    func (vp *ValidationPipeline) removeValidator(topic string)
    func (vp *ValidationPipeline) getValidator(topic string) Validator
    
    // Create libp2p validator function
    func (vp *ValidationPipeline) createLibp2pValidator(topic string) pubsub.ValidatorEx
    
    // Validation helpers
    func (vp *ValidationPipeline) validateMessage(ctx context.Context, topic string, msg *Message) (ValidationResult, error)
    func convertValidationResult(result ValidationResult) pubsub.ValidationResult
    ```
  - Context: Generic validation framework for message validation

- [ ] **Task #5**: Create message handler registry
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `handler.go`
  - Implements:
    ```go
    package pubsub
    
    // HandlerRegistry manages message handlers
    type handlerRegistry struct {
        log      logrus.FieldLogger
        handlers map[string]MessageHandler
        metrics  *Metrics
        emitter  *emission.Emitter
        mutex    sync.RWMutex
    }
    
    func newHandlerRegistry(log logrus.FieldLogger, metrics *Metrics, emitter *emission.Emitter) *handlerRegistry
    
    func (hr *handlerRegistry) register(topic string, handler MessageHandler)
    func (hr *handlerRegistry) unregister(topic string)
    func (hr *handlerRegistry) get(topic string) MessageHandler
    func (hr *handlerRegistry) exists(topic string) bool
    func (hr *handlerRegistry) getTopics() []string
    
    // Message processing
    func (hr *handlerRegistry) processMessage(ctx context.Context, msg *Message) error
    ```
  - Context: Manages generic message handlers

#### Group D: Supporting Components (Execute all in parallel after Group C)
- [ ] **Task #6**: Create event definitions
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `event.go`
  - Implements:
    ```go
    package pubsub
    
    // Event names
    var (
        // Lifecycle events
        PubsubStartedEvent  = "pubsub:started"
        PubsubStoppedEvent  = "pubsub:stopped"
        
        // Subscription events
        TopicSubscribedEvent    = "pubsub:topic:subscribed"
        TopicUnsubscribedEvent  = "pubsub:topic:unsubscribed"
        SubscriptionErrorEvent  = "pubsub:subscription:error"
        
        // Message events
        MessageReceivedEvent   = "pubsub:message:received"
        MessagePublishedEvent  = "pubsub:message:published"
        MessageValidatedEvent  = "pubsub:message:validated"
        MessageHandledEvent    = "pubsub:message:handled"
        
        // Error events
        ValidationFailedEvent  = "pubsub:validation:failed"
        HandlerErrorEvent      = "pubsub:handler:error"
        PublishErrorEvent      = "pubsub:publish:error"
        
        // Peer events
        PeerJoinedTopicEvent   = "pubsub:peer:joined"
        PeerLeftTopicEvent     = "pubsub:peer:left"
        PeerScoreUpdatedEvent  = "pubsub:peer:score:updated"
    )
    
    // Event callbacks
    type PubsubStartedCallback func()
    type PubsubStoppedCallback func()
    type TopicSubscribedCallback func(topic string)
    type TopicUnsubscribedCallback func(topic string)
    type SubscriptionErrorCallback func(topic string, err error)
    type MessageReceivedCallback func(topic string, from peer.ID)
    type MessagePublishedCallback func(topic string)
    type MessageValidatedCallback func(topic string, result ValidationResult)
    type MessageHandledCallback func(topic string, success bool)
    type ValidationFailedCallback func(topic string, from peer.ID, err error)
    type HandlerErrorCallback func(topic string, err error)
    type PublishErrorCallback func(topic string, err error)
    type PeerJoinedTopicCallback func(topic string, peerID peer.ID)
    type PeerLeftTopicCallback func(topic string, peerID peer.ID)
    type PeerScoreUpdatedCallback func(peerID peer.ID, score float64)
    
    // Event registration methods for Gossipsub
    func (g *Gossipsub) OnPubsubStarted(callback PubsubStartedCallback)
    func (g *Gossipsub) OnPubsubStopped(callback PubsubStoppedCallback)
    func (g *Gossipsub) OnTopicSubscribed(callback TopicSubscribedCallback)
    func (g *Gossipsub) OnTopicUnsubscribed(callback TopicUnsubscribedCallback)
    func (g *Gossipsub) OnSubscriptionError(callback SubscriptionErrorCallback)
    func (g *Gossipsub) OnMessageReceived(callback MessageReceivedCallback)
    func (g *Gossipsub) OnMessagePublished(callback MessagePublishedCallback)
    func (g *Gossipsub) OnMessageValidated(callback MessageValidatedCallback)
    func (g *Gossipsub) OnMessageHandled(callback MessageHandledCallback)
    func (g *Gossipsub) OnValidationFailed(callback ValidationFailedCallback)
    func (g *Gossipsub) OnHandlerError(callback HandlerErrorCallback)
    func (g *Gossipsub) OnPublishError(callback PublishErrorCallback)
    func (g *Gossipsub) OnPeerJoinedTopic(callback PeerJoinedTopicCallback)
    func (g *Gossipsub) OnPeerLeftTopic(callback PeerLeftTopicCallback)
    func (g *Gossipsub) OnPeerScoreUpdated(callback PeerScoreUpdatedCallback)
    
    // Internal emission methods
    func (g *Gossipsub) emitPubsubStarted()
    func (g *Gossipsub) emitPubsubStopped()
    func (g *Gossipsub) emitTopicSubscribed(topic string)
    func (g *Gossipsub) emitTopicUnsubscribed(topic string)
    func (g *Gossipsub) emitSubscriptionError(topic string, err error)
    func (g *Gossipsub) emitMessageReceived(topic string, from peer.ID)
    func (g *Gossipsub) emitMessagePublished(topic string)
    func (g *Gossipsub) emitMessageValidated(topic string, result ValidationResult)
    func (g *Gossipsub) emitMessageHandled(topic string, success bool)
    func (g *Gossipsub) emitValidationFailed(topic string, from peer.ID, err error)
    func (g *Gossipsub) emitHandlerError(topic string, err error)
    func (g *Gossipsub) emitPublishError(topic string, err error)
    func (g *Gossipsub) emitPeerJoinedTopic(topic string, peerID peer.ID)
    func (g *Gossipsub) emitPeerLeftTopic(topic string, peerID peer.ID)
    func (g *Gossipsub) emitPeerScoreUpdated(peerID peer.ID, score float64)
    ```
  - Exports: Generic event definitions and callbacks
  - Context: Comprehensive event system for all pubsub operations

- [ ] **Task #7**: Create metrics collection
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - File: `metrics.go`
  - Implements:
    ```go
    package pubsub
    
    import "github.com/prometheus/client_golang/prometheus"
    
    type Metrics struct {
        // Message metrics
        MessagesReceived      *prometheus.CounterVec
        MessagesPublished     *prometheus.CounterVec
        MessagesValidated     *prometheus.CounterVec
        MessagesHandled       *prometheus.CounterVec
        
        // Error metrics
        ValidationErrors      *prometheus.CounterVec
        HandlerErrors         *prometheus.CounterVec
        PublishErrors         *prometheus.CounterVec
        
        // Performance metrics
        ValidationDuration    *prometheus.HistogramVec
        HandlerDuration       *prometheus.HistogramVec
        PublishDuration       *prometheus.HistogramVec
        
        // State metrics
        ActiveSubscriptions   *prometheus.GaugeVec
        PeersPerTopic         *prometheus.GaugeVec
        MessageQueueSize      *prometheus.GaugeVec
        
        // Peer metrics
        PeerScores            *prometheus.GaugeVec
        PeersConnected        prometheus.Gauge
    }
    
    func NewMetrics(namespace string) *Metrics
    func (m *Metrics) Register(registry *prometheus.Registry) error
    
    // Metric update methods
    func (m *Metrics) RecordMessageReceived(topic string)
    func (m *Metrics) RecordMessagePublished(topic string, success bool)
    func (m *Metrics) RecordMessageValidated(topic string, result ValidationResult)
    func (m *Metrics) RecordMessageHandled(topic string, success bool, duration time.Duration)
    func (m *Metrics) RecordValidationError(topic string)
    func (m *Metrics) RecordHandlerError(topic string)
    func (m *Metrics) RecordPublishError(topic string)
    func (m *Metrics) RecordValidationDuration(topic string, duration time.Duration)
    func (m *Metrics) RecordHandlerDuration(topic string, duration time.Duration)
    func (m *Metrics) RecordPublishDuration(topic string, duration time.Duration)
    func (m *Metrics) SetActiveSubscriptions(count int)
    func (m *Metrics) SetPeersPerTopic(topic string, count int)
    func (m *Metrics) SetMessageQueueSize(topic string, size int)
    func (m *Metrics) SetPeerScore(peerID string, score float64)
    func (m *Metrics) SetPeersConnected(count int)
    ```
  - Exports: Generic metrics for pubsub operations
  - Context: Prometheus metrics for monitoring

#### Group E: Ethereum Extensions (Execute all in parallel after Group D)
- [ ] **Task #8**: Update p2p package exports and extend eth/topic.go
  - Files to modify:
    - `pkg/consensus/mimicry/p2p/p2p.go`
    - `pkg/consensus/mimicry/p2p/eth/topic.go`
  - Changes in p2p.go:
    ```go
    // Add to imports
    import "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
    
    // Export pubsub types
    type (
        Gossipsub = pubsub.Gossipsub
        GossipsubConfig = pubsub.Config
        Message = pubsub.Message
        MessageHandler = pubsub.MessageHandler
        Validator = pubsub.Validator
        ValidationResult = pubsub.ValidationResult
    )
    
    const (
        ValidationAccept = pubsub.ValidationAccept
        ValidationReject = pubsub.ValidationReject
        ValidationIgnore = pubsub.ValidationIgnore
    )
    
    // Constructor wrapper
    func NewGossipsub(log logrus.FieldLogger, host host.Host, config *pubsub.Config) (*pubsub.Gossipsub, error) {
        return pubsub.NewGossipsub(log, host, config)
    }
    ```
  - Changes in eth/topic.go:
    ```go
    // Add gossipsub topic constants
    const (
        // Topic format templates
        GossipsubTopicFormat = "/eth2/%x/%s/ssz_snappy"
        
        // Global topic names
        BeaconBlockTopicName           = "beacon_block"
        BeaconAggregateAndProofTopicName = "beacon_aggregate_and_proof"
        VoluntaryExitTopicName         = "voluntary_exit"
        ProposerSlashingTopicName      = "proposer_slashing"
        AttesterSlashingTopicName      = "attester_slashing"
        SyncContributionAndProofTopicName = "sync_committee_contribution_and_proof"
        
        // Subnet topic templates
        AttestationSubnetTopicTemplate = "beacon_attestation_%d"
        SyncCommitteeSubnetTopicTemplate = "sync_committee_%d"
        
        // Constants
        AttestationSubnetCount = 64
        SyncCommitteeSubnetCount = 4
    )
    
    // Topic construction helpers
    func BeaconBlockTopic(forkDigest [4]byte) string {
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconBlockTopicName)
    }
    
    func BeaconAggregateAndProofTopic(forkDigest [4]byte) string {
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, BeaconAggregateAndProofTopicName)
    }
    
    func VoluntaryExitTopic(forkDigest [4]byte) string {
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, VoluntaryExitTopicName)
    }
    
    func ProposerSlashingTopic(forkDigest [4]byte) string {
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, ProposerSlashingTopicName)
    }
    
    func AttesterSlashingTopic(forkDigest [4]byte) string {
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, AttesterSlashingTopicName)
    }
    
    func SyncContributionAndProofTopic(forkDigest [4]byte) string {
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, SyncContributionAndProofTopicName)
    }
    
    func AttestationSubnetTopic(forkDigest [4]byte, subnet uint64) string {
        name := fmt.Sprintf(AttestationSubnetTopicTemplate, subnet)
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
    }
    
    func SyncCommitteeSubnetTopic(forkDigest [4]byte, subnet uint64) string {
        name := fmt.Sprintf(SyncCommitteeSubnetTopicTemplate, subnet)
        return fmt.Sprintf(GossipsubTopicFormat, forkDigest, name)
    }
    
    // Topic parsing helpers
    func ParseGossipsubTopic(topic string) (forkDigest [4]byte, name string, err error)
    func IsAttestationTopic(topicName string) (bool, uint64)
    func IsSyncCommitteeTopic(topicName string) (bool, uint64)
    
    // Get all topics
    func AllGlobalTopics(forkDigest [4]byte) []string
    func AllAttestationTopics(forkDigest [4]byte) []string
    func AllSyncCommitteeTopics(forkDigest [4]byte) []string
    ```
  - Context: Exports generic types and adds Ethereum topic management

- [ ] **Task #9**: Create Ethereum-specific gossipsub wrapper
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - Files: `gossipsub.go`, `messages.go`
  - gossipsub.go implements:
    ```go
    package eth
    
    import (
        "github.com/ethpandaops/ethcore/pkg/consensus/mimicry/p2p/pubsub"
        "github.com/ethpandaops/ethcore/pkg/proto/eth"
        "github.com/ethpandaops/ethcore/pkg/encoding/encoder"
    )
    
    // Gossipsub wraps generic pubsub with Ethereum-specific functionality
    type Gossipsub struct {
        *pubsub.Gossipsub
        forkDigest [4]byte
        encoder    encoder.SszNetworkEncoder
        log        logrus.FieldLogger
    }
    
    // NewGossipsub creates an Ethereum-aware gossipsub instance
    func NewGossipsub(log logrus.FieldLogger, ps *pubsub.Gossipsub, forkDigest [4]byte, encoder encoder.SszNetworkEncoder) *Gossipsub
    
    // Global topic subscriptions
    func (g *Gossipsub) SubscribeBeaconBlock(ctx context.Context, handler BeaconBlockHandler) error
    func (g *Gossipsub) SubscribeBeaconAggregateAndProof(ctx context.Context, handler BeaconAggregateAndProofHandler) error
    func (g *Gossipsub) SubscribeVoluntaryExit(ctx context.Context, handler VoluntaryExitHandler) error
    func (g *Gossipsub) SubscribeProposerSlashing(ctx context.Context, handler ProposerSlashingHandler) error
    func (g *Gossipsub) SubscribeAttesterSlashing(ctx context.Context, handler AttesterSlashingHandler) error
    func (g *Gossipsub) SubscribeSyncContributionAndProof(ctx context.Context, handler SyncContributionAndProofHandler) error
    
    // Subnet subscriptions
    func (g *Gossipsub) SubscribeAttestation(ctx context.Context, subnet uint64, handler AttestationHandler) error
    func (g *Gossipsub) SubscribeSyncCommittee(ctx context.Context, subnet uint64, handler SyncCommitteeHandler) error
    
    // Bulk subscriptions
    func (g *Gossipsub) SubscribeAllGlobalTopics(ctx context.Context, handlers *GlobalTopicHandlers) error
    func (g *Gossipsub) SubscribeAttestationSubnets(ctx context.Context, subnets []uint64, handler AttestationHandler) error
    func (g *Gossipsub) SubscribeSyncCommitteeSubnets(ctx context.Context, subnets []uint64, handler SyncCommitteeHandler) error
    
    // Publishing helpers
    func (g *Gossipsub) PublishBeaconBlock(ctx context.Context, block *eth.SignedBeaconBlock) error
    func (g *Gossipsub) PublishAttestation(ctx context.Context, att *eth.Attestation, subnet uint64) error
    func (g *Gossipsub) PublishSyncCommitteeMessage(ctx context.Context, msg *eth.SyncCommitteeMessage, subnet uint64) error
    
    // Validation helpers
    func (g *Gossipsub) RegisterBeaconBlockValidator(validator BeaconBlockValidator) error
    func (g *Gossipsub) RegisterAttestationValidator(subnet uint64, validator AttestationValidator) error
    
    // Internal helpers
    func (g *Gossipsub) createMessageHandler(decoder func([]byte) (interface{}, error), handler func(interface{}, peer.ID)) pubsub.MessageHandler
    func (g *Gossipsub) createValidator(validator func(interface{}) error, decoder func([]byte) (interface{}, error)) pubsub.Validator
    ```
  - messages.go implements:
    ```go
    package eth
    
    import (
        "github.com/ethpandaops/ethcore/pkg/proto/eth"
        "github.com/libp2p/go-libp2p/core/peer"
    )
    
    // Typed message handlers
    type BeaconBlockHandler func(ctx context.Context, block *eth.SignedBeaconBlock, from peer.ID)
    type BeaconAggregateAndProofHandler func(ctx context.Context, msg *eth.SignedAggregateAndProof, from peer.ID)
    type AttestationHandler func(ctx context.Context, att *eth.Attestation, subnet uint64, from peer.ID)
    type SyncCommitteeHandler func(ctx context.Context, msg *eth.SyncCommitteeMessage, subnet uint64, from peer.ID)
    type SyncContributionAndProofHandler func(ctx context.Context, msg *eth.SignedContributionAndProof, from peer.ID)
    type VoluntaryExitHandler func(ctx context.Context, msg *eth.SignedVoluntaryExit, from peer.ID)
    type ProposerSlashingHandler func(ctx context.Context, msg *eth.ProposerSlashing, from peer.ID)
    type AttesterSlashingHandler func(ctx context.Context, msg *eth.AttesterSlashing, from peer.ID)
    
    // Typed validators
    type BeaconBlockValidator func(block *eth.SignedBeaconBlock) error
    type AttestationValidator func(att *eth.Attestation, subnet uint64) error
    type SyncCommitteeValidator func(msg *eth.SyncCommitteeMessage, subnet uint64) error
    
    // Handler collection for bulk subscription
    type GlobalTopicHandlers struct {
        BeaconBlock              BeaconBlockHandler
        BeaconAggregateAndProof  BeaconAggregateAndProofHandler
        VoluntaryExit            VoluntaryExitHandler
        ProposerSlashing         ProposerSlashingHandler
        AttesterSlashing         AttesterSlashingHandler
        SyncContributionAndProof SyncContributionAndProofHandler
    }
    
    // Event wrappers that convert generic events to typed events
    func WrapMessageReceivedAsBeaconBlock(callback pubsub.MessageReceivedCallback, handler BeaconBlockHandler) pubsub.MessageReceivedCallback
    func WrapMessageReceivedAsAttestation(callback pubsub.MessageReceivedCallback, handler AttestationHandler) pubsub.MessageReceivedCallback
    func WrapValidationFailedAsBeaconBlock(callback pubsub.ValidationFailedCallback) func(block *eth.SignedBeaconBlock, from peer.ID, err error)
    ```
  - Exports: Ethereum-specific wrapper and typed handlers
  - Context: Provides typed interfaces for Ethereum consensus messages

#### Group F: Testing (Execute all in parallel after Group E)
- [ ] **Task #10**: Create unit tests for generic pubsub
  - Folder: `pkg/consensus/mimicry/p2p/pubsub/`
  - Files: `gossipsub_test.go`, `validation_test.go`, `handler_test.go`
  - gossipsub_test.go implements:
    ```go
    func TestNewGossipsub(t *testing.T)
    func TestGossipsubLifecycle(t *testing.T)
    func TestSubscribeUnsubscribe(t *testing.T)
    func TestPublish(t *testing.T)
    func TestConcurrentOperations(t *testing.T)
    func TestMessageProcessing(t *testing.T)
    func TestPeerScoring(t *testing.T)
    func TestEventEmission(t *testing.T)
    ```
  - validation_test.go implements:
    ```go
    func TestValidationPipeline(t *testing.T)
    func TestValidatorRegistration(t *testing.T)
    func TestValidationResults(t *testing.T)
    func TestValidationErrors(t *testing.T)
    ```
  - handler_test.go implements:
    ```go
    func TestHandlerRegistry(t *testing.T)
    func TestMessageHandling(t *testing.T)
    func TestHandlerErrors(t *testing.T)
    func TestConcurrentHandlers(t *testing.T)
    ```
  - Context: Unit tests for generic functionality

- [ ] **Task #11**: Create tests for Ethereum wrapper
  - Folder: `pkg/consensus/mimicry/p2p/eth/`
  - File: `gossipsub_test.go`
  - Implements:
    ```go
    func TestEthereumGossipsub(t *testing.T)
    func TestTopicConstruction(t *testing.T)
    func TestTypedHandlers(t *testing.T)
    func TestMessageDecoding(t *testing.T)
    func TestValidatorIntegration(t *testing.T)
    func TestBulkSubscriptions(t *testing.T)
    ```
  - Context: Tests for Ethereum-specific functionality

- [ ] **Task #12**: Create integration tests
  - Folder: `pkg/consensus/mimicry/testnet/`
  - File: `gossipsub_integration_test.go`
  - Implements:
    ```go
    func TestGossipsubIntegration(t *testing.T) {
        // Create test network with multiple nodes
        // Test message propagation
        // Test validation and peer scoring
        // Test Ethereum message types
    }
    
    func TestGossipsubPerformance(t *testing.T) {
        // Test with high message volume
        // Measure latency and throughput
        // Test concurrent subscriptions
    }
    ```
  - Context: End-to-end integration testing

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
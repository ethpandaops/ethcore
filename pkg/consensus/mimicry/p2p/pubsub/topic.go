package pubsub

import (
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sirupsen/logrus"
)

// topicManager handles topic lifecycle within the gossipsub system
type topicManager struct {
	log    logrus.FieldLogger
	pubsub *pubsub.PubSub
	topics map[string]*pubsub.Topic
	mutex  sync.RWMutex
}

// newTopicManager creates a new topic manager
func newTopicManager(log logrus.FieldLogger, ps *pubsub.PubSub) *topicManager {
	return &topicManager{
		log:    log.WithField("component", "topic_manager"),
		pubsub: ps,
		topics: make(map[string]*pubsub.Topic),
	}
}

// getTopic retrieves an existing topic or returns an error if not found
func (tm *topicManager) getTopic(name string) (*pubsub.Topic, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	topic, exists := tm.topics[name]
	if !exists {
		return nil, fmt.Errorf("topic %s not found", name)
	}

	return topic, nil
}

// joinTopic joins a topic, creating it if it doesn't exist
func (tm *topicManager) joinTopic(name string) (*pubsub.Topic, error) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Check if we already have this topic
	if topic, exists := tm.topics[name]; exists {
		tm.log.WithField("topic", name).Debug("Topic already joined")
		return topic, nil
	}

	// Join the topic via libp2p
	topic, err := tm.pubsub.Join(name)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic %s: %w", name, err)
	}

	// Store the topic
	tm.topics[name] = topic

	tm.log.WithField("topic", name).Info("Successfully joined topic")
	return topic, nil
}

// leaveTopic leaves a topic and removes it from the manager
func (tm *topicManager) leaveTopic(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	topic, exists := tm.topics[name]
	if !exists {
		tm.log.WithField("topic", name).Debug("Topic not found for leaving")
		return nil // Not an error if we're not subscribed
	}

	// Close the topic
	if err := topic.Close(); err != nil {
		tm.log.WithError(err).WithField("topic", name).Warn("Error closing topic")
		// Continue with removal even if close failed
	}

	// Remove from our tracking
	delete(tm.topics, name)

	tm.log.WithField("topic", name).Info("Successfully left topic")
	return nil
}

// getTopicNames returns a list of all currently managed topic names
func (tm *topicManager) getTopicNames() []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	names := make([]string, 0, len(tm.topics))
	for name := range tm.topics {
		names = append(names, name)
	}

	return names
}

// close shuts down the topic manager and closes all topics
func (tm *topicManager) close() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.log.Info("Closing topic manager")

	var lastErr error
	for name, topic := range tm.topics {
		if err := topic.Close(); err != nil {
			tm.log.WithError(err).WithField("topic", name).Error("Failed to close topic")
			lastErr = err
		}
	}

	// Clear the topics map
	tm.topics = make(map[string]*pubsub.Topic)

	if lastErr != nil {
		return fmt.Errorf("errors occurred while closing topics: %w", lastErr)
	}

	tm.log.Info("Topic manager closed successfully")
	return nil
}

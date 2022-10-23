package callback_vibe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Open questions with this PoC:
//
// Is the consuimer/subscriber stuff too complicated? Is it too abstracted? It is pretty abstract, but it feels
// hard to predict,
//
// Is moving configGetting passint the buck?
//
// A lot of these interfaces feel like very simple things. Would they be better as typed functions?
//
// With the `lastLoaded` pattern, do we need any caching at all?
//
// Is type aliasing strings at all sensible?

// consumer is an interface for something that consumes a block of block of configuration.
type consumer interface {
	Update(io.Reader)
}

// subscriber is an interface for something that wants to be notified when a subsystem has been updated.
type subscriber interface {
	Ping()
}

// Note that objectStore might be a cache, or an http based client. Or, in production usage, one wrapped in the other.
type objectGetter interface {
	// Get returns a blob by name.
	Get(name string) (io.Reader, error)
}

// configGetter is an interface for something that can get the initial configuration. Any authentication
// and http calls live there
type configGetter interface {
	GetConfig() (io.Reader, error)
}

type subsystemType string

const (
	OptionsSubsystem     subsystemType = "options"
	TabularDataSubsystem               = "tabular_data"
)

// controlSystem is the main object that manages the control system. It is responsible for fetching control data,
// and updating onsumers and subscribers
type controlSystem struct {
	consumers    map[subsystemType]consumer
	subscribers  map[subsystemType][]subscriber
	logger       log.Logger
	objectGetter objectGetter
	configGetter configGetter
	lastLoaded   map[subsystemType]string
}

func (c *controlSystem) RegisterConsumer(subsystem subsystemType, consumer consumer) error {
	// Since consumers are based on io.Reader, we can only have one without using TeeReader. But, it's not clear we
	// need more than one anyhow. So, one it shall be. TBD
	if _, ok := c.consumers[subsystem]; ok {
		return errors.New("consumer already registered for subsystem %s", subsystem)
	}
	c.consumers[subsystem] = consumer
	return nil
}

func (c *controlSystem) RegisterSubscriber(subsystem subsystemType, subscriber subscriber) {
	c.subscribers[subsystem] = append(c.subscribers[subsystem], subscriber)
}

func (c *controlSystem) update(subsystem subsystemType, reader io.Reader) {
	// First, send to consumer, if any
	if consumer, ok := c.consumers[subsystem]; ok {
		consumer.Update(reader)
	}

	// Then send a ping to all subscribers
	for _, subscriber := range c.subscribers[subsystem] {
		subscriber.Ping()
	}
}

func (c *controlSystem) Fetch() error {
	data, err := c.configGetter.GetConfig()
	if err != nil {
		return fmt.Errorf("getting initial config: %w", err)
	}

	var controlData map[string]string
	if err := json.NewDecoder(data).Decode(&controlData); err != nil {
		return fmt.Errorf("decoding initial config map: %w", err)
	}

	// Probably need some type shenanging here. TBD

	for subsystem, objectName := range controlData {

		if objectName == c.lastLoaded[subsystem] {
			level.Debug(c.logger).Log(
				"msg", "No change. Skipping update",
				"subsystem", subsystem,
				"objectName", objectName,
			)
			continue
		}

		data, err := c.objectGetter.Get(objectName)
		if err != nil {
			return fmt.Errorf("fetching object %s: %w", objectName, err)
		}

		// We cast these to subsystemType, this is somewhat weird -- after all, those types may not be defined.
		// Two reasons. One, this is a PoC. Two, we want to be forgiving in what we accept from the server,
		// this lets us handle upgrades and change, but we want to be strict about what we use and register internally.
		c.update(subsystemType(subsystem), data)
		c.lastLoaded[subsystemType(subsystem)] = objectName
	}
}

// Execute for compatibility with rungroup TBD
func (c *controlSystem) Execute() error {
}

// Interrupt for compatibility with rungroup TBD
func (c *controlSystem) Interrupt(err error) {
}

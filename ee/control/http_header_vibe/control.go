package http_header_vibe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/go-kit/log/level"
)

// This PoC is very similar to the object_vibe, but to to solve the drawback with the object hashes, we use tags.
// The control response starts with a map, like this:
//   {
//     "options": "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4",
//     "data": "abc6fd595fc079d3114d4b71a4d84b1d1d0f79df1e70f8813212f2a65d8916df",
//   }
//
// Then, for each subsystem, we check if the object hash is in our local cache. But if not, we fetch by the _subsystem_
// name. We can include appropriate headers. This is something of a hybrid approach. It takes some of the simplicity of
// objects, and some of the caching of etags..

// consumer is an interface for something that consumes a block of block of configuration.
type consumer interface {
	Update(io.Reader)
}

// subscriber is an interface for something that wants to be notified when a subsystem has been updated.
type subscriber interface {
	Ping()
}

// subsystemGetter return data for a given subsystem. It's expected to be a light wrapper over an http client
type subsystemGetter interface {
	// Get returns a blob by name.
	Get(subsystem string) (objecthash string, data io.Reader, err error)
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
	consumers       map[subsystemType]consumer
	subscribers     map[subsystemType][]subscriber
	logger          log.Logger
	subsystemGetter subsystemGetter
	configGetter    configGetter
	lastLoaded      map[subsystemType]string
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

	for subsystem, objectHash := range controlData {

		if objectHash == c.lastLoaded[subsystem] {
			level.Debug(c.logger).Log(
				"msg", "No change. Skipping update",
				"subsystem", subsystem,
				"objectName", objectHash,
			)
			continue
		}

		newhash, data, err := c.subsystemGetter.Get(subsystem)
		if err != nil {
			return fmt.Errorf("fetching subsystem %s: %w", subsystem, err)
		}

		// We cast these to subsystemType, this is somewhat weird -- after all, those types may not be defined.
		// Two reasons. One, this is a PoC. Two, we want to be forgiving in what we accept from the server,
		// this lets us handle upgrades and change, but we want to be strict about what we use and register internally.
		c.update(subsystemType(subsystem), data)
		c.lastLoaded[subsystemType(subsystem)] = newhash
	}
}

// Execute for compatibility with rungroup TBD
func (c *controlSystem) Execute() error {
}

// Interrupt for compatibility with rungroup TBD
func (c *controlSystem) Interrupt(err error) {
}

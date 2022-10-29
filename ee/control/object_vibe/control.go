package callback_vibe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// This PoC is based on a very plain object model. The initial control structure is a map of subsystem to object
// hashes, like this:
//   {
//     "options": "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4",
//     "data": "abc6fd595fc079d3114d4b71a4d84b1d1d0f79df1e70f8813212f2a65d8916df",
//   }
//
// After that is fetched, the control client iterates through the objects, if it has the object in the local cache,
// great! If not, fetch it from upstream. This creates a very simple model.
//
// However, one drawback to this approach, is that if the server doesn't have an object, there is no way to generate
// it. The hashes are fundementally one way.
//
// Open questions with this PoC, not strictly related to the above:
//
//   - Is the consuimer/subscriber stuff too complicated? Is it too abstracted? It is pretty abstract, but it feels
//     hard to predict,
//
//   - Is moving configGetting passing the buck?
//
//   - A lot of these interfaces feel like very simple things. Would they be better as typed functions?
//
//   - With the `lastLoaded` pattern, do we need any caching at all?
//
//   - Is type aliasing strings at all sensible?

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

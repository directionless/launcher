package etag_vibe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/go-kit/log/level"
)

// This PoC is somewhat unlike the object vibe. It attempts to feel out ETAGs. I suspect that we cannot avoid
// multiple fetches -- using etags would mean fetching each blob, proper etags would have the server validate it. It
// would probably start with an request that was an array of subsystems:
//   {
//		 "options",
//  	 "data",
//   }
//
// Then, for each subsystem the client would request the resource, with the `If-None-Match` header for whatever
// is already in memory

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
	Get(subsystem, etag string) (etag string, data io.Reader, err error)
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

	var subsystems []string
	if err := json.NewDecoder(data).Decode(&subsystems); err != nil {
		return fmt.Errorf("decoding initial config array: %w", err)
	}

	// Probably need some type shenanging here. TBD

	for _, subsystem := range subsystems {
		etag, data, err := c.subsystemGetter.Get(subsystem, c.lastLoaded[subsystem])
		if err != nil {
			return fmt.Errorf("fetching subsystem %s: %w", subsystem, err)
		}

		// Did we get a new version?
		// BUG: I reused the subsystem getter, which doesnt expose an http 304 code. So pretend...
		if etag == "unmodified" {
			level.Debug(c.logger).Log(
				"msg", "No change. Skipping update",
				"subsystem", subsystem,
				"objectName", objectHash,
			)
			return
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

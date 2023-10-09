package cache

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/dispatcher"
)

type SessionCache struct {
	MaxEntries int
	Vehicles   map[string][]dispatcher.CacheEntry `json:"vehicles"`
	lock       sync.Mutex
}

// New returns a SessionCache with that holds session state for up to maxEntries vehicles.
// The SessionCache uses a least-recently-used (LRU) eviction strategy, with the caveat that for
// this purpose a session is "used" when its used to authorize a command, not when it's loaded from
// or saved to the SessionCache.
//
// Set maxEntries to zero for an unbounded cache.
func New(maxEntries int) *SessionCache {
	return &SessionCache{
		MaxEntries: maxEntries,
		Vehicles:   make(map[string][]dispatcher.CacheEntry),
	}
}

// Import a SessionCache using data in r.
// The data should previously have been generated using [SessionCache.Export].
func Import(r io.Reader) (*SessionCache, error) {
	var cache SessionCache
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// ImportFromFile reads a SessionCache from disk.
func ImportFromFile(filename string) (*SessionCache, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Import(file)
}

// Export writes a serialized SessionCache to w.
func (c *SessionCache) Export(w io.Writer) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return json.NewEncoder(w).Encode(c)
}

// ExportToFile writes a SessionCache to disk.
func (c *SessionCache) ExportToFile(filename string) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	return c.Export(file)
}

// Update the SessionCache's entry for a vin with current state.
// It's recommended that clients use the vehicle.UpdateCachedSessions method instead in order to
// avoid accessing the internal dispatcher package.
func (c *SessionCache) Update(vin string, sessions []dispatcher.CacheEntry) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Vehicles[vin] = sessions
	if c.MaxEntries > 0 && len(c.Vehicles) > c.MaxEntries {
		// TODO: Replace with a proper cache
		oldestVIN := vin
		oldestCreationTime := time.Now()
		for v, sessions := range c.Vehicles {
			// Each vehicle has multiple sessions associated with it, one for each domain. The age
			// of cache entry is the age of its most recent session.
			mostRecent := time.Time{}
			for _, entry := range sessions {
				if entry.CreatedAt.After(mostRecent) {
					mostRecent = entry.CreatedAt
				}
			}
			if mostRecent.Before(oldestCreationTime) {
				oldestVIN = v
				oldestCreationTime = mostRecent
			}
		}
		delete(c.Vehicles, oldestVIN)
	}
	return nil
}

// GetEntry returns the sessions associated with vin.
// This method intended for use by the internal dispatcher package; other clients should have no
// use for it.
func (c *SessionCache) GetEntry(vin string) ([]dispatcher.CacheEntry, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	session, ok := c.Vehicles[vin]
	return session, ok
}

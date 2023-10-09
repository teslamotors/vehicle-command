package cache

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/internal/dispatcher"
)

const testSessionCount = 3

func generateTestSessions(n int) []dispatcher.CacheEntry {
	var sessions []dispatcher.CacheEntry
	for j := 0; j < testSessionCount; j++ {
		sessions = append(sessions, dispatcher.CacheEntry{CreatedAt: time.Time{}.Add(time.Duration(n)), Domain: n, SessionInfo: []byte{byte(j)}})
	}
	return sessions
}

func generateTestCache(t *testing.T, vinCount int) *SessionCache {
	t.Helper()
	c := New(0)
	for i := 0; i < vinCount; i++ {
		c.Vehicles[strconv.Itoa(i)] = generateTestSessions(i)
	}
	return c
}

func verifyCache(t *testing.T, c *SessionCache, entries []int) {
	t.Helper()
	found := make(map[string]bool)
	for _, i := range entries {
		vin := strconv.Itoa(i)
		if sessions, ok := c.Vehicles[vin]; ok {
			if len(sessions) != 3 {
				t.Errorf("cache %d did not contain %d sessions", i, testSessionCount)
				return
			}
			for j, entry := range sessions {
				good := entry.CreatedAt.Equal(time.Time{}.Add(time.Duration(i))) &&
					entry.Domain == i &&
					len(entry.SessionInfo) == 1 &&
					entry.SessionInfo[0] == byte(j)
				if !good {
					t.Errorf("session cache contained invalid entry %d", i)
					return
				}
			}
		} else {
			t.Errorf("session cache did not contain entry %d", i)
		}
		found[vin] = true
	}
	for vin := range c.Vehicles {
		if _, ok := found[vin]; !ok {
			t.Errorf("session cache contained extraneous entry %s", vin)
		}
	}
}

func TestImportExport(t *testing.T) {
	var buffer bytes.Buffer
	c := generateTestCache(t, 5)
	if err := c.Export(&buffer); err != nil {
		t.Fatal(err)
	}
	cc, err := Import(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	verifyCache(t, cc, []int{0, 1, 2, 3, 4})
}

func TestEviction(t *testing.T) {
	c := generateTestCache(t, 0)
	c.MaxEntries = 5
	// Note that generateTestSessions(n) adds an entry with timestamp n, and entries are evicted
	// based on timestamp, not the order in which they were added to the cache.
	c.Update("7", generateTestSessions(7))
	c.Update("4", generateTestSessions(4))
	c.Update("5", generateTestSessions(5))
	c.Update("3", generateTestSessions(3))
	c.Update("6", generateTestSessions(6))
	verifyCache(t, c, []int{3, 4, 5, 6, 7})

	// Duplicate key updated in place
	c.Update("5", generateTestSessions(5))
	verifyCache(t, c, []int{3, 4, 5, 6, 7})

	// Evicts oldest entry
	c.Update("8", generateTestSessions(8))
	verifyCache(t, c, []int{4, 5, 6, 7, 8})

	// Older entry doesn't evict newer entry
	c.Update("1", generateTestSessions(1))
	verifyCache(t, c, []int{4, 5, 6, 7, 8})
}

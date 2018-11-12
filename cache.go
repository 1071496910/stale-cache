package cache

import (
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotExist   error = fmt.Errorf("Key not exists")
	ErrKeyTimeout error = fmt.Errorf("Key timeout")
)

type StaleCacheOpOpts func(*StaleCacheEntry)

type StaleCacheEntry struct {
	key     interface{}
	val     interface{}
	dealine time.Time
}

func WithTimeout(t time.Duration) StaleCacheOpOpts {
	return func(sce *StaleCacheEntry) {
		sce.dealine = time.Now().Add(t)
	}
}

type StaleCache interface {
	Add(key, val interface{}, opts ...StaleCacheOpOpts) error
	Del(key interface{}) error
	Get(key interface{}) (interface{}, error)
}

type staleCache struct {
	cache map[interface{}]*StaleCacheEntry
	mtx   sync.Mutex
}

func (sc *staleCache) Add(key, val interface{}, opts ...StaleCacheOpOpts) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	entry, ok := sc.cache[key]
	if ok {
		entry.val = val
	} else {
		entry = new(StaleCacheEntry)
		sc.cache[key] = entry
	}
	for _, opt := range opts {
		opt(entry)
	}

	return nil
}

func (sc *staleCache) Del(key interface{}) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	if _, ok := sc.cache[key]; ok {
		delete(sc.cache, key)
		return nil
	}
	return ErrNotExist
}

func (sc *staleCache) Get(key interface{}) (interface{}, error) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	if entry, ok := sc.cache[key]; ok {
		if time.Now().Before(entry.dealine) {
			return entry.val, nil
		}
		delete(sc.cache, key)
		return nil, ErrKeyTimeout
	}
	return nil, ErrNotExist
}

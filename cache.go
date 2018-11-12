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

type staleCacheOpOpts func(*staleCacheEntry)

type staleCacheEntry struct {
	key     interface{}
	val     interface{}
	dealine time.Time
}

func WithTimeout(t time.Duration) staleCacheOpOpts {
	return func(sce *staleCacheEntry) {
		sce.dealine = time.Now().Add(t)
	}
}

type StaleCache interface {
	Add(key, val interface{}, opts ...staleCacheOpOpts) error
	Del(key interface{}) error
	Get(key interface{}) (interface{}, error)
}

type staleCache struct {
	cache        map[interface{}]*staleCacheEntry
	mtx          sync.Mutex
	scanInterval time.Duration
}

func (sc *staleCache) start() {

	go func() {
		ticker := time.NewTicker(sc.scanInterval)
		for _ = range ticker.C {
			sc.mtx.Lock()
		scanLoop:
			for key, val := range sc.cache {
				select {
				case <-time.After(10 * time.Millisecond):
					break scanLoop
				default:
					if time.Now().After(val.dealine) {
						delete(sc.cache, key)
					}
				}
			}
			sc.mtx.Unlock()
		}
	}()
}

func NewStaleCache(scanInterval time.Duration) StaleCache {
	sc := &staleCache{
		cache:        make(map[interface{}]*staleCacheEntry),
		scanInterval: scanInterval,
	}
	sc.start()
	return sc
}

func (sc *staleCache) Add(key, val interface{}, opts ...staleCacheOpOpts) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	entry, ok := sc.cache[key]
	if ok {
		entry.val = val
	} else {
		entry = new(staleCacheEntry)
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

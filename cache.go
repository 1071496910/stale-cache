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

const (
	scanDeadline time.Duration = 10 * time.Microsecond
	scanInterval time.Duration = 10 * time.Millisecond
)

type staleCacheOpOpts func(*staleCacheEntry)

type staleCacheEntry struct {
	key         interface{}
	val         interface{}
	deadline    time.Time
	hasDeadline bool
}

func WithTimeout(t time.Duration) staleCacheOpOpts {
	return func(sce *staleCacheEntry) {
		sce.deadline = time.Now().Add(t)
		sce.hasDeadline = true
	}
}

type StaleCache interface {
	Add(key, val interface{}, opts ...staleCacheOpOpts) error
	Del(key interface{}) error
	Get(key interface{}) (interface{}, error)
	Len() int
}

type staleCacheOpt func(*staleCache)

func SetScanInterval(t time.Duration) staleCacheOpt {
	return func(s *staleCache) {
		s.scanInterval = t
	}
}

func SetScanDeadline(t time.Duration) staleCacheOpt {
	return func(s *staleCache) {
		s.scanDeadline = t
	}
}

type staleCache struct {
	cache        map[interface{}]*staleCacheEntry
	mtx          sync.Mutex
	scanInterval time.Duration
	scanDeadline time.Duration
}

func (sc *staleCache) start() {

	go func() {
		ticker := time.NewTicker(sc.scanInterval)
		for _ = range ticker.C {
			sc.mtx.Lock()
			taskStopClock := time.After(sc.scanDeadline)
			expired := 0
		scanLoop:
			for key, val := range sc.cache {
				select {
				case <-taskStopClock:
					break scanLoop
				default:
					if val.hasDeadline && time.Now().After(val.deadline) {
						expired++
						delete(sc.cache, key)
					}
				}
			}
			fmt.Println("expired num:", expired)
			sc.mtx.Unlock()
		}
	}()
}

func NewStaleCache(opts ...staleCacheOpt) StaleCache {
	sc := &staleCache{
		cache:        make(map[interface{}]*staleCacheEntry),
		scanInterval: scanInterval,
		scanDeadline: scanDeadline,
	}
	for _, opt := range opts {
		opt(sc)
	}
	sc.start()
	return sc
}

func (sc *staleCache) Add(key, val interface{}, opts ...staleCacheOpOpts) error {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	entry := &staleCacheEntry{
		key: key,
		val: val,
	}
	for _, opt := range opts {
		opt(entry)
	}
	sc.cache[key] = entry

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
		if (!entry.hasDeadline) || time.Now().Before(entry.deadline) {
			return entry.val, nil
		}
		delete(sc.cache, key)
		return nil, ErrKeyTimeout
	}
	return nil, ErrNotExist
}

func (sc *staleCache) Len() int {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	return len(sc.cache)

}

package cache

import (
	"fmt"
	"iter"
	"sync"

	"golang.org/x/sync/singleflight"
)

type Cache[K comparable, V any] struct {
	group singleflight.Group
	mutex sync.RWMutex
	queue chan K
	state map[K]V
}

func New[K comparable, V any](size int) *Cache[K, V] {
	cache := makeCache[K, V](size)
	return &cache
}

func makeCache[K comparable, V any](size int) Cache[K, V] {
	return Cache[K, V]{
		queue: make(chan K, size),
		state: make(map[K]V, size),
	}
}

func (c *Cache[K, V]) Load(key K, load func() (V, error)) (V, error) {
	c.mutex.RLock()
	value, ok := c.state[key]
	c.mutex.RUnlock()
	if ok {
		return value, nil
	}
	// TODO: when singleflight adopts a generic API, get rid of this
	// ugly string conversion.
	skey := fmt.Sprint(key)
	ret, err, _ := c.group.Do(skey, func() (any, error) {
		c.mutex.RLock()
		value, ok := c.state[key]
		c.mutex.RUnlock()
		if ok {
			return value, nil
		}
		value, err := load()
		if err != nil {
			return nil, err
		}
		c.mutex.Lock()
		defer c.mutex.Unlock()

		if len(c.queue) == cap(c.queue) {
			oldest := <-c.queue
			delete(c.state, oldest)
		}

		c.state[key] = value
		c.queue <- key
		return value, nil
	})
	value, _ = ret.(V)
	return value, err
}

type SeqCache[K comparable, V any] struct{ cache Cache[K, []V] }

func Seq[K comparable, V any](size int) *SeqCache[K, V] {
	return &SeqCache[K, V]{
		cache: makeCache[K, []V](size),
	}
}

func (s *SeqCache[K, V]) Load(key K, load iter.Seq2[V, error]) iter.Seq2[V, error] {
	return func(yield func(V, error) bool) {
		values, err := s.cache.Load(key, func() ([]V, error) {
			var values []V
			for v, err := range load {
				if err != nil {
					return nil, err
				}
				values = append(values, v)
			}
			return values, nil
		})
		for _, v := range values {
			if !yield(v, nil) {
				return
			}
		}
		if err != nil {
			var zero V
			yield(zero, err)
		}
	}
}

package cache

import (
	"sync"

	"github.com/firetiger-oss/storage/cache/lru"
)

var ready = make(chan struct{})

func init() {
	close(ready)
}

type Stat struct {
	Limit     int64
	Entries   int64
	Size      int64
	Hits      int64
	Misses    int64
	Evictions int64
}

type Promise[T any] struct {
	ready <-chan struct{}
	value T
	error error
}

func (p *Promise[T]) Wait() (T, error) {
	<-p.ready
	return p.value, p.error
}

type LRU[K comparable, V any] struct {
	Limit    int64
	mutex    sync.Mutex
	cache    lru.LRU[K, V]
	inflight map[K]*Promise[V]
}

func (c *LRU[K, V]) Stat() (stat Stat) {
	c.mutex.Lock()
	stat.Limit = c.Limit
	stat.Entries = c.cache.Entries
	stat.Size = c.cache.Size
	stat.Hits = c.cache.Hits
	stat.Misses = c.cache.Misses
	stat.Evictions = c.cache.Evictions
	c.mutex.Unlock()
	return
}

func (c *LRU[K, V]) Clear() {
	c.mutex.Lock()
	c.cache.Clear()
	c.mutex.Unlock()
}

func (c *LRU[K, V]) Drop(ks ...K) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, k := range ks {
		c.cache.Delete(k)
	}
}

func (c *LRU[K, V]) get(k K, fetch func() (int64, V, error)) *Promise[V] {
	p := c.inflight[k]
	if p == nil {
		ready := make(chan struct{})
		p = &Promise[V]{ready: ready}
		if c.inflight == nil {
			c.inflight = make(map[K]*Promise[V])
		}
		c.inflight[k] = p
		go func() {
			defer close(ready)

			size, v, err := fetch()
			p.value, p.error = v, err
			c.mutex.Lock()
			defer c.mutex.Unlock()

			if err == nil && size < (c.Limit/2) {
				c.cache.Insert(k, v, size)
				for c.cache.Size > c.Limit {
					c.cache.Evict()
				}
			}

			delete(c.inflight, k)
		}()
	}
	return p
}

func (c *LRU[K, V]) Get(k K, fetch func() (int64, V, error)) *Promise[V] {
	var p *Promise[V]
	c.mutex.Lock()
	v, ok := c.cache.Lookup(k)
	if !ok {
		p = c.get(k, fetch)
	}
	c.mutex.Unlock()
	if ok {
		p = &Promise[V]{ready: ready, value: v}
	}
	return p
}

func (c *LRU[K, V]) Load(k K, fetch func() (int64, V, error)) (V, error) {
	c.mutex.Lock()
	v, ok := c.cache.Lookup(k)
	c.mutex.Unlock()
	if ok {
		return v, nil
	}
	return c.Get(k, fetch).Wait()
}

func (c *LRU[K, V]) Peek(k K) (V, bool) {
	c.mutex.Lock()
	v, ok := c.cache.Lookup(k)
	if !ok {
		if p := c.inflight[k]; p != nil {
			c.mutex.Unlock()
			<-p.ready
			return p.value, p.error == nil
		}
	}
	c.mutex.Unlock()
	return v, ok
}

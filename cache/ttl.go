package cache

import (
	"context"
	"time"
)

type TTL[K comparable, V any] struct {
	Limit           int64
	NewFetchContext NewFetchContext
	lru             LRU[K, ttlEntry[V]]
}

type ttlEntry[V any] struct {
	value  V
	expire time.Time
}

type NewFetchContext func() (context.Context, context.CancelFunc)

func (c *TTL[K, V]) innerLRU() *LRU[K, ttlEntry[V]] {
	c.lru.Limit = c.Limit
	return &c.lru
}

func (c *TTL[K, V]) Stat() Stat {
	return c.innerLRU().Stat()
}

func (c *TTL[K, V]) Clear() {
	c.innerLRU().Clear()
}

func (c *TTL[K, V]) Drop(ks ...K) {
	c.innerLRU().Drop(ks...)
}

func (c *TTL[K, V]) Load(ctx context.Context, key K, now time.Time, update bool, fetch func(context.Context) (int64, V, time.Time, error)) (value V, expire time.Time, err error) {
	var promise *Promise[ttlEntry[V]]
	lru := c.innerLRU()

	lru.mutex.Lock()
	entry, ok := lru.cache.Lookup(key)
	if !ok || update || (!entry.expire.IsZero() && now.After(entry.expire)) {
		if err = ctx.Err(); err != nil {
			lru.mutex.Unlock()
			return value, expire, context.Cause(ctx)
		}
		newFetchContext := c.NewFetchContext
		promise = lru.get(key, func() (int64, ttlEntry[V], error) {
			fetchCtx := context.Background()
			cancel := func() {}
			if newFetchContext != nil {
				fetchCtx, cancel = newFetchContext()
			}
			defer cancel()

			size, value, expire, err := fetch(fetchCtx)
			if err != nil {
				return 0, ttlEntry[V]{}, err
			}
			return size, ttlEntry[V]{value: value, expire: expire}, nil
		})
	}
	lru.mutex.Unlock()

	if promise != nil {
		select {
		case <-promise.ready:
		case <-ctx.Done():
			return value, expire, context.Cause(ctx)
		}
		if promise.error != nil {
			return value, expire, promise.error
		}
		entry = promise.value
	}

	return entry.value, entry.expire, nil
}

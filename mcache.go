package goev

// Refer to github.com/patrickmn/go-cache

import (
	"runtime"
	"sync"
	"time"
)

type cacheItem struct {
	Object     any
	Expiration int64
}

// Returns true if the item has expired.
func (item cacheItem) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1

	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache struct {
	*cache
	// If this is confusing, see the comment at the bottom of New()
}

type cache struct {
	defaultExpiration time.Duration
	items             map[string]cacheItem
	mu                sync.RWMutex
	onEvicted         func(string, any)
	janitor           *janitor
}

// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	items := make(map[string]cacheItem)
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, items)
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache) Set(k string, x any, d time.Duration) {
	// "Inlining" of set
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	c.items[k] = cacheItem{
		Object:     x,
		Expiration: e,
	}
	// TODO: Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mu.Unlock()
}

func (c *cache) set(k string, x any, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = cacheItem{
		Object:     x,
		Expiration: e,
	}
}

// Add an item to the cache, replacing any existing item, using the default
// expiration.
func (c *cache) SetDefault(k string, x any) {
	c.Set(k, x, DefaultExpiration)
}

// Set item expire, Return error if item not found or had expired
func (c *cache) Expire(k string, d time.Duration) bool {
	c.mu.Lock()
	item, found := c.items[k]
	if !found {
		c.mu.Unlock()
		return false
	}
	delete(c.items, k)
	c.set(k, item, d)
	c.mu.Unlock()
	return true
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache) Add(k string, x any, d time.Duration) bool {
	c.mu.Lock()
	_, found := c.get(k)
	if found {
		c.mu.Unlock()
		return false
	}
	c.set(k, x, d)
	c.mu.Unlock()
	return true
}

// Set a new value for the cache key only if it already exists, and the existing
// item hasn't expired. Returns an error otherwise.
func (c *cache) Replace(k string, x any, d time.Duration) bool {
	c.mu.Lock()
	_, found := c.get(k)
	if !found {
		c.mu.Unlock()
		return false
	}
	c.set(k, x, d)
	c.mu.Unlock()
	return true
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache) Get(k string) (any, bool) {
	c.mu.RLock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return nil, false
	}
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return nil, false
		}
	}
	c.mu.RUnlock()
	return item.Object, true
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or nil, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.
func (c *cache) GetWithExpiration(k string) (any, time.Time, bool) {
	c.mu.RLock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return nil, time.Time{}, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return nil, time.Time{}, false
		}

		// Return the item and the expiration time
		c.mu.RUnlock()
		return item.Object, time.Unix(0, item.Expiration), true
	}

	// If expiration <= 0 (i.e. no expiration time set) then return the item
	// and a zeroed time.Time
	c.mu.RUnlock()
	return item.Object, time.Time{}, true
}

func (c *cache) get(k string) (any, bool) {
	item, found := c.items[k]
	if !found {
		return nil, false
	}
	// "Inlining" of Expired
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			return nil, false
		}
	}
	return item.Object, true
}

// Pop gets an item from the cache and deletes it.
//
// The bool return indicates if the item was set.
func (c *cache) Pop(k string) (any, bool) {
	c.mu.Lock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.Unlock()
		return nil, false
	}
	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		c.mu.Unlock()
		return nil, false
	}

	v, evicted := c.delete(k)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}

	return item.Object, true
}

// Return the keys exists or not
func (c *cache) Exist(k string) bool {
	c.mu.RLock()
	// "Inlining" of get and Expired
	item, found := c.items[k]
	if !found {
		c.mu.RUnlock()
		return false
	}
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return false
		}
	}
	c.mu.RUnlock()
	return true
}

// Increment an item of type int by n. Returns an error if the item's value is
// not an int. If there is no error, the incremented value is returned.
func (c *cache) IncrementInt(k string, n int, d time.Duration) (int, bool) {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.set(k, n, d)
		c.mu.Unlock()
		return n, true
	}
	rv, ok := v.Object.(int)
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	nv := rv + n
	v.Object = nv
	c.items[k] = v
	c.mu.Unlock()
	return nv, true
}

// Increment an item of type int64 by n. Returns an error if the item's value is
// not an int64. If there is no error, the incremented value is returned.
func (c *cache) IncrementInt64(k string, n int64, d time.Duration) (int64, bool) {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.set(k, n, d)
		c.mu.Unlock()
		return n, true
	}
	rv, ok := v.Object.(int64)
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	nv := rv + n
	v.Object = nv
	c.items[k] = v
	c.mu.Unlock()
	return nv, true
}

// Increment an item of type float64 by n. Returns an error if the item's value
// is not an float64. If there is no error, the incremented value is returned.
func (c *cache) IncrementFloat64(k string, n float64, d time.Duration) (float64, bool) {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.set(k, n, d)
		c.mu.Unlock()
		return n, true
	}
	rv, ok := v.Object.(float64)
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	nv := rv + n
	v.Object = nv
	c.items[k] = v
	c.mu.Unlock()
	return nv, true
}

// Decrement an item of type int by n. Returns an error if the item's value is
// not an int. If there is no error, the decremented value is returned.
func (c *cache) DecrementInt(k string, n int, d time.Duration) (int, bool) {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.set(k, n, d)
		c.mu.Unlock()
		return n, true
	}
	rv, ok := v.Object.(int)
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	nv := rv - n
	v.Object = nv
	c.items[k] = v
	c.mu.Unlock()
	return nv, true
}

// Decrement an item of type int64 by n. Returns an error if the item's value is
// not an int64. If there is no error, the decremented value is returned.
func (c *cache) DecrementInt64(k string, n int64, d time.Duration) (int64, bool) {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.set(k, n, d)
		c.mu.Unlock()
		return n, true
	}
	rv, ok := v.Object.(int64)
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	nv := rv - n
	v.Object = nv
	c.items[k] = v
	c.mu.Unlock()
	return nv, true
}

// Decrement an item of type float64 by n. Returns an error if the item's value
// is not an float64. If there is no error, the decremented value is returned.
func (c *cache) DecrementFloat64(k string, n float64, d time.Duration) (float64, bool) {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.set(k, n, d)
		c.mu.Unlock()
		return n, true
	}
	rv, ok := v.Object.(float64)
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	nv := rv - n
	v.Object = nv
	c.items[k] = v
	c.mu.Unlock()
	return nv, true
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache) Delete(k string) {
	c.mu.Lock()
	v, evicted := c.delete(k)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}
}

func (c *cache) delete(k string) (any, bool) {
	if c.onEvicted != nil {
		if v, found := c.items[k]; found {
			delete(c.items, k)
			return v.Object, true
		}
	}
	delete(c.items, k)
	return nil, false
}

type keyAndValue struct {
	key   string
	value any
}

// Delete all expired items from the cache.
func (c *cache) deleteExpired() {
	var evictedItems []keyAndValue
	now := time.Now().UnixNano()
	c.mu.Lock()
	for k, v := range c.items {
		// "Inlining" of expired
		if v.Expiration > 0 && now > v.Expiration {
			ov, evicted := c.delete(k)
			if evicted {
				evictedItems = append(evictedItems, keyAndValue{k, ov})
			}
		}
	}
	c.mu.Unlock()
	for _, v := range evictedItems {
		c.onEvicted(v.key, v.value)
	}
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *cache) OnEvicted(f func(string, any)) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *cache) Items() map[string]cacheItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := make(map[string]cacheItem, len(c.items))
	now := time.Now().UnixNano()
	for k, v := range c.items {
		// "Inlining" of Expired
		if v.Expiration > 0 {
			if now > v.Expiration {
				continue
			}
		}
		m[k] = v
	}
	return m
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *cache) ItemCount() int {
	c.mu.RLock()
	n := len(c.items)
	c.mu.RUnlock()
	return n
}

// Delete all items from the cache.
func (c *cache) Flush() {
	c.mu.Lock()
	c.items = map[string]cacheItem{}
	c.mu.Unlock()
}

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(c *cache) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c *Cache) {
	c.janitor.stop <- true
}

func runJanitor(c *cache, ci time.Duration) {
	j := &janitor{
		Interval: ci,
		stop:     make(chan bool),
	}
	c.janitor = j
	go j.Run(c)
}

func newCache(de time.Duration, m map[string]cacheItem) *cache {
	if de == 0 {
		de = -1
	}
	c := &cache{
		defaultExpiration: de,
		items:             m,
	}
	return c
}

func newCacheWithJanitor(de time.Duration, ci time.Duration, m map[string]cacheItem) *Cache {
	c := newCache(de, m)
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Cache{c}
	if ci > 0 {
		runJanitor(c, ci)
		runtime.SetFinalizer(C, stopJanitor)
	}
	return C
}

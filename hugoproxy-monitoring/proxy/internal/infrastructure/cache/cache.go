package cache

import (
	"sync"
	"time"
)

// Cache интерфейс для кэширования данных
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
}

// InMemoryCache реализация in-memory кэша
type InMemoryCache struct {
	mu      sync.RWMutex
	items   map[string]cacheItem
	stopCh  chan struct{}
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewInMemoryCache создает новый экземпляр in-memory кэша
func NewInMemoryCache() *InMemoryCache {
	cache := &InMemoryCache{
		items:  make(map[string]cacheItem),
		stopCh: make(chan struct{}),
	}
	go cache.startCleanup()
	return cache
}

// Get возвращает значение по ключу
func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists || time.Now().After(item.expiration) {
		return nil, false
	}
	return item.value, true
}

// Set устанавливает значение по ключу с TTL
func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// Delete удаляет значение по ключу
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Stop останавливает фоновую очистку
func (c *InMemoryCache) Stop() {
	close(c.stopCh)
}

// startCleanup запускает фоновую очистку устаревших записей
func (c *InMemoryCache) startCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			cleaned := 0
			for key, item := range c.items {
				if time.Now().After(item.expiration) {
					delete(c.items, key)
					cleaned++
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

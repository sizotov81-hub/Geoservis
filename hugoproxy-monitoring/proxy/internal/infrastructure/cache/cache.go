package cache

import (
	"log"
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
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewInMemoryCache создает новый экземпляр in-memory кэша
func NewInMemoryCache() *InMemoryCache {
	cache := &InMemoryCache{
		items: make(map[string]cacheItem),
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

	log.Printf("Setting cache for key: %s with TTL: %v", key, ttl)
	c.items[key] = cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// Delete удаляет значение по ключу
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Printf("Deleting cache for key: %s", key)
	delete(c.items, key)
}

// startCleanup запускает фоновую очистку устаревших записей
func (c *InMemoryCache) startCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		cleaned := 0
		for key, item := range c.items {
			if time.Now().After(item.expiration) {
				delete(c.items, key)
				cleaned++
			}
		}
		c.mu.Unlock()
		if cleaned > 0 {
			log.Printf("Cache cleanup: removed %d expired items", cleaned)
		}
	}
}

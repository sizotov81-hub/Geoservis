package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryCache_GetSet(t *testing.T) {
	cache := NewInMemoryCache()
	key := "testKey"
	value := "testValue"

	// Устанавливаем значение
	cache.Set(key, value, time.Minute)

	// Получаем значение
	result, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, value, result)
}

func TestInMemoryCache_Expiration(t *testing.T) {
	cache := NewInMemoryCache()
	key := "testKey"
	value := "testValue"

	// Устанавливаем значение с очень коротким TTL
	cache.Set(key, value, time.Millisecond)

	// Ждем истечения TTL
	time.Sleep(2 * time.Millisecond)

	// Пытаемся получить значение
	result, found := cache.Get(key)
	assert.False(t, found)
	assert.Nil(t, result)
}

func TestInMemoryCache_Delete(t *testing.T) {
	cache := NewInMemoryCache()
	key := "testKey"
	value := "testValue"

	// Устанавливаем значение
	cache.Set(key, value, time.Minute)

	// Убеждаемся, что значение есть
	result, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, value, result)

	// Удаляем значение
	cache.Delete(key)

	// Убеждаемся, что значение удалено
	result, found = cache.Get(key)
	assert.False(t, found)
	assert.Nil(t, result)
}

func TestInMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewInMemoryCache()
	key := "testKey"
	value := "testValue"

	// Запускаем несколько горутин для конкурентного доступа
	for i := 0; i < 10; i++ {
		go func() {
			cache.Set(key, value, time.Minute)
			cache.Get(key)
			cache.Delete(key)
		}()
	}

	// Даем время на выполнение
	time.Sleep(100 * time.Millisecond)
}

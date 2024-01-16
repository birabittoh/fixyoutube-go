package volatile

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

type Element[V any] struct {
	value     *V
	Timestamp time.Time
}

type Volatile[K comparable, V any] struct {
	data       map[K]Element[V]
	timeToLive time.Duration
}

func (v *Volatile[K, V]) clean() int {
	now := time.Now()
	keysToDelete := []K{}

	for key, value := range v.data {
		if now.Sub(value.Timestamp) > v.timeToLive {
			keysToDelete = append(keysToDelete, key)
		}
	}
	for _, key := range keysToDelete {
		delete(v.data, key)
	}
	return len(keysToDelete)
}

func (v *Volatile[K, V]) cleanupRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			amt := v.clean()
			logger.Debug(amt, " elements were automatically removed from cache.")
		}
	}
}

func NewVolatile[K comparable, V any](timeToLive time.Duration, cleanupInterval time.Duration) *Volatile[K, V] {
	v := &Volatile[K, V]{
		data:       make(map[K]Element[V]),
		timeToLive: timeToLive,
	}
	go v.cleanupRoutine(cleanupInterval)
	return v
}

func (v *Volatile[K, V]) Has(key K) bool {
	v.clean()
	_, ok := v.data[key]
	return ok
}

func (v *Volatile[K, V]) Get(key K) (*V, error) {
	v.clean()
	element, ok := v.data[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return element.value, nil
}

func (v *Volatile[K, V]) Remove(key K) (*V, error) {
	v.clean()
	value, ok := v.data[key]

	if ok {
		delete(v.data, key)
		return value.value, nil
	}

	return nil, fmt.Errorf("not found")
}

func (v *Volatile[K, V]) Set(key K, value *V) error {
	v.data[key] = Element[V]{value: value, Timestamp: time.Now()}
	v.clean()
	return nil
}

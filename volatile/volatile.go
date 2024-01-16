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

func reverseIntArray(arr []int) {
	length := len(arr)
	for i := 0; i < length/2; i++ {
		arr[i], arr[length-i-1] = arr[length-i-1], arr[i]
	}
}

func NewVolatile[K comparable, V any](timeToLive time.Duration) *Volatile[K, V] {
	return &Volatile[K, V]{
		data:       make(map[K]Element[V]),
		timeToLive: timeToLive,
	}
}

func (v *Volatile[K, V]) clean() {
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

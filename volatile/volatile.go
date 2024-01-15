package volatile

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

type Element[K comparable, V any] struct {
	key       K
	value     *V
	Timestamp time.Time
}

type Volatile[K comparable, V any] struct {
	data       []Element[K, V]
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
		data:       nil,
		timeToLive: timeToLive,
	}
}

func (v *Volatile[K, V]) removeIndex(index int) error {
	if index < 0 || index >= len(v.data) {
		return fmt.Errorf("Index out of bounds")
	}
	v.data = append(v.data[:index], v.data[index+1:]...)
	return nil
}

func (v *Volatile[K, V]) clean() error {
	now := time.Now()

	for i := len(v.data) - 1; i >= 0; i-- {
		if now.Sub(v.data[i].Timestamp) > v.timeToLive {
			err := v.removeIndex(i)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *Volatile[K, V]) indexOf(key K) int {
	for i := range v.data {
		e := v.data[i]
		if e.key == key {
			return i
		}
	}
	return -1
}

func (v *Volatile[K, V]) Has(key K) bool {
	err := v.clean()
	if err != nil {
		logger.Println(err)
		return false
	}
	return v.indexOf(key) != -1
}

func (v *Volatile[K, V]) Get(key K) (*V, error) {
	err := v.clean()
	if err != nil {
		logger.Println(err)
		return nil, err
	}

	i := v.indexOf(key)
	if i == -1 {
		return nil, fmt.Errorf("Not found")
	}
	return v.data[i].value, nil
}

func (v *Volatile[K, V]) Remove(key K) (*V, error) {
	i := v.indexOf(key)
	if i == -1 {
		err := fmt.Errorf("Can't remove unexisting index")
		logger.Warn("Trying to delete unexisting key: ", key)
		return nil, err
	}

	value := &v.data[i].value
	err := v.removeIndex(i)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	err = v.clean()
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return *value, nil
}

func (v *Volatile[K, V]) Set(key K, value *V) error {
	err := v.clean()
	if err != nil {
		logger.Error(err)
		return err
	}

	v.Remove(key)

	e := Element[K, V]{key: key, value: value, Timestamp: time.Now()}
	v.data = append(v.data, e)
	return nil
}

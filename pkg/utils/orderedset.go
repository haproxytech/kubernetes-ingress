package utils

import "sort"

type OrderedSet[K comparable, T any] struct {
	items  []T
	seen   map[K]struct{}
	keyFn  func(T) K
	lessFn func(a, b T) bool
}

func NewOrderedSet[K comparable, T any](keyFn func(T) K, lessFn func(a, b T) bool) *OrderedSet[K, T] {
	return &OrderedSet[K, T]{
		items:  []T{},
		seen:   make(map[K]struct{}),
		keyFn:  keyFn,
		lessFn: lessFn,
	}
}

func (os *OrderedSet[K, T]) Add(item T) {
	key := os.keyFn(item)
	if _, exists := os.seen[key]; exists {
		return
	}

	i := sort.Search(len(os.items), func(i int) bool {
		return os.lessFn(item, os.items[i])
	})
	os.items = append(os.items, item)
	copy(os.items[i+1:], os.items[i:])
	os.items[i] = item
	os.seen[key] = struct{}{}
}

func (os *OrderedSet[K, T]) Remove(item T) {
	key := os.keyFn(item)
	if _, exists := os.seen[key]; !exists {
		return
	}
	delete(os.seen, key)
	for i, v := range os.items {
		if os.keyFn(v) == key {
			os.items = append(os.items[:i], os.items[i+1:]...)
			break
		}
	}
}

func (os *OrderedSet[K, T]) Items() []T {
	return os.items
}

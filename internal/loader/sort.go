package loader

import (
	"cmp"
	"slices"
)

func sortedCopy[T any](items []T, key func(T) string) []T {
	cp := make([]T, len(items))
	copy(cp, items)
	slices.SortFunc(cp, func(a, b T) int {
		return cmp.Compare(key(a), key(b))
	})
	return cp
}

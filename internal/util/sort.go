package util

import (
	"github.com/fvbommel/sortorder"
)

// SortColumnsNaturally represents the type for sorting columns in a natural order from left to right.
type SortColumnsNaturally [][]string

func (a SortColumnsNaturally) Len() int {
	return len(a)
}

func (a SortColumnsNaturally) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a SortColumnsNaturally) Less(i, j int) bool {
	for k := range a[i] {
		if a[i][k] == a[j][k] {
			continue
		}

		if a[i][k] == "" {
			return false
		}

		if a[j][k] == "" {
			return true
		}

		return sortorder.NaturalLess(a[i][k], a[j][k])
	}

	return false
}

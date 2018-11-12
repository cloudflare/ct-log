// Package custom implements specific ways to get/modify ct-log data on cheap
// cloud infrastructure.
package custom

// int64Slice implements sort.Interface over a slice of int64s.
type int64Slice []int64

func (is int64Slice) Len() int           { return len(is) }
func (is int64Slice) Less(i, j int) bool { return is[i] < is[j] }
func (is int64Slice) Swap(i, j int)      { is[i], is[j] = is[j], is[i] }

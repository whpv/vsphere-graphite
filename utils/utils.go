package utils

import (
	"math"
) 

func Min(n ...int64) int64 {
	var min int64 = -1
	for _, i := range n {
		if i >= 0 {
			if min == -1 {
				min = i
			} else {
				if i < min {
					min = i
				}
			}
		}
	}
	return min
}

func Max(n ...int64) int64 {
	var max int64 = -1
	for _, i := range n {
		if i >= 0 {
			if max == -1 {
				max = i
			} else {
				if i > max {
					max = i
				}
			}
		}
	}
	return max
}

func Sum(n ...int64) int64 {
	var total int64 = 0
	for _, i := range n {
		if i > 0 {
			total += i
		}
	}
	return total
}

func Average(n ...int64) int64 {
	var total int64 = 0
	var count int64 = 0
	for _, i := range n {
		if i >= 0 {
			count += 1
			total += i
		}
	}
	favg := float64(total) / float64(count)
	return int64(math.Floor(favg + .5))
}

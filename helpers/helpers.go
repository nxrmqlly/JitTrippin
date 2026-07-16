package helpers

import "strconv"

// JobStepIndexMax returns a string formatted as:
//
//	'<jobName>/<stepName>' (<index>/<maxIndex>)
//
// Example:
//
//	'build/test' (2/5)
func JobStepIndexMax(jobName, stepName string, index, maxIndex int) string {
	return "'" + jobName + "/" + stepName + "' (" +
		strconv.Itoa(index) + "/" +
		strconv.Itoa(maxIndex) + ")"
}

func IndexOf[T comparable](s []T, v T) int {
	for i, item := range s {
		if item == v {
			return i
		}
	}
	return -1
}

func PopBack[T any](s *[]T) (T, bool) {
	if len(*s) == 0 {
		var zero T
		return zero, false
	}
	last := len(*s) - 1
	element := (*s)[last]

	// zero out to avoid memory leaks if T is a pointer
	var zero T
	(*s)[last] = zero

	*s = (*s)[:last]
	return element, true
}

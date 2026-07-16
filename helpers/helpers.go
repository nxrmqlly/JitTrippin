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

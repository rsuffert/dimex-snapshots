package common

// Any checks if any element in the provided slice satisfies the given predicate function.
// It iterates over the slice and applies the predicate to each element.
// If the predicate returns true for any element, Any returns true; otherwise, it returns false.
//
// Parameters:
//
//	items: A slice of elements of type T to be checked.
//	predicate: A function that takes an element of type T and returns a boolean,
//	           indicating whether the element satisfies a condition.
//
// Returns:
//
//	bool: True if any element in the slice satisfies the predicate, otherwise false.
func Any[T any](items []T, predicate func(T) bool) bool {
	for _, item := range items {
		if predicate(item) {
			return true
		}
	}
	return false
}

// Count iterates over a slice of items of any type and counts how many of them satisfy
// the given predicate function.
//
// Parameters:
//   - items: A slice of items of any type.
//   - predicate: A function that takes an item of the same type as the slice and returns
//     a boolean indicating whether the item satisfies a condition.
//
// Returns:
//   - An integer representing the number of items in the slice that satisfy the predicate.
func Count[T any](items []T, predicate func(T) bool) int {
	count := 0
	for _, item := range items {
		if predicate(item) {
			count++
		}
	}
	return count
}

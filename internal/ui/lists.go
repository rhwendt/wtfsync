package ui

// list returns s, or an empty list for nil: Wails sends a nil slice to the pages as null.
func list[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

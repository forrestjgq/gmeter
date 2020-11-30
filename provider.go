package gmeter

// provider should be a component that provides mapped information
type provider interface {
	// get content from a source by a key, index is used to tell provider the sequence of tests
	get(key string, index int64) string
}

package config

// Arcee gives a configuration of arcee server.
// gmeter will start an arcee server listening on Port, and save any
// received file into Path
type Arcee struct {
	Path string // directory where to save received files
	Port int    // Arcee server listening port
}

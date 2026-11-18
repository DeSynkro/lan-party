// Package config resolves where the app keeps its files.
package config

// Paths are the two directories the app owns. Settings are small and belong
// beside the user's other configuration; group data is larger and belongs in
// the data directory.
type Paths struct {
	ConfigDir string
	DataDir   string
}

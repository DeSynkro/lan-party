// Package buildinfo carries the version string stamped into the binary.
package buildinfo

// Version is replaced at build time with -ldflags. Untagged builds say dev.
var Version = "dev"

package main

import (
	"io"
	"log"
	"os"
	"runtime"

	"github.com/desynkro/lan-party/internal/applog"
	"github.com/desynkro/lan-party/internal/buildinfo"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/gtkui"
)

func main() {
	paths := config.Resolve()
	var logFile *os.File
	if file, err := applog.Open(paths.DataDir); err == nil {
		logFile = file
		log.SetOutput(io.MultiWriter(file, os.Stderr))
	} else {
		log.Printf("lan-party: log file: %v", err)
	}

	log.Printf("LAN Party %s starting on %s/%s", buildinfo.Version, runtime.GOOS, runtime.GOARCH)
	code := gtkui.Run(os.Args)
	log.Printf("LAN Party exiting with code %d", code)
	if logFile != nil {
		_ = logFile.Close()
	}
	os.Exit(code)
}

package main

import (
	"os"

	"github.com/desynkro/lan-party/internal/gtkui"
)

func main() {
	os.Exit(gtkui.RunProbe(os.Args))
}

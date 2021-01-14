package main

import (
	"io"
	"os"
	"path/filepath"

	controller "github.com/kvaster/topols/pkg/topols-controller/cmd"
	node "github.com/kvaster/topols/pkg/topols-node/cmd"
	scheduler "github.com/kvaster/topols/pkg/topols-scheduler/cmd"
)

func usage() {
	io.WriteString(os.Stderr, `Usage: hypertopols COMMAND [ARGS ...]

COMMAND:
    topols-controller:  TopoLS CSI controller service.
    topols-node:        TopoLS CSI node service.
    topols-scheduler:   Scheduler extender.
`)
}

func main() {
	name := filepath.Base(os.Args[0])
	if name == "hypertopols" {
		if len(os.Args) == 1 {
			usage()
			os.Exit(1)
		}
		name = os.Args[1]
		os.Args = os.Args[1:]
	}

	switch name {
	case "topols-scheduler":
		scheduler.Execute()
	case "topols-node":
		node.Execute()
	case "topols-controller":
		controller.Execute()
	default:
		usage()
		os.Exit(1)
	}
}

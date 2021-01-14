package main

import (
	"github.com/kvaster/topols/pkg/topols-node/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	cmd.Execute()
}

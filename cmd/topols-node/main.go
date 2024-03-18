package main

import (
	"github.com/kvaster/topols/cmd/topols-node/app"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	app.Execute()
}

//go:build tools
// +build tools

package topols

import (
	// These are to declare dependency on tools
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)

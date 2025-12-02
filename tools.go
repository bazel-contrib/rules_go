//go:build tools

package rules_go

// These imports only exist to keep go.mod entries for packages that are referenced in BUILD files,
// but not in Go code.

import (
	_ "github.com/gogo/protobuf/proto"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/golang/protobuf/protoc-gen-go"
	_ "golang.org/x/net/context"
	_ "golang.org/x/crypto/cryptobyte"
	_ "google.golang.org/genproto/protobuf/api"
	_ "google.golang.org/genproto/googleapis/bytestream"
	_ "google.golang.org/genproto/googleapis/rpc/code"
	_ "google.golang.org/genproto/googleapis/type/color"
	_ "google.golang.org/grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)

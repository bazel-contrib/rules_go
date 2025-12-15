package client

import (
	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
)

type Client interface {
	Connect(grpc.ClientConnInterface) *bytestream.ByteStreamClient
}

Testing that the protoc-gen-go-grpc plugin works
=======================================

hello_test
------------------

Verifies that the generated code for a simple gRPC service:
- exposes a `pb.UnimplementedGreetServer` style struct
- exposes a `pb.RegisterGreetServer` method that accepts a `grpc.ServiceRegistrar` instead of a raw `*grpc.Server`

package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

// rawCodec passes raw bytes without protobuf serialization.
type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error)         { return v.([]byte), nil }
func (rawCodec) Unmarshal(data []byte, v any) error     { *v.(*[]byte) = data; return nil }
func (rawCodec) Name() string                           { return "raw" }

func init() { encoding.RegisterCodec(rawCodec{}) }

// authService is the interface required by gRPC's RegisterService (HandlerType must be an interface).
type authService interface {
	Verify(ctx context.Context, req []byte) ([]byte, error)
}

type authServer struct{}

func (s *authServer) Verify(_ context.Context, _ []byte) ([]byte, error) {
	return []byte(`{"valid":true}`), nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth: failed to listen: %v\n", err)
		os.Exit(1)
	}

	s := grpc.NewServer()

	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "auth.Auth",
		HandlerType: (*authService)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Verify",
				Handler: func(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					var req []byte
					if err := dec(&req); err != nil {
						return nil, err
					}
					return []byte(`{"valid":true}`), nil
				},
			},
		},
		Streams: []grpc.StreamDesc{},
	}, &authServer{})

	fmt.Fprintln(os.Stderr, "auth: listening on :50051 (gRPC)")
	if err := s.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "auth: %v\n", err)
		os.Exit(1)
	}
}

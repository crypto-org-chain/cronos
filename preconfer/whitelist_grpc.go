package preconfer

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosmos/cosmos-sdk/client"
)

// Whitelist gRPC request/response types

type AddToWhitelistRequest struct {
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address,proto3"`
}

func (m *AddToWhitelistRequest) Reset()         { *m = AddToWhitelistRequest{} }
func (m *AddToWhitelistRequest) String() string { return fmt.Sprintf("Address: %s", m.Address) }
func (*AddToWhitelistRequest) ProtoMessage()    {}

type AddToWhitelistResponse struct {
	Success        bool   `json:"success,omitempty"         protobuf:"varint,1,opt,name=success,proto3"`
	Message        string `json:"message,omitempty"         protobuf:"bytes,2,opt,name=message,proto3"`
	WhitelistCount int32  `json:"whitelist_count,omitempty" protobuf:"varint,3,opt,name=whitelist_count,json=whitelistCount,proto3"`
}

func (m *AddToWhitelistResponse) Reset()         { *m = AddToWhitelistResponse{} }
func (m *AddToWhitelistResponse) String() string { return m.Message }
func (*AddToWhitelistResponse) ProtoMessage()    {}

type RemoveFromWhitelistRequest struct {
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address,proto3"`
}

func (m *RemoveFromWhitelistRequest) Reset()         { *m = RemoveFromWhitelistRequest{} }
func (m *RemoveFromWhitelistRequest) String() string { return fmt.Sprintf("Address: %s", m.Address) }
func (*RemoveFromWhitelistRequest) ProtoMessage()    {}

type RemoveFromWhitelistResponse struct {
	Success        bool   `json:"success,omitempty"         protobuf:"varint,1,opt,name=success,proto3"`
	Message        string `json:"message,omitempty"         protobuf:"bytes,2,opt,name=message,proto3"`
	WhitelistCount int32  `json:"whitelist_count,omitempty" protobuf:"varint,3,opt,name=whitelist_count,json=whitelistCount,proto3"`
}

func (m *RemoveFromWhitelistResponse) Reset()         { *m = RemoveFromWhitelistResponse{} }
func (m *RemoveFromWhitelistResponse) String() string { return m.Message }
func (*RemoveFromWhitelistResponse) ProtoMessage()    {}

type GetWhitelistRequest struct{}

func (m *GetWhitelistRequest) Reset()         { *m = GetWhitelistRequest{} }
func (m *GetWhitelistRequest) String() string { return "" }
func (*GetWhitelistRequest) ProtoMessage()    {}

type GetWhitelistResponse struct {
	Addresses []string `json:"addresses,omitempty" protobuf:"bytes,1,rep,name=addresses,proto3"`
	Count     int32    `json:"count,omitempty"     protobuf:"varint,2,opt,name=count,proto3"`
}

func (m *GetWhitelistResponse) Reset()         { *m = GetWhitelistResponse{} }
func (m *GetWhitelistResponse) String() string { return fmt.Sprintf("Count: %d", m.Count) }
func (*GetWhitelistResponse) ProtoMessage()    {}

type ClearWhitelistRequest struct{}

func (m *ClearWhitelistRequest) Reset()         { *m = ClearWhitelistRequest{} }
func (m *ClearWhitelistRequest) String() string { return "" }
func (*ClearWhitelistRequest) ProtoMessage()    {}

type ClearWhitelistResponse struct {
	Success bool   `json:"success,omitempty" protobuf:"varint,1,opt,name=success,proto3"`
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message,proto3"`
}

func (m *ClearWhitelistResponse) Reset()         { *m = ClearWhitelistResponse{} }
func (m *ClearWhitelistResponse) String() string { return m.Message }
func (*ClearWhitelistResponse) ProtoMessage()    {}

type SetWhitelistRequest struct {
	Addresses []string `json:"addresses,omitempty" protobuf:"bytes,1,rep,name=addresses,proto3"`
}

func (m *SetWhitelistRequest) Reset()         { *m = SetWhitelistRequest{} }
func (m *SetWhitelistRequest) String() string { return fmt.Sprintf("Addresses: %d", len(m.Addresses)) }
func (*SetWhitelistRequest) ProtoMessage()    {}

type SetWhitelistResponse struct {
	Success        bool   `json:"success,omitempty"         protobuf:"varint,1,opt,name=success,proto3"`
	Message        string `json:"message,omitempty"         protobuf:"bytes,2,opt,name=message,proto3"`
	WhitelistCount int32  `json:"whitelist_count,omitempty" protobuf:"varint,3,opt,name=whitelist_count,json=whitelistCount,proto3"`
}

func (m *SetWhitelistResponse) Reset()         { *m = SetWhitelistResponse{} }
func (m *SetWhitelistResponse) String() string { return m.Message }
func (*SetWhitelistResponse) ProtoMessage()    {}

// WhitelistServiceServer defines the gRPC service interface for whitelist management
type WhitelistServiceServer interface {
	AddToWhitelist(context.Context, *AddToWhitelistRequest) (*AddToWhitelistResponse, error)
	RemoveFromWhitelist(context.Context, *RemoveFromWhitelistRequest) (*RemoveFromWhitelistResponse, error)
	GetWhitelist(context.Context, *GetWhitelistRequest) (*GetWhitelistResponse, error)
	ClearWhitelist(context.Context, *ClearWhitelistRequest) (*ClearWhitelistResponse, error)
	SetWhitelist(context.Context, *SetWhitelistRequest) (*SetWhitelistResponse, error)
}

// WhitelistGRPCServer implements the gRPC server for whitelist management
type WhitelistGRPCServer struct {
	mempool *Mempool
}

// NewWhitelistGRPCServer creates a new whitelist gRPC server
func NewWhitelistGRPCServer(mempool *Mempool) *WhitelistGRPCServer {
	return &WhitelistGRPCServer{
		mempool: mempool,
	}
}

// AddToWhitelist adds an address to the whitelist
func (s *WhitelistGRPCServer) AddToWhitelist(
	ctx context.Context,
	req *AddToWhitelistRequest,
) (*AddToWhitelistResponse, error) {
	if req == nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}

	if s.mempool == nil {
		return nil, status.Error(codes.Internal, "mempool not available")
	}

	s.mempool.AddToWhitelist(req.Address)

	return &AddToWhitelistResponse{
		Success:        true,
		Message:        fmt.Sprintf("Address %s added to whitelist", req.Address),
		WhitelistCount: int32(s.mempool.WhitelistCount()),
	}, nil
}

// RemoveFromWhitelist removes an address from the whitelist
func (s *WhitelistGRPCServer) RemoveFromWhitelist(
	ctx context.Context,
	req *RemoveFromWhitelistRequest,
) (*RemoveFromWhitelistResponse, error) {
	if req == nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}

	if s.mempool == nil {
		return nil, status.Error(codes.Internal, "mempool not available")
	}

	s.mempool.RemoveFromWhitelist(req.Address)

	return &RemoveFromWhitelistResponse{
		Success:        true,
		Message:        fmt.Sprintf("Address %s removed from whitelist", req.Address),
		WhitelistCount: int32(s.mempool.WhitelistCount()),
	}, nil
}

// GetWhitelist returns all addresses in the whitelist
func (s *WhitelistGRPCServer) GetWhitelist(
	ctx context.Context,
	req *GetWhitelistRequest,
) (*GetWhitelistResponse, error) {
	if s.mempool == nil {
		return nil, status.Error(codes.Internal, "mempool not available")
	}

	addresses := s.mempool.GetWhitelist()

	return &GetWhitelistResponse{
		Addresses: addresses,
		Count:     int32(len(addresses)),
	}, nil
}

// ClearWhitelist removes all addresses from the whitelist
func (s *WhitelistGRPCServer) ClearWhitelist(
	ctx context.Context,
	req *ClearWhitelistRequest,
) (*ClearWhitelistResponse, error) {
	if s.mempool == nil {
		return nil, status.Error(codes.Internal, "mempool not available")
	}

	s.mempool.ClearWhitelist()

	return &ClearWhitelistResponse{
		Success: true,
		Message: "Whitelist cleared - all addresses now allowed",
	}, nil
}

// SetWhitelist replaces the entire whitelist
func (s *WhitelistGRPCServer) SetWhitelist(
	ctx context.Context,
	req *SetWhitelistRequest,
) (*SetWhitelistResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if s.mempool == nil {
		return nil, status.Error(codes.Internal, "mempool not available")
	}

	s.mempool.SetWhitelist(req.Addresses)

	message := "Whitelist updated"
	if len(req.Addresses) == 0 {
		message = "Whitelist cleared - all addresses now allowed"
	}

	return &SetWhitelistResponse{
		Success:        true,
		Message:        message,
		WhitelistCount: int32(s.mempool.WhitelistCount()),
	}, nil
}

// WhitelistQueryClient is the client API for whitelist queries
type WhitelistQueryClient interface {
	AddToWhitelist(ctx context.Context, in *AddToWhitelistRequest, opts ...grpc.CallOption) (*AddToWhitelistResponse, error)
	RemoveFromWhitelist(ctx context.Context, in *RemoveFromWhitelistRequest, opts ...grpc.CallOption) (*RemoveFromWhitelistResponse, error)
	GetWhitelist(ctx context.Context, in *GetWhitelistRequest, opts ...grpc.CallOption) (*GetWhitelistResponse, error)
	ClearWhitelist(ctx context.Context, in *ClearWhitelistRequest, opts ...grpc.CallOption) (*ClearWhitelistResponse, error)
	SetWhitelist(ctx context.Context, in *SetWhitelistRequest, opts ...grpc.CallOption) (*SetWhitelistResponse, error)
}

type whitelistQueryClient struct {
	cc grpc.ClientConnInterface
}

// NewWhitelistQueryClient creates a new whitelist query client from client context
func NewWhitelistQueryClient(clientCtx client.Context) WhitelistQueryClient {
	return &whitelistQueryClient{cc: clientCtx}
}

func (c *whitelistQueryClient) AddToWhitelist(ctx context.Context, in *AddToWhitelistRequest, opts ...grpc.CallOption) (*AddToWhitelistResponse, error) {
	out := new(AddToWhitelistResponse)
	err := c.cc.Invoke(ctx, "/preconfer.WhitelistService/AddToWhitelist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *whitelistQueryClient) RemoveFromWhitelist(ctx context.Context, in *RemoveFromWhitelistRequest, opts ...grpc.CallOption) (*RemoveFromWhitelistResponse, error) {
	out := new(RemoveFromWhitelistResponse)
	err := c.cc.Invoke(ctx, "/preconfer.WhitelistService/RemoveFromWhitelist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *whitelistQueryClient) GetWhitelist(ctx context.Context, in *GetWhitelistRequest, opts ...grpc.CallOption) (*GetWhitelistResponse, error) {
	out := new(GetWhitelistResponse)
	err := c.cc.Invoke(ctx, "/preconfer.WhitelistService/GetWhitelist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *whitelistQueryClient) ClearWhitelist(ctx context.Context, in *ClearWhitelistRequest, opts ...grpc.CallOption) (*ClearWhitelistResponse, error) {
	out := new(ClearWhitelistResponse)
	err := c.cc.Invoke(ctx, "/preconfer.WhitelistService/ClearWhitelist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *whitelistQueryClient) SetWhitelist(ctx context.Context, in *SetWhitelistRequest, opts ...grpc.CallOption) (*SetWhitelistResponse, error) {
	out := new(SetWhitelistResponse)
	err := c.cc.Invoke(ctx, "/preconfer.WhitelistService/SetWhitelist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RegisterWhitelistServiceServer registers the whitelist service with a gRPC server
func RegisterWhitelistServiceServer(s grpc.ServiceRegistrar, srv WhitelistServiceServer) {
	s.RegisterService(&WhitelistServiceDesc, srv)
}

// WhitelistServiceDesc is the gRPC service descriptor for whitelist management
var WhitelistServiceDesc = grpc.ServiceDesc{
	ServiceName: "preconfer.WhitelistService",
	HandlerType: (*WhitelistServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AddToWhitelist",
			Handler:    _WhitelistService_AddToWhitelist_Handler,
		},
		{
			MethodName: "RemoveFromWhitelist",
			Handler:    _WhitelistService_RemoveFromWhitelist_Handler,
		},
		{
			MethodName: "GetWhitelist",
			Handler:    _WhitelistService_GetWhitelist_Handler,
		},
		{
			MethodName: "ClearWhitelist",
			Handler:    _WhitelistService_ClearWhitelist_Handler,
		},
		{
			MethodName: "SetWhitelist",
			Handler:    _WhitelistService_SetWhitelist_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "preconfer/whitelist.proto",
}

func _WhitelistService_AddToWhitelist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddToWhitelistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WhitelistServiceServer).AddToWhitelist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.WhitelistService/AddToWhitelist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WhitelistServiceServer).AddToWhitelist(ctx, req.(*AddToWhitelistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WhitelistService_RemoveFromWhitelist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveFromWhitelistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WhitelistServiceServer).RemoveFromWhitelist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.WhitelistService/RemoveFromWhitelist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WhitelistServiceServer).RemoveFromWhitelist(ctx, req.(*RemoveFromWhitelistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WhitelistService_GetWhitelist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetWhitelistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WhitelistServiceServer).GetWhitelist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.WhitelistService/GetWhitelist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WhitelistServiceServer).GetWhitelist(ctx, req.(*GetWhitelistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WhitelistService_ClearWhitelist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClearWhitelistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WhitelistServiceServer).ClearWhitelist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.WhitelistService/ClearWhitelist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WhitelistServiceServer).ClearWhitelist(ctx, req.(*ClearWhitelistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WhitelistService_SetWhitelist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetWhitelistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WhitelistServiceServer).SetWhitelist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.WhitelistService/SetWhitelist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WhitelistServiceServer).SetWhitelist(ctx, req.(*SetWhitelistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

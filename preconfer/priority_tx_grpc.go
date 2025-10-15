package preconfer

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Ensure PriorityTxGRPCServer implements the gRPC service interface
var _ PriorityTxServiceServer = &PriorityTxGRPCServer{}

// PriorityTxGRPCServer implements the gRPC server for priority transactions
type PriorityTxGRPCServer struct {
	service *PriorityTxService
}

// NewPriorityTxGRPCServer creates a new gRPC server
func NewPriorityTxGRPCServer(service *PriorityTxService) *PriorityTxGRPCServer {
	return &PriorityTxGRPCServer{
		service: service,
	}
}

// Currently only priority level 1 is supported
const SupportedPriorityLevel = 1

// SubmitPriorityTx handles priority transaction submission via gRPC
func (s *PriorityTxGRPCServer) SubmitPriorityTx(
	ctx context.Context,
	req *SubmitPriorityTxRequest,
) (*SubmitPriorityTxResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if len(req.TxBytes) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty transaction bytes")
	}

	if req.PriorityLevel != SupportedPriorityLevel {
		return nil, status.Errorf(codes.InvalidArgument, "priority level must be %d", SupportedPriorityLevel)
	}

	// Submit transaction
	result, err := s.service.SubmitPriorityTx(ctx, req.TxBytes, req.PriorityLevel)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to submit transaction: %v", err))
	}

	// Build response
	resp := &SubmitPriorityTxResponse{
		TxHash:          result.TxHash,
		Accepted:        result.Accepted,
		Reason:          result.Reason,
		MempoolPosition: result.MempoolPosition,
	}

	// Add preconfirmation if available
	if result.Preconfirmation != nil {
		resp.Preconfirmation = convertPreconfirmationToProto(result.Preconfirmation)
	}

	return resp, nil
}

// GetPriorityTxStatus returns the status of a priority transaction
func (s *PriorityTxGRPCServer) GetPriorityTxStatus(
	ctx context.Context,
	req *GetPriorityTxStatusRequest,
) (*GetPriorityTxStatusResponse, error) {
	if req == nil || req.TxHash == "" {
		return nil, status.Error(codes.InvalidArgument, "tx hash is required")
	}

	// Get transaction status
	info, err := s.service.GetTxStatus(req.TxHash)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get tx status: %v", err))
	}

	resp := &GetPriorityTxStatusResponse{
		Status:          info.Status.String(),
		InMempool:       info.InMempool,
		BlockHeight:     info.BlockHeight,
		MempoolPosition: info.MempoolPosition,
		Timestamp:       info.Timestamp.Unix(),
	}

	if info.Preconfirmation != nil {
		resp.Preconfirmation = convertPreconfirmationToProto(info.Preconfirmation)
	}

	return resp, nil
}

// GetMempoolStats returns mempool statistics
func (s *PriorityTxGRPCServer) GetMempoolStats(
	ctx context.Context,
	req *GetMempoolStatsRequest,
) (*GetMempoolStatsResponse, error) {
	stats := s.service.GetMempoolStats()

	return &GetMempoolStatsResponse{
		TotalTxs:         stats.TotalTxs,
		PriorityTxs:      stats.PriorityTxs,
		NormalTxs:        stats.NormalTxs,
		PreconfirmedTxs:  stats.PreconfirmedTxs,
		AvgPriorityLevel: stats.AvgPriorityLevel,
		MempoolSizeBytes: stats.MempoolSizeBytes,
	}, nil
}

// ListPriorityTxs returns a list of priority transactions
func (s *PriorityTxGRPCServer) ListPriorityTxs(
	ctx context.Context,
	req *ListPriorityTxsRequest,
) (*ListPriorityTxsResponse, error) {
	// Default limit
	limit := uint32(100)
	if req.Pagination != nil && req.Pagination.Limit > 0 {
		limit = uint32(req.Pagination.Limit)
	}

	// Get priority transactions
	txs := s.service.ListPriorityTxs(limit)

	// Convert to proto
	protoTxs := make([]*PriorityTxInfo, len(txs))
	for i, tx := range txs {
		protoTxs[i] = &PriorityTxInfo{
			TxHash:        tx.TxHash,
			PriorityLevel: tx.PriorityLevel,
			Timestamp:     tx.Timestamp,
			SizeBytes:     tx.SizeBytes,
			Position:      tx.Position,
		}

		if req.IncludePreconfirmations && tx.Preconfirmation != nil {
			protoTxs[i].Preconfirmation = convertPreconfirmationToProto(tx.Preconfirmation)
		}
	}

	return &ListPriorityTxsResponse{
		Txs: protoTxs,
	}, nil
}

// Helper functions

func convertPreconfirmationToProto(preconf *PreconfirmationInfo) *Preconfirmation {
	return &Preconfirmation{
		TxHash:        preconf.TxHash,
		Timestamp:     preconf.Timestamp.Unix(),
		Validator:     preconf.Validator,
		PriorityLevel: preconf.PriorityLevel,
		Signature:     preconf.Signature,
		ExpiresAt:     preconf.ExpiresAt.Unix(),
	}
}

// Request and Response types (these would normally be generated from proto)
// For now, we define them here

// SubmitPriorityTxRequest is the request for submitting a priority transaction
type SubmitPriorityTxRequest struct {
	TxBytes          []byte
	PriorityLevel    uint32
	WaitForInclusion bool
}

// SubmitPriorityTxResponse is the response for a priority transaction submission
type SubmitPriorityTxResponse struct {
	TxHash          string
	Accepted        bool
	Reason          string
	Preconfirmation *Preconfirmation
	MempoolPosition uint32
}

// Preconfirmation represents an early confirmation
type Preconfirmation struct {
	TxHash        string
	Timestamp     int64
	Validator     string
	PriorityLevel uint32
	Signature     []byte
	ExpiresAt     int64
}

// GetPriorityTxStatusRequest is the request for querying tx status
type GetPriorityTxStatusRequest struct {
	TxHash string
}

// GetPriorityTxStatusResponse is the response for tx status query
type GetPriorityTxStatusResponse struct {
	Status          string
	InMempool       bool
	BlockHeight     int64
	MempoolPosition uint32
	Preconfirmation *Preconfirmation
	Timestamp       int64
}

// GetMempoolStatsRequest is the request for mempool statistics
type GetMempoolStatsRequest struct{}

// GetMempoolStatsResponse is the response for mempool statistics
type GetMempoolStatsResponse struct {
	TotalTxs         uint32
	PriorityTxs      uint32
	NormalTxs        uint32
	PreconfirmedTxs  uint32
	AvgPriorityLevel float32
	MempoolSizeBytes uint64
}

// ListPriorityTxsRequest is the request for listing priority transactions
type ListPriorityTxsRequest struct {
	Pagination              *Pagination
	IncludePreconfirmations bool
}

// ListPriorityTxsResponse is the response for listing priority transactions
type ListPriorityTxsResponse struct {
	Txs        []*PriorityTxInfo
	Pagination *PaginationResponse
}

// PriorityTxInfo contains information about a priority transaction
type PriorityTxInfo struct {
	TxHash          string
	PriorityLevel   uint32
	Timestamp       int64
	SizeBytes       uint32
	Preconfirmation *Preconfirmation
	Position        uint32
}

// Pagination for list requests
type Pagination struct {
	Limit  uint64
	Offset uint64
}

// PaginationResponse for list responses
type PaginationResponse struct {
	Total      uint64
	NextOffset uint64
}

// PriorityTxRESTHandler is the REST handler for priority transactions
type PriorityTxRESTHandler struct {
	grpcServer *PriorityTxGRPCServer
}

// NewPriorityTxRESTHandler creates a new REST handler
func NewPriorityTxRESTHandler(grpcServer *PriorityTxGRPCServer) *PriorityTxRESTHandler {
	return &PriorityTxRESTHandler{
		grpcServer: grpcServer,
	}
}

// HandleSubmitPriorityTx handles HTTP POST requests for submitting priority txs
func (h *PriorityTxRESTHandler) HandleSubmitPriorityTx(ctx sdk.Context, txBytes []byte, priorityLevel uint32) (*SubmitPriorityTxResponse, error) {
	req := &SubmitPriorityTxRequest{
		TxBytes:       txBytes,
		PriorityLevel: priorityLevel,
	}

	return h.grpcServer.SubmitPriorityTx(ctx, req)
}

// HandleGetPriorityTxStatus handles HTTP GET requests for tx status
func (h *PriorityTxRESTHandler) HandleGetPriorityTxStatus(ctx sdk.Context, txHash string) (*GetPriorityTxStatusResponse, error) {
	req := &GetPriorityTxStatusRequest{
		TxHash: txHash,
	}

	return h.grpcServer.GetPriorityTxStatus(ctx, req)
}

// HandleGetMempoolStats handles HTTP GET requests for mempool stats
func (h *PriorityTxRESTHandler) HandleGetMempoolStats(ctx sdk.Context) (*GetMempoolStatsResponse, error) {
	req := &GetMempoolStatsRequest{}

	return h.grpcServer.GetMempoolStats(ctx, req)
}

// HandleListPriorityTxs handles HTTP GET requests for listing priority txs
func (h *PriorityTxRESTHandler) HandleListPriorityTxs(ctx sdk.Context, limit uint32, includePreconf bool) (*ListPriorityTxsResponse, error) {
	req := &ListPriorityTxsRequest{
		Pagination: &Pagination{
			Limit: uint64(limit),
		},
		IncludePreconfirmations: includePreconf,
	}

	return h.grpcServer.ListPriorityTxs(ctx, req)
}

// PriorityTxServiceServer defines the gRPC service interface for priority transactions
type PriorityTxServiceServer interface {
	SubmitPriorityTx(context.Context, *SubmitPriorityTxRequest) (*SubmitPriorityTxResponse, error)
	GetPriorityTxStatus(context.Context, *GetPriorityTxStatusRequest) (*GetPriorityTxStatusResponse, error)
	GetMempoolStats(context.Context, *GetMempoolStatsRequest) (*GetMempoolStatsResponse, error)
	ListPriorityTxs(context.Context, *ListPriorityTxsRequest) (*ListPriorityTxsResponse, error)
}

// RegisterPriorityTxServiceServer registers the priority tx service with a gRPC server
func RegisterPriorityTxServiceServer(s interface {
	RegisterService(*grpc.ServiceDesc, interface{})
}, srv PriorityTxServiceServer,
) {
	s.RegisterService(&PriorityTxServiceDesc, srv)
}

// PriorityTxServiceDesc is the gRPC service descriptor for priority transactions
var PriorityTxServiceDesc = grpc.ServiceDesc{
	ServiceName: "preconfer.PriorityTxService",
	HandlerType: (*PriorityTxServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SubmitPriorityTx",
			Handler:    _PriorityTxService_SubmitPriorityTx_Handler,
		},
		{
			MethodName: "GetPriorityTxStatus",
			Handler:    _PriorityTxService_GetPriorityTxStatus_Handler,
		},
		{
			MethodName: "GetMempoolStats",
			Handler:    _PriorityTxService_GetMempoolStats_Handler,
		},
		{
			MethodName: "ListPriorityTxs",
			Handler:    _PriorityTxService_ListPriorityTxs_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "preconfer/priority_tx.proto",
}

func _PriorityTxService_SubmitPriorityTx_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SubmitPriorityTxRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PriorityTxServiceServer).SubmitPriorityTx(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.PriorityTxService/SubmitPriorityTx",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PriorityTxServiceServer).SubmitPriorityTx(ctx, req.(*SubmitPriorityTxRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PriorityTxService_GetPriorityTxStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPriorityTxStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PriorityTxServiceServer).GetPriorityTxStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.PriorityTxService/GetPriorityTxStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PriorityTxServiceServer).GetPriorityTxStatus(ctx, req.(*GetPriorityTxStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PriorityTxService_GetMempoolStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetMempoolStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PriorityTxServiceServer).GetMempoolStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.PriorityTxService/GetMempoolStats",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PriorityTxServiceServer).GetMempoolStats(ctx, req.(*GetMempoolStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PriorityTxService_ListPriorityTxs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListPriorityTxsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PriorityTxServiceServer).ListPriorityTxs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/preconfer.PriorityTxService/ListPriorityTxs",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PriorityTxServiceServer).ListPriorityTxs(ctx, req.(*ListPriorityTxsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

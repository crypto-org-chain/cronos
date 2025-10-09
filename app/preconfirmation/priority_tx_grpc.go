package preconfirmation

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Ensure PriorityTxGRPCServer implements the gRPC service interface
// var _ types.PriorityTxServiceServer = &PriorityTxGRPCServer{}

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

	if req.PriorityLevel < 1 || req.PriorityLevel > 10 {
		return nil, status.Error(codes.InvalidArgument, "priority level must be between 1 and 10")
	}

	// Submit transaction
	result, err := s.service.SubmitPriorityTx(ctx, req.TxBytes, req.PriorityLevel)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to submit transaction: %v", err))
	}

	// Build response
	resp := &SubmitPriorityTxResponse{
		TxHash:                 result.TxHash,
		Accepted:               result.Accepted,
		Reason:                 result.Reason,
		MempoolPosition:        result.MempoolPosition,
		EstimatedInclusionTime: result.EstimatedInclusionTime,
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
		Status:          convertTxStatusToProto(info.Status),
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

// GetPriorityMempoolStats returns mempool statistics
func (s *PriorityTxGRPCServer) GetPriorityMempoolStats(
	ctx context.Context,
	req *GetPriorityMempoolStatsRequest,
) (*GetPriorityMempoolStatsResponse, error) {
	stats := s.service.GetMempoolStats()

	return &GetPriorityMempoolStatsResponse{
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

func convertTxStatusToProto(status TxStatusType) TxStatus {
	switch status {
	case TxStatusPending:
		return TxStatus_TX_STATUS_PENDING
	case TxStatusPreconfirmed:
		return TxStatus_TX_STATUS_PRECONFIRMED
	case TxStatusIncluded:
		return TxStatus_TX_STATUS_INCLUDED
	case TxStatusRejected:
		return TxStatus_TX_STATUS_REJECTED
	case TxStatusExpired:
		return TxStatus_TX_STATUS_EXPIRED
	default:
		return TxStatus_TX_STATUS_UNKNOWN
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
	TxHash                 string
	Accepted               bool
	Reason                 string
	Preconfirmation        *Preconfirmation
	MempoolPosition        uint32
	EstimatedInclusionTime uint32
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
	Status          TxStatus
	InMempool       bool
	BlockHeight     int64
	MempoolPosition uint32
	Preconfirmation *Preconfirmation
	Timestamp       int64
}

// TxStatus represents the status of a transaction
type TxStatus int32

const (
	TxStatus_TX_STATUS_UNKNOWN      TxStatus = 0
	TxStatus_TX_STATUS_PENDING      TxStatus = 1
	TxStatus_TX_STATUS_PRECONFIRMED TxStatus = 2
	TxStatus_TX_STATUS_INCLUDED     TxStatus = 3
	TxStatus_TX_STATUS_REJECTED     TxStatus = 4
	TxStatus_TX_STATUS_EXPIRED      TxStatus = 5
)

// GetPriorityMempoolStatsRequest is the request for mempool statistics
type GetPriorityMempoolStatsRequest struct{}

// GetPriorityMempoolStatsResponse is the response for mempool statistics
type GetPriorityMempoolStatsResponse struct {
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

// HTTP REST handler
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

	return h.grpcServer.SubmitPriorityTx(sdk.WrapSDKContext(ctx), req)
}

// HandleGetPriorityTxStatus handles HTTP GET requests for tx status
func (h *PriorityTxRESTHandler) HandleGetPriorityTxStatus(ctx sdk.Context, txHash string) (*GetPriorityTxStatusResponse, error) {
	req := &GetPriorityTxStatusRequest{
		TxHash: txHash,
	}

	return h.grpcServer.GetPriorityTxStatus(sdk.WrapSDKContext(ctx), req)
}

// HandleGetMempoolStats handles HTTP GET requests for mempool stats
func (h *PriorityTxRESTHandler) HandleGetMempoolStats(ctx sdk.Context) (*GetPriorityMempoolStatsResponse, error) {
	req := &GetPriorityMempoolStatsRequest{}

	return h.grpcServer.GetPriorityMempoolStats(sdk.WrapSDKContext(ctx), req)
}

// HandleListPriorityTxs handles HTTP GET requests for listing priority txs
func (h *PriorityTxRESTHandler) HandleListPriorityTxs(ctx sdk.Context, limit uint32, includePreconf bool) (*ListPriorityTxsResponse, error) {
	req := &ListPriorityTxsRequest{
		Pagination: &Pagination{
			Limit: uint64(limit),
		},
		IncludePreconfirmations: includePreconf,
	}

	return h.grpcServer.ListPriorityTxs(sdk.WrapSDKContext(ctx), req)
}

package executionbook

import (
	"context"
	"fmt"
)

// SequencerGRPCServer implements gRPC API for sequencer transaction submission
type SequencerGRPCServer struct {
	book *ExecutionBook
}

// NewSequencerGRPCServer creates a new sequencer gRPC server
func NewSequencerGRPCServer(book *ExecutionBook) *SequencerGRPCServer {
	if book == nil {
		panic("execution book cannot be nil")
	}
	return &SequencerGRPCServer{
		book: book,
	}
}

// SubmitSequencerTxRequest is the request for submitting a sequencer transaction
type SubmitSequencerTxRequest struct {
	TxHash         []byte // 32-byte transaction hash
	SequenceNumber uint64
	Signature      []byte
	SequencerID    string
}

// SubmitSequencerTxResponse is the response for submitting a sequencer transaction
type SubmitSequencerTxResponse struct {
	Success bool
	Message string
}

// GetStatsRequest is the request for getting execution book statistics
type GetStatsRequest struct{}

// GetStatsResponse is the response for getting execution book statistics
type GetStatsResponse struct {
	TotalTransactions    int32
	PendingTransactions  int32
	IncludedTransactions int32
	NextSequence         uint64
	CurrentBlockHeight   uint64
	SequencerCount       int32
}

// SubmitSequencerTx handles the submission of a sequencer transaction
func (s *SequencerGRPCServer) SubmitSequencerTx(ctx context.Context, req *SubmitSequencerTxRequest) (*SubmitSequencerTxResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Validate request fields - validation is done in ExecutionBook.SubmitSequencerTx
	if req.SequencerID == "" {
		return nil, fmt.Errorf("sequencer_id cannot be empty")
	}

	// Submit to execution book (it will validate txHash and signature)
	err := s.book.SubmitSequencerTx(req.TxHash, req.SequenceNumber, req.Signature, req.SequencerID)
	if err != nil {
		return &SubmitSequencerTxResponse{
			Success: false,
			Message: fmt.Sprintf("failed to submit transaction: %v", err),
		}, nil
	}

	return &SubmitSequencerTxResponse{
		Success: true,
		Message: "transaction submitted successfully",
	}, nil
}

// GetStats returns statistics about the execution book
func (s *SequencerGRPCServer) GetStats(ctx context.Context, req *GetStatsRequest) (*GetStatsResponse, error) {
	stats := s.book.GetStats()

	return &GetStatsResponse{
		TotalTransactions:    int32(stats.TotalTransactions),
		PendingTransactions:  int32(stats.PendingTransactions),
		IncludedTransactions: int32(stats.IncludedTransactions),
		NextSequence:         stats.NextSequence,
		CurrentBlockHeight:   stats.CurrentBlockHeight,
		SequencerCount:       int32(stats.SequencerCount),
	}, nil
}

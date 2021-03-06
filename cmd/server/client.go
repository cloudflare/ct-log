// DO NOT MODIFY. THIS FILE IS AUTOMATICALLY GENERATED BY "go generate".

package main

import (
	"fmt"

	"github.com/google/trillian"
	"github.com/google/trillian/server"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// NOTE(brendan): The `opts` argument to each function call seems to be a
// technical detail of how GRPC works and carries no information.

type trillianLogClient struct {
	interceptor grpc.UnaryServerInterceptor
	server      *server.TrillianLogRPCServer
}

func (tlc trillianLogClient) QueueLeaf(ctx context.Context,
	req *trillian.QueueLeafRequest, opts ...grpc.CallOption) (
	*trillian.QueueLeafResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.QueueLeafRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.QueueLeafRequest, got %T", temp)
		}
		return tlc.server.QueueLeaf(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/QueueLeaf"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.QueueLeafResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.QueueLeafResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) AddSequencedLeaf(ctx context.Context,
	req *trillian.AddSequencedLeafRequest, opts ...grpc.CallOption) (
	*trillian.AddSequencedLeafResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.AddSequencedLeafRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.AddSequencedLeafRequest, got %T", temp)
		}
		return tlc.server.AddSequencedLeaf(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/AddSequencedLeaf"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.AddSequencedLeafResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.AddSequencedLeafResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetInclusionProof(ctx context.Context,
	req *trillian.GetInclusionProofRequest, opts ...grpc.CallOption) (
	*trillian.GetInclusionProofResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetInclusionProofRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetInclusionProofRequest, got %T", temp)
		}
		return tlc.server.GetInclusionProof(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetInclusionProof"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetInclusionProofResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetInclusionProofResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetInclusionProofByHash(ctx context.Context,
	req *trillian.GetInclusionProofByHashRequest, opts ...grpc.CallOption) (
	*trillian.GetInclusionProofByHashResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetInclusionProofByHashRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetInclusionProofByHashRequest, got %T", temp)
		}
		return tlc.server.GetInclusionProofByHash(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetInclusionProofByHash"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetInclusionProofByHashResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetInclusionProofByHashResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetConsistencyProof(ctx context.Context,
	req *trillian.GetConsistencyProofRequest, opts ...grpc.CallOption) (
	*trillian.GetConsistencyProofResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetConsistencyProofRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetConsistencyProofRequest, got %T", temp)
		}
		return tlc.server.GetConsistencyProof(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetConsistencyProof"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetConsistencyProofResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetConsistencyProofResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetLatestSignedLogRoot(ctx context.Context,
	req *trillian.GetLatestSignedLogRootRequest, opts ...grpc.CallOption) (
	*trillian.GetLatestSignedLogRootResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetLatestSignedLogRootRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetLatestSignedLogRootRequest, got %T", temp)
		}
		return tlc.server.GetLatestSignedLogRoot(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetLatestSignedLogRoot"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetLatestSignedLogRootResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetLatestSignedLogRootResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetSequencedLeafCount(ctx context.Context,
	req *trillian.GetSequencedLeafCountRequest, opts ...grpc.CallOption) (
	*trillian.GetSequencedLeafCountResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetSequencedLeafCountRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetSequencedLeafCountRequest, got %T", temp)
		}
		return tlc.server.GetSequencedLeafCount(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetSequencedLeafCount"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetSequencedLeafCountResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetSequencedLeafCountResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetEntryAndProof(ctx context.Context,
	req *trillian.GetEntryAndProofRequest, opts ...grpc.CallOption) (
	*trillian.GetEntryAndProofResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetEntryAndProofRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetEntryAndProofRequest, got %T", temp)
		}
		return tlc.server.GetEntryAndProof(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetEntryAndProof"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetEntryAndProofResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetEntryAndProofResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) InitLog(ctx context.Context,
	req *trillian.InitLogRequest, opts ...grpc.CallOption) (
	*trillian.InitLogResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.InitLogRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.InitLogRequest, got %T", temp)
		}
		return tlc.server.InitLog(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/InitLog"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.InitLogResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.InitLogResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) QueueLeaves(ctx context.Context,
	req *trillian.QueueLeavesRequest, opts ...grpc.CallOption) (
	*trillian.QueueLeavesResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.QueueLeavesRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.QueueLeavesRequest, got %T", temp)
		}
		return tlc.server.QueueLeaves(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/QueueLeaves"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.QueueLeavesResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.QueueLeavesResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) AddSequencedLeaves(ctx context.Context,
	req *trillian.AddSequencedLeavesRequest, opts ...grpc.CallOption) (
	*trillian.AddSequencedLeavesResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.AddSequencedLeavesRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.AddSequencedLeavesRequest, got %T", temp)
		}
		return tlc.server.AddSequencedLeaves(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/AddSequencedLeaves"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.AddSequencedLeavesResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.AddSequencedLeavesResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetLeavesByIndex(ctx context.Context,
	req *trillian.GetLeavesByIndexRequest, opts ...grpc.CallOption) (
	*trillian.GetLeavesByIndexResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetLeavesByIndexRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetLeavesByIndexRequest, got %T", temp)
		}
		return tlc.server.GetLeavesByIndex(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetLeavesByIndex"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetLeavesByIndexResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetLeavesByIndexResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetLeavesByRange(ctx context.Context,
	req *trillian.GetLeavesByRangeRequest, opts ...grpc.CallOption) (
	*trillian.GetLeavesByRangeResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetLeavesByRangeRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetLeavesByRangeRequest, got %T", temp)
		}
		return tlc.server.GetLeavesByRange(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetLeavesByRange"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetLeavesByRangeResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetLeavesByRangeResponse, got %T", temp)
	}

	return resp, nil
}

func (tlc trillianLogClient) GetLeavesByHash(ctx context.Context,
	req *trillian.GetLeavesByHashRequest, opts ...grpc.CallOption) (
	*trillian.GetLeavesByHashResponse, error) {

	handler := func(ctx context.Context, temp interface{}) (interface{}, error) {
		req, ok := temp.(*trillian.GetLeavesByHashRequest)
		if !ok {
			return nil, fmt.Errorf("expected request of type *trillian.GetLeavesByHashRequest, got %T", temp)
		}
		return tlc.server.GetLeavesByHash(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/trillian.TrillianLog/GetLeavesByHash"}
	temp, err := tlc.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}
	resp, ok := temp.(*trillian.GetLeavesByHashResponse)
	if !ok {
		return nil, fmt.Errorf("expected response of type *trillian.GetLeavesByHashResponse, got %T", temp)
	}

	return resp, nil
}

// Code generated by MockGen. DO NOT EDIT.
// Source: code.vegaprotocol.io/vega/core/blockchain/nullchain (interfaces: TimeService,ApplicationService)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"
	time "time"

	types "github.com/cometbft/cometbft/abci/types"
	gomock "github.com/golang/mock/gomock"
)

// MockTimeService is a mock of TimeService interface.
type MockTimeService struct {
	ctrl     *gomock.Controller
	recorder *MockTimeServiceMockRecorder
}

// MockTimeServiceMockRecorder is the mock recorder for MockTimeService.
type MockTimeServiceMockRecorder struct {
	mock *MockTimeService
}

// NewMockTimeService creates a new mock instance.
func NewMockTimeService(ctrl *gomock.Controller) *MockTimeService {
	mock := &MockTimeService{ctrl: ctrl}
	mock.recorder = &MockTimeServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTimeService) EXPECT() *MockTimeServiceMockRecorder {
	return m.recorder
}

// GetTimeNow mocks base method.
func (m *MockTimeService) GetTimeNow() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTimeNow")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

// GetTimeNow indicates an expected call of GetTimeNow.
func (mr *MockTimeServiceMockRecorder) GetTimeNow() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTimeNow", reflect.TypeOf((*MockTimeService)(nil).GetTimeNow))
}

// MockApplicationService is a mock of ApplicationService interface.
type MockApplicationService struct {
	ctrl     *gomock.Controller
	recorder *MockApplicationServiceMockRecorder
}

// MockApplicationServiceMockRecorder is the mock recorder for MockApplicationService.
type MockApplicationServiceMockRecorder struct {
	mock *MockApplicationService
}

// NewMockApplicationService creates a new mock instance.
func NewMockApplicationService(ctrl *gomock.Controller) *MockApplicationService {
	mock := &MockApplicationService{ctrl: ctrl}
	mock.recorder = &MockApplicationServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockApplicationService) EXPECT() *MockApplicationServiceMockRecorder {
	return m.recorder
}

// Commit mocks base method.
func (m *MockApplicationService) Commit(arg0 context.Context, arg1 *types.RequestCommit) (*types.ResponseCommit, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit", arg0, arg1)
	ret0, _ := ret[0].(*types.ResponseCommit)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Commit indicates an expected call of Commit.
func (mr *MockApplicationServiceMockRecorder) Commit(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockApplicationService)(nil).Commit), arg0, arg1)
}

// FinalizeBlock mocks base method.
func (m *MockApplicationService) FinalizeBlock(arg0 context.Context, arg1 *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FinalizeBlock", arg0, arg1)
	ret0, _ := ret[0].(*types.ResponseFinalizeBlock)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FinalizeBlock indicates an expected call of FinalizeBlock.
func (mr *MockApplicationServiceMockRecorder) FinalizeBlock(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FinalizeBlock", reflect.TypeOf((*MockApplicationService)(nil).FinalizeBlock), arg0, arg1)
}

// Info mocks base method.
func (m *MockApplicationService) Info(arg0 context.Context, arg1 *types.RequestInfo) (*types.ResponseInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Info", arg0, arg1)
	ret0, _ := ret[0].(*types.ResponseInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Info indicates an expected call of Info.
func (mr *MockApplicationServiceMockRecorder) Info(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockApplicationService)(nil).Info), arg0, arg1)
}

// InitChain mocks base method.
func (m *MockApplicationService) InitChain(arg0 context.Context, arg1 *types.RequestInitChain) (*types.ResponseInitChain, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitChain", arg0, arg1)
	ret0, _ := ret[0].(*types.ResponseInitChain)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InitChain indicates an expected call of InitChain.
func (mr *MockApplicationServiceMockRecorder) InitChain(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InitChain", reflect.TypeOf((*MockApplicationService)(nil).InitChain), arg0, arg1)
}

// PrepareProposal mocks base method.
func (m *MockApplicationService) PrepareProposal(arg0 context.Context, arg1 *types.RequestPrepareProposal) (*types.ResponsePrepareProposal, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PrepareProposal", arg0, arg1)
	ret0, _ := ret[0].(*types.ResponsePrepareProposal)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PrepareProposal indicates an expected call of PrepareProposal.
func (mr *MockApplicationServiceMockRecorder) PrepareProposal(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PrepareProposal", reflect.TypeOf((*MockApplicationService)(nil).PrepareProposal), arg0, arg1)
}

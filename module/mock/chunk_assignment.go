// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/dapperlabs/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// ChunkAssignment is an autogenerated mock type for the ChunkAssignment type
type ChunkAssignment struct {
	mock.Mock
}

// MyChunks provides a mock function with given fields: myID, verifiers, result
func (_m *ChunkAssignment) MyChunks(myID flow.Identifier, verifiers flow.IdentityList, result *flow.ExecutionResult) (flow.ChunkList, error) {
	ret := _m.Called(myID, verifiers, result)

	var r0 flow.ChunkList
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.IdentityList, *flow.ExecutionResult) flow.ChunkList); ok {
		r0 = rf(myID, verifiers, result)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.ChunkList)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier, flow.IdentityList, *flow.ExecutionResult) error); ok {
		r1 = rf(myID, verifiers, result)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

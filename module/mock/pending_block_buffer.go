// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import flow "github.com/dapperlabs/flow-go/model/flow"
import mock "github.com/stretchr/testify/mock"

// PendingBlockBuffer is an autogenerated mock type for the PendingBlockBuffer type
type PendingBlockBuffer struct {
	mock.Mock
}

// Add provides a mock function with given fields: block
func (_m *PendingBlockBuffer) Add(block *flow.PendingBlock) bool {
	ret := _m.Called(block)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*flow.PendingBlock) bool); ok {
		r0 = rf(block)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// ByParentID provides a mock function with given fields: parentID
func (_m *PendingBlockBuffer) ByParentID(parentID flow.Identifier) ([]*flow.PendingBlock, bool) {
	ret := _m.Called(parentID)

	var r0 []*flow.PendingBlock
	if rf, ok := ret.Get(0).(func(flow.Identifier) []*flow.PendingBlock); ok {
		r0 = rf(parentID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*flow.PendingBlock)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(parentID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// DropForParent provides a mock function with given fields: parentID
func (_m *PendingBlockBuffer) DropForParent(parentID flow.Identifier) {
	_m.Called(parentID)
}

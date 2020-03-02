// Code generated by MockGen. DO NOT EDIT.
// Source: entry_getter.go

// Package mock_dcron is a generated GoMock package.
package mock_dcron

import (
	gomock "github.com/golang/mock/gomock"
	cron "github.com/robfig/cron/v3"
	reflect "reflect"
)

// MockentryGetter is a mock of entryGetter interface
type MockentryGetter struct {
	ctrl     *gomock.Controller
	recorder *MockentryGetterMockRecorder
}

// MockentryGetterMockRecorder is the mock recorder for MockentryGetter
type MockentryGetterMockRecorder struct {
	mock *MockentryGetter
}

// NewMockentryGetter creates a new mock instance
func NewMockentryGetter(ctrl *gomock.Controller) *MockentryGetter {
	mock := &MockentryGetter{ctrl: ctrl}
	mock.recorder = &MockentryGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockentryGetter) EXPECT() *MockentryGetterMockRecorder {
	return m.recorder
}

// Entry mocks base method
func (m *MockentryGetter) Entry(id cron.EntryID) cron.Entry {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Entry", id)
	ret0, _ := ret[0].(cron.Entry)
	return ret0
}

// Entry indicates an expected call of Entry
func (mr *MockentryGetterMockRecorder) Entry(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Entry", reflect.TypeOf((*MockentryGetter)(nil).Entry), id)
}

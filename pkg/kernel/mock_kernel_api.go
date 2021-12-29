// Code generated by MockGen. DO NOT EDIT.
// Source: kernel.go

// Package kernel is a generated GoMock package.
package kernel

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockKernelData is a mock of KernelData interface.
type MockKernelData struct {
	ctrl     *gomock.Controller
	recorder *MockKernelDataMockRecorder
}

// MockKernelDataMockRecorder is the mock recorder for MockKernelData.
type MockKernelDataMockRecorder struct {
	mock *MockKernelData
}

// NewMockKernelData creates a new mock instance.
func NewMockKernelData(ctrl *gomock.Controller) *MockKernelData {
	mock := &MockKernelData{ctrl: ctrl}
	mock.recorder = &MockKernelDataMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockKernelData) EXPECT() *MockKernelDataMockRecorder {
	return m.recorder
}

// FullVersion mocks base method.
func (m *MockKernelData) FullVersion() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FullVersion")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FullVersion indicates an expected call of FullVersion.
func (mr *MockKernelDataMockRecorder) FullVersion() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FullVersion", reflect.TypeOf((*MockKernelData)(nil).FullVersion))
}

// IsObjectAffine mocks base method.
func (m *MockKernelData) IsObjectAffine(obj client.Object) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsObjectAffine", obj)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsObjectAffine indicates an expected call of IsObjectAffine.
func (mr *MockKernelDataMockRecorder) IsObjectAffine(obj interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsObjectAffine", reflect.TypeOf((*MockKernelData)(nil).IsObjectAffine), obj)
}

// PatchVersion mocks base method.
func (m *MockKernelData) PatchVersion(kernelFullVersion string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PatchVersion", kernelFullVersion)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PatchVersion indicates an expected call of PatchVersion.
func (mr *MockKernelDataMockRecorder) PatchVersion(kernelFullVersion interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PatchVersion", reflect.TypeOf((*MockKernelData)(nil).PatchVersion), kernelFullVersion)
}

// SetAffineAttributes mocks base method.
func (m *MockKernelData) SetAffineAttributes(obj *unstructured.Unstructured, kernelFullVersion, operatingSystemMajorMinor string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetAffineAttributes", obj, kernelFullVersion, operatingSystemMajorMinor)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetAffineAttributes indicates an expected call of SetAffineAttributes.
func (mr *MockKernelDataMockRecorder) SetAffineAttributes(obj, kernelFullVersion, operatingSystemMajorMinor interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetAffineAttributes", reflect.TypeOf((*MockKernelData)(nil).SetAffineAttributes), obj, kernelFullVersion, operatingSystemMajorMinor)
}

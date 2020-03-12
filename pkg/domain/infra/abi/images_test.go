package abi

//
// import (
// 	"context"
// 	"testing"
//
// 	"github.com/stretchr/testify/mock"
// )
//
// type MockImageRuntime struct {
// 	mock.Mock
// }
//
// func (m *MockImageRuntime) Delete(ctx context.Context, renderer func() interface{}, name string) error {
// 	_ = m.Called(ctx, renderer, name)
// 	return nil
// }
//
// func TestImageSuccess(t *testing.T) {
// 	actual := func() interface{} { return nil }
//
// 	m := new(MockImageRuntime)
// 	m.On(
// 		"Delete",
// 		mock.AnythingOfType("*context.emptyCtx"),
// 		mock.AnythingOfType("func() interface {}"),
// 		"fedora").
// 		Return(nil)
//
// 	r := DirectImageRuntime{m}
// 	err := r.Delete(context.TODO(), actual, "fedora")
// 	if err != nil {
// 		t.Errorf("error should be nil, got: %v", err)
// 	}
// 	m.AssertExpectations(t)
// }

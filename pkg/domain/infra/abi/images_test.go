package abi

import (
	"testing"

	"github.com/containers/common/libimage"
	"github.com/stretchr/testify/assert"
)

// This is really intended to verify what happens with a
// nil pointer in layer.Created, but we'll just sanity
// check round tripping 42.
func TestToDomainHistoryLayer(t *testing.T) {
	var layer libimage.ImageHistory
	layer.Size = 42
	newLayer := toDomainHistoryLayer(&layer)
	assert.Equal(t, layer.Size, newLayer.Size)
}

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
// 		t.Errorf("should be nil, got: %v", err)
// 	}
// 	m.AssertExpectations(t)
// }

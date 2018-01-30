package helper

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestMaxLengthStringSliceAdaptor(t *testing.T) {
	assert := assert.New(t)
	raw := []string{"foo", "bar"}
	mlss := NewMaxLengthStringSliceAdaptor(raw, 3)
	news := mlss.GetAll()
	assert.True(reflect.DeepEqual(raw, news))
	assert.Equal(2, mlss.Len())
	assert.Equal(3, mlss.MaxLen())

	raw[1] = "foobar"
	news = mlss.GetAll()
	assert.True(reflect.DeepEqual(raw, news), "Raw=%v\tMLSS=%v", raw, mlss)

	mlss.Put("2")
	assert.Equal(3, mlss.Len())
	assert.True(reflect.DeepEqual([]string{"foo", "foobar", "2"}, mlss.GetAll()))
	assert.False(reflect.DeepEqual(mlss, raw))

	mlss.Put("3")
	assert.Equal(3, mlss.Len())
	assert.True(reflect.DeepEqual([]string{"foobar", "2", "3"}, mlss.GetAll()))
	assert.False(reflect.DeepEqual(mlss, raw))
}

func TestMaxLengthSlice(t *testing.T) {
	assert := assert.New(t)
	raw := []string{"foo", "bar"}
	mlss := NewMaxLengthSlice(raw, 3)
	news := mlss.GetAll()
	assert.True(reflect.DeepEqual(raw, news))
	assert.Equal(2, mlss.Len())
	assert.Equal(3, mlss.MaxLen())

	raw[1] = "foobar"
	news = mlss.GetAll()
	assert.True(reflect.DeepEqual([]string{"foo", "bar"}, news))

	mlss.Put("2")
	assert.Equal(3, mlss.Len())
	assert.True(reflect.DeepEqual([]string{"foo", "bar", "2"}, mlss.GetAll()), "MLSS=%v", mlss.GetAll())
	assert.True(reflect.DeepEqual([]string{"foo", "foobar"}, raw))

	mlss.Put("3")
	assert.Equal(3, mlss.Len())
	assert.True(reflect.DeepEqual([]string{"bar", "2", "3"}, mlss.GetAll()))
	assert.True(reflect.DeepEqual([]string{"foo", "foobar"}, raw))
}

func TestDiskUsage(t *testing.T) {
	assert := assert.New(t)
	size, err := DiskUsage(".")
	assert.True(err == nil)
	assert.True(size > 0)
}

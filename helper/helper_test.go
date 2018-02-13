package helper

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestMaxLengthStringSliceAdaptor(t *testing.T) {
	asrt := assert.New(t)
	raw := []string{"foo", "bar"}
	mlss := NewMaxLengthStringSliceAdaptor(raw, 3)
	news := mlss.GetAll()
	asrt.True(reflect.DeepEqual(raw, news))
	asrt.Equal(2, mlss.Len())
	asrt.Equal(3, mlss.MaxLen())

	raw[1] = "foobar"
	news = mlss.GetAll()
	asrt.True(reflect.DeepEqual(raw, news), "Raw=%v\tMLSS=%v", raw, mlss)

	mlss.Put("2")
	asrt.Equal(3, mlss.Len())
	asrt.True(reflect.DeepEqual([]string{"foo", "foobar", "2"}, mlss.GetAll()))
	asrt.False(reflect.DeepEqual(mlss, raw))

	mlss.Put("3")
	asrt.Equal(3, mlss.Len())
	asrt.True(reflect.DeepEqual([]string{"foobar", "2", "3"}, mlss.GetAll()))
	asrt.False(reflect.DeepEqual(mlss, raw))
}

func TestMaxLengthSlice(t *testing.T) {
	asrt := assert.New(t)
	raw := []string{"foo", "bar"}
	mlss := NewMaxLengthSlice(raw, 3)
	news := mlss.GetAll()
	asrt.True(reflect.DeepEqual(raw, news))
	asrt.Equal(2, mlss.Len())
	asrt.Equal(3, mlss.MaxLen())

	raw[1] = "foobar"
	news = mlss.GetAll()
	asrt.True(reflect.DeepEqual([]string{"foo", "bar"}, news))

	mlss.Put("2")
	asrt.Equal(3, mlss.Len())
	asrt.True(reflect.DeepEqual([]string{"foo", "bar", "2"}, mlss.GetAll()), "MLSS=%v", mlss.GetAll())
	asrt.True(reflect.DeepEqual([]string{"foo", "foobar"}, raw))

	mlss.Put("3")
	asrt.Equal(3, mlss.Len())
	asrt.True(reflect.DeepEqual([]string{"bar", "2", "3"}, mlss.GetAll()))
	asrt.True(reflect.DeepEqual([]string{"foo", "foobar"}, raw))
}

func TestDiskUsage(t *testing.T) {
	asrt := assert.New(t)
	size, err := DiskUsage(".")
	asrt.True(err == nil)
	asrt.True(size > 0)
}

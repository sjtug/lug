package worker

import (
	"testing"

	"github.com/op/go-logging"
	"github.com/sjtug/lug/config"
	"github.com/stretchr/testify/assert"
)

func TestNewWorker(t *testing.T) {
	var c config.RepoConfig = make(map[string]string)
	c["a"] = "aaa"
	c["b"] = "bbb"

	assert := assert.New(t)

	w := NewWorker(&c, logging.MustGetLogger("worker"))

	assert.Equal(true, w.GetStatus().Result)
	assert.Equal("aaa", (*w.getConfig())["a"])
	assert.Equal("bbb", (*w.getConfig())["b"])

}

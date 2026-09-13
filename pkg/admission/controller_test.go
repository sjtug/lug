package admission

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func fixedProbe(s Signals) func() Signals {
	return func() Signals { return s }
}

func newTestController(cfg Config, s Signals) *Controller {
	c := NewController(cfg)
	c.probe = fixedProbe(s)
	return c
}

func TestAdmitWithHeadroom(t *testing.T) {
	asrt := assert.New(t)
	c := newTestController(Config{}, Signals{
		MemAvailableBytes: 8 << 30,
		Valid:             true,
	})
	v := c.TryAdmit(Request{Name: "a", MemEstimate: 1 << 30})
	asrt.True(v.Admit)
}

func TestDeferOnMemoryPressure(t *testing.T) {
	asrt := assert.New(t)
	c := newTestController(Config{}, Signals{
		MemAvailableBytes: 8 << 30,
		MemPSI:            50,
		Valid:             true,
	})
	v := c.TryAdmit(Request{Name: "a", MemEstimate: 1 << 30})
	asrt.False(v.Admit)
	asrt.Contains(v.Reason, "memory pressure")
}

func TestDeferOnInsufficientMemoryAndRelease(t *testing.T) {
	asrt := assert.New(t)
	c := newTestController(Config{}, Signals{
		MemAvailableBytes: 4 << 30,
		Valid:             true,
	})
	// reserve = 0.2*4G; first job takes 2G, second 2G won't fit
	v1 := c.TryAdmit(Request{Name: "a", MemEstimate: 2 << 30})
	asrt.True(v1.Admit)
	v2 := c.TryAdmit(Request{Name: "b", MemEstimate: 2 << 30})
	asrt.False(v2.Admit)
	asrt.Contains(v2.Reason, "memory headroom")

	c.Release("a")
	v3 := c.TryAdmit(Request{Name: "b", MemEstimate: 2 << 30})
	asrt.True(v3.Admit)
}

func TestInvalidSignalsAdmit(t *testing.T) {
	asrt := assert.New(t)
	c := newTestController(Config{}, Signals{})
	asrt.True(c.TryAdmit(Request{Name: "a"}).Admit)
}

func TestEstimate(t *testing.T) {
	asrt := assert.New(t)
	asrt.Equal(uint64(DefaultMemEstimate), Estimate(0))
	asrt.Equal(uint64(1500), Estimate(1000))
}

package zmey

import (
	// "context"
	// "log"
	// "math/rand"
	// "sort"
	"testing"
	// "time"

	"github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/require"
)

type DummyProcess struct{}

func (DummyProcess) ReceiveNet(int, interface{})                          {}
func (DummyProcess) ReceiveCall(interface{})                              {}
func (DummyProcess) Tick(uint)                                            {}
func (DummyProcess) Start()                                               {}
func (DummyProcess) CallbackSend(sendF func(to int, payload interface{})) {}
func (DummyProcess) CallbackReturn(returnF func(payload interface{}))     {}
func (DummyProcess) CallbackTrace(traceF func(payload interface{}))       {}
func (DummyProcess) CallbackError(errorF func(error))                     {}

func TestSetProcess(t *testing.T) {

	z := NewZmey(&Config{})

	assert.Equal(t, map[int]*pack{}, z.packs)
	assert.Equal(t, []int{}, z.pids)

	z.SetProcess(5, DummyProcess{})

	assert.Equal(t, 1, len(z.packs))
	assert.Equal(t, []int{5}, z.pids)

	z.SetProcess(3, DummyProcess{})
	z.SetProcess(1, DummyProcess{})
	z.SetProcess(8, DummyProcess{})
	z.SetProcess(2, DummyProcess{})
	z.SetProcess(6, DummyProcess{})

	assert.Equal(t, 6, len(z.packs))
	assert.Equal(t, []int{1, 2, 3, 5, 6, 8}, z.pids)

	z.SetProcess(3, DummyProcess{})

	assert.Equal(t, 6, len(z.packs))
	assert.Equal(t, []int{1, 2, 3, 5, 6, 8}, z.pids)

	z.SetProcess(2, nil)

	assert.Equal(t, 5, len(z.packs))
	assert.Equal(t, []int{1, 3, 5, 6, 8}, z.pids)

	z.SetProcess(1, nil)
	z.SetProcess(8, nil)
	z.SetProcess(5, nil)
	z.SetProcess(6, nil)
	z.SetProcess(3, nil)

	assert.Equal(t, 0, len(z.packs))
	assert.Equal(t, []int{}, z.pids)

	z.SetProcess(6, nil)

	assert.Equal(t, 0, len(z.packs))
	assert.Equal(t, []int{}, z.pids)

}

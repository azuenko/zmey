package zmey

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPid(t *testing.T) {
	pids := []int{4, 7, -273, 42}

	ctx := context.Background()
	var wg sync.WaitGroup
	n := NewNet(ctx, &wg, pids, NewSession())

	var err error

	err = n.Send(4, 7, struct{}{})
	assert.NoError(t, err)

	err = n.Send(4, -273, struct{}{})
	assert.NoError(t, err)

	err = n.Send(-273, 42, struct{}{})
	assert.NoError(t, err)

	_, err = n.Recv(42, 7)
	assert.NoError(t, err)

	err = n.Send(4, 20, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	err = n.Send(20, 4, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	_, err = n.Recv(100, 200)
	assert.Error(t, ErrIncorrectPid, err)

}

func TestSingle(t *testing.T) {
	const scale = 4

	ctx := context.Background()
	var wg sync.WaitGroup
	n := NewNet(ctx, &wg, []int{3, 2, 1, 0}, NewSession())

	var err error
	data := 42
	err = n.Send(1, 2, data)

	require.NoError(t, err)

	recvC, err := n.Recv(2, 1)

	require.NoError(t, err)

	select {
	case recv := <-recvC:
		assert.Equal(t, data, recv)
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestLoopback(t *testing.T) {
	const scale = 1

	ctx := context.Background()
	var wg sync.WaitGroup
	n := NewNet(ctx, &wg, []int{0}, NewSession())

	var err error
	data := 42
	err = n.Send(0, 0, data)

	require.NoError(t, err)

	recvC, err := n.Recv(0, 0)

	require.NoError(t, err)

	select {
	case recv := <-recvC:
		assert.Equal(t, data, recv)
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestQueue(t *testing.T) {
	const scale = 4

	ctx := context.Background()
	var wg sync.WaitGroup
	n := NewNet(ctx, &wg, []int{1, 2, 3, 4}, NewSession())

	var err error

	data := make([]int, 100)
	for i := 0; i < len(data); i++ {
		data[i] = i
	}

	doneC := make(chan interface{})

	go func() {
		for i := 0; i < len(data); i++ {
			err = n.Send(1, 2, data[i])
			require.NoError(t, err)
		}
		doneC <- struct{}{}
	}()

	select {
	case <-doneC:
		break
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout")
	}

	recv := make([]int, 100)

	go func() {
		for i := 0; i < len(recv); i++ {
			recvC, err := n.Recv(2, 1)
			require.NoError(t, err)
			recv[i] = (<-recvC).(int)
		}
		doneC <- struct{}{}
	}()

	select {
	case <-doneC:
		break
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout")
	}

	assert.Equal(t, data, recv)

}

func TestBufferStats(t *testing.T) {
	const scale = 4

	ctx := context.Background()
	var wg sync.WaitGroup
	n := NewNet(ctx, &wg, []int{-5, 27, 8, 0}, NewSession())

	_ = n.Send(-5, 27, struct{}{})
	_ = n.Send(-5, 27, struct{}{})
	_ = n.Send(27, 8, struct{}{})
	_ = n.Send(27, 8, struct{}{})
	_ = n.Send(27, 8, struct{}{})
	_ = n.Send(-5, 0, struct{}{})
	_ = n.Send(0, 27, struct{}{})
	_ = n.Send(0, 27, struct{}{})

	expected := `
    |  to|
----+----+----+----+----+----+
from|    |  -5|  27|   8|   0|
----+----+----+----+----+----+
    |  -5|    |    |    |    |
    |  27|   2|    |    |   2|
    |   8|    |   3|    |    |
    |   0|   1|    |    |    |
----+----+----+----+----+----+
`[1:] // remove first linebreak

	done := make(chan struct{})
	var actual string

	go func() {
		for {
			time.Sleep(10 * time.Millisecond)
			actual = n.BufferStats()
			if expected == actual {
				done <- struct{}{}
				return
			}
		}
	}()

	select {
	case <-done:
		break
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected: \n%s\n actual: \n%s\n", expected, actual)
	}
}

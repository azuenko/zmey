package zmey

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncorrectPid(t *testing.T) {
	const scale = 4

	n := NewNet(scale, NewSession(scale))

	var err error

	err = n.Send(0, 5, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	err = n.Send(5, 0, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	err = n.Send(5, 5, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	err = n.Send(-1, 0, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	err = n.Send(0, -5, struct{}{})
	assert.Error(t, ErrIncorrectPid, err)

	_, err = n.Recv(0, 5)
	assert.Error(t, ErrIncorrectPid, err)

	_, err = n.Recv(5, 0)
	assert.Error(t, ErrIncorrectPid, err)

	_, err = n.Recv(5, 5)
	assert.Error(t, ErrIncorrectPid, err)

	_, err = n.Recv(-1, 0)
	assert.Error(t, ErrIncorrectPid, err)

	_, err = n.Recv(0, -5)
	assert.Error(t, ErrIncorrectPid, err)

}

func TestSingle(t *testing.T) {
	const scale = 4

	n := NewNet(scale, NewSession(scale))

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

func TestQueue(t *testing.T) {
	const scale = 4

	n := NewNet(scale, NewSession(scale))

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

	session := NewSession(scale)
	n := NewNet(scale, session)

	n.Send(0, 1, struct{}{})
	n.Send(0, 1, struct{}{})
	n.Send(1, 2, struct{}{})
	n.Send(1, 2, struct{}{})
	n.Send(1, 2, struct{}{})
	n.Send(0, 3, struct{}{})
	n.Send(3, 1, struct{}{})
	n.Send(3, 1, struct{}{})

	expected := `
    |  to|
----+----+----+----+----+----+
from|    |   0|   1|   2|   3|
----+----+----+----+----+----+
    |   0|    |    |    |    |
    |   1|   2|    |    |   2|
    |   2|    |   3|    |    |
    |   3|   1|    |    |    |
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

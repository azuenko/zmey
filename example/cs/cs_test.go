package cs

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stratumn/zmey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	const timeout = 1000
	const tick = 2000
	const serverPid = 1
	const clientAPid = 1501
	const clientBPid = 1502

	z := zmey.NewZmey(&zmey.Config{
		Debug: true,
	})

	z.SetProcess(serverPid, NewServer(serverPid))
	z.SetProcess(clientAPid, NewClient(clientAPid, serverPid, timeout))
	z.SetProcess(clientBPid, NewClient(clientBPid, serverPid, timeout))

	requests := map[int][]Call{
		clientAPid: {
			{ID: 1, Payload: getTag(t)},
			{ID: 2, Payload: getTag(t)},
		},
		clientBPid: {
			{ID: 3, Payload: getTag(t)},
			{ID: 4, Payload: getTag(t)},
		},
	}

	injectF := func(pid int, c zmey.Client) {
		for _, rr := range requests[pid] {
			c.Call(rr)
		}
	}

	z.Inject(injectF)

	// Cut client B from server. Its calls will be returned with timeout
	filterF := func(from, to int) bool {
		if from == clientBPid && to == serverPid {
			return false
		}
		return true
	}

	z.Filter(filterF)

	var responses, traces map[int][]interface{}
	var err error
	var ctx context.Context
	var cancelF context.CancelFunc

	ctx, cancelF = context.WithTimeout(context.Background(), 3*time.Minute)
	responses, traces, err = z.Round(ctx)
	cancelF()

	require.NoError(t, err)

	assert.Equal(t, map[int][]interface{}{
		serverPid: nil,
		clientAPid: []interface{}{
			Return{
				ID:      requests[clientAPid][0].ID,
				Payload: requests[clientAPid][0].Payload,
				Timeout: false,
			},
			Return{
				ID:      requests[clientAPid][1].ID,
				Payload: requests[clientAPid][1].Payload,
				Timeout: false,
			},
		},
		clientBPid: nil,
	}, responses)

	t.Log("Round 1 traces")
	for pid := range traces {
		for tid := range traces[pid] {
			t.Logf("[%4d]: %s", pid, traces[pid][tid])
		}
	}

	z.Tick(tick)

	ctx, cancelF = context.WithTimeout(context.Background(), 3*time.Minute)

	responses, traces, err = z.Round(ctx)

	cancelF()

	require.NoError(t, err)

	assert.Equal(t, map[int][]interface{}{
		serverPid:  nil,
		clientAPid: nil,
		clientBPid: []interface{}{
			Return{
				ID:      requests[clientBPid][0].ID,
				Payload: requests[clientBPid][0].Payload,
				Timeout: true,
			},
			Return{
				ID:      requests[clientBPid][1].ID,
				Payload: requests[clientBPid][1].Payload,
				Timeout: true,
			},
		},
	}, responses)

	t.Log("Round 2 traces")
	for pid := range traces {
		for tid := range traces[pid] {
			t.Logf("[%4d]: %s", pid, traces[pid][tid])
		}
	}

}

// Give me some random bytes
func getTag(t *testing.T) []byte {
	tag := make([]byte, 2)
	_, err := rand.Read(tag)
	if err != nil {
		t.Fatal(err)
	}
	return tag
}

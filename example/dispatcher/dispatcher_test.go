package dispatcher

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stratumn/zmey"
	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	const scale = 2
	const timeout = 1000

	z := zmey.NewZmey(&zmey.Config{
		Scale:   scale,
		Factory: func(pid int) zmey.Process { return NewDispatcher(pid, timeout) },
		// Debug:   true,
	})

	filterF := func(from, to int) bool {
		if from == 0 || to == 0 {
			return false
		}
		return true
	}

	z.Filter(filterF)
	z.Filter(nil)

	clientRequests := [][]ClientRequest{
		{
			{ID: 0, Payload: getTag(t)},
		},
		{
			{ID: 1, Payload: getTag(t)},
			{ID: 2, Payload: getTag(t)},
		},
	}

	injectF := func(pid int, c zmey.Client) {
		for i := range clientRequests[pid] {
			c.Call(clientRequests[pid][i])
		}
	}

	z.Inject(injectF)

	var ctx context.Context

	ctx, _ = context.WithTimeout(context.Background(), 3*time.Minute)

	responses, traces, err := z.Round(ctx)

	require.NoError(t, err)
	// require.Equal(t, scale, len(responses), "inject output size mismatch")
	// require.Equal(t, scale, len(traces), "traces size mismatch")

	for pid := range traces {
		for tid := range traces[pid] {
			t.Logf("T [%2d]: %s", pid, traces[pid][tid])
		}
	}

	for pid := range responses {
		for tid := range responses[pid] {
			t.Logf("R [%2d]: %+v", pid, responses[pid][tid])
		}
	}

	z.Tick(2000)

	ctx, _ = context.WithTimeout(context.Background(), 3*time.Minute)

	responses, traces, err = z.Round(ctx)

	require.NoError(t, err)

	for pid := range traces {
		for tid := range traces[pid] {
			t.Logf("T [%2d]: %s", pid, traces[pid][tid])
		}
	}

	for pid := range responses {
		for tid := range responses[pid] {
			t.Logf("R [%2d]: %+v", pid, responses[pid][tid])
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

package forwarder

import (
	"context"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stratumn/zmey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefault runs the forwarder and injects the messages. Each node sends
// `perNode` messages to next `perNode` nodes. If `pid` + `to` exceeds `scale`,
// modular arithmetics apply.
func TestDefault(t *testing.T) {
	const scale = 10
	const perNode = 1000

	z := zmey.NewZmey(&zmey.Config{
		Scale:   scale,
		Factory: NewForwarder,
	})

	go func() {
		for {
			t.Log(z.Status())
			t.Log("\n" + z.NetBufferStats())
			time.Sleep(1 * time.Second)
		}
	}()

	callMap := make([][]FCall, scale)
	seq := 0
	for pid := 0; pid < scale; pid++ {
		callMap[pid] = make([]FCall, perNode)
		for k := 0; k < perNode; k++ {
			callMap[pid][k] = FCall{SequenceNumber: seq, To: (pid + k + 1) % scale, Payload: getTag(t)}
			seq++
		}
	}

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Minute)

	injectOutput, err := z.Inject(ctx, func(pid int, c zmey.Client) {
		for k := 0; k < perNode; k++ {
			c.Call(callMap[pid][k])
		}
	})

	require.NoError(t, err)

	require.Equal(t, scale, len(injectOutput), "inject output size mismatch")

	for pid, messages := range injectOutput {
		require.Equal(t, perNode, len(messages), "received messages for pid %d", pid)
		expectedFCalls := make([]FCall, perNode)

		// Constructing what to expect
		for k := 0; k < perNode; k++ {
			// Go implemenation for `%` operator is not algebraic for negative operands
			// It was decided to maintain the invariant (-a) % b === - (a % b)
			// rather than (-a) % b === (b - a) % b
			// Otherwise, the expression for p would be simpler: p := (pid-perNode+k)%scale
			p := ((pid-perNode+k)%scale + scale) % scale
			q := perNode - k - 1
			expectedFCalls[k] = callMap[p][q]
		}
		// Also sorting expected messages
		sort.Slice(expectedFCalls, func(i, j int) bool {
			return expectedFCalls[i].SequenceNumber < expectedFCalls[j].SequenceNumber
		})
		actualFCalls := make([]FCall, perNode)
		for k := 0; k < perNode; k++ {
			actualFCall, ok := injectOutput[pid][k].(FCall)
			if !ok {
				t.Fatalf("cannot coerce to FCall")
			}
			actualFCalls[k] = actualFCall
		}
		// Received messages have no particual ordering, need to sort them
		sort.Slice(actualFCalls, func(i, j int) bool {
			return actualFCalls[i].SequenceNumber < actualFCalls[j].SequenceNumber
		})

		assert.Equal(t, expectedFCalls, actualFCalls)
	}
}

// Give me two random bytes
func getTag(t *testing.T) []byte {
	tag := make([]byte, 2)
	_, err := rand.Read(tag)
	if err != nil {
		t.Fatal(err)
	}
	return tag
}

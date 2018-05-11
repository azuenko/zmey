package forwarder

import (
	"context"
	"log"
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
	const scale = 30
	const perNode = 100

	z := zmey.NewZmey(&zmey.Config{
	// Debug: true,
	})

	for i := 0; i < scale; i++ {
		z.AddProcess(NewForwarder)
	}

	go func() {
		for msg := range z.Status() {
			log.Printf(msg)
			time.Sleep(250 * time.Millisecond)
		}
	}()

	go func() {
		for msg := range z.BufferStats() {
			log.Printf("\n" + msg)
			time.Sleep(1 * time.Second)
		}
	}()

	testWithZmeyInstance(t, z, scale, perNode)
}

func TestMultirun(t *testing.T) {
	const scale = 10
	const perNode = 10

	z := zmey.NewZmey(&zmey.Config{
	// Debug: true,
	})

	for i := 0; i < scale; i++ {
		z.AddProcess(NewForwarder)
	}
	testWithZmeyInstance(t, z, scale, perNode)
	testWithZmeyInstance(t, z, scale, perNode)
	testWithZmeyInstance(t, z, scale, perNode)
}

func testWithZmeyInstance(t *testing.T, z *zmey.Zmey, scale, perNode int) {

	callMap := make([][]FCall, scale)
	seq := 0
	for pid := 0; pid < scale; pid++ {
		callMap[pid] = make([]FCall, perNode)
		for k := 0; k < perNode; k++ {
			callMap[pid][k] = FCall{SequenceNumber: seq, To: (pid + k + 1) % scale, Payload: getTag(t)}
			seq++
		}
	}

	injectF := func(pid int, c zmey.Client) {
		for k := 0; k < perNode; k++ {
			c.Call(callMap[pid][k])
		}
	}

	z.Inject(injectF)

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Minute)

	responses, traces, err := z.Round(ctx)

	require.NoError(t, err)

	require.Equal(t, scale, len(responses), "responses size mismatch")
	require.Equal(t, scale, len(traces), "traces size mismatch")

	for pid := range responses {
		require.Equal(t, perNode, len(responses[pid]), "received responses for pid %d", pid)
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
			actualFCall, ok := responses[pid][k].(FCall)
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

	for pid := range traces {
		expectedLenTraces := 2*perNode + perNode - perNode/scale
		assert.Equal(t, expectedLenTraces, len(traces[pid]), "received traced for pid %d", pid)
	}

}

func TestCrash(t *testing.T) {

	const scale = 5

	calls := make([]FCall, scale)

	calls[0] = FCall{SequenceNumber: 0, To: 3, Payload: getTag(t)}
	calls[1] = FCall{SequenceNumber: 1, To: 2, Payload: getTag(t)}
	calls[2] = FCall{SequenceNumber: 2, To: 0, Payload: getTag(t)}
	calls[3] = FCall{SequenceNumber: 3, To: 1, Payload: getTag(t)}
	calls[4] = FCall{SequenceNumber: 4, To: 3, Payload: getTag(t)}

	filterF := func(from, to int) bool {
		if from == 0 || to == 0 {
			return false
		}
		return true
	}

	z := zmey.NewZmey(&zmey.Config{
		Debug: false,
	})

	for i := 0; i < scale; i++ {
		z.AddProcess(NewForwarder)
	}

	z.Filter(filterF)

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Minute)

	injectF := func(pid int, c zmey.Client) {
		c.Call(calls[pid])
	}
	z.Inject(injectF)

	responses, _, err := z.Round(ctx)

	require.NoError(t, err)

	require.Equal(t, scale, len(responses), "inject output size mismatch")

	fcallOutput := make([][]FCall, scale)
	for i := range responses {
		fcallOutput[i] = make([]FCall, len(responses[i]))
		for j := range responses[i] {
			fcallOutput[i][j] = responses[i][j].(FCall)
		}
		sort.Slice(fcallOutput[i], func(p, q int) bool {
			return fcallOutput[i][p].SequenceNumber < fcallOutput[i][q].SequenceNumber
		})
	}

	assert.Equal(t, []FCall{}, fcallOutput[0])
	assert.Equal(t, []FCall{calls[3]}, fcallOutput[1])
	assert.Equal(t, []FCall{calls[1]}, fcallOutput[2])
	assert.Equal(t, []FCall{calls[4]}, fcallOutput[3])
	assert.Equal(t, []FCall{}, fcallOutput[4])

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

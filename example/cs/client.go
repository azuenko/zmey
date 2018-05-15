package cs

import (
	"github.com/stratumn/zmey"
)

// Client implements Process interface and runs client algorithm.
type Client struct {
	pid             int
	api             zmey.API
	pendingRequests []TimestampedCall
	serverPid       int
	time            uint
	timeout         uint
}

// Call represents the data passed from injector to client.
type Call struct {
	ID      int
	Payload []byte
}

// Return represents the data passed from client to collector.
type Return struct {
	ID      int
	Payload []byte
	Timeout bool
}

// TimestampedCall wraps Call and adds a timestamp.
type TimestampedCall struct {
	Call      Call
	Timestamp uint
}

// NewClient creates and initializes an instance of a Client
func NewClient(pid, serverPid int, timeout uint) zmey.Process {
	return &Client{
		pid:       pid,
		serverPid: serverPid,
		timeout:   timeout,
	}
}

// Bind implements Process.Bind
func (c *Client) Bind(api zmey.API) {
	c.api = api
}

// ReceiveNet implements Process.ReceiveNet
func (c *Client) ReceiveNet(from int, payload interface{}) {
	t := zmey.NewTracer("ReceiveNet [Client]")
	msg, ok := payload.(Response)
	if !ok {
		c.api.ReportError(t.Errorf("cannot coerse to the Response type: %+v", payload))
	}

	t = t.Fork("response %d", msg.ID)
	c.api.Trace(t.Logf("received"))
	clientReturn := Return{ID: msg.ID, Payload: msg.Payload, Timeout: false}
	c.api.Return(clientReturn)
	c.api.Trace(t.Logf("returned"))

	remainingRequests := make([]TimestampedCall, 0, len(c.pendingRequests))
	for i := range c.pendingRequests {
		if c.pendingRequests[i].Call.ID != msg.ID {
			remainingRequests = append(remainingRequests, c.pendingRequests[i])
		}
	}

	c.pendingRequests = remainingRequests

}

// ReceiveCall implements Process.ReceiveCall
func (c *Client) ReceiveCall(call interface{}) {
	t := zmey.NewTracer("ReceiveCall [Client]")
	msg, ok := call.(Call)
	if !ok {
		c.api.ReportError(t.Errorf("cannot coerce to Call: %+v", call))
		return
	}

	t = t.Fork("client call %d", msg.ID)
	c.api.Trace(t.Logf("received"))

	c.pendingRequests = append(c.pendingRequests, TimestampedCall{Call: msg, Timestamp: c.time})

	c.api.Send(c.serverPid, Request{ID: msg.ID, Payload: msg.Payload})
}

// Tick implements Process.Tick
func (c *Client) Tick(tick uint) {
	t := zmey.NewTracer("Tick [Client]")
	t = t.Fork("%d", tick)
	c.api.Trace(t.Logf("received"))

	c.time += tick

	remainingRequests := make([]TimestampedCall, 0, len(c.pendingRequests))
	timeoutRequests := make([]TimestampedCall, 0, len(c.pendingRequests))
	for i := range c.pendingRequests {
		if c.time > c.pendingRequests[i].Timestamp+c.timeout && c.timeout != 0 {
			timeoutRequests = append(timeoutRequests, c.pendingRequests[i])
		} else {
			remainingRequests = append(remainingRequests, c.pendingRequests[i])
		}
	}

	c.pendingRequests = remainingRequests

	for i := range timeoutRequests {
		c.api.Trace(t.Logf("timeout for call %d, returning request", timeoutRequests[i].Call.ID))
		clientReturn := Return{
			ID:      timeoutRequests[i].Call.ID,
			Payload: timeoutRequests[i].Call.Payload,
			Timeout: true,
		}
		c.api.Return(clientReturn)
	}
}

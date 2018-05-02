package dispatcher

import (
	"github.com/stratumn/zmey"
)

// Dispatcher implements Process interface and runs dispatcher algorithm.
type Dispatcher struct {
	pid             int
	api             zmey.API
	pendingRequests []TimestampedClientRequest
	time            uint
	timeout         uint
}

// ClientRequest represents the message sent by client to follower.
type ClientRequest struct {
	ID      int
	Payload []byte
}

// ClientResponse represents the message send by follower to client.
type ClientResponse struct {
	ID      int
	Payload []byte
	Timeout bool
}

// DispatchedRequest represents the message send by follower to leader.
type DispatchedRequest struct {
	ID      int
	Payload []byte
}

// DispatchedResponse represents the message send by leader to follower.
type DispatchedResponse struct {
	ID      int
	Payload []byte
}

// TimestampedClientRequest wraps ClientRequest and adds a timestamp.
type TimestampedClientRequest struct {
	Request   ClientRequest
	Timestamp uint
}

// NewDispatcher creates and initializes an instance of a Dispatcher
func NewDispatcher(pid int, timeout uint) zmey.Process {
	return &Dispatcher{pid: pid, timeout: timeout}
}

// Bind implements Process.Bind
func (d *Dispatcher) Bind(api zmey.API) {
	d.api = api
}

// ReceiveNet implements Process.ReceiveNet
func (d *Dispatcher) ReceiveNet(from int, payload interface{}) {
	t := zmey.NewTracer("ReceiveNet")
	switch msg := payload.(type) {
	case DispatchedRequest:
		t = t.Fork("dispatched request %d", msg.ID)
		if d.pid == 0 {
			t = t.Fork("leader")
			d.api.Trace(t.Logf("received"))
			dispatchedResponse := DispatchedResponse{ID: msg.ID, Payload: msg.Payload}
			d.api.Send(from, dispatchedResponse)
			d.api.Trace(t.Logf("sent response %d", dispatchedResponse.ID))
		} else {
			d.api.ReportError(t.Errorf("followers are not supposed to receive DispatchedRequest: %+v", msg))
		}
	case DispatchedResponse:
		t = t.Fork("dispatched response %d", msg.ID)
		if d.pid != 0 {
			t = t.Fork("leader")
			d.api.Trace(t.Logf("received"))
			clientResponse := ClientResponse{ID: msg.ID, Payload: msg.Payload, Timeout: false}
			d.api.Return(clientResponse)
			d.api.Trace(t.Logf("sent response %d", clientResponse.ID))
		} else {
			d.api.ReportError(t.Errorf("leader is not supposed to receive DispatchedResponse: %+v", msg))
		}
	default:
		d.api.ReportError(t.Errorf("cannot coerse to the correct type: %+v", payload))
	}

}

// ReceiveCall implements Process.ReceiveCall
func (d *Dispatcher) ReceiveCall(c interface{}) {
	t := zmey.NewTracer("ReceiveCall")
	msg, ok := c.(ClientRequest)
	if !ok {
		d.api.ReportError(t.Errorf("cannot coerce to ClientRequest: %+v", c))
		return
	}

	if d.pid == 0 {
		d.api.ReportError(t.Errorf("leader is not supposed to receive client requests"))
		return
	}
	t = t.Fork("cliend request %d", msg.ID)

	d.api.Trace(t.Logf("received"))

	d.pendingRequests = append(d.pendingRequests, TimestampedClientRequest{Request: msg, Timestamp: d.time})

	d.api.Send(0, DispatchedRequest{ID: msg.ID, Payload: msg.Payload})
}

// Tick implements Process.Tick
func (d *Dispatcher) Tick(tick uint) {
	t := zmey.NewTracer("Tick")
	t = t.Fork("%d", tick)
	d.api.Trace(t.Logf("received"))

	d.time += tick

	remainingRequests := make([]TimestampedClientRequest, 0, len(d.pendingRequests))
	timeoutRequests := make([]TimestampedClientRequest, 0, len(d.pendingRequests))
	for i := range d.pendingRequests {
		if d.time > d.pendingRequests[i].Timestamp+d.timeout && d.timeout != 0 {
			timeoutRequests = append(timeoutRequests, d.pendingRequests[i])
		} else {
			remainingRequests = append(remainingRequests, d.pendingRequests[i])
		}
	}

	d.pendingRequests = remainingRequests

	for i := range timeoutRequests {
		d.api.Trace(t.Logf("timeout for request %d", timeoutRequests[i].Request.ID))
		clientResponse := ClientResponse{ID: timeoutRequests[i].Request.ID, Timeout: true}
		d.api.Return(clientResponse)
	}
}

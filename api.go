package zmey

import (
	"log"
)

// API lets the process to communicate the data to the framework.
type API struct {
	pid     int
	scale   int
	net     *Net
	returnC chan *Call
}

// Send should be called whenever a process needs to send a message to
// another process.
func (a API) Send(m *Message) {
	if m.To < 0 || m.To >= a.scale {
		log.Printf("[%4d] Send: error: %d: invalid message recepient id, should be between 0 and %d", a.pid, m.To, a.scale)
		return
	}

	if debug {
		log.Printf("[%4d] Send: sending message %+v", a.pid, m)
	}
	a.net.Send(a.pid, m.To, m)
	if debug {
		log.Printf("[%4d] Send: done", a.pid)
	}
}

// Return should be called whenever a process needs return a call to its client.
func (a API) Return(c *Call) {
	if debug {
		log.Printf("[%4d] Return: returning call %+v", a.pid, c)
	}
	a.returnC <- c
	if debug {
		log.Printf("[%4d] Return: done", a.pid)
	}
}

// ReportError should be used for any errors to be escalated to the upper layer
// There is no specific error handling implemented, currently the errors are
// just printed via log package
func (a API) ReportError(err error) {
	log.Printf("[%4d] ReportError: %s", a.pid, err)
}

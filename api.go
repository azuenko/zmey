package zmey

import (
	"log"
)

// API lets the process to communicate the data to the framework.
type API interface {
	// Send should be called whenever a process needs to send a message to
	// another process.
	Send(to int, payload interface{})
	// Return should be called whenever a process needs to return a call to its client.
	Return(payload interface{})
	// Trace used for logging
	Trace(payload interface{})
	// ReportError should be used for any errors to be escalated to the upper layer
	// There is no specific error handling implemented, currently the errors are
	// just printed via log package
	ReportError(error)
}

// api implements exported API interface
type api struct {
	pid     int
	scale   int
	net     *Net
	returnC chan interface{}
	traceC  chan interface{}
	debug   bool
}

func (a api) Send(to int, payload interface{}) {
	if to < 0 || to >= a.scale {
		log.Printf("[%4d] Send: error: %d: invalid message recepient id, should be between 0 and %d", a.pid, to, a.scale)
		return
	}

	if a.debug {
		log.Printf("[%4d] Send: sending message %+v", a.pid, payload)
	}
	a.net.Send(a.pid, to, payload)
	if a.debug {
		log.Printf("[%4d] Send: done", a.pid)
	}
}

func (a api) Return(c interface{}) {
	if a.debug {
		log.Printf("[%4d] Return: returning call %+v", a.pid, c)
	}
	a.returnC <- c
	if a.debug {
		log.Printf("[%4d] Return: done", a.pid)
	}
}

func (a api) Trace(t interface{}) {
	a.traceC <- t
}

func (a api) ReportError(err error) {
	log.Printf("[%4d] ReportError: %s", a.pid, err)
	a.traceC <- err
}

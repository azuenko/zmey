package zmey

import (
	"log"
)

// Client lets the injector to communicate with the process.
type Client struct {
	pid   int
	callC chan interface{}
	debug bool
}

// Call sends a payload to the process.
func (c Client) Call(payload interface{}) {
	if c.debug {
		log.Printf("[%4d] Call: received %+v", c.pid, payload)
	}
	c.callC <- payload
	if c.debug {
		log.Printf("[%4d] Call: done", c.pid)
	}
}

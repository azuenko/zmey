package zmey

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrIncorrectPid is returned by the methods requiring process id.
	// Process id should be in the range 0 <= pid < scale.
	ErrIncorrectPid = errors.New("process id out of range")
	// ErrInjectCancelled is returned by zmey.Inject if the context is cancelled.
	ErrInjectCancelled = errors.New("inject cancelled")
)

var debug = false

const (
	timeoutProcess = 10 * time.Millisecond
	timeoutNetwork = 10 * time.Millisecond
	timeoutCollect = 10 * time.Millisecond

	sleepProcess = 100 * time.Millisecond
	sleepNetwork = 100 * time.Millisecond
	sleepCollect = 100 * time.Millisecond
)

// Zmey is the core structure of the framework.
type Zmey struct {
	c          *Config
	net        *Net
	session    *Session
	clients    []Client
	returnCs   []chan *Call
	responses  [][]interface{}
	injectLock sync.Mutex
}

// Process in the interface that has to be implemented by the distributed
// algorithm being tested. The interface lets framework to communicate with
// the process.
type Process interface {
	// Bind is called by the framework right after the creation of a process
	// instance. It conveys API which should be used by the process to send back
	// messages, calls and errors
	Bind(API)
	// ReceiveNet is called by the framework each time a process receives a message
	// from the network.
	ReceiveNet(*Message)
	// ReceiveCall is called by the framework each time an injector executes Call function
	// from Client interface
	ReceiveCall(*Call)
}

// Message is used in inter-process communication.
type Message struct {
	// From reprenests sender id.
	From int
	// From reprenests recepient id.
	To int
	// Payload contains application-specific data.
	Payload interface{}
}

// Call encapsulates data exchanged between the process and its client.
type Call struct {
	Payload interface{}
}

// Config is used to initialize new zmey.Zmey instance.
type Config struct {
	// Scale is another word for the number of processes in the system.
	Scale int
	// Factory hold a function creating new processes, given the process id.
	Factory func(int) Process
}

// InjectFunc specifies the type of a function used to inject new data into
// the system. The function receives process id as its first argument and
// Client as the second one. InjectFunc should be thread-safe as many of them
// are started in separate goroutines.
type InjectFunc func(pid int, c Client)

// NewZmey creates and returns an instance of Zmey framework.
func NewZmey(c *Config) *Zmey {
	session := NewSession(c.Scale)

	z := Zmey{
		c:        c,
		net:      NewNet(c.Scale, session),
		clients:  make([]Client, c.Scale),
		returnCs: make([]chan *Call, c.Scale),
		session:  session,
	}

	for pid := 0; pid < z.c.Scale; pid++ {
		process := z.c.Factory(pid)
		callC := make(chan *Call)
		returnC := make(chan *Call)
		api := API{pid: pid, scale: c.Scale, net: z.net, returnC: returnC}
		client := Client{pid: pid, callC: callC}
		process.Bind(api)
		z.clients[pid] = client
		z.returnCs[pid] = returnC

		go processFunc(process, pid, c.Scale, z.net, callC, z.session)
	}

	go collectResponsesFunc(c.Scale, z.returnCs, &z.responses, z.session)
	return &z
}

// Inject method runs multiple instances of InjectFunc in parallel. It returns
// the slice of slices of responses. An item `i` of the outer slice represents
// the responces of the process with the id `i`. The responses of a particular
// process are not in order, so some sorting is required for determenistic
// behaviour. The ErrInjectCancelled is returned if the context is cancelled
// before the processing ends. The method is thread-safe, however no
// parallel execution is implemented so far.
func (z *Zmey) Inject(ctx context.Context, injectF InjectFunc) ([][]interface{}, error) {
	z.injectLock.Lock()
	defer z.injectLock.Unlock()

	z.responses = make([][]interface{}, z.c.Scale)
	for pid := 0; pid < z.c.Scale; pid++ {
		go injectF(pid, z.clients[pid])
	}

	done := make(chan struct{})

	go func() {
		z.session.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return z.responses, nil
	case <-ctx.Done():
		return nil, ErrInjectCancelled
	}

}

// Status returns a string providing insights on the internal state of the
// execution.
func (z *Zmey) Status() string {
	receivedN, bufferedN, sentN := z.net.Stats()
	return fmt.Sprintf("net [%5d/%5d/%5d] session %s profs %s",
		receivedN, bufferedN, sentN,
		z.session.Status(),
		z.session.Profs(),
	)
}

// NetBufferStats redirects the call to the underlying BufferStats of Net object
func (z *Zmey) NetBufferStats() string {
	return z.net.BufferStats()
}

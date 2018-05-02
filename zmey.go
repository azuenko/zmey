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
	// ErrCancelled is returned if the context is cancelled.
	ErrCancelled = errors.New("context cancelled")
)

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
	c             *Config
	net           *Net
	session       *Session
	clients       []Client
	callCs        []chan interface{}
	returnCs      []chan interface{}
	traceCs       []chan interface{}
	tickCs        []chan uint
	responses     [][]interface{}
	traces        [][]interface{}
	processes     []Process
	roundLock     sync.Mutex
	responsesLock sync.Mutex
	tracesLock    sync.Mutex
	tick          uint
	injectF       InjectFunc
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
	ReceiveNet(from int, payload interface{})
	// ReceiveCall is called by the framework each time an injector executes Call function
	// from Client interface
	ReceiveCall(payload interface{})
	// Tick represents time
	Tick(uint)
}

// Config is used to initialize new zmey.Zmey instance.
type Config struct {
	// Scale is another word for the number of processes in the system.
	Scale int
	// Factory hold a function creating new processes, given the process id.
	Factory func(int) Process
	// Debug enables verbose logging
	Debug bool
}

// InjectFunc specifies the type of a function used to inject new data into
// the system. The function receives process id as its first argument and
// Client as the second one. InjectFunc should be thread-safe as multiple
// are started in separate goroutines.
type InjectFunc func(pid int, c Client)

// FilterFunc specifies a fuction used to selectively cut communication channels.
// If the fuction evaluates to true, the channel between process `from` and `to`
// is open.
type FilterFunc func(from int, to int) bool

// NewZmey creates and returns an instance of Zmey framework.
func NewZmey(c *Config) *Zmey {
	session := NewSession(c.Scale)

	z := Zmey{
		c:         c,
		net:       NewNet(c.Scale, session),
		processes: make([]Process, c.Scale),
		clients:   make([]Client, c.Scale),
		callCs:    make([]chan interface{}, c.Scale),
		returnCs:  make([]chan interface{}, c.Scale),
		traceCs:   make([]chan interface{}, c.Scale),
		tickCs:    make([]chan uint, c.Scale),
		responses: make([][]interface{}, c.Scale),
		traces:    make([][]interface{}, c.Scale),
		session:   session,
	}

	for pid := 0; pid < z.c.Scale; pid++ {
		process := z.c.Factory(pid)
		callC := make(chan interface{})
		returnC := make(chan interface{})
		traceC := make(chan interface{})
		tickC := make(chan uint)
		api := api{
			pid:     pid,
			scale:   c.Scale,
			net:     z.net,
			returnC: returnC,
			traceC:  traceC,
			debug:   c.Debug,
		}
		client := Client{
			pid:   pid,
			callC: callC,
			debug: c.Debug,
		}
		process.Bind(api)

		z.processes[pid] = process
		z.clients[pid] = client
		z.callCs[pid] = callC
		z.returnCs[pid] = returnC
		z.traceCs[pid] = traceC
		z.tickCs[pid] = tickC

		go z.processLoop(pid)
	}

	go z.collectLoop()
	return &z
}

// Inject sets inject function. The actual call of the injector occurs
// in Round() method. Inject is thread-safe.
func (z *Zmey) Inject(injectF InjectFunc) {
	z.roundLock.Lock()
	defer z.roundLock.Unlock()

	z.injectF = injectF
}

// Filter sets filter function. If `filterF` is `nil`, no filtering occurs,
// all communication channels are open. Filter is thread-safe.
func (z *Zmey) Filter(filterF FilterFunc) {
	z.roundLock.Lock()
	defer z.roundLock.Unlock()

	z.net.Filter(filterF)
}

// Tick simulates time by calling `Tick()` method of all processes.
// Each process receives the same time unit `t`. Tick is thread-safe.
func (z *Zmey) Tick(t uint) {
	z.roundLock.Lock()
	defer z.roundLock.Unlock()

	z.tick = t
}

// Round runs the simulation (inject and/or tick functions). It returns
// the slice of slices of responses. An item `i` of the outer slice represents
// the responces of the process with the id `i`. The responses of a particular
// process are not in order, so some sorting is required for determenistic
// behaviour. The ErrCancelled is returned if the context is cancelled
// before the processing ends. The method is thread-safe, however no
// parallel execution is implemented so far.
func (z *Zmey) Round(ctx context.Context) ([][]interface{}, [][]interface{}, error) {
	z.roundLock.Lock()
	defer z.roundLock.Unlock()

	if z.injectF != nil {
		for pid := 0; pid < z.c.Scale; pid++ {
			go z.injectF(pid, z.clients[pid])
		}
		z.injectF = nil
	}

	if z.tick != 0 {
		for pid := 0; pid < z.c.Scale; pid++ {
			go z.tickF(pid, z.tick)
		}
		z.tick = 0
	}

	done := make(chan struct{})

	go func() {
		z.session.WaitBusy() // We need to make sure the processes started
		z.session.WaitIdle() // Then we wait them to finish
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return nil, nil, ErrCancelled
	}

	responses := make([][]interface{}, z.c.Scale)
	traces := make([][]interface{}, z.c.Scale)

	z.tracesLock.Lock()
	for pid := range z.traces {
		traces[pid] = z.traces[pid]
		z.traces[pid] = nil
	}
	z.tracesLock.Unlock()

	z.responsesLock.Lock()
	for pid := range z.responses {
		responses[pid] = z.responses[pid]
		z.responses[pid] = nil
	}
	z.responsesLock.Unlock()

	return responses, traces, nil

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

// BufferStats redirects the call to the underlying BufferStats of Net object
func (z *Zmey) BufferStats() string {
	return z.net.BufferStats()
}

package zmey

import (
	"context"
	"errors"
	// "fmt"
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
	sync.Mutex

	c *Config

	packs []pack

	tick    uint
	injectF InjectFunc
	filterF FilterFunc

	statusC      chan string
	bufferStatsC chan string
}

// pack wraps Process and adds some context used by the framework
type pack struct {
	pid       int
	process   Process
	client    *client
	api       *api
	callC     chan interface{}
	returnC   chan interface{}
	traceC    chan interface{}
	tickC     chan uint
	responses []interface{}
	traces    []interface{}
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
	// Debug enables verbose logging
	Debug bool
}

// FactoryFunc creates an instance of a process provided the process id
type FactoryFunc func(int) Process

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

	z := Zmey{
		c:            c,
		statusC:      make(chan string),
		bufferStatsC: make(chan string),
	}

	return &z
}

// AddProcess adds new process to the existing configugation.
// The method is thread-safe
func (z *Zmey) AddProcess(factory FactoryFunc) {
	z.Lock()
	defer z.Unlock()

	pid := len(z.packs)
	process := factory(pid)
	callC := make(chan interface{})
	returnC := make(chan interface{})
	traceC := make(chan interface{})
	tickC := make(chan uint)
	api := api{
		pid:     pid,
		returnC: returnC,
		traceC:  traceC,
		debug:   z.c.Debug,
	}
	client := client{
		pid:   pid,
		callC: callC,
		debug: z.c.Debug,
	}
	process.Bind(&api)

	p := pack{
		pid:     pid,
		process: process,
		api:     &api,
		client:  &client,
		callC:   callC,
		returnC: returnC,
		traceC:  traceC,
		tickC:   tickC,
	}

	z.packs = append(z.packs, p)

}

// Inject sets inject function. The actual call of the injector occurs
// in Round() method. Inject is thread-safe.
func (z *Zmey) Inject(injectF InjectFunc) {
	z.Lock()
	defer z.Unlock()

	z.injectF = injectF
}

// Filter sets filter function. If `filterF` is `nil`, no filtering occurs,
// all communication channels are open. Filter is thread-safe.
func (z *Zmey) Filter(filterF FilterFunc) {
	z.Lock()
	defer z.Unlock()

	z.filterF = filterF
}

// Tick simulates time by calling `Tick()` method of all processes.
// Each process receives the same time unit `t`. Tick is thread-safe.
func (z *Zmey) Tick(t uint) {
	z.Lock()
	defer z.Unlock()

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
	z.Lock()
	defer z.Unlock()

	scale := len(z.packs)

	var wg sync.WaitGroup
	cancelFs := []context.CancelFunc{}

	session := NewSession(scale)

	ctxNet, cancelF := context.WithCancel(ctx)
	net := NewNet(ctxNet, &wg, scale, session)
	cancelFs = append(cancelFs, cancelF)

	for pid := range z.packs {
		z.packs[pid].api.BindNet(net)
	}

	if z.filterF != nil {
		net.Filter(z.filterF)
		z.filterF = nil
	}

	for pid := range z.packs {
		ctxProcess, cancelF := context.WithCancel(ctx)
		cancelFs = append(cancelFs, cancelF)
		go z.processLoop(ctxProcess, &wg, &z.packs[pid], session, net)
	}

	if z.injectF != nil {
		for pid := range z.packs {
			go z.injectF(pid, z.packs[pid].client)
		}
		z.injectF = nil
	}

	if z.tick != 0 {
		for pid := 0; pid < scale; pid++ {
			go z.tickF(pid, &wg, z.tick)
		}
		z.tick = 0
	}

	ctxCollect, cancelF := context.WithCancel(ctx)
	cancelFs = append(cancelFs, cancelF)
	go z.collectLoop(ctxCollect, &wg, session)

	ctxStatus, cancelF := context.WithCancel(ctx)
	cancelFs = append(cancelFs, cancelF)
	go z.statusLoop(ctxStatus, &wg, net, session)

	done := make(chan struct{})

	go func() {
		session.WaitBusy()        // We need to make sure the processes started
		session.WaitIdle()        // Then we wait them to finish processing
		for i := range cancelFs { // Then call goroutines to exit
			cancelFs[i]()
		}
		wg.Wait() // And wait for them
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return nil, nil, ErrCancelled
	}

	responses := make([][]interface{}, len(z.packs))
	traces := make([][]interface{}, len(z.packs))

	for pid := range z.packs {
		traces[pid] = z.packs[pid].traces
		z.packs[pid].traces = nil
	}

	for pid := range z.packs {
		responses[pid] = z.packs[pid].responses
		z.packs[pid].responses = nil
	}

	return responses, traces, nil

}

// Status returns a channel of strings which provides insights on the internal
// state of the execution.
func (z *Zmey) Status() <-chan string {
	return z.statusC
}

// BufferStats returns a channel of strings, each string is a table of buffered
// messages in the network.
func (z *Zmey) BufferStats() <-chan string {
	return z.bufferStatsC
}

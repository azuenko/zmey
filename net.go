package zmey

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"
)

// TODO: add context

// Net abstracts the inter-process connections
type Net struct {
	scale      int
	session    *Session
	filterF    FilterFunc
	inputCs    []chan interface{}
	outputCs   []chan interface{}
	buffer     [][]interface{}
	bufferLock sync.RWMutex
	bufferedN  int
	sentN      int
	receivedN  int
}

// NewNet creates and returns a new instance of Net. Scale indicates the size
// of the network, session may be optionally provided to report status and stats.
func NewNet(scale int, session *Session) *Net {

	n := Net{
		scale:    scale,
		inputCs:  make([]chan interface{}, scale*scale),
		outputCs: make([]chan interface{}, scale*scale),
		buffer:   make([][]interface{}, scale*scale),
		session:  session,
	}

	for i := 0; i < scale; i++ {
		for j := 0; j < scale; j++ {
			n.inputCs[i*scale+j] = make(chan interface{})
			n.outputCs[i*scale+j] = make(chan interface{})
		}
	}

	go n.loop()

	return &n
}

func (n *Net) loop() {
	if n.session != nil {
		n.session.ProfNetworkStart()
	}

	cases := make([]reflect.SelectCase, 2*n.scale*n.scale+1)
	for i := 0; i < n.scale*n.scale; i++ {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(n.inputCs[i]),
		}
	}

	for {
		cases[2*n.scale*n.scale] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(time.After(timeoutNetwork)),
		}
		for i := n.scale * n.scale; i < 2*n.scale*n.scale; i++ {
			if len(n.buffer[i-n.scale*n.scale]) > 0 {
				cases[i] = reflect.SelectCase{
					Dir:  reflect.SelectSend,
					Chan: reflect.ValueOf(n.outputCs[i-n.scale*n.scale]),
					Send: reflect.ValueOf(n.buffer[i-n.scale*n.scale][0]),
				}
			} else {
				// It's easier to add nil channel and keep the array length fixed
				cases[i] = reflect.SelectCase{
					Dir:  reflect.SelectSend,
					Chan: reflect.ValueOf(nil),
					Send: reflect.ValueOf(struct{}{}),
				}
			}
		}
		if n.session != nil {
			n.session.ProfNetworkSelectStart()
		}
		chosen, value, ok := reflect.Select(cases)
		if n.session != nil {
			n.session.ProfNetworkSelectEnd()
		}

		switch {
		case 0 <= chosen && chosen < n.scale*n.scale: // send
			if !ok {
				log.Printf("[   N] channel %d is closed", chosen)
				continue
			}

			from := chosen % n.scale
			to := chosen / n.scale

			if n.filterF == nil || n.filterF(from, to) {
				n.push(chosen, value.Interface())
			}
		case n.scale*n.scale <= chosen && chosen < 2*n.scale*n.scale: // receive
			// No need to use the returned value, it's been already sent
			// over the channel in Select() statement
			_ = n.pop(chosen - n.scale*n.scale)
		case chosen == 2*n.scale*n.scale: // timeout
			if n.bufferedN == 0 {

				if n.session != nil {
					n.session.ReportNetworkIdle()
				}
				time.Sleep(sleepNetwork)
				if n.session != nil {
					n.session.ReportNetworkBusy()
				}
			}
		default:
			log.Printf("[   N] chosen incorrect channel %d", chosen)
		}
	}

}

// Filter sets FilterFunc to selectively cut communicational channels
func (n *Net) Filter(filterF FilterFunc) {
	n.filterF = filterF
}

// Send sens the message `m` to the recepeint with process id `to`. `as` should
// represent the id of sender process. If either `as` or `to` is out of range,
// ErrIncorrectPid is returned
func (n *Net) Send(as, to int, m interface{}) error {
	if as < 0 || as >= n.scale || to < 0 || to >= n.scale {
		return ErrIncorrectPid
	}

	n.inputCs[to*n.scale+as] <- m

	return nil
}

// Recv returns the channel of messages. Reading from the channel would
// yield the messages sent by `from` to `as`. If either `as` or `from`
// is out of range, ErrIncorrectPid is returned.
func (n *Net) Recv(as, from int) (chan interface{}, error) {
	if as < 0 || as >= n.scale || from < 0 || from >= n.scale {
		return nil, ErrIncorrectPid
	}

	return n.outputCs[as*n.scale+from], nil
}

// BufferStats returns an ASCII-formatted matrix of the sizes of buffers
// (not yet delivered messages)
func (n *Net) BufferStats() string {
	s := "    |  to|\n"
	s += "----+----+" + strings.Repeat("----+", n.scale)
	s += "\n"
	s += "from|    |"
	for i := 0; i < n.scale; i++ {
		s += fmt.Sprintf("%4d|", i)
	}
	s += "\n"
	s += "----+----+" + strings.Repeat("----+", n.scale)
	s += "\n"

	func() {
		n.bufferLock.RLock()
		defer n.bufferLock.RUnlock()

		for i := 0; i < n.scale; i++ {
			s += fmt.Sprintf("    |%4d|", i)
			for j := 0; j < n.scale; j++ {
				nm := fmt.Sprintf("%4d|", len(n.buffer[i*n.scale+j]))
				if nm == "   0|" {
					nm = "    |"
				}
				s += nm
			}
			s += "\n"
		}
	}()

	s += "----+----+" + strings.Repeat("----+", n.scale)
	s += "\n"

	return s
}

// Stats returns statistics of the network since its creation: number of
// received, buffered and sent messages.
func (n *Net) Stats() (int, int, int) {
	n.bufferLock.RLock()
	defer n.bufferLock.RUnlock()

	return n.receivedN, n.bufferedN, n.sentN
}

func (n *Net) push(index int, item interface{}) {
	n.bufferLock.Lock()
	defer n.bufferLock.Unlock()

	n.buffer[index] = append(n.buffer[index], item)
	n.bufferedN++
	n.receivedN++
}

func (n *Net) pop(index int) interface{} {
	n.bufferLock.Lock()
	defer n.bufferLock.Unlock()

	item := n.buffer[index][0]
	n.buffer[index] = n.buffer[index][1:]
	n.bufferedN--
	n.sentN++

	return item
}

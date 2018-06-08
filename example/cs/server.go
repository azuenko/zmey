package cs

import (
	"github.com/stratumn/zmey"
)

// Server implements Process interface and runs server algorithm.
type Server struct {
	pid int

	sendF   func(to int, payload interface{})
	returnF func(payload interface{})
	traceF  func(payload interface{})
	errorF  func(error)
}

// Request represents the message send from client to server.
type Request struct {
	ID      int
	Payload []byte
}

// Response represents the message send back from server to client.
type Response struct {
	ID      int
	Payload []byte
}

// NewServer creates and initializes an instance of a Server
func NewServer(pid int) zmey.Process {
	return &Server{pid: pid}
}

func (s *Server) CallbackSend(sendF func(to int, payload interface{})) {
	s.sendF = sendF
}
func (s *Server) CallbackReturn(returnF func(payload interface{})) {
	s.returnF = returnF
}
func (s *Server) CallbackTrace(traceF func(payload interface{})) {
	s.traceF = traceF
}
func (s *Server) CallbackError(errorF func(error)) {
	s.errorF = errorF
}

// ReceiveNet implements Process.ReceiveNet
func (s *Server) ReceiveNet(from int, payload interface{}) {
	t := zmey.NewTracer("ReceiveNet [Server]")
	switch msg := payload.(type) {
	case Request:
		t = t.Fork("received request %d", msg.ID)
		s.traceF(t.Logf("received"))
		response := Response{ID: msg.ID, Payload: msg.Payload}
		s.sendF(from, response)
		s.traceF(t.Logf("sent response %d", response.ID))
	case Response:
		s.errorF(t.Errorf("server is not supposed to receive Response: %+v", msg))
	default:
		s.errorF(t.Errorf("cannot coerse to the correct type: %+v", payload))
	}

}

// ReceiveCall implements Process.ReceiveCall
func (s *Server) ReceiveCall(call interface{}) {
	t := zmey.NewTracer("ReceiveCall [Server]")
	s.errorF(t.Errorf("server is not supposed to receive client calls"))
}

// Tick implements Process.Tick
func (s *Server) Tick(tick uint) {
	t := zmey.NewTracer("Tick [Server]")
	s.traceF(t.Logf("received"))
}

func (s *Server) Start() {}

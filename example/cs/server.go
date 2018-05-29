package cs

import (
	"github.com/stratumn/zmey"
)

// Server implements Process interface and runs server algorithm.
type Server struct {
	pid int
	api zmey.API
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

// Bind implements Process.Bind
func (s *Server) Bind(api zmey.API) {
	s.api = api
}

// ReceiveNet implements Process.ReceiveNet
func (s *Server) ReceiveNet(from int, payload interface{}) {
	t := zmey.NewTracer("ReceiveNet [Server]")
	switch msg := payload.(type) {
	case Request:
		t = t.Fork("received request %d", msg.ID)
		s.api.Trace(t.Logf("received"))
		response := Response{ID: msg.ID, Payload: msg.Payload}
		s.api.Send(from, response)
		s.api.Trace(t.Logf("sent response %d", response.ID))
	case Response:
		s.api.ReportError(t.Errorf("server is not supposed to receive Response: %+v", msg))
	default:
		s.api.ReportError(t.Errorf("cannot coerse to the correct type: %+v", payload))
	}

}

// ReceiveCall implements Process.ReceiveCall
func (s *Server) ReceiveCall(call interface{}) {
	t := zmey.NewTracer("ReceiveCall [Server]")
	s.api.ReportError(t.Errorf("server is not supposed to receive client calls"))
}

// Tick implements Process.Tick
func (s *Server) Tick(tick uint) {
	t := zmey.NewTracer("Tick [Server]")
	s.api.Trace(t.Logf("received"))
}

func (s *Server) Start() {}

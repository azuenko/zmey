package zmey

import (
	"fmt"
	"sync"
	"time"
)

// Session manages locks and stats for Net and Zmey
type Session struct {
	sync.Mutex

	scale       int
	networkIdle bool
	processIdle []bool
	collectIdle bool

	tNetwork       time.Time
	tNetworkSelect time.Time
	tNetworkSleep  time.Time
	dNetworkSelect time.Duration
	dNetworkSleep  time.Duration
	tCollect       time.Time
	tCollectSelect time.Time
	tCollectSleep  time.Time
	dCollectSelect time.Duration
	dCollectSleep  time.Duration

	tProcess       []time.Time
	tProcessSelect []time.Time
	tProcessSleep  []time.Time
	dProcessSelect []time.Duration
	dProcessSleep  []time.Duration
}

// NewSession creates and returns a new instance of Session
func NewSession(scale int) *Session {
	s := Session{scale: scale}

	s.processIdle = make([]bool, scale)

	s.tProcess = make([]time.Time, scale)
	s.tProcessSelect = make([]time.Time, scale)
	s.tProcessSleep = make([]time.Time, scale)
	s.dProcessSelect = make([]time.Duration, scale)
	s.dProcessSleep = make([]time.Duration, scale)

	return &s
}

// ReportNetworkIdle reports the network is in idle state
func (s *Session) ReportNetworkIdle() {
	s.Lock()
	defer s.Unlock()

	s.tNetworkSleep = time.Now()

	s.networkIdle = true
}

// ReportNetworkBusy reports the network is in busy state
func (s *Session) ReportNetworkBusy() {
	s.Lock()
	defer s.Unlock()

	s.dNetworkSleep += time.Since(s.tNetworkSleep)

	s.networkIdle = false
}

// ReportCollectIdle reports the collect function is in idle state
func (s *Session) ReportCollectIdle() {
	s.Lock()
	defer s.Unlock()

	s.tCollectSleep = time.Now()

	s.collectIdle = true
}

// ReportCollectBusy reports the collect function is in busy state
func (s *Session) ReportCollectBusy() {
	s.Lock()
	defer s.Unlock()

	s.dCollectSleep += time.Since(s.tCollectSleep)

	s.collectIdle = false
}

// ReportProcessIdle reports the process with id `pid` is in idle state
func (s *Session) ReportProcessIdle(pid int) error {
	s.Lock()
	defer s.Unlock()

	if pid < 0 || pid >= s.scale {
		return ErrIncorrectPid
	}

	s.tProcessSleep[pid] = time.Now()

	s.processIdle[pid] = true

	return nil
}

// ReportProcessBusy reports the process with id `pid` is in busy state
func (s *Session) ReportProcessBusy(pid int) error {
	s.Lock()
	defer s.Unlock()

	if pid < 0 || pid >= s.scale {
		return ErrIncorrectPid
	}

	s.dProcessSleep[pid] += time.Since(s.tProcessSleep[pid])

	s.processIdle[pid] = false

	return nil
}

// ProfNetworkStart should be called right after network is started
func (s *Session) ProfNetworkStart() {
	s.Lock()
	defer s.Unlock()

	s.tNetwork = time.Now()
}

// ProfNetworkSelectStart should be called right before network blocks at select call
func (s *Session) ProfNetworkSelectStart() {
	s.Lock()
	defer s.Unlock()

	s.tNetworkSelect = time.Now()
}

// ProfNetworkSelectEnd should be called right after network executes its select call
func (s *Session) ProfNetworkSelectEnd() {
	s.Lock()
	defer s.Unlock()

	s.dNetworkSelect += time.Since(s.tNetworkSelect)
}

// ProfCollectStart should be called right after collect function is started
func (s *Session) ProfCollectStart() {
	s.Lock()
	defer s.Unlock()

	s.tCollect = time.Now()
}

// ProfCollectSelectStart should be called right before collect function blocks at select call
func (s *Session) ProfCollectSelectStart() {
	s.Lock()
	defer s.Unlock()

	s.tCollectSelect = time.Now()
}

// ProfCollectSelectEnd should be called right after collect function executes its select call
func (s *Session) ProfCollectSelectEnd() {
	s.Lock()
	defer s.Unlock()

	s.dCollectSelect += time.Since(s.tCollectSelect)
}

// ProfProcessStart should be called right after the process with id `pid` is started
func (s *Session) ProfProcessStart(pid int) error {
	s.Lock()
	defer s.Unlock()

	if pid < 0 || pid >= s.scale {
		return ErrIncorrectPid
	}

	s.tProcess[pid] = time.Now()

	return nil
}

// ProfProcessSelectStart should be called right before the process with id `pid` blocks at select call
func (s *Session) ProfProcessSelectStart(pid int) error {
	s.Lock()
	defer s.Unlock()

	if pid < 0 || pid >= s.scale {
		return ErrIncorrectPid
	}

	s.tProcessSelect[pid] = time.Now()

	return nil
}

// ProfProcessSelectEnd should be called right after process with id `pid` executes its select call
func (s *Session) ProfProcessSelectEnd(pid int) {
	s.Lock()
	defer s.Unlock()

	if pid < 0 || pid >= s.scale {
		return
	}

	s.dProcessSelect[pid] += time.Since(s.tProcessSelect[pid])
}

// IsIdle returns `true` if all network, collect function and all processes are
// in idle state. Otherwise it returns false
func (s *Session) IsIdle() bool {
	s.Lock()
	defer s.Unlock()

	if !s.networkIdle {
		return false
	}

	if !s.collectIdle {
		return false
	}

	for _, v := range s.processIdle {
		if !v {
			return false
		}
	}

	return true
}

// Wait blocks until IsIdle is true
func (s *Session) Wait() {
	for {
		if s.IsIdle() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Status retuns string representation of active/idle network, collect function and processes.
func (s *Session) Status() string {
	s.Lock()
	defer s.Unlock()

	str := "n["
	if s.networkIdle {
		str += " "
	} else {
		str += "A"
	}

	str += "] c["

	if s.collectIdle {
		str += " "
	} else {
		str += "A"
	}

	str += "] pp["
	for i := 0; i < len(s.processIdle); i++ {
		if s.processIdle[i] {
			str += " "
		} else {
			str += "A"
		}
	}

	str += "]"

	return str
}

// Profs returns a string that describes where goroutines spend its time.
// For network, collect and (average of) all processes a triple is returned.
// First value in the triple corresponds to the actual time spend on execution,
// second value -- time spent waiting at select, third value -- time spent
// waiting at time.Sleep. The values in the triple sum up to 100 (percent).
func (s *Session) Profs() string {
	s.Lock()
	defer s.Unlock()

	tNow := time.Now()

	dCollect := tNow.Sub(s.tCollect)
	pCollectSelect := 100 * s.dCollectSelect / dCollect
	pCollectSleep := 100 * s.dCollectSleep / dCollect

	dNetwork := tNow.Sub(s.tNetwork)
	pNetworkSelect := 100 * s.dNetworkSelect / dNetwork
	pNetworkSleep := 100 * s.dNetworkSleep / dNetwork

	var dProcessAll time.Duration
	var dProcessSelectAll, dProcessSleepAll time.Duration

	for pid := 0; pid < s.scale; pid++ {
		dProcessAll += tNow.Sub(s.tProcess[pid])
		dProcessSelectAll += s.dProcessSelect[pid]
		dProcessSleepAll += s.dProcessSleep[pid]
	}

	pProcessSelect := 100 * dProcessSelectAll / dProcessAll
	pProcessSleep := 100 * dProcessSleepAll / dProcessAll

	return fmt.Sprintf("n[%2d/%2d/%2d] c[%2d/%2d/%2d] p[%2d/%2d/%2d]",
		100-pNetworkSelect-pNetworkSleep,
		pNetworkSelect,
		pNetworkSleep,
		100-pCollectSelect-pCollectSleep,
		pCollectSelect,
		pCollectSleep,
		100-pProcessSelect-pProcessSleep,
		pProcessSelect,
		pProcessSleep,
	)
}

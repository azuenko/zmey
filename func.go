package zmey

import (
	"log"
	"reflect"
	"time"
)

func (z *Zmey) processLoop(pid int) {
	z.session.ProfProcessStart(pid)

	cases := make([]reflect.SelectCase, z.c.Scale+3)
	for i := 0; i < z.c.Scale; i++ {
		recvC, err := z.net.Recv(pid, i)
		if err != nil {
			log.Printf("[%4d] processLoop: error: %s", pid, err)
			continue
		}
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(recvC),
		}
	}
	cases[z.c.Scale] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(z.callCs[pid]),
	}

	cases[z.c.Scale+1] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(z.tickCs[pid]),
	}
	for {

		cases[z.c.Scale+2] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(time.After(timeoutProcess)),
		}

		z.session.ProfProcessSelectStart(pid)
		chosen, value, ok := reflect.Select(cases)
		z.session.ProfProcessSelectEnd(pid)

		if !ok {
			log.Printf("[%4d] processLoop: channel %d is closed", pid, chosen)
			continue
		}

		switch {
		case 0 <= chosen && chosen < z.c.Scale: // network recv
			payload := value.Interface()

			if z.c.Debug {
				log.Printf("[%4d] processLoop: received message from %d : %+v", pid, chosen, payload)
			}
			z.processes[pid].ReceiveNet(chosen, payload)
			if z.c.Debug {
				log.Printf("[%4d] processLoop: message processed", pid)
			}
		case chosen == z.c.Scale: // client recv
			call := value.Interface()

			if z.c.Debug {
				log.Printf("[%4d] processLoop: received call: %+v", pid, call)
			}
			z.processes[pid].ReceiveCall(call)
			if z.c.Debug {
				log.Printf("[%4d] processLoop: call processed", pid)
			}
		case chosen == z.c.Scale+1: // tick
			t, ok := value.Interface().(uint)
			if !ok {
				log.Printf("[%4d] processLoop: value cannot be converted to tick: %+v", pid, value)
				continue
			}

			if z.c.Debug {
				log.Printf("[%4d] processLoop: received tick: %d", pid, t)
			}
			z.processes[pid].Tick(t)
		case chosen == z.c.Scale+2: // timeout
			if z.c.Debug {
				log.Printf("[%4d] processLoop: idle", pid)
			}
			if err := z.session.ReportProcessIdle(pid); err != nil {
				log.Printf("[%4d] processLoop: error %s", pid, err)
			}
			time.Sleep(sleepProcess)
			if err := z.session.ReportProcessBusy(pid); err != nil {
				log.Printf("[%4d] processLoop: error %s", pid, err)
			}
		default:
			log.Printf("[%4d] processLoop: chosen incorrect channel %d", pid, chosen)
		}

	}

}

func (z *Zmey) collectLoop() {
	z.session.ProfCollectStart()

	cases := make([]reflect.SelectCase, 2*z.c.Scale+1)

	for i := 0; i < z.c.Scale; i++ {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(z.returnCs[i]),
		}
	}

	for i := z.c.Scale; i < 2*z.c.Scale; i++ {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(z.traceCs[i-z.c.Scale]),
		}
	}

	for {
		cases[2*z.c.Scale] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(time.After(timeoutCollect)),
		}

		z.session.ProfCollectSelectStart()
		chosen, value, ok := reflect.Select(cases)
		z.session.ProfCollectSelectEnd()

		if !ok {
			log.Printf("[   C] channel %d is closed", chosen)
			continue
		}

		switch {
		case 0 <= chosen && chosen < z.c.Scale: // return call
			call := value.Interface()
			if z.c.Debug {
				log.Printf("[   C] appending response for pid %d", chosen)
			}
			z.responsesLock.Lock()
			z.responses[chosen] = append(z.responses[chosen], call)
			z.responsesLock.Unlock()
		case z.c.Scale <= chosen && chosen < 2*z.c.Scale: // trace call
			trace := value.Interface()
			if z.c.Debug {
				log.Printf("[   C] appending trace for pid %d", chosen)
			}
			z.tracesLock.Lock()
			z.traces[chosen-z.c.Scale] = append(z.traces[chosen-z.c.Scale], trace)
			z.tracesLock.Unlock()
		case chosen == 2*z.c.Scale: // timeout
			if z.c.Debug {
				log.Printf("[   C] idle")
			}
			z.session.ReportCollectIdle()
			time.Sleep(sleepCollect)
			z.session.ReportCollectBusy()
		default:
			log.Printf("[   C] chosen incorrect channel %d", chosen)
		}

	}
}

func (z *Zmey) tickF(pid int, t uint) {
	if z.c.Debug {
		log.Printf("[%4d] tickF: received %d", pid, t)
	}
	z.tickCs[pid] <- t
	if z.c.Debug {
		log.Printf("[%4d] tickF: done", pid)
	}
}

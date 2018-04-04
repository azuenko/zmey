package zmey

import (
	"log"
	"reflect"
	"time"
)

func processFunc(process Process, pid int, scale int, net *Net, callC chan *Call, session *Session) {
	session.ProfProcessStart(pid)

	cases := make([]reflect.SelectCase, scale+2)
	for i := 0; i < scale; i++ {
		recvC, err := net.Recv(pid, i)
		if err != nil {
			log.Printf("[%4d] processFunc: error: %s", pid, err)
			continue
		}
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(recvC),
		}
	}
	cases[scale] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(callC),
	}

	for {
		cases[scale+1] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(time.After(timeoutProcess)),
		}

		session.ProfProcessSelectStart(pid)
		chosen, value, ok := reflect.Select(cases)
		session.ProfProcessSelectEnd(pid)

		if chosen == scale+1 { // timeout
			if err := session.ReportProcessIdle(pid); err != nil {
				log.Printf("[%4d] processFunc: error %s", pid, err)
			}
			time.Sleep(sleepProcess)
			if err := session.ReportProcessBusy(pid); err != nil {
				log.Printf("[%4d] processFunc: error %s", pid, err)
			}
			continue
		}

		if !ok {
			log.Printf("[%4d] processFunc: network/client channel %d is closed", pid, chosen)
			continue
		}

		if chosen == scale { // client call
			call, ok := value.Interface().(*Call)
			if !ok {
				log.Printf("[%4d] processFunc: value cannot be converted to call: %+v", pid, value)
				continue
			}

			if debug {
				log.Printf("[%4d] processFunc: received call: %+v", pid, call)
			}
			process.ReceiveCall(call)
			if debug {
				log.Printf("[%4d] processFunc: call processed", pid)
			}
			continue
		}

		message, ok := value.Interface().(*Message)
		if !ok {
			log.Printf("[%4d] processFunc: value cannot be converted to message %+v", pid, value)
			continue
		}

		if debug {
			log.Printf("[%4d] processFunc: received message: %+v", pid, message)
		}
		process.ReceiveNet(message)
		if debug {
			log.Printf("[%4d] processFunc: message processed", pid)
		}

	}

}

func collectResponsesFunc(scale int, returnCs []chan *Call, responses *[][]interface{}, session *Session) {
	session.ProfCollectStart()

	cases := make([]reflect.SelectCase, scale+1)
	for i := 0; i < scale; i++ {
		cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(returnCs[i]),
		}
	}

	for {
		cases[scale] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(time.After(timeoutCollect)),
		}

		session.ProfCollectSelectStart()
		chosen, value, ok := reflect.Select(cases)
		session.ProfCollectSelectEnd()

		if chosen == scale { // timeout

			if debug {
				log.Printf("[   C] collector is idle")
			}
			session.ReportCollectIdle()
			time.Sleep(sleepCollect)
			session.ReportCollectBusy()
			continue
		}

		if !ok {
			log.Printf("[   C] return channel %d is closed", chosen)
			continue
		}

		call, ok := value.Interface().(*Call)
		if !ok {
			log.Printf("[   C] value cannot be converted to call %+v", value)
			continue
		}
		if debug {
			log.Printf("[   C] appending value for pid %d", chosen)
		}
		(*responses)[chosen] = append((*responses)[chosen], call.Payload)

	}
}

Zmey is a micro-framework for testing distributed algorithms.

[![GoDoc](https://godoc.org/github.com/stratumn/zmey?status.svg)](https://godoc.org/github.com/stratumn/zmey)

---

### About

Zmey hides the complexity of setting up and running a distributed system; and managing inter-process and client-process communication. You're only required to implement a simple interface, telling how the process should react to incoming messages and client calls; and the injector, supplying the initial set of messages. Zmey will then run the implemenation of the algorithms at a given scale and collect the returned messages.

### Example

The best place to use Zmey is unit tests -- it is fast and quite deterministic:

```go
func TestProcess(t *testing.T) {
    // Create Zmey instance configured to run the system of 10 processes,
    // with each process created by NewProcess function
    z := zmey.NewZmey(&zmey.Config{
        Scale:   10,
        Factory: NewProcess,
    })

    // Prepare the slices of initial calls, one slice per process
    initialCalls := make([][]interface{}, 10)
    // ...
    // Fill initialCalls
    // ...

    // Create inject function. An inject function receives process id and
    // zmey.Client c, which is used to perform the calls from initialCalls
    injectF := func(pid int, c zmey.Client) {
        for k := 0; k < len(initialCalls[pid]); k++ {
            c.Call(initialCalls[pid][k])
        }
    }

    // Set inject function to be lazily executed
    z.Inject(injectF)

    // (optionally) Isolate process 3 from the rest of the network, and simulate its failure to other processes by creating and setting simple filter function.
    filterF := func(from, to int) bool {
        if from == 3 || to == 3 {
            return false
        }
        return true
    }

    z.Filter(filterF)

    ctx, _ := context.WithTimeout(context.Background(), 1*time.Minute)

    // Run Zmey round. Processes' responses are captured and
    // collected in actualOutput slices, one slice per corresponding process
    actualOutput, _, _ := z.Round(ctx)

    // You may need to sort actualOutput to make it more deterministic

    expectedOutput := make([][]interface{}, 10)
    // ...
    // Fill expectedOutput
    // ...

    // Finally, compare expected and actual responses
    assert.Equal(t, expectedOutput, actualOutput)
}
```

The process being tested is created by `NewProcess` function. It should return an object, implementing `zmey.Process` interface:

```go
type Process interface {
    Bind(API)
    ReceiveNet(from int, payload interface{})
    ReceiveCall(payload interface{})
    Tick(uint)
}
```

The framework notifies the process about incoming messages and client calls by calling `ReceiveNet` and `ReceiveCall` methods. Time passing is conveyed through `Tick` events. The process talks back to the framework through `zmey.API` interface, received via `Bind` method:

```go
type interface API {
    Send(to int, payload interface{})
    Return(payload interface{})
    Trace(payload interface{})
    ReportError(error)
}
```

`Send` is used to communicate to other processes over the network, `Return` -- to return the data back to the client. `Trace` and `ReportError` are mainly used for logging. The returned data would be then collected and returned by the `Round` method.

For more details check out the forwarder example.

### Status

Zmey is in its early development stage. Current version is good for launching algorithms, and do some simple failure simulation.

Next releases expect to receive new nice features, such as:

* live reconfiguration, including adding/removing processes
* advanced network simulation (selective message drop and reordering)
* multiple `Process` implementations in a single network to simulate Byzantine fault tolerant algorithms
* performance opmitisations for scaled networks




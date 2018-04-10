Zmey is a micro-framework for testing distributed algorithms.

### About

Zmey hides the complexity of setting up and running a distributed system; and managing inter-process and client-process communication. You're only required to implement a simple interface, telling how the process should react to incoming messages and client calls; and the injector, supplying the initial set of messages. Zmey will then run the implemenation of the algorithms at a given scale and collect the returned messages.

### Example

The best place to use Zmey is unit tests -- it is fast and quite deterministic:

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

        ctx, _ := context.WithTimeout(context.Background(), 1*time.Minute)

        // Run Zmey by injecting the calls. Processes' responses are captured and
        // collected in actualOutput slices, one slice per corresponding process
        actualOutput, _ := z.Inject(ctx, injectF)

        // You may need to sort actualOutput to make it more deterministic

        expectedOutput := make([][]interface{}, 10)
        // ...
        // Fill expectedOutput
        // ...

        // Finally, compare expected and actual responses
        assert.Equal(t, expectedOutput, actualOutput)
    }

The process being tested is created by `NewProcess` function. It should return an object, implementing `zmey.Process` interface:

    type Process interface {
        Bind(API)
        ReceiveNet(*Message)
        ReceiveCall(*Call)
    }

The framework notifies the process about incoming messages and client calls by calling `ReceiveNet` and `ReceiveCall` methods. The process talks back to the framework through `zmey.API` interface, received via `Bind` method:

    type interface API {
        Send(*Message)
        Return(*Call)
        ReportError(error)
    }

`Send` is used to communicate to other processes over the network, `Return` -- to return the data back to the client. The returned data would be then collected and returned by the `Inject` method.

For more details check out the forwarder example.

### Status

Zmey is in its early development stage. Current version is good for launching algorithms, and that's essentially all it can do.

Next releases expect to receive new nice features, such as:

* time simulation through `Tick` method added to `Process` interface
* basic network simulation (controllable message drop and reordering)
* live reconfiguration, including adding/removing processes
* multiple `Process` implementations in a single network to simulate Byzantine fault tolerant algorithms
* performance opmitisations for scaled networks



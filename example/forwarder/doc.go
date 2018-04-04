/*
Package forwarder implements very simple forwading algorithm. Each process receives
messages from its client and other processes. A messages contains `to` field. Upon
reception, the process checks the field. If it equals to process id, the process
returns the message via `API.Return(c)` call. If `to` does not equal to the process id,
there are two additional cases. If the message came from the client, the process
forwards the message using `API.Send(m)`. Otherwise, it reports an error via
`API.ReportError(err)` to prevent from forwarding misrouted/invalid messages.

More formal description as follows:

    Implements:
        Forwarder, instance fw

    Uses:
        PerfectPointToPointLinks, instance pl
        Client, instance c

    upon event < fw, Init | pid > do
        pid := pid

    upon event < fw, Call | to, payload > do
        if to == pid then
            trigger < c, Return | payload >
        else
            trigger < pl, Send | (to, payload) >

    upon event < pl, Recv | (to, payload) > do
        if to == pid then
            trigger < c, Return | payload >
        else
            // report error

*/
package forwarder

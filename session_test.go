package zmey

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProfs(t *testing.T) {
	// Play with these timings to make test run as fast as possible, without
	// failing and with small delta
	const delta = 3
	const timeUnit = 4 * time.Millisecond

	var profs string
	var nb, ns, ni, cb, cs, ci, pb, ps, pi, xx int

	session := NewSession()

	session.ProfNetworkStart()
	time.Sleep(20 * timeUnit) // busy
	session.ProfNetworkSelectStart()
	time.Sleep(25 * timeUnit) // select
	session.ProfNetworkSelectEnd()
	time.Sleep(20 * timeUnit) // busy
	session.ReportNetworkIdle()
	time.Sleep(15 * timeUnit) // idle
	session.ReportNetworkBusy()
	time.Sleep(20 * timeUnit) // busy

	profs = session.Profs()

	fmt.Sscanf(profs, "n[%d/%d/%d] c[%d/%d/%d] p[%d/%d/%d]",
		&nb, &ns, &ni, &xx, &xx, &xx, &xx, &xx, &xx)

	t.Log(profs)
	t.Log(nb, ns, ni, cb, cs, ci, pb, ps, pi)

	session.ProfCollectStart()
	time.Sleep(10 * timeUnit) // busy
	session.ProfCollectSelectStart()
	time.Sleep(42 * timeUnit) // select
	session.ProfCollectSelectEnd()
	time.Sleep(5 * timeUnit) // busy
	session.ReportCollectIdle()
	time.Sleep(37 * timeUnit) // idle
	session.ReportCollectBusy()
	time.Sleep(6 * timeUnit) // busy

	profs = session.Profs()

	fmt.Sscanf(profs, "n[%d/%d/%d] c[%d/%d/%d] p[%d/%d/%d]",
		&xx, &xx, &xx, &cb, &cs, &ci, &xx, &xx, &xx)

	t.Log(profs)
	t.Log(nb, ns, ni, cb, cs, ci, pb, ps, pi)

	// process 4: ---------------sssssssssssssss----------iiiiiiiiiiiiiii-----
	// process 7:                     -----ssssssssss----------iiiii----------
	//            ^    ^    ^    ^    ^    ^    ^    ^    ^    ^    ^    ^    ^
	// time unit: 0    5    10   15   20   25   30   35   40   45   50   55   60

	session.ProfProcessStart(4) // 0
	time.Sleep(15 * timeUnit)
	session.ProfProcessSelectStart(4) // 15
	time.Sleep(5 * timeUnit)
	session.ProfProcessStart(7) // 20
	time.Sleep(5 * timeUnit)
	session.ProfProcessSelectStart(7) // 25
	time.Sleep(5 * timeUnit)
	session.ProfProcessSelectEnd(4) // 30
	time.Sleep(5 * timeUnit)
	session.ProfProcessSelectEnd(7) // 35
	time.Sleep(5 * timeUnit)
	session.ReportProcessIdle(4) // 40
	time.Sleep(5 * timeUnit)
	session.ReportProcessIdle(7) // 45
	time.Sleep(5 * timeUnit)
	session.ReportProcessBusy(7) // 50
	time.Sleep(5 * timeUnit)
	session.ReportProcessBusy(4) // 55
	time.Sleep(5 * timeUnit)
	profs = session.Profs() // 60

	fmt.Sscanf(profs, "n[%d/%d/%d] c[%d/%d/%d] p[%d/%d/%d]",
		&xx, &xx, &xx, &xx, &xx, &xx, &pb, &ps, &pi)

	t.Log(profs)
	t.Log(nb, ns, ni, cb, cs, ci, pb, ps, pi)

	assert.Equal(t, 100, nb+ns+ni)
	assert.Equal(t, 100, cb+cs+ci)
	assert.Equal(t, 100, pb+ps+pi)

	assert.InDelta(t, 60, nb, delta)
	assert.InDelta(t, 25, ns, delta)
	assert.InDelta(t, 15, ni, delta)

	assert.InDelta(t, 21, cb, delta)
	assert.InDelta(t, 42, cs, delta)
	assert.InDelta(t, 37, ci, delta)

	assert.InDelta(t, 55, pb, delta)
	assert.InDelta(t, 25, ps, delta)
	assert.InDelta(t, 20, pi, delta)

}

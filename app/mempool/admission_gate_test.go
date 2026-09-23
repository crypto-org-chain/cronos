package mempool

import (
	"strconv"
	"sync"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	gateWait    = 5 * time.Second
	pathInsert  = "insert"
	pathCheckTx = "checktx"
)

type AdmissionGateTestSuite struct {
	suite.Suite
}

func TestAdmissionGateTestSuite(t *testing.T) {
	suite.Run(t, new(AdmissionGateTestSuite))
}

// gateFixture is a Manager whose first RunTx blocks until release is closed,
// so admissions can be parked deterministically inside and behind the mutex.
type gateFixture struct {
	a       *Manager
	runner  *stubRunner
	release chan struct{}
}

func newGateFixture(maxInflight int) *gateFixture {
	f := &gateFixture{release: make(chan struct{})}
	var once sync.Once
	f.runner = &stubRunner{runTx: func([]byte) error {
		once.Do(func() { <-f.release })
		return nil
	}}
	f.a = newManager(f.runner, nil, noopEncoder, nil)
	f.a.SetAdmissionMaxInflight(maxInflight)
	return f
}

// admitVia runs one admission through the named path and returns its ABCI code.
func (f *gateFixture) admitVia(path string, tx []byte) uint32 {
	switch path {
	case pathInsert:
		resp, _ := f.a.InsertTxHandler()(&abci.RequestInsertTx{Tx: tx})
		return resp.Code
	case pathCheckTx:
		runTx := func(bz []byte, _ sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
			gas, _, events, err := f.runner.RunTx(sdk.ExecModeCheck, bz, nil, -1, nil, nil)
			return gas, &sdk.Result{}, events, err
		}
		resp, _ := f.a.CheckTxHandler()(runTx, &abci.RequestCheckTx{Tx: tx})
		return resp.Code
	default:
		panic("unknown path " + path)
	}
}

// startAdmits launches n concurrent admissions and returns a channel of their codes.
func (f *gateFixture) startAdmits(path string, n int) <-chan uint32 {
	codes := make(chan uint32, n)
	for i := range n {
		go func(i int) {
			codes <- f.admitVia(path, []byte(path+":"+strconv.Itoa(i)))
		}(i)
	}
	return codes
}

func (s *AdmissionGateTestSuite) waitInflight(f *gateFixture, n int) {
	s.Require().Eventually(func() bool { return f.a.gate.inflight.Load() == int64(n) }, gateWait, time.Millisecond)
}

func (s *AdmissionGateTestSuite) collect(codes <-chan uint32, n int) []uint32 {
	got := make([]uint32, 0, n)
	for range n {
		select {
		case c := <-codes:
			got = append(got, c)
		case <-time.After(gateWait):
			s.Require().FailNow("admissions did not complete", "got %d of %d", len(got), n)
		}
	}
	return got
}

func (s *AdmissionGateTestSuite) TestShedWhenQueueFull() {
	testCases := []struct {
		name        string
		path        string
		maxInflight int
		occupied    int
		wantShed    bool
	}{
		{"insert shed at cap", pathInsert, 2, 2, true},
		{"checktx shed at cap", pathCheckTx, 2, 2, true},
		{"insert admitted below cap", pathInsert, 3, 2, false},
		{"unbounded never sheds", pathInsert, 0, 16, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			f := newGateFixture(tc.maxInflight)
			occupied := f.startAdmits(tc.path, tc.occupied)
			s.waitInflight(f, tc.occupied)
			callsBefore := f.runner.calls.Load()

			probe := make(chan uint32, 1)
			go func() { probe <- f.admitVia(tc.path, []byte("probe")) }()

			if tc.wantShed {
				select {
				case code := <-probe:
					s.Equal(abci.CodeTypeRetry, code)
				case <-time.After(gateWait):
					s.FailNow("shed admission should return immediately")
				}
				s.Equal(callsBefore, f.runner.calls.Load(), "shed tx must not reach RunTx")
				s.Equal(int64(tc.occupied), f.a.gate.inflight.Load(), "shed tx must not hold a slot")
			}

			close(f.release)
			for _, code := range s.collect(occupied, tc.occupied) {
				s.Equal(abci.CodeTypeOK, code, "queued admissions are never shed once admitted")
			}
			if !tc.wantShed {
				s.Equal(abci.CodeTypeOK, <-probe)
			}
			s.waitInflight(f, 0)
			s.Equal(abci.CodeTypeOK, f.admitVia(tc.path, []byte("after")), "slots are released once admissions finish")
		})
	}
}

func (s *AdmissionGateTestSuite) TestCommitPriorityOverQueuedAdmissions() {
	const queued = 32
	testCases := []struct {
		name string
		path string
	}{
		{"insert path yields to Commit", pathInsert},
		{"checktx path yields to Commit", pathCheckTx},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			f := newGateFixture(0)
			// One admission inside RunTx, the rest queued behind it on the mutex.
			codes := f.startAdmits(tc.path, queued+1)
			s.waitInflight(f, queued+1)
			s.Require().Eventually(func() bool { return f.runner.calls.Load() == 1 }, gateWait, time.Millisecond)

			acquired := make(chan int64, 1)
			unlocked := make(chan struct{})
			go func() {
				unlock := f.a.LockForCommit()
				acquired <- f.runner.calls.Load()
				<-unlocked
				unlock()
			}()
			s.Require().Eventually(func() bool { return f.a.gate.commit.Load() != nil }, gateWait, time.Millisecond)

			select {
			case <-acquired:
				s.FailNow("Commit acquired the mutex while a RunTx was in flight")
			case <-time.After(20 * time.Millisecond):
			}

			close(f.release)
			select {
			case calls := <-acquired:
				s.Equal(int64(1), calls, "queued admissions ran ahead of Commit")
			case <-time.After(gateWait):
				s.FailNow("Commit did not acquire the mutex after the in-flight RunTx finished")
			}
			s.Equal(int64(1), f.runner.calls.Load(), "admissions ran while Commit held the mutex")

			close(unlocked)
			for _, code := range s.collect(codes, queued+1) {
				s.Equal(abci.CodeTypeOK, code, "yielding to Commit must not reject the tx")
			}
			s.Equal(int64(queued+1), f.runner.calls.Load())
			s.waitInflight(f, 0)
		})
	}
}

func (s *AdmissionGateTestSuite) TestRecheckYieldsToCommit() {
	f := newGateFixture(0)
	// Recheck holds the mutex per RunTx; park it inside the first one.
	txs := make([]sdk.Tx, 8)
	for i := range txs {
		txs[i] = &ptrTx{id: i}
	}
	done := make(chan struct{})
	go func() {
		f.a.runRecheck(txs)
		close(done)
	}()
	s.Require().Eventually(func() bool { return f.runner.calls.Load() == 1 }, gateWait, time.Millisecond)

	acquired := make(chan int64, 1)
	go func() {
		unlock := f.a.LockForCommit()
		acquired <- f.runner.calls.Load()
		unlock()
	}()
	s.Require().Eventually(func() bool { return f.a.gate.commit.Load() != nil }, gateWait, time.Millisecond)

	close(f.release)
	select {
	case calls := <-acquired:
		s.Equal(int64(1), calls, "recheck re-took the mutex ahead of Commit")
	case <-time.After(gateWait):
		s.FailNow("Commit did not acquire the mutex")
	}
	select {
	case <-done:
	case <-time.After(gateWait):
		s.FailNow("recheck did not finish after Commit released the mutex")
	}
	s.Equal(int64(len(txs)), f.runner.calls.Load())
}

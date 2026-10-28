package mempool

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/suite"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	gateWait      = 5 * time.Second
	pathGossip    = "gossip"
	pathRPCInsert = "rpc-insert"
	pathCheckTx   = "checktx"
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

func newGateFixture(rpcMaxInflight int) *gateFixture {
	f := &gateFixture{release: make(chan struct{})}
	var once sync.Once
	f.runner = &stubRunner{runTx: func([]byte) error {
		once.Do(func() { <-f.release })
		return nil
	}}
	f.a = newManager(f.runner, nil, noopEncoder, nil)
	f.a.SetRPCMaxInflight(rpcMaxInflight)
	return f
}

func (f *gateFixture) admitVia(path string, tx []byte) uint32 {
	code, _ := f.admitWithLog(path, tx)
	return code
}

// admitWithLog returns the ABCI code and log; gossip responses carry no log.
func (f *gateFixture) admitWithLog(path string, tx []byte) (uint32, string) {
	switch path {
	case pathGossip:
		resp, _ := f.a.InsertTxHandler()(&abci.RequestInsertTx{Tx: tx})
		return resp.Code, ""
	case pathRPCInsert:
		resp, _ := f.a.InsertTx(tx)
		return resp.Code, resp.RawLog
	case pathCheckTx:
		runTx := func(bz []byte, _ sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
			gas, _, events, err := f.runner.RunTx(sdk.ExecModeCheck, bz, nil, -1, nil, nil)
			return gas, &sdk.Result{}, events, err
		}
		resp, _ := f.a.CheckTxHandler()(runTx, &abci.RequestCheckTx{Tx: tx})
		return resp.Code, resp.Log
	default:
		panic("unknown path " + path)
	}
}

func (f *gateFixture) startAdmits(path string, n int) <-chan uint32 {
	codes := make(chan uint32, n)
	for i := range n {
		go func(i int) {
			codes <- f.admitVia(path, []byte(path+":"+strconv.Itoa(i)))
		}(i)
	}
	return codes
}

func (s *AdmissionGateTestSuite) waitRPCInflight(f *gateFixture, n int) {
	s.Require().Eventually(func() bool { return f.a.rpcTxLimit.n.Load() == int64(n) }, gateWait, time.Millisecond)
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

func (s *AdmissionGateTestSuite) TestInflightLimit() {
	testCases := []struct {
		name     string
		max      int64
		held     int
		wantMore bool
	}{
		{"unlimited", 0, 100, true},
		{"below limit", 3, 2, true},
		{"at limit", 3, 3, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			var l inflightLimit
			l.max.Store(tc.max)
			for range tc.held {
				s.Require().True(l.tryAcquire())
			}
			s.Equal(tc.wantMore, l.tryAcquire())
			if !tc.wantMore {
				s.Equal(int64(tc.held), l.n.Load(), "a refused acquire must not hold a slot")
				l.release()
				s.True(l.tryAcquire(), "release frees a slot")
			}
		})
	}
}

func (s *AdmissionGateTestSuite) TestInflightLimitConcurrentNeverExceedsMax() {
	const (
		limit   = 4
		workers = 64
		rounds  = 500
	)
	var (
		l    inflightLimit
		held atomic.Int64
		peak atomic.Int64
		wg   sync.WaitGroup
	)
	l.max.Store(limit)
	for range workers {
		wg.Go(func() {
			for range rounds {
				if !l.tryAcquire() {
					continue
				}
				n := held.Add(1)
				for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
				}
				held.Add(-1)
				l.release()
			}
		})
	}
	wg.Wait()
	s.LessOrEqual(peak.Load(), int64(limit))
	s.Equal(int64(0), l.n.Load())
}

func (s *AdmissionGateTestSuite) TestRPCShedAtLimit() {
	testCases := []struct {
		name           string
		occupyPath     string
		probePath      string
		rpcMaxInflight int
		occupied       int
		wantShed       bool
	}{
		{"rpc insert shed at limit", pathRPCInsert, pathRPCInsert, 2, 2, true},
		{"checktx shed at limit", pathCheckTx, pathCheckTx, 2, 2, true},
		{"rpc paths share one limit", pathCheckTx, pathRPCInsert, 2, 2, true},
		{"admitted below limit", pathRPCInsert, pathRPCInsert, 3, 2, false},
		{"unlimited never sheds", pathCheckTx, pathCheckTx, 0, 16, false},
		{"gossip not limited", pathRPCInsert, pathGossip, 2, 2, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			f := newGateFixture(tc.rpcMaxInflight)
			occupied := f.startAdmits(tc.occupyPath, tc.occupied)
			s.waitRPCInflight(f, tc.occupied)
			s.Require().Eventually(func() bool { return f.runner.calls.Load() == 1 }, gateWait, time.Millisecond)
			callsBefore := f.runner.calls.Load()

			probe := make(chan uint32, 1)
			probeLog := make(chan string, 1)
			go func() {
				code, log := f.admitWithLog(tc.probePath, []byte("probe"))
				probeLog <- log
				probe <- code
			}()

			if tc.wantShed {
				select {
				case code := <-probe:
					s.Equal(abci.CodeTypeRetry, code)
					s.Equal(rpcTxLimitFullLog, <-probeLog)
				case <-time.After(gateWait):
					s.FailNow("shed admission should return immediately")
				}
				s.Equal(callsBefore, f.runner.calls.Load(), "shed tx must not reach RunTx")
				s.Equal(int64(tc.occupied), f.a.rpcTxLimit.n.Load(), "shed tx must not hold a slot")
			} else {
				// A shed returns at once; an admitted probe waits behind the blocked RunTx.
				select {
				case code := <-probe:
					s.FailNow("probe returned while slots were full", "code %d", code)
				case <-time.After(50 * time.Millisecond):
				}
			}

			close(f.release)
			for _, code := range s.collect(occupied, tc.occupied) {
				s.Equal(abci.CodeTypeOK, code, "admissions already holding a slot are never shed")
			}
			if !tc.wantShed {
				s.Equal(abci.CodeTypeOK, s.collect(probe, 1)[0])
			}
			s.waitRPCInflight(f, 0)
			s.Equal(abci.CodeTypeOK, f.admitVia(tc.probePath, []byte("after")), "slots are released once admissions finish")
		})
	}
}

func (s *AdmissionGateTestSuite) TestCommitPriorityOverQueuedAdmissions() {
	const queued = 32
	testCases := []struct {
		name string
		path string
	}{
		{"rpc insert yields to Commit", pathRPCInsert},
		{"checktx yields to Commit", pathCheckTx},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			f := newGateFixture(0)
			// One admission inside RunTx, the rest queued behind it on the mutex.
			codes := f.startAdmits(tc.path, queued+1)
			s.waitRPCInflight(f, queued+1)
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
			s.waitRPCInflight(f, 0)
		})
	}
}

func (s *AdmissionGateTestSuite) TestRecheckYieldsToCommit() {
	f := newGateFixture(0)
	// Recheck takes the mutex per RunTx; park it inside the first one.
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

// exclusiveRunner fails the test if two RunTx calls, or a RunTx and Commit, overlap.
type exclusiveRunner struct {
	holders atomic.Int32
	overlap atomic.Bool
}

func (r *exclusiveRunner) enter() {
	if r.holders.Add(1) != 1 {
		r.overlap.Store(true)
	}
}

func (r *exclusiveRunner) exit() { r.holders.Add(-1) }

func (r *exclusiveRunner) RunTx(sdk.ExecMode, []byte, sdk.Tx, int, storetypes.MultiStore, map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	r.enter()
	defer r.exit()
	time.Sleep(time.Microsecond)
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

func (s *AdmissionGateTestSuite) TestConcurrentAdmissionsAndCommits() {
	const (
		admitters = 64
		perAdmit  = 50
		commits   = 200
	)
	runner := &exclusiveRunner{}
	a := newManager(runner, nil, noopEncoder, nil)
	insert := a.InsertTxHandler()

	var wg sync.WaitGroup
	for g := range admitters {
		wg.Go(func() {
			for i := range perAdmit {
				tx := []byte(strconv.Itoa(g) + ":" + strconv.Itoa(i))
				if g%2 == 0 {
					insert(&abci.RequestInsertTx{Tx: tx}) //nolint:errcheck
				} else {
					a.InsertTx(tx) //nolint:errcheck
				}
			}
		})
	}
	wg.Go(func() {
		for range commits {
			unlock := a.LockForCommit()
			runner.enter()
			runner.exit()
			unlock()
		}
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		s.FailNow("admissions and commits deadlocked")
	}
	s.False(runner.overlap.Load(), "RunTx overlapped another RunTx or Commit")
	s.Equal(int64(0), a.rpcTxLimit.n.Load())
}

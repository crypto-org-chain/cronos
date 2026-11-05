package mempool

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	antecache "github.com/evmos/ethermint/ante/cache"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// recheckScheduler is the recheck half of the app mempool: it stages the senders
// each block touched, picks which pending txs to re-validate, and evicts the ones
// the new state invalidated.
type recheckScheduler struct {
	exec   *txExec
	mpool  sdkmempool.Mempool
	signer sdkmempool.SignerExtractionAdapter
	// maxRecheckBatch softly caps RunTx(ReCheck) calls per cycle, splitting only
	// between signer groups. 0 = unlimited.
	maxRecheckBatch int
	// stagingMu guards the staging fields, apart from the gate so FinalizeBlock
	// staging never waits on a recheck chunk.
	stagingMu sync.Mutex
	// recheckSenders accumulates senders awaiting recheck across blocks until drained.
	recheckSenders map[string]struct{}
	// deferred front-loads capRecheckGroups' overflow next cycle. Ordering hint
	// only: re-staging those senders is what guarantees a re-pick after a fee bump.
	deferred            []sdk.Tx
	lastCommittedHeight int64
	// arrival is the height each pooled tx was first seen, for TTL eviction; guarded by recheckMu.
	arrival map[sdk.Tx]int64
	// ttlNumBlocks evicts txs older than this many blocks by arrival height; 0 = off.
	ttlNumBlocks int64

	recheckMu sync.Mutex // serializes RecheckTxs; lock order recheckMu > exec.gate > stagingMu
	// worker is zero in tests (newManager), so triggerRecheck runs inline.
	worker recheckWorker
	// recheckDisabled mirrors mempool.recheck=false, TTL and expiry eviction included.
	recheckDisabled bool
	// anteCache is ethermint's (sender, nonce) cache. evict drops a tx's entries so
	// a resubmit after a cascade or TTL eviction can't skip nonce verification.
	anteCache *antecache.AnteCache
}

// recheckDecodingEnabled reports whether sender decoding/bookkeeping should run.
func (s *recheckScheduler) recheckDecodingEnabled() bool {
	return !s.recheckDisabled && s.signer != nil && s.exec.decoder != nil
}

// stageSkippedSenders stages the senders of proposal-gate-rejected txs, leaving the height.
func (s *recheckScheduler) stageSkippedSenders(txs [][]byte) {
	if !s.recheckDecodingEnabled() || len(txs) == 0 {
		return
	}
	senders := make(map[string]struct{}, len(txs))
	for _, bz := range txs {
		tx, err := s.exec.decoder(bz)
		if err != nil {
			continue
		}
		for _, sg := range s.signers(tx) {
			senders[sg] = struct{}{}
		}
	}
	if len(senders) == 0 {
		return
	}
	s.stagingMu.Lock()
	s.mergeRecheckSenders(senders)
	s.stagingMu.Unlock()
}

// mergeRecheckSenders merges rather than overwrites, so undrained senders survive. Caller holds stagingMu.
func (s *recheckScheduler) mergeRecheckSenders(senders map[string]struct{}) {
	if s.recheckSenders == nil {
		s.recheckSenders = senders
	} else {
		maps.Copy(s.recheckSenders, senders)
	}
}

// stageRecheckSenders stages the block's height and senders for the next recheck.
func (s *recheckScheduler) stageRecheckSenders(height int64, txs [][]byte) {
	s.exec.pending.invalidate()

	// Decode unlocked; publish height and senders together.
	var senders map[string]struct{}
	if s.recheckDecodingEnabled() {
		senders = make(map[string]struct{}, len(txs))
		for _, bz := range txs {
			tx, err := s.exec.decoder(bz)
			if err != nil {
				continue // non-sdk txs (e.g. vote extensions) have no mempool entry
			}
			for _, sg := range s.signers(tx) {
				senders[sg] = struct{}{}
			}
		}
	}

	s.stagingMu.Lock()
	s.lastCommittedHeight = height
	s.mergeRecheckSenders(senders)
	s.stagingMu.Unlock()
}

// triggerRecheck schedules an async recheck. Call only from App.Commit.
func (s *recheckScheduler) triggerRecheck() {
	if s.worker.trigger == nil {
		s.RecheckTxs()
		return
	}
	s.worker.recheck()
}

// RecheckTxs evicts pool txs invalidated by the last block.
func (s *recheckScheduler) RecheckTxs() {
	if s.mpool == nil || s.recheckDisabled {
		return
	}
	s.recheckMu.Lock() // lock order: see the recheckMu field comment
	defer s.recheckMu.Unlock()
	committed, recheckSenders, height, deferred := s.pinAndDrain()
	// Nothing to scan before the first block.
	if len(recheckSenders) == 0 && len(deferred) == 0 && height == 0 {
		return
	}
	// A panic here would crash the node and, on the worker, skip close(ready).
	defer func() {
		if r := recover(); r != nil {
			s.restoreStaging(recheckSenders, deferred)
			s.exec.pending.invalidate() // evictions before the panic must reach pending queries
			logRecoveredPanic(s.exec.logger, panicPathRecheck, r)
		}
	}()

	snapshot := PoolSnapshot(context.Background(), s.mpool)
	candidates := s.selectTxs(snapshot, recheckSenders, height, deferred)
	s.runRecheck(s.capRecheckGroups(s.groupCandidates(candidates)), committed)

	s.exec.pending.invalidate()
	telemetry.SetGauge(float32(s.mpool.CountTx()), "cronos", "mempool", "pool", "size")
}

// pinAndDrain reads the committed height and drains staging under one gate
// hold, or a Commit between them goes undetected.
func (s *recheckScheduler) pinAndDrain() (committed int64, recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) {
	s.exec.gate.lock()
	defer s.exec.gate.unlock()
	recheckSenders, height, deferred = s.drainStaging()
	return s.exec.committedHeight(), recheckSenders, height, deferred
}

// drainStaging atomically takes and clears the staged senders, height, and carry.
func (s *recheckScheduler) drainStaging() (recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()
	recheckSenders, height, deferred = s.recheckSenders, s.lastCommittedHeight, s.deferred
	s.recheckSenders = nil
	s.deferred = nil
	return recheckSenders, height, deferred
}

// restoreStaging puts drained work back after an aborted cycle so the next cycle retries it.
func (s *recheckScheduler) restoreStaging(recheckSenders map[string]struct{}, deferred []sdk.Tx) {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()
	s.mergeRecheckSenders(recheckSenders)
	if s.deferred == nil {
		s.deferred = deferred
	}
}

// selectTxs evicts expired txs and returns the candidates to recheck.
func (s *recheckScheduler) selectTxs(snapshot []sdk.Tx, recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) []sdk.Tx {
	// deferredLive: carried-over tx -> still in pool. Sized to the small carry; nil if none.
	var deferredLive map[sdk.Tx]bool
	if len(deferred) > 0 {
		deferredLive = make(map[sdk.Tx]bool, len(deferred))
		for _, tx := range deferred {
			deferredLive[tx] = false
		}
	}

	var expiredEvicted, ttlEvicted int
	// Rebuild arrival from this cycle's snapshot so txs gone from the pool fall out.
	var newArrival map[sdk.Tx]int64
	if s.ttlNumBlocks > 0 {
		newArrival = make(map[sdk.Tx]int64, len(snapshot))
	}

	// Pass 1: evictions. An evicted tx's sender is rechecked, since its higher nonces are now gapped.
	var evictedSet map[sdk.Tx]struct{} // nil until first eviction; nil-map read is safe
	// Wall clock is fine here: eviction is node-local, and the ante re-checks a
	// tx's timeout against block time before it can land in a block.
	now := time.Now()
	for _, tx := range snapshot {
		evictedPoison := s.guardRecheckTx(tx, func() {
			if txTimedout(tx, height, now) {
				evictedSet, recheckSenders = s.evictForRecheck(tx, evictedSet, recheckSenders)
				expiredEvicted++
				return
			}
			if s.ttlNumBlocks > 0 {
				arrived, expired := txTTLExpired(s.arrival, tx, height, s.ttlNumBlocks)
				if expired {
					evictedSet, recheckSenders = s.evictForRecheck(tx, evictedSet, recheckSenders)
					ttlEvicted++
					return
				}
				newArrival[tx] = arrived
			}
		})
		if evictedPoison {
			evictedSet = markEvicted(evictedSet, tx)
		}
	}
	s.arrival = newArrival
	if expiredEvicted > 0 {
		telemetry.IncrCounter(float32(expiredEvicted), "cronos", "mempool", "recheck", "expired")
	}
	if ttlEvicted > 0 {
		telemetry.IncrCounter(float32(ttlEvicted), "cronos", "mempool", "recheck", "ttl_expired")
	}

	// Pass 2: candidate selection over surviving (non-evicted) txs.
	var candidates []sdk.Tx
	for _, tx := range snapshot {
		if _, wasEvicted := evictedSet[tx]; wasEvicted {
			continue
		}
		if deferredLive != nil {
			if _, isDeferred := deferredLive[tx]; isDeferred {
				deferredLive[tx] = true
			}
		}
		if len(recheckSenders) == 0 {
			continue
		}
		var sigs []string
		if s.guardRecheckTx(tx, func() { sigs = s.signers(tx) }) {
			delete(deferredLive, tx) // evicted, so don't recheck it as carry
			continue
		}
		for _, sg := range sigs {
			if _, ok := recheckSenders[sg]; ok {
				candidates = append(candidates, tx)
				break
			}
		}
	}

	if len(deferred) == 0 {
		return candidates
	}
	// Front-load the carry, or capRecheckGroups re-takes the same priority prefix and starves the tail.
	ordered := make([]sdk.Tx, 0, len(deferred)+len(candidates))
	for _, tx := range deferred {
		if deferredLive[tx] {
			ordered = append(ordered, tx) // skip txs included/evicted since carry
		}
	}
	for _, tx := range candidates {
		if _, isDeferred := deferredLive[tx]; isDeferred {
			continue // this tx is already in the deferred carry; avoid double recheck
		}
		ordered = append(ordered, tx)
	}
	return ordered
}

// evictForRecheck evicts tx and stages its signers; maps are allocated lazily.
func (s *recheckScheduler) evictForRecheck(tx sdk.Tx, evictedSet map[sdk.Tx]struct{}, recheckSenders map[string]struct{}) (map[sdk.Tx]struct{}, map[string]struct{}) {
	s.evict(tx)
	evictedSet = markEvicted(evictedSet, tx)
	sigs := s.signers(tx)
	if len(sigs) > 0 && recheckSenders == nil {
		recheckSenders = make(map[string]struct{})
	}
	for _, sg := range sigs {
		recheckSenders[sg] = struct{}{}
	}
	return evictedSet, recheckSenders
}

func markEvicted(evictedSet map[sdk.Tx]struct{}, tx sdk.Tx) map[sdk.Tx]struct{} {
	if evictedSet == nil {
		evictedSet = make(map[sdk.Tx]struct{})
	}
	evictedSet[tx] = struct{}{}
	return evictedSet
}

// guardRecheckTx evicts tx if step panics, so one poison tx can't stall every cycle.
func (s *recheckScheduler) guardRecheckTx(tx sdk.Tx, step func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(s.exec.logger, panicPathRecheckTx, r)
			s.evictPoisonTx(tx)
			panicked = true
		}
	}()
	step()
	return false
}

// evictPoisonTx may panic on the same tx methods; the tx then stays pooled and is guarded next cycle.
func (s *recheckScheduler) evictPoisonTx(tx sdk.Tx) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(s.exec.logger, panicPathRecheckTx, r, "evict_failed", true)
		}
	}()
	s.evict(tx)
}

// capRecheckGroups carries whole groups past maxRecheckBatch into deferred; the
// first group always runs. The carried senders are re-staged too: deferred is
// keyed on tx identity, so a fee bump would drop out of it and its tail would
// fail as a false gap.
func (s *recheckScheduler) capRecheckGroups(groups []recheckGroup) []recheckGroup {
	if s.maxRecheckBatch <= 0 {
		return groups
	}
	count := 0
	for i, g := range groups {
		if count > 0 && count+len(g.txs) > s.maxRecheckBatch {
			carry := unreachedTxs(groups[i:])
			s.stagingMu.Lock()
			s.deferred = carry
			s.stagingMu.Unlock()
			s.recoverSenders(carry)
			return groups[:i]
		}
		count += len(g.txs)
	}
	return groups
}

// recheckCandidate keeps the signer nonce, which tells a gap from a stale nonce.
type recheckCandidate struct {
	tx  sdk.Tx
	bz  []byte
	seq uint64
}

// recheckGroup holds one signer's candidates sorted by seq. cascadable is false
// unless they are that signer's whole ascending-nonce view, since the cascade
// rule assumes the next expected nonce is lastOK+1.
type recheckGroup struct {
	key        string
	txs        []recheckCandidate
	cascadable bool
}

// runRecheck runs the groups one at a time. Once a Commit moves past
// pinnedHeight it re-stages the rest: run on the reset checkState, they
// would re-apply nonces that the Commit's own recheck then rejects as stale.
func (s *recheckScheduler) runRecheck(groups []recheckGroup, pinnedHeight int64) {
	var evicted, cascaded, restaged int
	for i, g := range groups {
		e, c, r := s.runGroup(g, pinnedHeight)
		evicted += e
		cascaded += c
		if r > 0 {
			rest := unreachedTxs(groups[i+1:])
			s.recoverSenders(rest)
			restaged = r + len(rest)
			break
		}
	}
	if evicted > 0 {
		telemetry.IncrCounter(float32(evicted), "cronos", "mempool", "recheck", "evicted")
	}
	if cascaded > 0 {
		telemetry.IncrCounter(float32(cascaded), "cronos", "mempool", "recheck", "cascade_evicted")
	}
	if restaged > 0 {
		telemetry.IncrCounter(float32(restaged), "cronos", "mempool", "recheck", "restaged")
	}
}

// groupCandidates buckets candidates by first signer, the one the pool orders
// by, keeping first-appearance (priority) order across groups for
// capRecheckGroups. Each group is sorted by seq since the carry can arrive out
// of order. Encoding happens here, outside the gate.
func (s *recheckScheduler) groupCandidates(candidates []sdk.Tx) []recheckGroup {
	var groups []recheckGroup
	index := make(map[string]int)
	// coSigned: a multi-signer tx can fill a co-signer's nonce from another group,
	// and a batch eth tx reports only its first nonce, so neither gap is provable.
	var coSigned map[string]struct{}
	markCoSigned := func(signers []string) {
		if len(signers) == 0 {
			return
		}
		if coSigned == nil {
			coSigned = make(map[string]struct{})
		}
		for _, sg := range signers {
			coSigned[sg] = struct{}{}
		}
	}
	for _, tx := range candidates {
		var g *recheckGroup
		evictedPoison := s.guardRecheckTx(tx, func() {
			key, seq, known, multiSigner := s.firstSigner(tx)
			gi, seen := index[key]
			if !seen {
				groups = append(groups, recheckGroup{key: key, cascadable: known})
				gi = len(groups) - 1
				index[key] = gi
			}
			g = &groups[gi]
			if multiSigner {
				markCoSigned(s.signers(tx))
			}
			markCoSigned(batchEthSenders(tx))
			if unordered, ok := tx.(sdk.TxWithUnordered); ok && unordered.GetUnordered() {
				g.cascadable = false // keyed by timeout, not sequence
			}
			bz, _, err := EncodeTx(s.exec.encCache, s.exec.txEncoder, tx)
			if err != nil {
				g.cascadable = false // a dropped tx may fill a gap
				return
			}
			g.txs = append(g.txs, recheckCandidate{tx: tx, bz: bz, seq: seq})
		})
		if evictedPoison && g != nil {
			g.cascadable = false // a dropped tx may fill a gap
		}
	}
	for i := range groups {
		g := &groups[i]
		if _, ok := coSigned[g.key]; ok {
			g.cascadable = false
		}
		slices.SortStableFunc(g.txs, func(a, b recheckCandidate) int {
			return cmp.Compare(a.seq, b.seq)
		})
		// A duplicate seq sorts adjacent.
		for j := 1; j < len(g.txs); j++ {
			if g.txs[j].seq <= g.txs[j-1].seq {
				g.cascadable = false
				break
			}
		}
	}
	return groups
}

// batchEthSenders returns each MsgEthereumTx sender of a batch eth tx, nil otherwise.
func batchEthSenders(tx sdk.Tx) []string {
	msgs := tx.GetMsgs()
	if len(msgs) < 2 {
		return nil
	}
	var senders []string
	for _, msg := range msgs {
		if m, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			senders = append(senders, m.GetFrom().String())
		}
	}
	if len(senders) < 2 {
		return nil
	}
	return senders
}

// recheckChunkSize bounds one gate hold, so Commit never waits out a sender's whole queue.
const recheckChunkSize = 256

// runGroup re-validates one signer's candidates in bounded chunks, so a deep
// queue can't hold the gate indefinitely. A same-sender admission may land
// between chunks; a Commit stops the group, re-staging the candidates left
// (counted by restaged).
func (s *recheckScheduler) runGroup(g recheckGroup, pinnedHeight int64) (evicted, cascaded, restaged int) {
	for start := 0; start < len(g.txs); start += recheckChunkSize {
		e, c, ok := s.runChunkLocked(g, start, min(start+recheckChunkSize, len(g.txs)), pinnedHeight)
		evicted += e
		cascaded += c
		if !ok {
			s.recoverSenders(candidateTxs(g.txs[start:]))
			return evicted, cascaded, len(g.txs) - start
		}
	}
	return evicted, cascaded, 0
}

// runChunkLocked runs g.txs[start:end] under one gate hold, evicting each
// failure. A nonce error strictly above a nonce accepted in this same hold is a
// proven gap, since nothing can fill it while the gate is held, so the rest of
// the chunk is cascade-evicted without a RunTx each. Nothing carries across
// chunks: once the gate is released an admission can move the nonce, making a
// nonce error stale rather than a gap. ok is false, with nothing run, once a
// Commit has moved past pinnedHeight.
func (s *recheckScheduler) runChunkLocked(g recheckGroup, start, end int, pinnedHeight int64) (evicted, cascaded int, ok bool) {
	s.exec.gate.lock()
	defer s.exec.gate.unlock()
	if s.exec.committedHeight() != pinnedHeight {
		return 0, 0, false
	}
	var lastOK uint64
	accepted := false
	for i := start; i < end; i++ {
		c := g.txs[i]
		var err error
		if s.guardRecheckTx(c.tx, func() {
			if _, _, _, err = s.exec.runTxLocked(sdk.ExecModeReCheck, c.bz, c.tx); err != nil {
				s.evict(c.tx)
			}
		}) {
			continue // like a non-nonce failure, a panicking tx can't start a cascade
		}
		if err == nil {
			lastOK, accepted = c.seq, true
			continue
		}
		evicted++
		if g.cascadable && accepted && c.seq > lastOK+1 && isNonceErr(err) {
			for _, rest := range g.txs[i+1 : end] {
				if !s.guardRecheckTx(rest.tx, func() { s.evict(rest.tx) }) {
					cascaded++
				}
			}
			return evicted, cascaded, true
		}
	}
	return evicted, cascaded, true
}

// isNonceErr matches both ante paths: cosmos sig verification reports
// ErrWrongSequence, the EVM nonce check reports ErrInvalidSequence.
func isNonceErr(err error) bool {
	return errorsmod.IsOf(err, sdkerrors.ErrWrongSequence, sdkerrors.ErrInvalidSequence)
}

func unreachedTxs(groups []recheckGroup) []sdk.Tx {
	var txs []sdk.Tx
	for _, g := range groups {
		txs = append(txs, candidateTxs(g.txs)...)
	}
	return txs
}

func candidateTxs(cs []recheckCandidate) []sdk.Tx {
	txs := make([]sdk.Tx, len(cs))
	for i, c := range cs {
		txs[i] = c.tx
	}
	return txs
}

// recoverSenders re-stages txs' senders, leaving deferred alone since
// capRecheckGroups may have set it this cycle. A tx with no extractable signer
// is dropped and waits for TTL.
func (s *recheckScheduler) recoverSenders(txs []sdk.Tx) {
	if len(txs) == 0 {
		return
	}
	senders := make(map[string]struct{})
	for _, tx := range txs {
		for _, sg := range s.signers(tx) {
			senders[sg] = struct{}{}
		}
	}
	if len(senders) == 0 {
		return
	}
	s.stagingMu.Lock()
	s.mergeRecheckSenders(senders)
	s.stagingMu.Unlock()
}

// txTimedout reports whether tx's own timeout height or timestamp has passed.
func txTimedout(tx sdk.Tx, height int64, now time.Time) bool {
	if t, ok := tx.(sdk.TxWithTimeoutHeight); ok {
		th := t.GetTimeoutHeight()
		if th > 0 && uint64(height) >= th {
			return true
		}
	}
	if t, ok := tx.(sdk.TxWithTimeoutTimeStamp); ok {
		ts := t.GetTimeoutTimeStamp()
		if !ts.IsZero() && !now.Before(ts) {
			return true
		}
	}
	return false
}

// txTTLExpired reports whether tx has aged past ttlNumBlocks since first seen.
func txTTLExpired(arrival map[sdk.Tx]int64, tx sdk.Tx, height, ttlNumBlocks int64) (int64, bool) {
	arrived, ok := arrival[tx]
	if !ok {
		arrived = height
	}
	return arrived, height-arrived >= ttlNumBlocks
}

// evict removes tx from the pool and both caches, so no cache outlives the pool entry.
func (s *recheckScheduler) evict(tx sdk.Tx) {
	_ = s.mpool.Remove(tx)
	s.exec.encCache.Evict(tx)
	s.evictAnteCache(tx)
}

// evictAnteCache drops one (from, nonce) entry per MsgEthereumTx, as the ante cached them.
func (s *recheckScheduler) evictAnteCache(tx sdk.Tx) {
	if s.anteCache == nil || tx == nil {
		return
	}
	for _, msg := range tx.GetMsgs() {
		ethTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if !ok {
			continue
		}
		asTx := ethTx.AsTransaction()
		if asTx == nil {
			continue
		}
		s.anteCache.Delete(ethTx.GetFrom().String(), asTx.Nonce())
	}
}

// firstSigner returns the signer the pool orders by, its nonce, and whether tx has co-signers.
func (s *recheckScheduler) firstSigner(tx sdk.Tx) (key string, seq uint64, known, multiSigner bool) {
	sigs := s.allSigners(tx)
	if len(sigs) == 0 {
		return "", 0, false, false
	}
	return sigs[0].Signer.String(), sigs[0].Sequence, true, len(sigs) > 1
}

// signers returns every account tx advances a nonce for, including the batch
// eth senders the EVM signer adapter leaves out.
func (s *recheckScheduler) signers(tx sdk.Tx) []string {
	sigs := s.allSigners(tx)
	if len(sigs) == 0 {
		return nil
	}
	keys := make([]string, len(sigs))
	for i, sg := range sigs {
		keys[i] = sg.Signer.String()
	}
	for _, sender := range batchEthSenders(tx) {
		if !slices.Contains(keys, sender) {
			keys = append(keys, sender)
		}
	}
	return keys
}

// allSigners is nil on a missing extractor or a lookup error.
func (s *recheckScheduler) allSigners(tx sdk.Tx) []sdkmempool.SignerData {
	if s.signer == nil {
		return nil
	}
	sigs, err := s.signer.GetSigners(tx)
	if err != nil {
		return nil
	}
	return sigs
}

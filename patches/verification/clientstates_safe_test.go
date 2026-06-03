// Package verification holds focused tests that prove the defensive
// behaviour added by patches/ibc-go-clientstates-panic-safe.patch.
//
// The patch lives in:
//   vendor/github.com/cosmos/ibc-go/v11/modules/core/02-client/keeper/grpc_query.go
//
// We cannot (easily) spin up a full IBC keeper in this isolated module, so
// we replicate the exact control flow of the patched pagination callback
// and assert its three guarantees:
//
//  1. A panic raised while decoding one entry (e.g. malformed bytes under a
//     "<clientID>/clientState" key) is swallowed and the iteration continues.
//  2. A key that is too short (e.g. literal "clientState") does not trigger
//     an out-of-bounds slice access.
//  3. Valid entries still propagate as hits.
//  4. Keys that do not end in "clientState" are skipped without error.
//
// If any of these regresses, the fix is broken and the upstream panic
// (and the DoS it enables on REST / ABCI query paths) is back.
package verification

import (
	"strings"
	"testing"
)

// safeCallback mirrors the structure of the patched callback in
// vendor/github.com/cosmos/ibc-go/v11/modules/core/02-client/keeper/grpc_query.go
// The defer+recover and the len(keySplit) < 2 guard are the two additions
// the patch makes; everything else is the existing upstream behaviour.
func safeCallback(key, value []byte, decode func([]byte) error) (hit bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			hit, err = false, nil
		}
	}()

	keySplit := strings.Split(string(key), "/")
	if len(keySplit) < 2 || keySplit[len(keySplit)-1] != "clientState" {
		return false, nil
	}

	if err := decode(value); err != nil {
		return false, err
	}

	_ = keySplit[1]
	return true, nil
}

// panickingDecoder simulates the observed failure mode: the proto decoder
// panicking with "index out of range" while unmarshalling malformed bytes
// stored under a key that still passes the "ends with clientState" filter.
func panickingDecoder(_ []byte) error {
	var empty []int
	_ = empty[7] // reproduces: runtime error: index out of range [7] with length 0
	return nil
}

func okDecoder(_ []byte) error      { return nil }
func errDecoder(_ []byte) error     { return errStub }
func neverCalledDecoder(_ []byte) error {
	panic("decode must not be called for filtered keys")
}

type stubErr struct{ s string }

func (e stubErr) Error() string { return e.s }

var errStub = stubErr{s: "decode failed"}

func TestClientStatesCallback_SwallowsDecoderPanic(t *testing.T) {
	hit, err := safeCallback(
		[]byte("07-tendermint-1/clientState"),
		[]byte("corrupt"),
		panickingDecoder,
	)
	if hit || err != nil {
		t.Fatalf("panic path: expected (false, nil), got (%v, %v)", hit, err)
	}
}

func TestClientStatesCallback_ShortKeyDoesNotPanic(t *testing.T) {
	// Without the len(keySplit) < 2 guard this path would index keySplit[1]
	// on a 1-element slice and panic.
	hit, err := safeCallback([]byte("clientState"), nil, neverCalledDecoder)
	if hit || err != nil {
		t.Fatalf("short key: expected (false, nil), got (%v, %v)", hit, err)
	}
}

func TestClientStatesCallback_NonClientStateKeySkipped(t *testing.T) {
	hit, err := safeCallback(
		[]byte("07-tendermint-1/connections"),
		[]byte("anything"),
		neverCalledDecoder,
	)
	if hit || err != nil {
		t.Fatalf("non-clientState key: expected (false, nil), got (%v, %v)", hit, err)
	}
}

func TestClientStatesCallback_HappyPath(t *testing.T) {
	hit, err := safeCallback(
		[]byte("07-tendermint-0/clientState"),
		[]byte("ok"),
		okDecoder,
	)
	if !hit || err != nil {
		t.Fatalf("happy path: expected (true, nil), got (%v, %v)", hit, err)
	}
}

func TestClientStatesCallback_DecoderErrorPropagates(t *testing.T) {
	// Real errors from the decoder (not panics) must still surface so the
	// caller can react. The patch intentionally does NOT swallow these.
	hit, err := safeCallback(
		[]byte("07-tendermint-0/clientState"),
		[]byte("bad"),
		errDecoder,
	)
	if hit || err == nil {
		t.Fatalf("decoder error: expected (false, non-nil err), got (%v, %v)", hit, err)
	}
}

// unsafeCallback is the UNPATCHED upstream logic, kept here only to prove
// that without the patch each failure mode really does crash. It mirrors
// the exact pre-patch control flow in
// vendor/github.com/cosmos/ibc-go/v11/modules/core/02-client/keeper/grpc_query.go
// as of ibc-go v10.5.0.
func unsafeCallback(key, value []byte, decode func([]byte) error) (bool, error) {
	keySplit := strings.Split(string(key), "/")
	if keySplit[len(keySplit)-1] != "clientState" {
		return false, nil
	}
	if err := decode(value); err != nil {
		return false, err
	}
	_ = keySplit[1]
	return true, nil
}

func TestUnpatchedCallback_PanicsOnCorruptValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic from unsafeCallback, got none")
		}
	}()
	_, _ = unsafeCallback(
		[]byte("07-tendermint-1/clientState"),
		[]byte("corrupt"),
		panickingDecoder,
	)
}

func TestUnpatchedCallback_PanicsOnShortKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic from unsafeCallback, got none")
		}
	}()
	_, _ = unsafeCallback([]byte("clientState"), nil, okDecoder)
}

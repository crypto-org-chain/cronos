package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReplayBlockRequestUnmarshalCapsMsgs ensures the generated
// ReplayBlockRequest.Unmarshal aborts once the wire-encoded Msgs field
// exceeds MaxReplayBlockMsgs, instead of decoding the whole attacker-supplied
// batch first. Each empty MsgEthereumTx element encodes as just the 2-byte
// field-1 tag+length, so this reproduces the CC-271 decode-time OOM shape
// without needing megabytes of payload.
func TestReplayBlockRequestUnmarshalCapsMsgs(t *testing.T) {
	elem := []byte{0x0a, 0x00} // field 1, wiretype 2 (length-delimited), length 0

	buildPayload := func(count int) []byte {
		data := make([]byte, 0, len(elem)*count)
		for i := 0; i < count; i++ {
			data = append(data, elem...)
		}
		return data
	}

	t.Run("over cap rejected before fully decoding", func(t *testing.T) {
		m := &ReplayBlockRequest{}
		err := m.Unmarshal(buildPayload(MaxReplayBlockMsgs + 1))
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds max allowed count")
		require.LessOrEqual(t, len(m.Msgs), MaxReplayBlockMsgs)
	})

	t.Run("at cap accepted", func(t *testing.T) {
		m := &ReplayBlockRequest{}
		err := m.Unmarshal(buildPayload(MaxReplayBlockMsgs))
		require.NoError(t, err)
		require.Len(t, m.Msgs, MaxReplayBlockMsgs)
	})
}

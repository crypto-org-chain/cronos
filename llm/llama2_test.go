package llm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInference(t *testing.T) {
	m, err := LoadModel("./stories15M.bin", "./tokenizer.bin")
	require.NoError(t, err)

	fmt.Println(m.Inference("say hello", 1.0, 0, 256))
}

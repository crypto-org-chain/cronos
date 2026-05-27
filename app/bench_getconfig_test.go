package app

import (
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func BenchmarkGetConfig(b *testing.B) {
	prev, hadPrev := os.LookupEnv(sdk.EnvConfigScope)
	b.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(sdk.EnvConfigScope, prev)
		} else {
			_ = os.Unsetenv(sdk.EnvConfigScope)
		}
	})

	b.Run("env_unset", func(b *testing.B) {
		_ = os.Unsetenv(sdk.EnvConfigScope)
		_ = sdk.GetConfig()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sdk.GetConfig()
		}
	})

	b.Run("env_set", func(b *testing.B) {
		_ = os.Setenv(sdk.EnvConfigScope, "cronos")
		_ = sdk.GetConfig()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sdk.GetConfig()
		}
	})
}

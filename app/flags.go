package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cast"
)

func parseBoolFlag(flag string, v interface{}) bool {
	switch v.(type) {
	case bool, string:
	default:
		panic(fmt.Errorf("invalid %s %v: must be a boolean, got %T", flag, v, v))
	}
	parsed, err := cast.ToBoolE(v)
	if err != nil {
		panic(fmt.Errorf("invalid %s %q: must be a boolean", flag, v))
	}
	return parsed
}

func parseNonNegativeIntFlag(flag string, v any) int {
	var (
		parsed int
		err    error
	)
	switch t := v.(type) {
	case string:
		parsed, err = strconv.Atoi(strings.TrimSpace(t))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		parsed, err = cast.ToIntE(v)
	default:
		panic(fmt.Errorf("invalid %s %v: must be a non-negative integer, got %T", flag, v, v))
	}
	if err != nil || parsed < 0 {
		panic(fmt.Errorf("invalid %s %v: must be a non-negative integer", flag, v))
	}
	return parsed
}

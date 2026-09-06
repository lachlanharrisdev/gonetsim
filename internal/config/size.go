package config

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("size must be positive")
		}
		return n, nil
	}

	var mult int64
	switch {
	case strings.HasSuffix(s, "kib"):
		mult, s = 1<<10, s[:len(s)-3]
	case strings.HasSuffix(s, "mib"):
		mult, s = 1<<20, s[:len(s)-3]
	case strings.HasSuffix(s, "gib"):
		mult, s = 1<<30, s[:len(s)-3]
	case strings.HasSuffix(s, "k"):
		mult, s = 1<<10, s[:len(s)-1]
	case strings.HasSuffix(s, "m"):
		mult, s = 1<<20, s[:len(s)-1]
	case strings.HasSuffix(s, "g"):
		mult, s = 1<<30, s[:len(s)-1]
	default:
		return 0, fmt.Errorf("invalid size %q (expected e.g. 64MiB)", s)
	}

	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q (expected e.g. 64MiB)", s)
	}
	return n * mult, nil
}

package testing_util

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

const ENV_TEST_TIMEOUT_IN_SECS = "TEST_TIMEOUT_IN_SECS"

func GetTestCtxWithEnvTimeoutOrDefault(parentCtx context.Context, defaultVal time.Duration) (context.Context, context.CancelFunc) {
	envVal := os.Getenv(ENV_TEST_TIMEOUT_IN_SECS)
	if envVal != "" {
		envDur, err := strconv.Atoi(envVal)
		if err != nil {
			panic(fmt.Sprintf("invalid %s provided, given: %q, expected integer", ENV_TEST_TIMEOUT_IN_SECS, envVal))
		}
		// debattable
		defaultVal = time.Duration(envDur) * time.Second
	}
	return context.WithTimeoutCause(parentCtx, defaultVal, fmt.Errorf("test timeout of %ds reached", defaultVal))
}

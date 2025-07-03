package features

import (
	"fmt"
	"io"
	"os"
	"reflect"
)

func createDirIfNotExists(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func MustClose(writers ...io.Closer) {
	for idx, w := range writers {
		if w == nil {
			// Skip nil closers and continue to the next one
			continue
		}
		if err := w.Close(); err != nil {
			t := reflect.TypeOf(w)
			panic(fmt.Errorf("failed to close writer at index %d of type '%s/%s': %w", idx, t.PkgPath(), t.String(), err))
		}
	}
}

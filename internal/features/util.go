package features

import "os"

func createDirIfNotExists(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

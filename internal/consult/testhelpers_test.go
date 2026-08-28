package consult

import (
	"os"
	"path/filepath"
)

// Small os/path helpers so the test file stays readable.
func osReadFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	return string(b), err
}

func osMkdirAll(p string) error {
	return os.MkdirAll(p, 0755)
}

func osWriteFile(p, content string) error {
	return os.WriteFile(p, []byte(content), 0644)
}

func filepathJoin(parts ...string) string {
	return filepath.Join(parts...)
}

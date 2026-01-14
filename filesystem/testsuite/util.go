package testsuite

import (
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

// shasum calculates the SHA-256 hash of a reader's content
func shasum(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", errors.WithStack(err)
	}

	hash := sha256.Sum256(data)

	return fmt.Sprintf("%x", hash), nil
}
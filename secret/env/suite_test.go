package env

import (
	"os"
	"testing"

	"github.com/firetiger-oss/storage/secret"
	"github.com/firetiger-oss/storage/test"
)

// TestEnvBackendWithSuite runs the comprehensive test suite against the env backend.
func TestEnvBackendWithSuite(t *testing.T) {
	// Set up some test environment variables
	os.Setenv("TEST_SECRET_1", "value1")
	os.Setenv("TEST_SECRET_2", "value2")
	os.Setenv("TEST_OTHER", "value3")
	defer func() {
		os.Unsetenv("TEST_SECRET_1")
		os.Unsetenv("TEST_SECRET_2")
		os.Unsetenv("TEST_OTHER")
	}()

	test.TestManager(t, func(t *testing.T) (secret.Manager, error) {
		return NewManager(), nil
	})
}

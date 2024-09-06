//go:generate go build -o ./mock/dist/mock.exe mock/main.go

package terraform_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/scaleoutllc/terrallel/internal/terraform"
)

func TestJobCancel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dir, _ := os.Getwd()
	job := &terraform.Job{
		Name:   "test-failure",
		Bin:    filepath.Join(dir, "mock", "dist", "mock.exe"),
		Args:   []string{},
		Stdout: &stdout,
		Stderr: &stderr,
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	var jobErr error
	go func() {
		defer wg.Done()
		jobErr = job.Run(false)
	}()
	time.Sleep(10 * time.Millisecond)
	if err := job.Cancel(); err != nil {
		t.Fatalf("Unexpected error cancelling, %s", err)
	}
	wg.Wait()
	expectedResult := "test-failure: interrupted"
	if job.Result() != expectedResult {
		t.Errorf("expected result %s, got %s with error: %s", expectedResult, job.Result(), jobErr)
	}
}

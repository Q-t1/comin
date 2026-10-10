package builder

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"testing"
	"time"

	_ "net/http/pprof"

	"github.com/stretchr/testify/assert"
)

type RunnableDummy struct {
	result int
}

func (r *RunnableDummy) Run(ctx context.Context) error {
	r.result = 1
	return nil
}

func TestNewExec(t *testing.T) {
	r := &RunnableDummy{}
	e := NewExec(r, time.Second)
	assert.Equal(t, 0, r.result)
	e.Start(t.Context())
	e.Wait()
	assert.Equal(t, 1, r.result)
	assert.True(t, e.finished.Load())
	assert.Nil(t, e.getErr())
}

type RunnableContext struct{}

func (r *RunnableContext) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sleep", "3")
	err := cmd.Run()
	return err
}
func TestExecTimeout(t *testing.T) {
	r := &RunnableContext{}
	e := NewExec(r, time.Second)
	e.Start(t.Context())
	e.Wait()
	assert.Equal(t, context.DeadlineExceeded, e.getErr())
}

func TestExecStop(t *testing.T) {
	r := &RunnableContext{}
	e := NewExec(r, 5*time.Second)
	e.Start(t.Context())
	time.Sleep(500 * time.Millisecond)
	e.Stop()
	e.Wait()
	assert.Equal(t, context.Canceled, e.getErr())
}

type RunnableError struct{}

func (r *RunnableError) Run(ctx context.Context) error {
	return fmt.Errorf("An error occurred")
}
func TestExecError(t *testing.T) {
	r := &RunnableError{}
	e := NewExec(r, 5*time.Second)
	e.Start(t.Context())
	e.Wait()
	assert.True(t, e.finished.Load())
	assert.ErrorContains(t, e.getErr(), "An error occurred")
}

type immediateRunnable struct{}

func (immediateRunnable) Run(context.Context) error { return nil }

// TestExecWaitFastRunnable ensures Wait returns when the runnable
// finishes before Wait starts to wait for it.
func TestExecWaitFastRunnable(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20000 {
				e := NewExec(immediateRunnable{}, time.Second)
				e.Start(t.Context())
				// As in the builder, Wait is called from another goroutine
				waited := make(chan struct{})
				go func() {
					e.Wait()
					close(waited)
				}()
				<-waited
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Exec.Wait never returned")
	}
}

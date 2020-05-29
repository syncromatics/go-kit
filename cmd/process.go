package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/syncromatics/go-kit/log"

	"golang.org/x/sync/errgroup"
)

// ProcessGroup is an errgroup that listens for OS process signals
type ProcessGroup struct {
	ctx    context.Context
	cancel context.CancelFunc
	group  *errgroup.Group
}

// NewProcessGroup creates a new ProcessGroup
func NewProcessGroup(outerCtx context.Context) *ProcessGroup {
	ctx, cancel := context.WithCancel(outerCtx)
	group, ctx := errgroup.WithContext(ctx)
	return &ProcessGroup{
		ctx,
		cancel,
		group,
	}
}

// Context returns the context used by the ProcessGroup
func (gw *ProcessGroup) Context() context.Context {
	return gw.ctx
}

// Go calls the given function in a new goroutine.
//
// The first call to return a non-nil error cancels the group; its error will be
// returned by Wait.
func (gw *ProcessGroup) Go(f func() error) {
	gw.group.Go(f)
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (gw *ProcessGroup) Wait() error {
	signals := make(chan os.Signal)
	defer close(signals)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	log.Info("started process")

	select {
	case sig := <-signals:
		log.Debug("caught signal", "signal", sig)
	case <-gw.ctx.Done():
		log.Debug("cancelled context")
	}

	log.Info("stopping process")

	gw.cancel()

	err := gw.group.Wait()
	if err != nil {
		return err
	}

	return nil
}

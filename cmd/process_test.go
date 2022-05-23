package cmd_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/syncromatics/go-kit/v2/cmd"

	"github.com/stretchr/testify/assert"
)

func Test_ProcessGroup_Go_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	group := cmd.NewProcessGroup(ctx)

	group.Go(func() error {
		cancel()

		select {
		case <-group.Context().Done():
			return nil
		case <-time.After(3 * time.Second):
			return errors.New("group context not derived from input context")
		}
	})

	err := group.Wait()
	assert.Nil(t, err)
}

func Test_ProcessGroup_Start_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	group := cmd.NewProcessGroup(ctx)

	group.Start(func(argCtx context.Context) error {
		cancel()

		select {
		case <-argCtx.Done():
			return nil
		case <-time.After(3 * time.Second):
			return errors.New("argument context not derived from input context")
		}
	})

	err := group.Wait()
	assert.Nil(t, err)
}

func Test_ProcessGroup_SuccessWithoutContextCancellation(t *testing.T) {
	group := cmd.NewProcessGroup(context.Background())

	group.Go(func() error {
		return nil
	})

	err := group.Wait()
	assert.Nil(t, err)
}

func Test_ProcessGroup_Go_Failure(t *testing.T) {
	ctx := context.Background()
	group := cmd.NewProcessGroup(ctx)

	group.Go(func() error {
		return nil
	})
	group.Go(func() error {
		return errors.New("intentional failure")
	})

	err := group.Wait()
	assert.Equal(t, "intentional failure", err.Error())
}

func Test_ProcessGroup_Start_Failure(t *testing.T) {
	ctx := context.Background()
	group := cmd.NewProcessGroup(ctx)

	group.Start(func(context.Context) error {
		return nil
	})
	group.Start(func(context.Context) error {
		return errors.New("intentional failure")
	})

	err := group.Wait()
	assert.Equal(t, "intentional failure", err.Error())
}

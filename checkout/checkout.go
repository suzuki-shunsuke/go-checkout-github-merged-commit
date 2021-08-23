package checkout

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"time"

	"github.com/Songmu/timeout"
)

// Input is an input of Checkout function.
type Input struct {
	// Base is a base ref (e.g. main).
	Base string
	// Head is a ref of head (feature branch name or SHA).
	Head   string
	Stdout io.Writer
	Stderr io.Writer
}

// Checkout fetches and checkouts the base ref and merges the head ref into the base ref. Note that the current branch would be changed.
func Checkout(ctx context.Context, input *Input) error {
	if input == nil {
		return errors.New("input is nil")
	}
	if input.Base == "" {
		return errors.New("input.Base is empty")
	}
	if err := fetch(ctx, input); err != nil {
		return err
	}
	if err := checkout(ctx, input); err != nil {
		return err
	}
	if err := merge(ctx, input); err != nil {
		return err
	}
	return nil
}

func fetch(ctx context.Context, input *Input) error {
	msg := "git fetch origin " + input.Base
	log.Println("[INFO] " + msg)
	cmd := exec.Command("git", "fetch", "origin", input.Base) //nolint:gosec
	cmd.Stdout = input.Stdout
	cmd.Stderr = input.Stderr
	tio := &timeout.Timeout{
		Cmd:       cmd,
		Duration:  10 * time.Minute, //nolint:gomnd
		KillAfter: 10 * time.Second, //nolint:gomnd
	}
	status, err := tio.RunContext(ctx)
	if err != nil {
		return fmt.Errorf(msg+": %w", err)
	}
	if status.Code != 0 {
		return errors.New("exit code: " + strconv.Itoa(status.Code))
	}
	return nil
}

func checkout(ctx context.Context, input *Input) error {
	msg := "git checkout " + input.Base
	log.Println("[INFO] " + msg)
	cmd := exec.Command("git", "checkout", input.Base) //nolint:gosec
	cmd.Stdout = input.Stdout
	cmd.Stderr = input.Stderr
	tio := &timeout.Timeout{
		Cmd:       cmd,
		Duration:  30 * time.Second, //nolint:gomnd
		KillAfter: 10 * time.Second, //nolint:gomnd
	}
	status, err := tio.RunContext(ctx)
	if err != nil {
		return fmt.Errorf(msg+": %w", err)
	}
	if status.Code != 0 {
		return errors.New("exit code: " + strconv.Itoa(status.Code))
	}
	return nil
}

func merge(ctx context.Context, input *Input) error {
	msg := "git merge " + input.Head
	log.Println("[INFO] " + msg)
	cmd := exec.Command("git", "merge", input.Head) //nolint:gosec
	cmd.Stdout = input.Stdout
	cmd.Stderr = input.Stderr
	tio := &timeout.Timeout{
		Cmd:       cmd,
		Duration:  30 * time.Second, //nolint:gomnd
		KillAfter: 10 * time.Second, //nolint:gomnd
	}
	status, err := tio.RunContext(ctx)
	if err != nil {
		return fmt.Errorf(msg+": %w", err)
	}
	if status.Code != 0 {
		return errors.New("exit code: " + strconv.Itoa(status.Code))
	}
	return nil
}

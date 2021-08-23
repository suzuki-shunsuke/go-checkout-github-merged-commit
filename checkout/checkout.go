package checkout

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/Songmu/timeout"
	"github.com/google/go-github/v38/github"
	"golang.org/x/oauth2"
)

// Input is an input of Checkout function.
type Input struct {
	Owner           string
	Repo            string
	PRNumber        int
	Mergeable       bool
	GitHub          *github.Client
	Stdout          io.Writer
	Stderr          io.Writer
	PollingTimeout  time.Duration
	PollingInterval time.Duration
}

// Checkout waits until the pull request becomes mergeable and checkouts the merged commit.
// Checkout fails if the pull request doesn't become mergeable.
func Checkout(ctx context.Context, input *Input) (*github.PullRequest, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}
	setInput(ctx, input)
	logger := log.New(input.Stderr, "", log.LstdFlags)

	var pr *github.PullRequest
	if !input.Mergeable {
		p, err := polling(ctx, input, logger)
		if err != nil {
			return nil, err
		}
		pr = p
		if err := fetch(ctx, input, logger); err != nil {
			return nil, err
		}
	}

	if err := checkout(ctx, input, logger); err != nil {
		return pr, err
	}
	return pr, nil
}

func setInput(ctx context.Context, input *Input) {
	if input.Stdout == nil {
		input.Stdout = os.Stdout
	}
	if input.Stderr == nil {
		input.Stderr = os.Stderr
	}
	if input.GitHub == nil {
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			input.GitHub = github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: token},
			)))
		}
	}
	if input.PollingInterval == 0 {
		input.PollingInterval = 5 * time.Second //nolint:gomnd
	}
	if input.PollingTimeout == 0 {
		input.PollingTimeout = 50 * time.Second //nolint:gomnd
	}
}

func polling(ctx context.Context, input *Input, logger *log.Logger) (*github.PullRequest, error) {
	maxCnt := int(input.PollingTimeout / input.PollingInterval)
	for i := 1; i <= maxCnt; i++ {
		logger.Println("[INFO] check the pull request is mergeable")
		pr, _, err := input.GitHub.PullRequests.Get(ctx, input.Owner, input.Repo, input.PRNumber)
		if err != nil {
			return pr, err //nolint:wrapcheck
		}
		if pr.Mergeable == nil {
			if i == maxCnt {
				break
			}
			timer := time.NewTimer(input.PollingInterval)
			logger.Printf("[INFO] wait %s: (%d/%d)", input.PollingInterval.String(), i, maxCnt)
			select {
			case <-timer.C:
				continue
			case <-ctx.Done():
				return nil, ctx.Err() //nolint:wrapcheck
			}
		}
		if pr.GetMergeable() {
			return pr, nil
		}
		return nil, errors.New("the pull request isn't mergeable")
	}
	return nil, errors.New("timeout")
}

func validateInput(input *Input) error {
	if input == nil {
		return errors.New("input is nil")
	}
	if input.Owner == "" {
		return errors.New("input.Owner is empty")
	}
	if input.Repo == "" {
		return errors.New("input.Repo is empty")
	}
	if input.PRNumber <= 0 {
		return errors.New("input.PRNumber <= 0")
	}
	return nil
}

func fetch(ctx context.Context, input *Input, logger *log.Logger) error {
	prS := strconv.Itoa(input.PRNumber)
	ref := "pull/" + prS + "/merge:pr/" + prS + "/merge"
	msg := "git fetch --depth 1 origin " + ref
	logger.Println("[INFO] " + msg)
	cmd := exec.Command("git", "fetch", "--depth", "1", "origin", ref)
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

func checkout(ctx context.Context, input *Input, logger *log.Logger) error {
	ref := "pr/" + strconv.Itoa(input.PRNumber) + "/merge"
	msg := "git checkout " + ref
	logger.Println("[INFO] " + msg)
	cmd := exec.Command("git", "checkout", ref)
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

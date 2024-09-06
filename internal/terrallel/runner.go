package terrallel

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/tkellen/treeprint"
)

type Job interface {
	Run(bool) error
	Cancel() error
	Result() string
}

type Tree struct {
	Name  string
	Jobs  []Job
	Group []*Tree
	Next  *Tree
}

func (t *Tree) String() string {
	return t.Report(treeprint.NewWithRoot(t.Name)).String()
}

func (t *Tree) Cancel() error {
	return nil
}

func (t *Tree) Do(reverse bool, dryrun bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	termReceived := false
	termMessage := false
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range sigChan {
			if termReceived {
				if !termMessage {
					termMessage = true
					fmt.Printf("\nTerrallel forcefully shutting down...\n")
				}
			} else {
				fmt.Printf("\nTerrallel shutting down gracefully...\n")
				termReceived = true
			}
			cancel()
		}
	}()
	var err error
	if reverse {
		err = t.Reverse(ctx, dryrun)
	} else {
		err = t.Forward(ctx, dryrun)
	}
	if err != nil {
		return fmt.Errorf("some jobs failed to complete.\n%s", err)
	}
	return nil
}

func (t *Tree) Forward(ctx context.Context, dryrun bool) error {
	if err := parallel(ctx, t.Group, func(child *Tree) error {
		return child.Forward(ctx, dryrun)
	}); err != nil {
		return err
	}
	if err := parallel(ctx, t.Jobs, func(j Job) error {
		return j.Run(dryrun)
	}); err != nil {
		return err
	}
	if t.Next != nil {
		return t.Next.Forward(ctx, dryrun)
	}
	return nil
}

func (t *Tree) Reverse(ctx context.Context, dryrun bool) error {
	if t.Next != nil {
		if err := t.Next.Reverse(ctx, dryrun); err != nil {
			return err
		}
	}
	if err := parallel(ctx, t.Jobs, func(j Job) error {
		return j.Run(dryrun)
	}); err != nil {
		return err
	}
	if err := parallel(ctx, t.Group, func(child *Tree) error {
		return child.Reverse(ctx, dryrun)
	}); err != nil {
		return err
	}
	return nil
}

func (t *Tree) Report(root treeprint.Tree) treeprint.Tree {
	if len(t.Group) != 0 {
		groups := root.AddBranch("groups")
		for _, g := range t.Group {
			g.Report(groups.AddBranch(g.Name))
		}
	}
	if len(t.Jobs) != 0 {
		workspaces := root.AddBranch("workspaces")
		for _, ws := range t.Jobs {
			workspaces.AddNode(ws.Result())
		}
	}
	if t.Next != nil {
		t.Next.Report(root.AddBranch("next"))
	}
	return root
}

type Cancellable interface {
	Cancel() error
}

func parallel[T Cancellable](ctx context.Context, items []T, run func(T) error) error {
	if len(items) == 0 {
		return nil
	}
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(items))
	for _, item := range items {
		wg.Add(1)
		go func(item T) {
			defer wg.Done()
			runCh := make(chan error, 1)
			go func() {
				runCh <- run(item)
			}()
			select {
			case <-ctx.Done():
				errCh <- item.Cancel()
			case err := <-runCh:
				errCh <- err
			}
		}(item)
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		errOut := make([]string, len(errs))
		for i, err := range errs {
			errOut[i] += err.Error()
		}
		return fmt.Errorf("%s", strings.Join(errOut, "\n"))
	}
	return nil
}

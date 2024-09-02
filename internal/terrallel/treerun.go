package terrallel

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/fatih/color"
	"github.com/tkellen/treeprint"
)

type TreeRunner struct {
	Name       string
	Workspaces []*Job
	Group      []*TreeRunner
	Next       *TreeRunner
}

func NewTreeRunner(t *Target, fn Worker) *TreeRunner {
	node := &TreeRunner{
		Name:       t.Name,
		Workspaces: make([]*Job, len(t.Workspaces)),
		Group:      make([]*TreeRunner, len(t.Group)),
	}
	for i, ws := range t.Workspaces {
		node.Workspaces[i] = fn(ws)
	}
	for i, g := range t.Group {
		node.Group[i] = NewTreeRunner(g, fn)
	}
	if t.Next != nil {
		node.Next = NewTreeRunner(t.Next, fn)
	}
	return node
}

func (tr *TreeRunner) String() string {
	return tr.Report(treeprint.NewWithRoot(tr.Name)).String()
}

func (tr *TreeRunner) Forward(ctx context.Context) error {
	if err := parallel(tr.Group, func(group *TreeRunner) error {
		return group.Forward(ctx)
	}); err != nil {
		return err
	}
	if err := parallel(tr.Workspaces, func(ws *Job) error {
		return ws.run(ctx)
	}); err != nil {
		return err
	}
	if tr.Next != nil {
		return tr.Next.Forward(ctx)
	}
	return nil
}

func (tr *TreeRunner) Reverse(ctx context.Context) error {
	if tr.Next != nil {
		if err := tr.Next.Reverse(ctx); err != nil {
			return err
		}
	}
	if err := parallel(tr.Workspaces, func(ws *Job) error {
		return ws.run(ctx)
	}); err != nil {
		return err
	}
	if err := parallel(tr.Group, func(group *TreeRunner) error {
		return group.Reverse(ctx)
	}); err != nil {
		return err
	}
	return nil
}

func (tr *TreeRunner) Signal(sig os.Signal) {
	if len(tr.Group) != 0 {
		for _, group := range tr.Group {
			group.Signal(sig)
		}
	}
	if len(tr.Workspaces) != 0 {
		for _, workspace := range tr.Workspaces {
			workspace.signal(sig)
		}
	}
	if tr.Next != nil {
		tr.Next.Signal(sig)
	}
}

func (tr *TreeRunner) Report(root treeprint.Tree) treeprint.Tree {
	if len(tr.Group) != 0 {
		groups := root.AddBranch("groups")
		for _, g := range tr.Group {
			g.Report(groups.AddBranch(g.Name))
		}
	}
	if len(tr.Workspaces) != 0 {
		workspaces := root.AddBranch("workspaces")
		for _, ws := range tr.Workspaces {
			workspaces.AddNode(ws.String())
		}
	}
	if tr.Next != nil {
		tr.Next.Report(root.AddBranch("next"))
	}
	return root
}

func parallel[T any](items []T, action func(T) error) error {
	if len(items) == 0 {
		return nil
	}
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(items))
	for _, item := range items {
		wg.Add(1)
		go func(item T) {
			defer wg.Done()
			errCh <- action(item)
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
		return fmt.Errorf("%v", errs)
	}
	return nil
}

type Worker func(string) *Job

type Job struct {
	Name   string
	Cmd    *exec.Cmd
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

func (j *Job) String() string {
	return fmt.Sprintf("%s: %s", j.Name, j.result())
}

func (j *Job) signal(sig os.Signal) {
	if j.Cmd.Process != nil {
		j.Cmd.Process.Signal(sig)
	}
}

func (j *Job) run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := j.Cmd.Start(); err == nil {
		if err := j.Cmd.Wait(); err != nil {
			return fmt.Errorf("%s: %w", j.Name, err)
		}
	}
	return nil
}

func (j *Job) ExitCode() int {
	if j.Cmd.ProcessState == nil {
		return -1
	}
	return j.Cmd.ProcessState.ExitCode()
}

func (j *Job) result() string {
	if j.Cmd.ProcessState == nil {
		return color.CyanString("skipped")
	}
	exitCode := j.Cmd.ProcessState.ExitCode()
	if exitCode == 0 {
		return color.GreenString("success")
	}
	return color.RedString(fmt.Sprintf("failure (exit code: %d)", exitCode))
}

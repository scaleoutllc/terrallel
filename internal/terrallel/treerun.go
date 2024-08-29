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

type treeRunner struct {
	name       string
	workspaces []*work
	groups     []*treeRunner
	next       *treeRunner
}

func newTreeRunner(t *Target, fn execWork) *treeRunner {
	node := &treeRunner{
		name:       t.Name,
		workspaces: make([]*work, len(t.Workspaces)),
		groups:     make([]*treeRunner, len(t.Group)),
	}
	for i, ws := range t.Workspaces {
		node.workspaces[i] = fn(ws)
	}
	for i, g := range t.Group {
		node.groups[i] = newTreeRunner(g, fn)
	}
	if t.Next != nil {
		node.next = newTreeRunner(t.Next, fn)
	}
	return node
}

func (tr *treeRunner) String() string {
	return tr.Report(treeprint.NewWithRoot(tr.name)).String()
}

func (tr *treeRunner) Forward(ctx context.Context) error {
	if err := parallel(tr.groups, func(group *treeRunner) error {
		return group.Forward(ctx)
	}); err != nil {
		return err
	}
	if err := parallel(tr.workspaces, func(ws *work) error {
		return ws.run(ctx)
	}); err != nil {
		return err
	}
	if tr.next != nil {
		return tr.next.Forward(ctx)
	}
	return nil
}

func (tr *treeRunner) Reverse(ctx context.Context) error {
	if tr.next != nil {
		if err := tr.next.Reverse(ctx); err != nil {
			return err
		}
	}
	if err := parallel(tr.workspaces, func(ws *work) error {
		return ws.run(ctx)
	}); err != nil {
		return err
	}
	if err := parallel(tr.groups, func(group *treeRunner) error {
		return group.Reverse(ctx)
	}); err != nil {
		return err
	}
	return nil
}

func (tr *treeRunner) Signal(sig os.Signal) {
	if len(tr.groups) != 0 {
		for _, group := range tr.groups {
			group.Signal(sig)
		}
	}
	if len(tr.workspaces) != 0 {
		for _, workspace := range tr.workspaces {
			workspace.signal(sig)
		}
	}
	if tr.next != nil {
		tr.next.Signal(sig)
	}
}

func (tr *treeRunner) Report(root treeprint.Tree) treeprint.Tree {
	if len(tr.groups) != 0 {
		groups := root.AddBranch("groups")
		for _, g := range tr.groups {
			g.Report(groups.AddBranch(g.name))
		}
	}
	if len(tr.workspaces) != 0 {
		workspaces := root.AddBranch("workspaces")
		for _, ws := range tr.workspaces {
			workspaces.AddNode(ws.String())
		}
	}
	if tr.next != nil {
		tr.next.Report(root.AddBranch("next"))
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

type execWork func(string) *work

type work struct {
	name   string
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (w *work) String() string {
	return fmt.Sprintf("%s: %s", w.name, w.result())
}

func (w *work) signal(sig os.Signal) {
	if w.cmd.Process != nil {
		w.cmd.Process.Signal(sig)
	}
}

func (w *work) run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := w.cmd.Start(); err == nil {
		if err := w.cmd.Wait(); err != nil {
			return fmt.Errorf("%s: %w", w.name, err)
		}
	}
	return nil
}

func (w *work) exitCode() int {
	if w.cmd.ProcessState == nil {
		return -1
	}
	return w.cmd.ProcessState.ExitCode()
}

func (w *work) result() string {
	if w.cmd.ProcessState == nil {
		return color.CyanString("skipped")
	}
	exitCode := w.cmd.ProcessState.ExitCode()
	if exitCode == 0 {
		return color.GreenString("success")
	}
	return color.RedString(fmt.Sprintf("failure (exit code: %d)", exitCode))
}

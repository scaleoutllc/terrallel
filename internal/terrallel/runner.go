package terrallel

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sync"
	"syscall"

	"github.com/fatih/color"
	"github.com/tkellen/treeprint"
)

type traversalFn func(context.Context, *Target, treeprint.Tree) error
type runner struct {
	terrallel *Terrallel
	command   string
	args      []string
	procChan  chan *exec.Cmd
	wg        *sync.WaitGroup
	traverse  traversalFn
}

// this can only be used once and not concurrently. gross.
func (r *runner) start(target *Target) (treeprint.Tree, []*exec.Cmd, error) {
	r.wg = &sync.WaitGroup{}
	r.procChan = make(chan *exec.Cmd)
	r.traverse = r.up
	for _, arg := range r.args {
		if arg == "destroy" {
			r.traverse = r.down
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	procs := []*exec.Cmd{}
	handleAbort(cancel, procs, r.terrallel.stderr)
	go func() {
		for proc := range r.procChan {
			procs = append(procs, proc)
		}
	}()
	results := treeprint.NewWithRoot(target.Name)
	err := r.traverse(ctx, target, results)
	r.wg.Wait()
	close(r.procChan)
	return results, procs, err
}

func (r *runner) up(ctx context.Context, target *Target, results treeprint.Tree) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := r.groups(ctx, target.Group, results); err != nil {
		return err
	}
	if err := r.workspaces(target.Workspaces, results); err != nil {
		return err
	}
	if target.Next != nil {
		return r.up(ctx, target.Next, results.AddBranch("next"))
	}
	return nil
}

func (r *runner) down(ctx context.Context, target *Target, results treeprint.Tree) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if target.Next != nil {
		if err := r.down(ctx, target.Next, results.AddBranch("next")); err != nil {
			return err
		}
	}
	if err := r.groups(ctx, target.Group, results); err != nil {
		return err
	}
	if err := r.workspaces(target.Workspaces, results); err != nil {
		return err
	}
	return nil
}

func (r *runner) groups(ctx context.Context, groups []*Target, results treeprint.Tree) error {
	if len(groups) == 0 {
		return nil
	}
	branch := results.AddBranch("groups")
	errCh := make(chan error, len(groups))
	var wg sync.WaitGroup
	for _, group := range groups {
		wg.Add(1)
		go func(group *Target) {
			defer wg.Done()
			errCh <- r.traverse(ctx, group, branch.AddBranch(group.Name))
		}(group)
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) workspaces(workspaces []string, results treeprint.Tree) error {
	if len(workspaces) == 0 {
		return nil
	}
	errCh := make(chan error, len(workspaces))
	var wg sync.WaitGroup
	branch := results.AddBranch("workspaces")
	for _, workspace := range workspaces {
		wg.Add(1)
		go func(workspace string) {
			defer wg.Done()
			errCh <- r.exec(workspace, branch)
		}(workspace)
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) exec(workspace string, results treeprint.Tree) error {
	cmd := exec.Command(r.command, r.args...)
	cmd.Dir = path.Join(r.terrallel.Config.Basedir, workspace)
	prefix := fmt.Sprintf("[%s]: ", workspace)
	cmd.Stdout = prefixWriter(r.terrallel.stdout, prefix)
	cmd.Stderr = prefixWriter(r.terrallel.stderr, prefix)
	r.wg.Add(1)
	defer r.wg.Done()
	r.procChan <- cmd
	err := cmd.Start()
	if err == nil {
		err = cmd.Wait()
		if err != nil {
			err := fmt.Errorf("%s: %w", workspace, err)
			results.AddNode(fmt.Sprintf("%s: %s", workspace, color.RedString("failure")))
			return err
		}
	}
	results.AddNode(fmt.Sprintf("%s: %s", workspace, color.GreenString("success")))
	return nil
}

func handleAbort(
	cancel context.CancelFunc,
	procs []*exec.Cmd,
	stderr io.Writer,
) {
	termReceived := false
	termMessage := false
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for sig := range sigChan {
			if termReceived {
				if !termMessage {
					termMessage = true
					stderr.Write([]byte("\nTerrallel forcefully shutting down...\n"))
				}
			} else {
				stderr.Write([]byte("\nTerrallel shutting down gracefully...\n"))
				termReceived = true
			}
			cancel()
			for _, proc := range procs {
				if proc.Process != nil {
					_ = proc.Process.Signal(sig)
				}
			}
		}
	}()
}

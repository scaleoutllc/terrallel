package terrallel

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/tkellen/treeprint"
	"gopkg.in/yaml.v3"
)

type config struct {
	Basedir string
}

type Terrallel struct {
	Config   config
	Refs     map[string]*TargetRef `yaml:"targets"`
	Manifest map[string]*Target
	stdout   io.Writer
	stderr   io.Writer
}

type Target struct {
	Name       string
	Group      []*Target `yaml:"group,omitempty"`
	Workspaces []string  `yaml:"workspaces,omitempty"`
	Next       *Target   `yaml:"next,omitempty"`
}

func New(path string, stdout io.Writer, stderr io.Writer) (*Terrallel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failure reading manifest: %w", err)
	}
	t := &Terrallel{
		Manifest: map[string]*Target{},
		stdout:   stdout,
		stderr:   stderr,
	}
	if err = yaml.Unmarshal(raw, t); err != nil {
		return nil, fmt.Errorf("failure loading manifest: %w", err)
	}
	for name, ref := range t.Refs {
		if err := ref.validate(); err != nil {
			return nil, fmt.Errorf("invalid key %s: %w", name, err)
		}
	}
	for name, ref := range t.Refs {
		ref.parent = name
		target, err := ref.build(t, 0, name)
		if err != nil {
			return nil, fmt.Errorf("failure processing group %v: %w", name, err)
		}
		t.Manifest[name] = target
	}
	return t, nil
}

func (t *Terrallel) Run(command string, args []string, targetName string) (treeprint.Tree, error) {
	target, ok := t.Manifest[targetName]
	if !ok {
		return nil, fmt.Errorf("target %s not found", targetName)
	}
	ctx, cancel := context.WithCancel(context.Background())
	procs := []*exec.Cmd{}
	handleAbort(cancel, procs, t.stderr)
	procChan := make(chan *exec.Cmd)
	go func() {
		for proc := range procChan {
			procs = append(procs, proc)
		}
	}()
	wg := sync.WaitGroup{}
	results, err := (&runner{
		command:  command,
		args:     args,
		config:   t.Config,
		stdout:   t.stdout,
		stderr:   t.stderr,
		procChan: procChan,
		wg:       &wg,
	}).start(ctx, target)
	wg.Wait()
	close(procChan)
	return results, err
}

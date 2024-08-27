package terrallel

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"

	"gopkg.in/yaml.v3"
)

type Terrallel struct {
	Config   *Config `yaml:"terrallel,omitempty"`
	Manifest map[string]*Target
}

type Config struct {
	Basedir string
	Import  []string
	stdout  io.Writer
	stderr  io.Writer
}

func New(path string, stdout io.Writer, stderr io.Writer) (*Terrallel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failure reading manifest: %w", err)
	}
	t := &Terrallel{
		Manifest: map[string]*Target{},
	}
	if err = yaml.Unmarshal(raw, t); err != nil {
		return nil, fmt.Errorf("failure loading manifest: %w", err)
	}
	if err := resolveTargets(t.Manifest, append(t.Config.Import, path)); err != nil {
		return nil, fmt.Errorf("failure processing manifest: %w", err)
	}
	t.Config.stdout = stdout
	t.Config.stderr = stderr
	return t, nil
}

func (t *Terrallel) Runner(args []string, target *Target) *treeRun {
	exec := terraform(t.Config, args)
	runTree := target.depthFirst(exec)
	for _, arg := range args {
		if arg == "destroy" {
			runTree = target.breadthFirst(exec)
		}
	}
	return runTree
}

func terraform(config *Config, args []string) execWork {
	return func(name string) *work {
		prefix := fmt.Sprintf("[%s]: ", name)
		stdout := prefixWriter(config.stdout, prefix)
		stderr := prefixWriter(config.stderr, prefix)
		cmd := exec.Command("terraform", args...)
		cmd.Dir = path.Join(config.Basedir, name)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return &work{
			name:   name,
			cmd:    cmd,
			stdout: stdout.buf,
			stderr: stderr.buf,
		}
	}
}

package terrallel

import (
	"fmt"
	"io"
	"os"

	"github.com/tkellen/treeprint"
	"gopkg.in/yaml.v3"
)

type Terrallel struct {
	Config   *Config `yaml:"terrallel,omitempty"`
	Manifest map[string]*Target
	stdout   io.Writer
	stderr   io.Writer
}

type Config struct {
	Basedir string
	Import  []string
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
	if err := resolveTargets(t.Manifest, append(t.Config.Import, path)); err != nil {
		return nil, fmt.Errorf("failure processing manifest: %w", err)
	}
	return t, nil
}

func (t *Terrallel) Run(command string, args []string, targetName string) (treeprint.Tree, error) {
	target, ok := t.Manifest[targetName]
	if !ok {
		return nil, fmt.Errorf("target %s not found", targetName)
	}
	treeprint, _, err := (&runner{
		terrallel: t,
		command:   command,
		args:      args,
	}).start(target)
	return treeprint, err
}

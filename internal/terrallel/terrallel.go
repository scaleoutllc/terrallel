package terrallel

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Terrallel struct {
	Config   *Config `yaml:"terrallel,omitempty"`
	Manifest map[string]*Target
}

type Target struct {
	Name       string
	Group      []*Target `yaml:"group,omitempty"`
	Workspaces []string  `yaml:"workspaces,omitempty"`
	Next       *Target   `yaml:"next,omitempty"`
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
	t.Manifest, err = loadTargets(append(t.Config.Import, path))
	if err != nil {
		return nil, fmt.Errorf("failure processing manifest: %w", err)
	}
	t.Config.stdout = stdout
	t.Config.stderr = stderr
	return t, nil
}

func (t *Terrallel) Runner(args []string, target *Target) *treeRunner {
	return newTreeRunner(target, func(name string) *work {
		prefix := fmt.Sprintf("[%s]: ", name)
		stdout := prefixWriter(t.Config.stdout, prefix)
		stderr := prefixWriter(t.Config.stderr, prefix)
		cmd := exec.Command("terraform", args...)
		cmd.Dir = path.Join(t.Config.Basedir, name)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return &work{
			name:   name,
			cmd:    cmd,
			stdout: stdout.buf,
			stderr: stderr.buf,
		}
	})
}

func loadTargets(imports []string) (map[string]*Target, error) {
	targets := make(map[string]*target)
	for _, glob := range imports {
		paths, err := filepath.Glob(glob)
		if err != nil {
			return nil, fmt.Errorf("failure expanding glob pattern %s: %w", glob, err)
		}
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failure reading import file %s: %w", path, err)
			}
			temp := struct {
				Targets map[string]*target `yaml:"targets"`
			}{}
			if err := yaml.Unmarshal(content, &temp); err != nil {
				return nil, fmt.Errorf("failure unmarshalling: %w", err)
			}

			for name, target := range temp.Targets {
				if _, exists := targets[name]; exists {
					return nil, fmt.Errorf("duplicate target %s found in import file %s", name, path)
				}
				targets[name] = target
			}
		}
	}

	finalTargets := make(map[string]*Target)
	for name, target := range targets {
		resolved, err := target.build(targets, name)
		if err != nil {
			return nil, fmt.Errorf("failure processing target %s: %w", name, err)
		}
		finalTargets[name] = resolved
	}

	return finalTargets, nil
}

type target struct {
	parent     string
	Group      []string
	Workspaces []string
	Next       *target
}

func (t *target) build(targets map[string]*target, name string) (*Target, error) {
	if len(t.Group) != 0 && len(t.Workspaces) != 0 {
		return nil, fmt.Errorf("workspaces and group cannot coexist at the same level")
	}
	target := &Target{
		Name:       name,
		Workspaces: t.Workspaces,
	}
	var err error
	if len(t.Group) != 0 {
		target.Group, err = t.resolveGroup(targets)
		if err != nil {
			return nil, err
		}
	}
	if t.Next != nil {
		t.Next.parent = t.parent
		target.Next, err = t.Next.build(targets, "next")
		if err != nil {
			return nil, err
		}
	}
	return target, nil
}

func (t *target) resolveGroup(targets map[string]*target) ([]*Target, error) {
	var children []*Target
	for _, name := range t.Group {
		if resolved, ok := targets[name]; ok {
			group, err := resolved.build(targets, name)
			if err != nil {
				return nil, err
			}
			children = append(children, group)
		} else {
			return nil, fmt.Errorf("group %s does not exist", name)
		}
	}
	return children, nil
}

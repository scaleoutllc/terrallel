package terrallel

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Terrallel struct {
	Config   *Config `yaml:"terrallel,omitempty"`
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

type Config struct {
	Basedir string
	Import  []string
}

func New(stdout io.Writer, stderr io.Writer) *Terrallel {
	return &Terrallel{
		Manifest: map[string]*Target{},
		Config:   &Config{},
		stdout:   stdout,
		stderr:   stderr,
	}
}

func (t *Terrallel) Load(path string) error {
	manifest, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading: %w", err)
	}
	if err = yaml.Unmarshal(manifest, t); err != nil {
		return fmt.Errorf("parsing manifest %s: %w", path, err)
	}
	if t.Config.Import == nil {
		t.Config.Import = []string{}
	}
	importBytes, err := readImports(filepath.Dir(path), t.Config.Import)
	if err != nil {
		return fmt.Errorf("reading import files: %w", err)
	}
	unresolved, err := newUnresolved(append(importBytes, manifest))
	if err != nil {
		return fmt.Errorf("parsing imports: %w", err)
	}
	for name, target := range unresolved {
		if resolved, err := target.resolve(unresolved, name, map[string]bool{}); err != nil {
			return fmt.Errorf("resolving targets: %w", err)
		} else {
			t.Manifest[name] = resolved
		}
	}
	return nil
}

func (t *Terrallel) Runner(target *Target, args []string) *TreeRunner {
	return NewTreeRunner(target, func(name string) *Job {
		prefix := fmt.Sprintf("[%s]: ", name)
		stdout := prefixWriter(t.stdout, prefix)
		stderr := prefixWriter(t.stderr, prefix)
		cmd := exec.Command("terraform", args...)
		cmd.Dir = path.Join(t.Config.Basedir, name)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return &Job{
			Name:   name,
			Cmd:    cmd,
			Stdout: stdout.saved,
			Stderr: stderr.saved,
		}
	})
}

func readImports(basedir string, globs []string) ([][]byte, error) {
	var imports [][]byte
	for _, pattern := range globs {
		paths, err := filepath.Glob(path.Join(basedir, pattern))
		if err != nil {
			return nil, fmt.Errorf("expanding file path: %w", err)
		}
		// a pattern that isn't a glob should be looked for explictly
		if len(paths) == 0 {
			if !strings.ContainsAny(pattern, "*?[]") {
				paths = []string{path.Join(basedir, pattern)}
			}
		}
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading import %s: %w", path, err)
			}
			imports = append(imports, content)
		}
	}
	return imports, nil
}

type unresolved map[string]*target

func newUnresolved(imports [][]byte) (unresolved, error) {
	all := unresolved{}
	for _, content := range imports {
		temp := struct {
			Targets map[string]*target `yaml:"targets"`
		}{}
		if err := yaml.Unmarshal(content, &temp); err != nil {
			return nil, err
		}
		if temp.Targets != nil {
			for name, target := range temp.Targets {
				if _, exists := all[name]; exists {
					return nil, fmt.Errorf("duplicate: %s", name)
				}
				all[name] = target
			}
		}
	}
	return all, nil
}

type target struct {
	parent     string
	Group      []string
	Workspaces []string
	Next       *target
}

func (t *target) resolve(targets unresolved, name string, visited map[string]bool) (*Target, error) {
	if len(t.Group) != 0 && len(t.Workspaces) != 0 {
		return nil, fmt.Errorf("workspaces and group cannot coexist at the same level")
	}
	if visited[name] {
		return nil, fmt.Errorf("recursive loop detected for target %s", name)
	}
	visited[name] = true
	target := &Target{
		Name:       name,
		Workspaces: t.Workspaces,
	}
	var err error
	if len(t.Group) != 0 {
		var children []*Target
		for _, groupName := range t.Group {
			// don't resolve the same target more than once
			if childTarget, ok := targets[groupName]; ok {
				resolvedChild, err := childTarget.resolve(targets, groupName, visited)
				if err != nil {
					return nil, err
				}
				children = append(children, resolvedChild)
			} else {
				return nil, fmt.Errorf("target %s does not exist", groupName)
			}
		}
		target.Group = children
	}
	if t.Next != nil {
		t.Next.parent = t.parent
		target.Next, err = t.Next.resolve(targets, "next", visited)
		if err != nil {
			return nil, err
		}
	}
	delete(visited, name)
	return target, nil
}

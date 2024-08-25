package terrallel

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type manifest struct {
	Targets map[string]*target `yaml:"targets"`
}

func resolveTargets(dest map[string]*Target, imports []string) error {
	m := &manifest{
		Targets: map[string]*target{},
	}
	for _, glob := range imports {
		paths, err := filepath.Glob(glob)
		if err != nil {
			return fmt.Errorf("failure expanding glob pattern %s: %w", glob, err)
		}
		for _, manifestPath := range paths {
			content, err := os.ReadFile(manifestPath)
			if err != nil {
				return fmt.Errorf("failure reading import file %s: %w", manifestPath, err)
			}
			temp := &manifest{
				Targets: map[string]*target{},
			}
			if err := yaml.Unmarshal(content, temp); err != nil {
				return fmt.Errorf("failure unmarshalling: %w", err)
			}
			for name, target := range temp.Targets {
				if _, exists := m.Targets[name]; exists {
					return fmt.Errorf("duplicate target %s found in import file %s", name, manifestPath)
				}
				m.Targets[name] = target
			}
		}
	}
	for name, target := range m.Targets {
		resolved, err := target.build(m, name, dest)
		if err != nil {
			return fmt.Errorf("failure processing target %s: %w", name, err)
		}
		dest[name] = resolved
	}
	return nil
}

type target struct {
	parent     string
	Group      []string
	Workspaces []string
	Next       *target
}

func (t *target) build(m *manifest, name string, dest map[string]*Target) (*Target, error) {
	target := &Target{
		Name:       name,
		Workspaces: t.Workspaces,
	}
	if len(t.Group) != 0 && len(t.Workspaces) != 0 {
		return nil, fmt.Errorf("workspaces and group at same level")
	}
	var err error
	if t.Group != nil {
		if target.Group, err = t.resolve(m, dest); err != nil {
			return nil, err
		}
	}
	if t.Next != nil {
		t.Next.parent = t.parent
		if target.Next, err = t.Next.build(m, "next", dest); err != nil {
			return nil, err
		}
	}
	dest[name] = target
	return target, nil
}

func (t *target) resolve(m *manifest, dest map[string]*Target) ([]*Target, error) {
	var children []*Target
	for _, name := range t.Group {
		if resolved, ok := dest[name]; ok {
			children = append(children, resolved)
			continue
		}
		if unresolvedGroup, ok := m.Targets[name]; ok {
			group, err := unresolvedGroup.build(m, name, dest)
			if err != nil {
				return nil, err
			}
			children = append(children, group)
		} else {
			return nil, fmt.Errorf("group %v does not exist", name)
		}
	}
	return children, nil
}

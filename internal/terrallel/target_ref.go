package terrallel

import (
	"fmt"
)

type TargetRef struct {
	parent     string
	Group      []string
	Workspaces []string
	Next       *TargetRef
}

func (ref *TargetRef) validate() error {
	if len(ref.Group) != 0 && len(ref.Workspaces) != 0 {
		return fmt.Errorf("workspaces and group at same level")
	}
	return nil
}

func (ref *TargetRef) build(t *Terrallel, depth int, name string) (*Target, error) {
	target := &Target{
		Name:       name,
		Workspaces: ref.Workspaces,
	}
	var err error
	if ref.Group != nil {
		if target.Group, err = ref.resolve(t, depth); err != nil {
			return nil, err
		}
	}
	if ref.Next != nil {
		ref.Next.parent = ref.parent
		if target.Next, err = ref.Next.build(t, depth+1, "next"); err != nil {
			return nil, err
		}
	}
	return target, nil
}

func (ref *TargetRef) resolve(t *Terrallel, depth int) ([]*Target, error) {
	var children []*Target
	for _, name := range ref.Group {
		if resolved, ok := t.Manifest[name]; ok {
			children = append(children, resolved)
			continue
		}
		if unresolvedGroup, ok := t.Refs[name]; ok {
			group, err := unresolvedGroup.build(t, depth, name)
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

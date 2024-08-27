package terrallel

type Target struct {
	Name       string
	Group      []*Target `yaml:"group,omitempty"`
	Workspaces []string  `yaml:"workspaces,omitempty"`
	Next       *Target   `yaml:"next,omitempty"`
}

func (t *Target) breadthFirst(fn execWork) *treeRun {
	node := &treeRun{
		name:       t.Name,
		workspaces: make([]*work, len(t.Workspaces)),
		groups:     make([]*treeRun, len(t.Group)),
	}
	for i, ws := range t.Workspaces {
		node.workspaces[i] = fn(ws)
	}
	for i, g := range t.Group {
		node.groups[i] = g.breadthFirst(fn)
	}
	if t.Next != nil {
		node.next = t.Next.breadthFirst(fn)
	}
	return node
}

func (t *Target) depthFirst(fn execWork) *treeRun {
	node := &treeRun{
		name:       t.Name,
		workspaces: make([]*work, len(t.Workspaces)),
		groups:     make([]*treeRun, len(t.Group)),
	}
	if t.Next != nil {
		node.next = t.Next.depthFirst(fn)
	}
	for i, g := range t.Group {
		node.groups[i] = g.depthFirst(fn)
	}
	for i, ws := range t.Workspaces {
		node.workspaces[i] = fn(ws)
	}
	return node
}

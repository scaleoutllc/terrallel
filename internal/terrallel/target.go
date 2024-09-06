package terrallel

type Target struct {
	Name       string
	Group      []*Target `yaml:"group,omitempty"`
	Workspaces []string  `yaml:"workspaces,omitempty"`
	Next       *Target   `yaml:"next,omitempty"`
}

func (t *Target) Runner(fn func(string) Job) *Tree {
	node := &Tree{
		Name:  t.Name,
		Jobs:  make([]Job, len(t.Workspaces)),
		Group: make([]*Tree, len(t.Group)),
	}
	for i, ws := range t.Workspaces {
		node.Jobs[i] = fn(ws)
	}
	for i, g := range t.Group {
		node.Group[i] = g.Runner(fn)
	}
	if t.Next != nil {
		node.Next = t.Next.Runner(fn)
	}
	return node
}

package terrallel

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

type exitTree struct {
	Workspaces []int       `yaml:"workspaces"`
	Groups     []*exitTree `yaml:"groups"`
	Next       *exitTree   `yaml:"next"`
}

func (et *exitTree) String() string {
	yamlData, _ := yaml.Marshal(et)
	return string(yamlData)
}

func exitCmd(name string, code int) *work {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", fmt.Sprintf("exit %d", code))
	} else {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
	}
	return &work{
		name:   name,
		cmd:    cmd,
		stdout: new(bytes.Buffer),
		stderr: new(bytes.Buffer),
	}
}

func collectExits(tree *treeRun) *exitTree {
	et := &exitTree{}
	for _, ws := range tree.workspaces {
		et.Workspaces = append(et.Workspaces, ws.exitCode())
	}
	for _, group := range tree.groups {
		et.Groups = append(et.Groups, collectExits(group))
	}
	if tree.next != nil {
		et.Next = collectExits(tree.next)
	}
	return et
}

func TestRunTreeDo(t *testing.T) {
	tests := []struct {
		name     string
		root     *treeRun
		expected *exitTree
	}{
		{
			name: "clean exit allows traversal of tree",
			root: &treeRun{
				name: "parent",
				workspaces: []*work{
					exitCmd("parent.1", 0),
					exitCmd("parent.2", 0),
				},
				next: &treeRun{
					name: "child",
					workspaces: []*work{
						exitCmd("child.1", 0),
					},
				},
			},
			expected: &exitTree{
				Workspaces: []int{0, 0},
				Next: &exitTree{
					Workspaces: []int{0},
				},
			},
		},
		{
			name: "sibling failures don't affect eachother",
			root: &treeRun{
				name: "parent",
				workspaces: []*work{
					exitCmd("parent.1", 0),
					exitCmd("parent.2", 1),
					exitCmd("parent.3", 0),
					exitCmd("parent.4", 0),
					exitCmd("parent.5", 0),
				},
			},
			expected: &exitTree{
				Workspaces: []int{0, 1, 0, 0, 0},
			},
		},
		{
			name: "failure in parent prevents traversal of children",
			root: &treeRun{
				name: "parent",
				workspaces: []*work{
					exitCmd("parent.1", 0),
					exitCmd("parent.2", 1),
				},
				next: &treeRun{
					name: "child",
					workspaces: []*work{
						exitCmd("child.1", 0),
					},
				},
			},
			expected: &exitTree{
				Workspaces: []int{0, 1},
				Next: &exitTree{
					Workspaces: []int{-1},
				},
			},
		},
		{
			name: "failure stops outer children but allows sibling to complete",
			root: &treeRun{
				groups: []*treeRun{
					{
						workspaces: []*work{
							exitCmd("sibling.1", 1),
						},
						next: &treeRun{
							workspaces: []*work{
								exitCmd("skipped", 0),
							},
						},
					},
					{
						workspaces: []*work{
							exitCmd("sibling.2", 0),
						},
						next: &treeRun{
							workspaces: []*work{
								exitCmd("child.1", 0),
							},
						},
					},
				},
				next: &treeRun{
					workspaces: []*work{
						exitCmd("last", 0),
					},
				},
			},
			expected: &exitTree{
				Groups: []*exitTree{
					{
						Workspaces: []int{1},
						Next: &exitTree{
							Workspaces: []int{-1},
						},
					},
					{
						Workspaces: []int{0},
						Next: &exitTree{
							Workspaces: []int{0},
						},
					},
				},
				Next: &exitTree{
					Workspaces: []int{-1},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.root.Do(context.Background())
			got := collectExits(tt.root)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

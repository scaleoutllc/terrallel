package terrallel_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scaleoutllc/terrallel/internal/terrallel"
	"gopkg.in/yaml.v3"
)

type jobMock struct {
	name       string
	runtime    int
	errWhenRun bool
	result
}

type result struct {
	started     bool
	finished    bool
	interrupted bool
	errored     bool
}

func (j *jobMock) Run(dryrun bool) error {
	j.result.started = true
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(time.Duration(j.runtime) * time.Millisecond)
		j.result.finished = true
	}()
	wg.Wait()
	if j.interrupted {
		return errors.New("interrupted")
	}
	if j.errWhenRun {
		j.result.errored = true
		return errors.New("failure")
	}
	return nil
}

func (j *jobMock) Result() string {
	result := "DidNotRun"
	if j.result.started {
		if j.result.finished {
			result = "Success"
			if j.result.errored {
				result = "Failure"
			}
		}
	}
	if j.result.interrupted {
		result = "Interrupted"
	}
	if j.name != "" {
		return fmt.Sprintf("%s: %s", j.name, result)
	}
	return result
}

func (j *jobMock) Cancel() error {
	if j.result.started {
		j.result.interrupted = true
	}
	return nil
}

type resultTree struct {
	Results []string      `yaml:"results"`
	Group   []*resultTree `yaml:"groups"`
	Next    *resultTree   `yaml:"next"`
}

func (rt *resultTree) String() string {
	yamlData, _ := yaml.Marshal(rt)
	return string(yamlData)
}

func collectResults(tree *terrallel.Tree) *resultTree {
	rt := &resultTree{}
	for _, j := range tree.Jobs {
		rt.Results = append(rt.Results, j.Result())
	}
	for _, group := range tree.Group {
		rt.Group = append(rt.Group, collectResults(group))
	}
	if tree.Next != nil {
		rt.Next = collectResults(tree.Next)
	}
	return rt
}

func TestTreeForward(t *testing.T) {
	tests := []struct {
		name     string
		runner   *terrallel.Tree
		expected *resultTree
	}{
		{
			name: "clean exit allows traversal of tree",
			runner: &terrallel.Tree{
				Jobs: []terrallel.Job{
					&jobMock{runtime: 10},
					&jobMock{runtime: 10},
				},
				Next: &terrallel.Tree{
					Jobs: []terrallel.Job{
						&jobMock{runtime: 10},
					},
				},
			},
			expected: &resultTree{
				Results: []string{
					"Success",
					"Success",
				},
				Next: &resultTree{
					Results: []string{"Success"},
				},
			},
		},
		{
			name: "sibling failures don't affect each other",
			runner: &terrallel.Tree{
				Jobs: []terrallel.Job{
					&jobMock{runtime: 30},
					&jobMock{runtime: 20},
					&jobMock{runtime: 10, errWhenRun: true},
					&jobMock{runtime: 40},
					&jobMock{runtime: 30},
				},
			},
			expected: &resultTree{
				Results: []string{
					"Success",
					"Success",
					"Failure",
					"Success",
					"Success",
				},
			},
		},
		{
			name: "failure in parent prevents traversal of children",
			runner: &terrallel.Tree{
				Jobs: []terrallel.Job{
					&jobMock{runtime: 10},
					&jobMock{runtime: 10, errWhenRun: true},
				},
				Next: &terrallel.Tree{
					Jobs: []terrallel.Job{
						&jobMock{runtime: 10},
					},
				},
			},
			expected: &resultTree{
				Results: []string{
					"Success",
					"Failure",
				},
				Next: &resultTree{
					Results: []string{"DidNotRun"},
				},
			},
		},
		{
			name: "failure stops outer children but allows sibling to complete",
			runner: &terrallel.Tree{
				Group: []*terrallel.Tree{
					{
						Jobs: []terrallel.Job{
							&jobMock{runtime: 10, errWhenRun: true},
						},
						Next: &terrallel.Tree{
							Jobs: []terrallel.Job{
								&jobMock{runtime: 10},
							},
						},
					},
					{
						Jobs: []terrallel.Job{
							&jobMock{runtime: 10},
						},
						Next: &terrallel.Tree{
							Jobs: []terrallel.Job{
								&jobMock{runtime: 10},
							},
						},
					},
				},
				Next: &terrallel.Tree{
					Jobs: []terrallel.Job{
						&jobMock{runtime: 10},
					},
				},
			},
			expected: &resultTree{
				Group: []*resultTree{
					{
						Results: []string{"Failure"},
						Next: &resultTree{
							Results: []string{"DidNotRun"},
						},
					},
					{
						Results: []string{"Success"},
						Next: &resultTree{
							Results: []string{"Success"},
						},
					},
				},
				Next: &resultTree{
					Results: []string{"DidNotRun"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.runner.Forward(context.Background(), false)
			got := collectResults(tt.runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

func TestTreeReverse(t *testing.T) {
	tests := []struct {
		name     string
		runner   *terrallel.Tree
		expected *resultTree
	}{
		{
			name: "clean exit allows traversal of tree",
			runner: &terrallel.Tree{
				Jobs: []terrallel.Job{
					&jobMock{runtime: 10},
					&jobMock{runtime: 10},
				},
				Next: &terrallel.Tree{
					Jobs: []terrallel.Job{
						&jobMock{runtime: 10},
					},
				},
			},
			expected: &resultTree{
				Results: []string{
					"Success",
					"Success",
				},
				Next: &resultTree{
					Results: []string{"Success"},
				},
			},
		},
		{
			name: "sibling failures don't affect each other",
			runner: &terrallel.Tree{
				Jobs: []terrallel.Job{
					&jobMock{runtime: 30},
					&jobMock{runtime: 20},
					&jobMock{runtime: 10, errWhenRun: true},
					&jobMock{runtime: 40},
					&jobMock{runtime: 30},
				},
			},
			expected: &resultTree{
				Results: []string{
					"Success",
					"Success",
					"Failure",
					"Success",
					"Success",
				},
			},
		},
		{
			name: "failure in parent prevents traversal of children",
			runner: &terrallel.Tree{
				Jobs: []terrallel.Job{
					&jobMock{runtime: 10},
					&jobMock{runtime: 10},
				},
				Next: &terrallel.Tree{
					Jobs: []terrallel.Job{
						&jobMock{runtime: 10, errWhenRun: true},
					},
				},
			},
			expected: &resultTree{
				Results: []string{"DidNotRun", "DidNotRun"},
				Next: &resultTree{
					Results: []string{"Failure"},
				},
			},
		},
		{
			name: "failure stops outer children but allows sibling to complete",
			runner: &terrallel.Tree{
				Group: []*terrallel.Tree{
					{
						Jobs: []terrallel.Job{
							&jobMock{runtime: 10},
						},
						Next: &terrallel.Tree{
							Jobs: []terrallel.Job{
								&jobMock{runtime: 10},
							},
						},
					},
					{
						Jobs: []terrallel.Job{
							&jobMock{runtime: 10},
						},
						Next: &terrallel.Tree{
							Jobs: []terrallel.Job{
								&jobMock{runtime: 10, errWhenRun: true},
							},
							Next: &terrallel.Tree{
								Jobs: []terrallel.Job{
									&jobMock{runtime: 10},
								},
							},
						},
					},
				},
			},
			expected: &resultTree{
				Group: []*resultTree{
					{
						Results: []string{"Success"},
						Next: &resultTree{
							Results: []string{"Success"},
						},
					},
					{
						Results: []string{"DidNotRun"},
						Next: &resultTree{
							Results: []string{"Failure"},
							Next: &resultTree{
								Results: []string{"Success"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.runner.Reverse(context.Background(), false)
			got := collectResults(tt.runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

func TestTreeCancel(t *testing.T) {
	tests := []struct {
		name        string
		runner      *terrallel.Tree
		expected    *resultTree
		exitAfterMs int64
	}{
		{
			name:        "cancelling context interrupts running processes and stops dependents",
			exitAfterMs: 50,
			runner: &terrallel.Tree{
				Group: []*terrallel.Tree{
					{
						Jobs: []terrallel.Job{
							&jobMock{runtime: 5},
							&jobMock{runtime: 5},
						},
						Next: &terrallel.Tree{
							Jobs: []terrallel.Job{
								&jobMock{runtime: 250},
							},
							Next: &terrallel.Tree{
								Jobs: []terrallel.Job{
									&jobMock{runtime: 10},
								},
							},
						},
					},
					{
						Jobs: []terrallel.Job{
							&jobMock{runtime: 250},
							&jobMock{runtime: 5},
						},
						Next: &terrallel.Tree{
							Jobs: []terrallel.Job{
								&jobMock{runtime: 10},
							},
						},
					},
				},
			},
			expected: &resultTree{
				Group: []*resultTree{
					{
						Results: []string{
							"Success",
							"Success",
						},
						Next: &resultTree{
							Results: []string{"Interrupted"},
							Next: &resultTree{
								Results: []string{"DidNotRun"},
							},
						},
					},
					{
						Results: []string{"Interrupted", "Success"},
						Next: &resultTree{
							Results: []string{"DidNotRun"},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				tt.runner.Forward(ctx, false)
			}()
			time.Sleep(time.Duration(tt.exitAfterMs) * time.Millisecond)
			cancel()
			wg.Wait()
			got := collectResults(tt.runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

func TestTreeReport(t *testing.T) {
	runner := &terrallel.Tree{
		Group: []*terrallel.Tree{
			{
				Name: "parent1",
				Jobs: []terrallel.Job{
					&jobMock{name: "a", runtime: 50},
					&jobMock{name: "b", runtime: 50},
				},
				Next: &terrallel.Tree{
					Name: "child1",
					Jobs: []terrallel.Job{
						&jobMock{name: "c", runtime: 50},
					},
					Next: &terrallel.Tree{
						Name: "grandchild1",
						Jobs: []terrallel.Job{
							&jobMock{name: "d", runtime: 10},
						},
					},
				},
			},
			{
				Name: "parent2",
				Jobs: []terrallel.Job{
					&jobMock{name: "a", runtime: 150},
					&jobMock{name: "b", runtime: 10},
				},
				Next: &terrallel.Tree{
					Name: "child2",
					Jobs: []terrallel.Job{
						&jobMock{name: "c", runtime: 10},
					},
				},
			},
		},
	}
	expected := `
└─ groups
  ├─ parent1
  │ ├─ workspaces
  │ │ ├─ a: DidNotRun
  │ │ └─ b: DidNotRun
  │ └─ next
  │   ├─ workspaces
  │   │ └─ c: DidNotRun
  │   └─ next
  │     └─ workspaces
  │       └─ d: DidNotRun
  └─ parent2
    ├─ workspaces
    │ ├─ a: DidNotRun
    │ └─ b: DidNotRun
    └─ next
      └─ workspaces
        └─ c: DidNotRun
`
	actual := runner.String()
	if expected != actual {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

package terrallel_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scaleoutllc/terrallel/internal/terrallel"
	"gopkg.in/yaml.v2"
)

type exitTree struct {
	Workspaces []int       `yaml:"workspaces"`
	Group      []*exitTree `yaml:"groups"`
	Next       *exitTree   `yaml:"next"`
}

func (et *exitTree) String() string {
	yamlData, _ := yaml.Marshal(et)
	return string(yamlData)
}

func collectExits(tree *terrallel.TreeRunner) *exitTree {
	et := &exitTree{}
	for _, ws := range tree.Workspaces {
		et.Workspaces = append(et.Workspaces, ws.ExitCode())
	}
	for _, group := range tree.Group {
		et.Group = append(et.Group, collectExits(group))
	}
	if tree.Next != nil {
		et.Next = collectExits(tree.Next)
	}
	return et
}

func testWorker(ws string) *terrallel.Job {
	// Regular expression to parse delay and exit codes
	re := regexp.MustCompile(`\.delay\((\d+)\)\.exit\((\d+)\)`)
	matches := re.FindStringSubmatch(ws)

	delayms := 0
	exit := 0
	if len(matches) == 3 {
		delayms, _ = strconv.Atoi(matches[1])
		exit, _ = strconv.Atoi(matches[2])
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", fmt.Sprintf(`
@echo off
SET WORKSPACE="%s"
SET /A SLEEP=%d/1000
SET EXITCODE=%d
echo running %%WORKSPACE%% for %%SLEEP%% seconds then exiting %%EXITCODE%%
rem Trap CTRL+C (Interrupt signal)
:loop
ping -n %%SLEEP%% 127.0.0.1 > nul || goto interrupted
goto end

:interrupted
echo INTERRUPTED
exit /b 1

:end
exit /b %%EXITCODE%%
		`, ws, delayms, exit))
	} else {
		cmd = exec.Command("sh", "-c", fmt.Sprintf(`
trap 'echo INTERRUPTED; exit 130' INT
WORKSPACE="%s"
SLEEP="%.3f"
EXIT=%d
echo "running ${WORKSPACE} for ${SLEEP}s then exiting ${EXIT}"
sleep ${SLEEP}
exit ${EXIT}
		`, ws, float64(delayms)/1000, exit))
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return &terrallel.Job{
		Name:   ws,
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
	}
}

func TestRunTreeForward(t *testing.T) {
	tests := []struct {
		name     string
		root     *terrallel.Target
		expected *exitTree
	}{
		{
			name: "clean exit allows traversal of tree",
			root: &terrallel.Target{
				Workspaces: []string{"a.delay(10).exit(0)", "b.delay(10).exit(0)"},
				Next: &terrallel.Target{
					Workspaces: []string{"c.delay(10).exit(0)"},
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
			name: "sibling failures don't affect each other",
			root: &terrallel.Target{
				Workspaces: []string{"a.delay(30).exit(0)", "b.delay(20).exit(0)", "c.delay(10).exit(1)", "d.delay(40).exit(0)", "e.delay(30).exit(0)"},
			},
			expected: &exitTree{
				Workspaces: []int{0, 0, 1, 0, 0},
			},
		},
		{
			name: "failure in parent prevents traversal of children",
			root: &terrallel.Target{
				Workspaces: []string{"a.delay(10).exit(0)", "b.delay(10).exit(1)"},
				Next: &terrallel.Target{
					Workspaces: []string{"c.delay(10).exit(0)"},
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
			root: &terrallel.Target{
				Group: []*terrallel.Target{
					{
						Workspaces: []string{"a.delay(10).exit(1)"},
						Next: &terrallel.Target{
							Workspaces: []string{"b.delay(10).exit(0)"},
						},
					},
					{
						Workspaces: []string{"c.delay(10).exit(0)"},
						Next: &terrallel.Target{
							Workspaces: []string{"d.delay(10).exit(0)"},
						},
					},
				},
				Next: &terrallel.Target{
					Workspaces: []string{"last.delay(10).exit(0)"},
				},
			},
			expected: &exitTree{
				Group: []*exitTree{
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
			runner := terrallel.NewTreeRunner(tt.root, testWorker)
			runner.Forward(context.Background())
			got := collectExits(runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

func TestRunTreeReverse(t *testing.T) {
	tests := []struct {
		name     string
		root     *terrallel.Target
		expected *exitTree
	}{
		{
			name: "clean exit allows traversal of tree",
			root: &terrallel.Target{
				Workspaces: []string{"c.delay(10).exit(0)", "b.delay(10).exit(0)"},
				Next: &terrallel.Target{
					Workspaces: []string{"a.delay(10).exit(0)"},
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
			name: "sibling failures don't affect each other",
			root: &terrallel.Target{
				Workspaces: []string{"a.delay(30).exit(0)", "b.delay(20).exit(0)", "c.delay(10).exit(1)", "d.delay(40).exit(0)", "e.delay(30).exit(0)"},
			},
			expected: &exitTree{
				Workspaces: []int{0, 0, 1, 0, 0},
			},
		},
		{
			name: "failure in parent prevents traversal of children",
			root: &terrallel.Target{
				Workspaces: []string{"c.delay(10).exit(0)", "b.delay(10).exit(0)"},
				Next: &terrallel.Target{
					Workspaces: []string{"a.delay(10).exit(1)"},
				},
			},
			expected: &exitTree{
				Workspaces: []int{-1, -1},
				Next: &exitTree{
					Workspaces: []int{1},
				},
			},
		},
		{
			name: "failure stops outer children but allows sibling to complete",
			root: &terrallel.Target{
				Group: []*terrallel.Target{
					{
						Workspaces: []string{"e.delay(10).exit(0)"},
						Next: &terrallel.Target{
							Workspaces: []string{"d.delay(10).exit(0)"},
						},
					},
					{
						Workspaces: []string{"c.delay(10).exit(0)"},
						Next: &terrallel.Target{
							Workspaces: []string{"b.delay(10).exit(1)"},
							Next: &terrallel.Target{
								Workspaces: []string{"a.delay(10).exit(0)"},
							},
						},
					},
				},
			},
			expected: &exitTree{
				Group: []*exitTree{
					{
						Workspaces: []int{0},
						Next: &exitTree{
							Workspaces: []int{0},
						},
					},
					{
						Workspaces: []int{-1},
						Next: &exitTree{
							Workspaces: []int{1},
							Next: &exitTree{
								Workspaces: []int{0},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := terrallel.NewTreeRunner(tt.root, testWorker)
			runner.Reverse(context.Background())
			got := collectExits(runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

func TestRunSignal(t *testing.T) {
	tests := []struct {
		name        string
		root        *terrallel.Target
		expected    *exitTree
		exitAfterMs int64
	}{
		{
			name:        "sigint terminates running processes at any level, skips dependent work",
			exitAfterMs: 75,
			root: &terrallel.Target{
				Group: []*terrallel.Target{
					{
						Workspaces: []string{"a.delay(50).exit(0)", "b.delay(50).exit(0)"},
						Next: &terrallel.Target{
							Workspaces: []string{"c.delay(50).exit(0)"},
							Next: &terrallel.Target{
								Workspaces: []string{"d.delay(10).exit(0)"},
							},
						},
					},
					{
						Workspaces: []string{"e.delay(75).exit(0)", "f.delay(10).exit(0)"},
						Next: &terrallel.Target{
							Workspaces: []string{"g.delay(10).exit(0)"},
						},
					},
				},
			},
			expected: &exitTree{
				Group: []*exitTree{
					{
						Workspaces: []int{0, 0},
						Next: &exitTree{
							Workspaces: []int{130},
							Next: &exitTree{
								Workspaces: []int{-1},
							},
						},
					},
					{
						Workspaces: []int{130, 0},
						Next: &exitTree{
							Workspaces: []int{-1},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := &sync.WaitGroup{}
			wg.Add(1)
			runner := terrallel.NewTreeRunner(tt.root, testWorker)
			go func() {
				defer wg.Done()
				runner.Forward(context.Background())
			}()
			time.Sleep(time.Duration(tt.exitAfterMs) * time.Millisecond)
			runner.Signal(os.Interrupt)
			wg.Wait()
			got := collectExits(runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

func TestRunContextCancel(t *testing.T) {
	tests := []struct {
		name          string
		root          *terrallel.Target
		expected      *exitTree
		cancelAfterMs int64
	}{
		{
			name:          "caoncelled context prevents future work from being scheduled, has no effect on running",
			cancelAfterMs: 75,
			root: &terrallel.Target{
				Group: []*terrallel.Target{
					{
						Workspaces: []string{"a.delay(50).exit(0)", "b.delay(50).exit(0)"},
						Next: &terrallel.Target{
							Workspaces: []string{"c.delay(100).exit(0)"},
							Next: &terrallel.Target{
								Workspaces: []string{"d.delay(10).exit(0)"},
							},
						},
					},
				},
			},
			expected: &exitTree{
				Group: []*exitTree{
					{
						Workspaces: []int{0, 0},
						Next: &exitTree{
							Workspaces: []int{0},
							Next: &exitTree{
								Workspaces: []int{-1},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := &sync.WaitGroup{}
			wg.Add(1)
			ctx, cancel := context.WithCancel(context.Background())
			runner := terrallel.NewTreeRunner(tt.root, testWorker)
			go func() {
				defer wg.Done()
				runner.Forward(ctx)
			}()
			time.Sleep(time.Duration(tt.cancelAfterMs) * time.Millisecond)
			cancel()
			wg.Wait()
			got := collectExits(runner)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Fatalf("exit trees do not match, expected\n%s\n---\ngot\n%s", tt.expected, got)
			}
		})
	}
}

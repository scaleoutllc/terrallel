package terrallel

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// command generates the command to be executed based on the current OS
func command() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"cmd", "/C", `if exist bad* (echo fail && exit /b 1) else (echo success && exit /b 0)`}
	default: // Linux and macOS
		return []string{"sh", "-c", `
			basename=$(basename "$PWD")
			if [[ "$basename" == bad* ]]; then 
				echo fail && exit 1
			else 
				echo success
			fi
		`}
	}
}

func TestRunnerParallelGroupFailure(t *testing.T) {
	tests := []struct {
		name    string
		runner  *runner
		target  *Target
		command []string
	}{
		{
			name: "Parallel group with one failure per group",
			runner: &runner{
				terrallel: &Terrallel{
					Config: &Config{
						Basedir: t.TempDir(),
					},
					stdout: &bytes.Buffer{},
					stderr: &bytes.Buffer{},
				},
				command: command()[0],
				args:    command()[1:],
			},
			target: &Target{
				Name: "root",
				Group: []*Target{
					{
						Name:       "group1",
						Workspaces: []string{"good", "bad", "good"},
						Next: &Target{
							Workspaces: []string{"skipped"},
						},
					},
					{
						Name:       "group2",
						Workspaces: []string{"good", "good", "good"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, dir := range []string{"good", "bad", "skipped"} {
				err := os.Mkdir(filepath.Join(tt.runner.terrallel.Config.Basedir, dir), 0755)
				if err != nil {
					t.Fatalf("Failed to create workspace directory %s: %v", dir, err)
				}
			}
			_, procs, _ := tt.runner.start(tt.target)
			for _, proc := range procs {
				fmt.Printf("found %v\n", proc.ProcessState.ExitCode())
				//state := proc.ProcessState
				//if state == nil {
				//	t.Errorf("Process %s did not complete", proc.Path)
				//} else {
				//	exitCode := state.ExitCode()
				//	t.Logf("Process %s exited with code %d", proc.Path, exitCode)
				//}
			}
		})
	}
}

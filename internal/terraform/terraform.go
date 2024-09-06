package terraform

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

type Job struct {
	Name    string
	Basedir string
	Bin     string
	Args    []string
	Stdout  io.Writer
	Stderr  io.Writer
	cmd     *exec.Cmd
	result  string
}

func (j *Job) Run(dryrun bool) error {
	if j.Bin == "" {
		j.Bin = "terraform"
	}
	prefix := fmt.Sprintf("[%s]: ", j.Name)
	stdout := prefixWriter(j.Stdout, prefix)
	stderr := prefixWriter(j.Stderr, prefix)
	cmd := exec.Command(j.Bin, j.Args...)
	if j.Basedir != "" {
		cmd.Dir = filepath.Join(j.Basedir, j.Name)
	}
	cmd.SysProcAttr = procAttrs
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	j.cmd = cmd
	runInfo := fmt.Sprintf("%s %s (in %s)", j.Bin, strings.Join(j.Args, " "), cmd.Dir)
	if dryrun {
		j.Stdout.Write([]byte(fmt.Sprintf("%s\n", runInfo)))
		return nil
	}
	if err := j.cmd.Start(); err != nil {
		j.result = color.RedString("failed-to-start")
		return fmt.Errorf("failed-to-start: %s: %w", runInfo, err)
	}
	if err := j.cmd.Wait(); err != nil {
		if j.result != color.YellowString("interrupted") {
			j.result = color.RedString("failed")
		}
		return fmt.Errorf("run: %s: %w", runInfo, err)
	}
	j.result = color.GreenString("success")
	return nil
}

func (j *Job) Cancel() error {
	if j.cmd != nil && j.cmd.Process != nil {
		j.result = color.YellowString("interrupted")
		return interrupt(j.cmd)
	}
	return nil
}

func (j *Job) Result() string {
	if j.result == "" {
		j.result = color.CyanString("never-ran")
	}
	return fmt.Sprintf("%s: %s", j.Name, j.result)
}

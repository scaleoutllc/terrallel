package cli

import (
	"fmt"
	"os"

	"github.com/scaleoutllc/terrallel/internal/terraform"
	"github.com/scaleoutllc/terrallel/internal/terrallel"
)

func Root(
	manifestPath string,
	targetName string,
	args []string,
	dryrun bool,
) error {
	var reverse bool
	for _, arg := range args {
		if arg == "destroy" {
			reverse = true
		}
	}
	infra, err := terrallel.New(manifestPath)
	if err != nil {
		return err
	}
	target, ok := infra.Manifest[targetName]
	if !ok {
		return fmt.Errorf("target %s not found", targetName)
	}
	runner := target.Runner(func(name string) terrallel.Job {
		return &terraform.Job{
			Name:    name,
			Basedir: infra.Config.Basedir,
			Args:    args,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}
	})
	err = runner.Do(reverse, dryrun)
	if !dryrun {
		os.Stdout.Write([]byte("\n" + runner.String()))
	}
	return err
}

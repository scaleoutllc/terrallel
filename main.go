package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/scaleoutllc/terrallel/internal/terrallel"
)

func main() {

	var manifestPath string
	flag.StringVar(&manifestPath, "m", "Infrafile", "Path to the terrallel manifest file (default: Infrafile)")
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		fail(errors.New("terrallel [-m] <target> -- <terraform command>"))
	}
	target := args[0]
	sepIndex := -1
	for i, arg := range args {
		if arg == "--" {
			sepIndex = i
			break
		}
	}
	if sepIndex == -1 || sepIndex == len(args)-1 {
		fail(errors.New("missing or incomplete -- separator"))
	}
	infra, err := terrallel.New(manifestPath, os.Stdout, os.Stderr)
	if err != nil {
		fail(err)
	}
	tfArgs := args[sepIndex+1:]
	results, err := infra.Run("terraform", tfArgs, target)
	fmt.Println(results)
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Println(err)
	os.Exit(1)
}

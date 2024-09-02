package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	infra := terrallel.New(os.Stdout, os.Stderr)
	if err := infra.Load(manifestPath); err != nil {
		fail(err)
	}
	targetName := args[0]
	target, ok := infra.Manifest[targetName]
	if !ok {
		fail(fmt.Errorf("target %s not found", targetName))
	}
	tfArgs, err := argsAfterDoubleDash(args)
	if err != nil {
		fail(err)
	}
	runner := infra.Runner(target, tfArgs)

	ctx, cancel := context.WithCancel(context.Background())
	termReceived := false
	termMessage := false
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChan {
			if termReceived {
				if !termMessage {
					termMessage = true
					os.Stderr.Write([]byte("\nTerrallel forcefully shutting down...\n"))
				}
			} else {
				os.Stderr.Write([]byte("\nTerrallel shutting down gracefully...\n"))
				termReceived = true
			}
			cancel()
			runner.Signal(sig)
		}
	}()
	var treeErr error
	var reverse bool
	for _, arg := range tfArgs {
		if arg == "destroy" {
			reverse = true
		}
	}
	if reverse {
		treeErr = runner.Reverse(ctx)
	} else {
		treeErr = runner.Forward(ctx)
	}
	fmt.Println(runner)
	if treeErr != nil {
		fail(treeErr)
	}
}

func argsAfterDoubleDash(args []string) ([]string, error) {
	sepIndex := -1
	for i, arg := range args {
		if arg == "--" {
			sepIndex = i
			break
		}
	}
	if sepIndex == -1 || sepIndex == len(args)-1 {
		return nil, fmt.Errorf("missing or incomplete -- separator")
	}
	return args[sepIndex+1:], nil
}

func fail(err error) {
	fmt.Println(err)
	os.Exit(1)
}

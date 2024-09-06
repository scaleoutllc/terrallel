package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"
)

func main() {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	go func() {
		<-sigchan
		fmt.Println("Interrupted.")
		os.Exit(1)
	}()
	fmt.Println("Running for 500ms...")
	time.Sleep(time.Duration(500) * time.Millisecond)
}

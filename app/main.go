package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {

	// Uncomment this block to pass the first stage!
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	cmd := exec.Command(command, args...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			os.Exit(err.ExitCode())
		default:
			fmt.Printf("Child process exited abnormally %s", err.Error())
			os.Exit(255)
		}
	}

}

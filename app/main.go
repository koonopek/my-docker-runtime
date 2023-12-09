package main

import (
	"fmt"
	"os"
	"os/exec"
)

const JAIL_DIR = "jail"

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {

	// Uncomment this block to pass the first stage!
	command := os.Args[3]
	userArgs := os.Args[4:len(os.Args)]

	os.Mkdir(JAIL_DIR, 0666)

	args := append([]string{JAIL_DIR, command}, userArgs...)

	fmt.Printf("args %s", args)

	cmd := exec.Command("chroot", args...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	switch err := err.(type) {
	case nil:
		os.Exit(0)
	case *exec.ExitError:
		os.Exit(err.ExitCode())
	default:
		fmt.Printf("Child process exited abnormally %s", err.Error())
		os.Exit(124)
	}

}

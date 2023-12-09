package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
)

const JAIL_DIR = "jail"

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	var err error

	// Uncomment this block to pass the first stage!
	command := os.Args[3]
	userArgs := os.Args[4:len(os.Args)]

	os.Mkdir(JAIL_DIR, 0666)

	copyFrom, err := os.Open(command)
	if err != nil {
		fmt.Printf("Failed to open %s, error %s", command, err.Error())
		os.Exit(1)
	}
	defer copyFrom.Close()

	newPath := path.Join(JAIL_DIR, command)

	os.MkdirAll(path.Dir(newPath), 0666)

	copyTo, err := os.OpenFile(newPath, os.O_CREATE|os.O_RDWR, os.ModeAppend)
	if err != nil {
		fmt.Printf("Failed to open %s, error %s", newPath, err.Error())
		os.Exit(2)
	}
	defer copyTo.Close()

	bytesCopied, err := io.Copy(copyFrom, copyTo)
	if err != nil {
		fmt.Printf("Failed to copy files, error %s", err.Error())
		os.Exit(3)
	}
	fmt.Printf("Succesfully copied %d from %s to %s", bytesCopied, command, newPath)

	args := append([]string{JAIL_DIR, command}, userArgs...)

	cmd := exec.Command("chroot", args...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()

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

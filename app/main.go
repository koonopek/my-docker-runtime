//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"syscall"
)

const JAIL_DIR = "jail"

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	var err error

	// Uncomment this block to pass the first stage!
	command := os.Args[3]
	userArgs := os.Args[4:len(os.Args)]

	os.Mkdir(JAIL_DIR, 0777)

	copyFrom, err := os.Open(command)
	if err != nil {
		panic(fmt.Sprintf("Failed to open %s, error %s", command, err.Error()))
	}
	defer copyFrom.Close()

	newPath := path.Join(JAIL_DIR, command)

	os.MkdirAll(path.Dir(newPath), 0777)

	copyTo, err := os.OpenFile(newPath, os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		panic(fmt.Sprintf("Failed to open %s, error %s", newPath, err.Error()))
	}

	_, err = io.Copy(copyTo, copyFrom)
	if err != nil {
		panic(fmt.Sprintf("Failed to copy file %s, error %s", newPath, err.Error()))
	}

	copyTo.Close()

	args := append([]string{JAIL_DIR, command}, userArgs...)

	cmd := exec.Command("chroot", args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(syscall.CLONE_NEWPID),
	}

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

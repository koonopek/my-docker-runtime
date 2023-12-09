//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"syscall"
)

const JAIL_DIR = "jail"

func main() {
	var err error

	command := os.Args[3]
	userArgs := os.Args[4:len(os.Args)]

	os.Mkdir(JAIL_DIR, 0777)

	copyFileToJail(command)

	err = runInContainer(command, userArgs, err)

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

func runInContainer(command string, userArgs []string, err error) error {
	args := append([]string{JAIL_DIR, command}, userArgs...)
	cmd := exec.Command("chroot", args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(syscall.CLONE_NEWPID),
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func copyFileToJail(toCopy string) {
	copyFrom, err := os.Open(toCopy)
	if err != nil {
		panic(fmt.Sprintf("Failed to open %s, error %s", toCopy, err.Error()))
	}
	defer copyFrom.Close()

	copyToPath := path.Join(JAIL_DIR, toCopy)

	os.MkdirAll(path.Dir(copyToPath), 0777)

	copyTo, err := os.OpenFile(copyToPath, os.O_CREATE|os.O_RDWR, 0777)
	if err != nil {
		panic(fmt.Sprintf("Failed to open %s, error %s", copyToPath, err.Error()))
	}

	_, err = io.Copy(copyTo, copyFrom)
	if err != nil {
		panic(fmt.Sprintf("Failed to copy file %s, error %s", copyToPath, err.Error()))
	}

	copyTo.Close()
}

func fetchImage(imageName *string) error {
	response, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", imageName))

	if err != nil || response.StatusCode != 200 {
		return fmt.Errorf("Failed to fetch authorization token")
	}

	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return fmt.Errorf("Failed to read body")
	}

	fmt.Sprintf(string(body))

	return nil
}

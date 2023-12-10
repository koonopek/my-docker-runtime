package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"syscall"
)

const JAIL_DIR = "jail"

func main() {
	var err error

	imageName := "ubuntu"
	err = fetchImage(imageName, JAIL_DIR)
	if err != nil {
		fmt.Printf("Failed to fetch image, error: %s", err.Error())
		os.Exit(1)
	}

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

	// other flags could be added to make it more docker like network namespaces for example
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

type AuthOutput struct {
	Token string `json:"token"`
}

type ManifestOutput struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int    `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

func fetchImage(imageName string, dst string) error {
	httpClient := http.Client{}

	authUrl, err := url.Parse(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", imageName))

	if err != nil {
		return fmt.Errorf("Failed to parse auth url, error: %s", err.Error())
	}

	authResponse, err := httpClient.Do(&http.Request{
		URL: authUrl,
	})

	if err != nil || authResponse.StatusCode != 200 {
		return fmt.Errorf("Failed to fetch authorization token, error: %s", err.Error())
	}

	body, err := io.ReadAll(authResponse.Body)

	if err != nil {
		return fmt.Errorf("Failed to read body")
	}

	authOutput := AuthOutput{}
	err = json.Unmarshal(body, &authOutput)

	if err != nil {
		return fmt.Errorf("Failed to unmarshal auth response")
	}

	headers := map[string][]string{
		"Accept":        {"application/vnd.docker.distribution.manifest.v2+json"},
		"Authorization": {fmt.Sprintf("Bearer %s", authOutput.Token)},
	}

	manifestUrl, err := url.Parse(fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/latest", imageName))

	request := http.Request{Method: "GET", URL: manifestUrl, Header: headers}
	manifestResponse, err := httpClient.Do(&request)

	if err != nil || manifestResponse.StatusCode != 200 {
		return fmt.Errorf("Failed to fetch manifest, error %d", manifestResponse.StatusCode)
	}

	manifestBody, err := io.ReadAll(manifestResponse.Body)

	if err != nil {
		return fmt.Errorf("Failed to read manifest body")
	}

	manifestOutput := ManifestOutput{}
	err = json.Unmarshal(manifestBody, &manifestOutput)

	if err != nil {
		return fmt.Errorf("failed to unmarshal manifest body, %s", err.Error())
	}

	layersCount := len(manifestOutput.Layers)

	if layersCount == 0 {
		fmt.Printf("Cant read manifest properly %s", string(manifestBody))
	}

	doneChan := make(chan bool, layersCount)
	wg := sync.WaitGroup{}

	fmt.Printf("Layers to fetch %d \n", layersCount)

	for j := 0; j < layersCount; j++ {
		wg.Add(1)
		go func(digest string, jobNumber int) {
			defer wg.Done()
			fmt.Printf("Fetching %s:%s [%d/%d]\n", imageName, digest, jobNumber+1, layersCount)
			err := fetchLayer(imageName, digest, authOutput.Token, dst)
			doneChan <- err == nil
		}(manifestOutput.Layers[j].Digest, j)
	}

	wg.Wait()
	close(doneChan)

	successCount := 0
	for result := range doneChan {
		if result == true {
			successCount++
		}
	}
	if successCount == layersCount {
		fmt.Printf("Successfully fetched all %d layers", layersCount)
	} else {
		fmt.Printf("Failed to download  %d layers", layersCount-successCount)
	}

	return nil
}

func fetchLayer(imageName string, layerHash string, authToken string, dstPath string) error {
	httpClient := http.Client{}
	url, err := url.Parse(fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/blobs/%s", imageName, layerHash))

	if err != nil {
		return fmt.Errorf("Failed to parse url error: %s", err.Error())
	}

	request := http.Request{
		Method: "GET",
		URL:    url,
		Header: map[string][]string{
			"Authorization": {fmt.Sprintf("Bearer %s", authToken)},
		},
	}

	layerResponse, err := httpClient.Do(&request)

	if err != nil {
		return fmt.Errorf("Failed to fetch layer %s, error: %s", layerHash, err.Error())
	}

	err = untar(layerResponse.Body, dstPath)
	if err != nil {
		fmt.Printf("Failed to untar, error: %s", err.Error())
	}

	return nil
}

func untar(reader io.Reader, dst string) error {

	gzip, err := gzip.NewReader(reader)

	if err != nil {
		return err
	}

	data, err := io.ReadAll(gzip)

	if err != nil {
		return err
	}

	tr := tar.NewReader(bytes.NewReader(data))

	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		target := filepath.Join(dst, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy contents to file
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
}

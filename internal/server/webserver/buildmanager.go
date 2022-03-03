package webserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
)

type file struct {
	Timestamp time.Time
	Expiry    time.Duration
	timer     *time.Timer `json:"-"`
	Path      string
	Goos      string
	Goarch    string
}

var Autocomplete = trie.NewTrie()

const cacheDescriptionFile = "description.json"

var validPlatforms = make(map[string]bool)
var validArchs = make(map[string]bool)

var c sync.RWMutex
var cache map[string]file = make(map[string]file) // random id to actual file path
var cachePath string

func Build(expiry time.Duration, goos, goarch, connectBackAdress string) (string, error) {
	if !webserverOn {
		return "", fmt.Errorf("Web server is not enabled.")
	}

	if len(goarch) != 0 && !validArchs[goarch] {
		return "", fmt.Errorf("GOARCH supplied is not valid: " + goarch)
	}

	if len(goos) != 0 && !validPlatforms[goos] {
		return "", fmt.Errorf("GOOS supplied is not valid: " + goos)
	}

	if len(connectBackAdress) == 0 {
		connectBackAdress = connectBack
	}

	c.Lock()
	defer c.Unlock()

	var f file

	filename, err := internal.RandomString(16)
	if err != nil {
		return "", err
	}

	id, err := internal.RandomString(16)
	if err != nil {
		return "", err
	}

	f.Path = filepath.Join(cachePath, filename)
	f.Timestamp = time.Now()
	f.Expiry = expiry

	cmd := exec.Command("go", "build", fmt.Sprintf("-ldflags=-X main.destination=%s", connectBackAdress), "-o", f.Path, filepath.Join(projectRoot, "/cmd/client"))
	cmd.Env = append(cmd.Env, os.Environ()...)

	if len(connectBack) != 0 {
		cmd.Env = append(cmd.Env)
	}

	f.Goos = runtime.GOOS
	if len(goos) > 0 {
		cmd.Env = append(cmd.Env, "GOOS="+goos)
		f.Goos = goos
	}

	f.Goarch = runtime.GOARCH
	if len(goarch) > 0 {
		cmd.Env = append(cmd.Env, "GOARCH="+goarch)
		f.Goarch = goarch
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error: " + err.Error() + "\n" + string(output))
	}

	if expiry > 0 {
		f.timer = time.AfterFunc(f.Expiry, func() {
			Delete(id)
		})
	}
	cache[id] = f

	Autocomplete.Add(id)

	writeCache()

	return "http://" + connectBackAdress + "/" + id, nil
}

func Get(key string) (file, error) {
	c.RLock()
	defer c.RUnlock()

	cacheEntry, ok := cache[key]
	if !ok {
		return cacheEntry, errors.New("Unable to find cache entry: " + key)
	}

	return cacheEntry, nil
}

func List(filter string) (matchingFiles map[string]file, err error) {
	_, err = filepath.Match(filter, "")
	if err != nil {
		return nil, fmt.Errorf("Filter is not well formed")
	}

	matchingFiles = make(map[string]file)

	c.RLock()
	defer c.RUnlock()

	for id := range cache {
		if filter == "" {
			matchingFiles[id] = cache[id]
			continue
		}

		if match, _ := filepath.Match(filter, id); match {
			matchingFiles[id] = cache[id]
			continue
		}

		file := cache[id]

		if match, _ := filepath.Match(filter, file.Goos); match {
			matchingFiles[id] = cache[id]
			continue
		}

		if match, _ := filepath.Match(filter, file.Goarch); match {
			matchingFiles[id] = cache[id]
			continue
		}
	}

	return
}

func Delete(key string) error {
	c.Lock()
	defer c.Unlock()

	cacheEntry, ok := cache[key]
	if !ok {
		return errors.New("Unable to find cache entry: " + key)
	}

	if cacheEntry.timer != nil {
		cacheEntry.timer.Stop()
	}

	delete(cache, key)

	writeCache()

	Autocomplete.Remove(key)

	return os.Remove(cacheEntry.Path)
}

func writeCache() {
	content, err := json.Marshal(cache)
	if err != nil {
		panic(err)
	}
	os.WriteFile(filepath.Join(cachePath, cacheDescriptionFile), content, 0700)
}

func startBuildManager(cPath string) error {

	c.Lock()
	defer c.Unlock()

	clientSource := filepath.Join(projectRoot, "/cmd/client")
	info, err := os.Stat(clientSource)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("The server doesnt appear to be in {project_root}/bin, please put it there.")
	}

	cmd := exec.Command("go", "tool", "dist", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Unable to run the go compiler to get a list of compilation targets: %s", err)
	}

	platformAndArch := bytes.Split(output, []byte("\n"))

	for _, line := range platformAndArch {
		parts := bytes.Split(line, []byte("/"))
		if len(parts) == 2 {
			validPlatforms[string(parts[0])] = true
			validArchs[string(parts[1])] = true
		}
	}

	info, err = os.Stat(cPath)
	if os.IsNotExist(err) {
		err = os.Mkdir(cPath, 0700)
		if err != nil {
			return err
		}
		info, err = os.Stat(cPath)
	}

	if !info.IsDir() {
		return errors.New("Cache path '" + cPath + "' already exists, but is a file instead of directory")
	}

	err = os.WriteFile(filepath.Join(cPath, "test"), []byte("test"), 0700)
	if err != nil {
		return errors.New("Unable to write file into cache directory: " + err.Error())
	}

	err = os.Remove(filepath.Join(cPath, "test"))
	if err != nil {
		return errors.New("Unable to delete file in cache directory: " + err.Error())
	}

	contents, err := os.ReadFile(filepath.Join(cPath, cacheDescriptionFile))
	if err == nil {
		err = json.Unmarshal(contents, &cache)
		if err == nil {
			for id, v := range cache {
				Autocomplete.Add(id)

				if v.Expiry != 0 {
					v.timer = time.AfterFunc(v.Expiry, func() {
						Delete(id)
					})
				}
			}
		} else {
			fmt.Println("Unable to load cache: ", err)
		}
	} else {
		fmt.Println("Unable to load cache: ", err)
	}

	cachePath = cPath

	return nil
}

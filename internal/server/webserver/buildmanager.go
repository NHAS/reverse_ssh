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
	FileType  string
	Hits      int
}

var Autocomplete = trie.NewTrie()

const cacheDescriptionFile = "description.json"

var validPlatforms = make(map[string]bool)
var validArchs = make(map[string]bool)

var c sync.RWMutex
var cache map[string]file = make(map[string]file) // random id to actual file path
var cachePath string

func Build(expiry time.Duration, goos, goarch, suppliedConnectBackAdress, name string, shared bool) (string, error) {
	if !webserverOn {
		return "", fmt.Errorf("Web server is not enabled.")
	}

	if len(goarch) != 0 && !validArchs[goarch] {
		return "", fmt.Errorf("GOARCH supplied is not valid: " + goarch)
	}

	if len(goos) != 0 && !validPlatforms[goos] {
		return "", fmt.Errorf("GOOS supplied is not valid: " + goos)
	}

	if len(suppliedConnectBackAdress) == 0 {
		suppliedConnectBackAdress = defaultConnectBack
	}

	c.Lock()
	defer c.Unlock()

	var f file

	filename, err := internal.RandomString(16)
	if err != nil {
		return "", err
	}

	if len(name) == 0 {
		name, err = internal.RandomString(16)
		if err != nil {
			return "", err
		}
	}

	if _, ok := cache[name]; ok {
		return "", errors.New("This link name is already in use")
	}

	f.Goos = runtime.GOOS
	if len(goos) > 0 {
		f.Goos = goos
	}

	f.Goarch = runtime.GOARCH
	if len(goarch) > 0 {
		f.Goarch = goarch
	}

	f.Path = filepath.Join(cachePath, filename)
	f.Timestamp = time.Now()
	f.Expiry = expiry
	f.FileType = "executable"

	buildArguments := []string{"build"}
	if shared {
		buildArguments = append(buildArguments, "-buildmode=c-shared")
		buildArguments = append(buildArguments, "-tags=cshared")
		f.FileType = "shared-object"
		if f.Goos != "windows" {
			f.Path += ".so"
		} else {
			f.Path += ".dll"
		}

	}

	buildArguments = append(buildArguments, fmt.Sprintf("-ldflags=-s -w -X main.destination=%s", suppliedConnectBackAdress))
	buildArguments = append(buildArguments, "-o", f.Path, filepath.Join(projectRoot, "/cmd/client"))

	cmd := exec.Command("go", buildArguments...)

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, "GOOS="+f.Goos)
	cmd.Env = append(cmd.Env, "GOARCH="+f.Goarch)
	cmd.Env = append(cmd.Env, "RSSH_HOMESERVER="+suppliedConnectBackAdress)

	//Building a shared object for windows needs some extra beans
	cgoOn := "0"
	if shared {

		var crossCompiler string
		if runtime.GOOS == "linux" && f.Goos == "windows" && f.Goarch == "amd64" {
			crossCompiler = "x86_64-w64-mingw32-gcc"
		}

		cmd.Env = append(cmd.Env, "CC="+crossCompiler)
		cgoOn = "1"
	}

	cmd.Env = append(cmd.Env, "CGO_ENABLED="+cgoOn)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error: " + err.Error() + "\n" + string(output))
	}

	if expiry > 0 {
		f.timer = time.AfterFunc(f.Expiry, func() {
			Delete(name)
		})
	}
	cache[name] = f

	Autocomplete.Add(name)

	writeCache()

	return "http://" + suppliedConnectBackAdress + "/" + name, nil
}

func Get(key string) (file, error) {
	c.RLock()
	defer c.RUnlock()

	cacheEntry, ok := cache[key]
	if !ok {
		return cacheEntry, errors.New("Unable to find cache entry: " + key)
	}

	cacheEntry.Hits++

	cache[key] = cacheEntry

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

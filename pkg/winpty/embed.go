//go:build windows
// +build windows

package winpty

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed executables/*
var binaries embed.FS

func writeBinaries(dllPath, agentPage string) error {

	vsn := windows.RtlGetVersion()

	/*
		https://msdn.microsoft.com/en-us/library/ms724832(VS.85).aspx
		Windows 10					10.0*
		Windows Server 2016			10.0*
		Windows 8.1					6.3*
		Windows Server 2012 R2		6.3*
		Windows 8					6.2
		Windows Server 2012			6.2
		Windows 7					6.1
		Windows Server 2008 R2		6.1
		Windows Server 2008			6.0
		Windows Vista				6.0
		Windows Server 2003 R2		5.2
		Windows Server 2003			5.2
		Windows XP 64-Bit Edition	5.2
		Windows XP					5.1
		Windows 2000				5.0
	*/

	dllType := "regular"
	if vsn.MajorVersion == 5 {
		// Funny enough arm64 does not have a xp build
		if runtime.GOARCH == "arm64" {

			log.Println("xp doesnt have an arm64 version so uh, Im just going to die here")
			return errors.New("tried to run an arm64 windows xp winpty session")
		}
		dllType = "xp"
	}

	dll, err := binaries.ReadFile(path.Join("executables", runtime.GOARCH, dllType, "winpty.dll"))
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(dllPath, dll, 0700)
	if err != nil {
		return err
	}

	exe, err := binaries.ReadFile(path.Join("executables", runtime.GOARCH, dllType, "winpty-agent.exe"))
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(agentPage, exe, 0700)
	if err != nil {
		return err
	}

	return nil
}

func createAgentCfg(flags uint32) (uintptr, error) {
	var errorPtr uintptr

	if winpty_error_free == nil {
		return uintptr(0), errors.New("winpty was not initalised")
	}

	err := winpty_error_free.Find() // check if dll available
	if err != nil {
		return uintptr(0), err
	}

	defer winpty_error_free.Call(errorPtr)

	agentCfg, _, _ := winpty_config_new.Call(uintptr(flags), uintptr(unsafe.Pointer(errorPtr)))
	if agentCfg == uintptr(0) {
		return 0, fmt.Errorf("Unable to create agent config, %s", GetErrorMessage(errorPtr))
	}

	return agentCfg, nil
}

func createSpawnCfg(flags uint32, appname, cmdline, cwd string, env []string) (uintptr, error) {
	var errorPtr uintptr
	defer winpty_error_free.Call(errorPtr)

	cmdLineStr, err := syscall.UTF16PtrFromString(cmdline)
	if err != nil {
		return 0, fmt.Errorf("Failed to convert cmd to pointer.")
	}

	appNameStr, err := syscall.UTF16PtrFromString(appname)
	if err != nil {
		return 0, fmt.Errorf("Failed to convert app name to pointer.")
	}

	cwdStr, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		return 0, fmt.Errorf("Failed to convert working directory to pointer.")
	}

	envStr, err := UTF16PtrFromStringArray(env)

	if err != nil {
		return 0, fmt.Errorf("Failed to convert cmd to pointer.")
	}

	var spawnCfg uintptr
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "amd64" {
		spawnCfg, _, _ = winpty_spawn_config_new.Call(
			uintptr(flags),
			uintptr(unsafe.Pointer(appNameStr)),
			uintptr(unsafe.Pointer(cmdLineStr)),
			uintptr(unsafe.Pointer(cwdStr)),
			uintptr(unsafe.Pointer(envStr)),
			uintptr(unsafe.Pointer(errorPtr)),
		)
	} else {
		spawnCfg, _, _ = winpty_spawn_config_new.Call(
			uintptr(flags),
			uintptr(0), // winpty expects a UINT64 so we need to pad the call on 386
			uintptr(unsafe.Pointer(appNameStr)),
			uintptr(unsafe.Pointer(cmdLineStr)),
			uintptr(unsafe.Pointer(cwdStr)),
			uintptr(unsafe.Pointer(envStr)),
			uintptr(unsafe.Pointer(errorPtr)),
		)
	}

	if spawnCfg == uintptr(0) {
		return 0, fmt.Errorf("Unable to create spawn config, %s", GetErrorMessage(errorPtr))
	}

	return spawnCfg, nil
}

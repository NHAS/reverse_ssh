//go:build windows
// +build windows

package winpty

import (
	"errors"
	"log"
	"os"
	"path"
	"runtime"
	"syscall"
)

const (
	WINPTY_SPAWN_FLAG_AUTO_SHUTDOWN            = 1
	WINPTY_FLAG_ALLOW_CURPROC_DESKTOP_CREATION = 0x8
)

var (
	modWinPTY *syscall.LazyDLL

	// Error handling...
	winpty_error_code *syscall.LazyProc
	winpty_error_msg  *syscall.LazyProc
	winpty_error_free *syscall.LazyProc

	// Configuration of a new agent.
	winpty_config_new               *syscall.LazyProc
	winpty_config_free              *syscall.LazyProc
	winpty_config_set_initial_size  *syscall.LazyProc
	winpty_config_set_mouse_mode    *syscall.LazyProc
	winpty_config_set_agent_timeout *syscall.LazyProc

	// Start the agent.
	winpty_open          *syscall.LazyProc
	winpty_agent_process *syscall.LazyProc

	// I/O Pipes
	winpty_conin_name  *syscall.LazyProc
	winpty_conout_name *syscall.LazyProc
	winpty_conerr_name *syscall.LazyProc

	// Agent RPC Calls
	winpty_spawn_config_new  *syscall.LazyProc
	winpty_spawn_config_free *syscall.LazyProc
	winpty_spawn             *syscall.LazyProc
	winpty_set_size          *syscall.LazyProc
	winpty_free              *syscall.LazyProc
)

func loadWinPty() error {

	if modWinPTY != nil {
		return nil
	}

	switch runtime.GOARCH {
	case "amd64", "arm64", "386":
	default:
		return errors.New("unsupported winpty platform " + runtime.GOARCH)
	}

	var (
		winptyDllName   = "winpty.dll"
		winptyAgentName = "winpty-agent.exe"
	)

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Println("unable to get cache directory for writing winpty pe's writing may fail if directory is read only")
	}

	if err == nil {
		winptyDllName = path.Join(cacheDir, winptyDllName)
		winptyAgentName = path.Join(cacheDir, winptyAgentName)
	}

	err = writeBinaries(winptyDllName, winptyAgentName)
	if err != nil {
		return errors.New("writing PEs to disk failed: " + err.Error())
	}

	modWinPTY = syscall.NewLazyDLL(winptyDllName)
	if modWinPTY == nil {
		return errors.New("creating lazy dll failed")
	}

	// Error handling...
	winpty_error_code = modWinPTY.NewProc("winpty_error_code")

	winpty_error_msg = modWinPTY.NewProc("winpty_error_msg")
	winpty_error_free = modWinPTY.NewProc("winpty_error_free")

	// Configuration of a new agent.
	winpty_config_new = modWinPTY.NewProc("winpty_config_new")
	winpty_config_free = modWinPTY.NewProc("winpty_config_free")
	winpty_config_set_initial_size = modWinPTY.NewProc("winpty_config_set_initial_size")
	winpty_config_set_mouse_mode = modWinPTY.NewProc("winpty_config_set_mouse_mode")
	winpty_config_set_agent_timeout = modWinPTY.NewProc("winpty_config_set_agent_timeout")

	// Start the agent.
	winpty_open = modWinPTY.NewProc("winpty_open")
	winpty_agent_process = modWinPTY.NewProc("winpty_agent_process")

	// I/O Pipes
	winpty_conin_name = modWinPTY.NewProc("winpty_conin_name")
	winpty_conout_name = modWinPTY.NewProc("winpty_conout_name")
	winpty_conerr_name = modWinPTY.NewProc("winpty_conerr_name")

	// Agent RPC Calls
	winpty_spawn_config_new = modWinPTY.NewProc("winpty_spawn_config_new")
	winpty_spawn_config_free = modWinPTY.NewProc("winpty_spawn_config_free")
	winpty_spawn = modWinPTY.NewProc("winpty_spawn")
	winpty_set_size = modWinPTY.NewProc("winpty_set_size")
	winpty_free = modWinPTY.NewProc("winpty_free")

	return nil
}

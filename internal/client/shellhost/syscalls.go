package shellhost

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	procSetCurrentConsoleFont = kernel32.NewProc("SetCurrentConsoleFontEx")
	procAttachConsole         = kernel32.NewProc("AttachConsole")

	procGetLargestConsoleWindowSize = kernel32.NewProc("GetLargestConsoleWindowSize")
	procSetConsoleWindowInfo        = kernel32.NewProc("SetConsoleWindowInfo")
	procSetConsoleScreenBufferSize  = kernel32.NewProc("SetConsoleScreenBufferSize")

	procReadConsoleOutput = kernel32.NewProc("ReadConsoleOutputW")
	procWriteConsoleInput = kernel32.NewProc("WriteConsoleInputW")

	procGenerateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")

	user32               = windows.NewLazySystemDLL("user32.dll")
	procGetMessage       = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessage")
	procMapVirtualKey    = user32.NewProc("MapVirtualKeyA")

	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	procPostMessage       = user32.NewProc("PostMessageW")
)

func FreeConsole() error {
	kernel32.Load()
	ret, _, _ := procFreeConsole.Call()
	if ret == 0 {
		return windows.GetLastError()
	}
	return nil

}

// AttachConsole from windows kernel32 API
func AttachConsole(consoleOwner uint32) bool {
	ret, _, _ := procAttachConsole.Call(uintptr(consoleOwner))

	return ret != 0
}

func SetCurrentConsoleFont(hConsoleOutput windows.Handle, bMaximumWindow bool, lpConsoleCurrentFontEx *CONSOLE_FONT_INFOEX) error {

	var _p0 uint32
	if bMaximumWindow {
		_p0 = 1
	}

	ret, _, err := procSetCurrentConsoleFont.Call(
		uintptr(hConsoleOutput),
		uintptr(_p0),
		uintptr(unsafe.Pointer(lpConsoleCurrentFontEx)))

	if ret == 0 {
		return err
	}

	return nil
}

func GetLargestConsoleWindowSize(hConsoleOutput windows.Handle) windows.Coord {

	ret, _, _ := procGetLargestConsoleWindowSize.Call(uintptr(hConsoleOutput))

	var c windows.Coord
	u32 := uint32(ret)
	c.X = *(*int16)(unsafe.Pointer(&u32))
	c.Y = *(*int16)(unsafe.Pointer(uintptr(unsafe.Pointer(&u32)) + uintptr(2)))
	return c

}

func SetConsoleWindowInfo(hConsoleOutput windows.Handle, bAbsolute bool, lpConsoleWindow *windows.SmallRect) error {
	var p0 uint32 = 0
	if bAbsolute {
		p0 = 1
	}

	ret, _, err := procSetConsoleWindowInfo.Call(uintptr(hConsoleOutput), uintptr(p0), uintptr(unsafe.Pointer(lpConsoleWindow)))
	if ret == 0 {
		return err
	}

	return nil
}

func SetConsoleScreenBufferSize(hConsoleOutput windows.Handle, dwSize windows.Coord) error {

	ret, _, err := procSetConsoleScreenBufferSize.Call(uintptr(hConsoleOutput), uintptr(*((*uint32)(unsafe.Pointer(&dwSize)))))
	if ret == 0 {
		return err
	}

	return nil
}

func WriteConsoleInput(hConsoleOutput windows.Handle, lpBuffer *InputRecord, nLength uint32) (uint32, error) {

	var written uint32 = 0

	ret, _, err := procWriteConsoleInput.Call(uintptr(hConsoleOutput), uintptr(unsafe.Pointer(lpBuffer)), uintptr(nLength), uintptr(unsafe.Pointer(&written)))
	if ret == 0 {
		return 0, err
	}

	return written, nil
}

func ReadConsoleOutput(hConsoleOutput windows.Handle, lpBuffer []CharInfo, dwBufferSize, dwBufferCoord windows.Coord, lpReadRegion *windows.SmallRect) error {

	ret, _, err := procReadConsoleOutput.Call(
		uintptr(hConsoleOutput),
		uintptr(unsafe.Pointer(&lpBuffer[0])),
		uintptr(*((*uint32)(unsafe.Pointer(&dwBufferSize)))),
		uintptr(*((*uint32)(unsafe.Pointer(&dwBufferCoord)))),
		uintptr(unsafe.Pointer(lpReadRegion)))
	if ret == 0 {
		return err
	}

	return nil
}

func GetMessage(msg *Msg, hWnd windows.Handle, wMsgFilterMin, wMsgFilterMax uint32) error {
	ret, _, err := procGetMessage.Call(uintptr(unsafe.Pointer(msg)), uintptr(hWnd), uintptr(wMsgFilterMin), uintptr(wMsgFilterMax))
	if ret == 0 {
		return err
	}

	return nil
}

func TranslateMessage(msg *Msg) error {
	ret, _, err := procTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
	if ret == 0 {
		return err
	}

	return nil
}

func DispatchMessage(msg *Msg) error {
	ret, _, err := procDispatchMessage.Call(uintptr(unsafe.Pointer(msg)))
	if ret == 0 {
		return err
	}

	return nil
}

func MapVirtualKey(uCode, uMapType uint32) uint32 {
	ret, _, _ := procMapVirtualKey.Call(uintptr(uCode), uintptr(uMapType))

	return uint32(ret)
}

func GenerateConsoleCtrlEvent(pid uint32) error {
	r, _, e := procGenerateConsoleCtrlEvent.Call(windows.CTRL_C_EVENT, uintptr(pid))
	if r == 0 {
		return e
	}
	return nil
}

func PostThreadMessage(idThread uint32, msg uint32, wParam, lParam uintptr) error {
	r, _, e := procPostThreadMessage.Call(uintptr(idThread), uintptr(msg), wParam, lParam)
	if r == 0 {
		return e
	}
	return nil
}

func PostMessage(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) error {
	r, _, err := procPostMessage.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	if r == 0 {
		return fmt.Errorf("PostMessage failed: %v", err)
	}
	return nil
}

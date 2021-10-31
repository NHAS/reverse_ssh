//go:build windows
// +build windows

package shellhost

import "golang.org/x/sys/windows"

type CONSOLE_FONT_INFOEX struct {
	cbSize     uint32
	nFont      uint32
	dwFontSize windows.Coord
	FontFamily uint32
	FontWeight uint32
	FaceName   [32]uint16
}

type Msg struct {
	Wnd     windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type POINT struct {
	x uint32
	y uint32
}

type InputRecord struct {
	EventType uint16

	KeyEvent KEY_EVENT_RECORD
}

type KEY_EVENT_RECORD struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16

	UnicodeChar     uint16
	ControlKeyState uint32
}

type CharInfo struct {
	UnicodeChar uint16
	Attributes  uint16
}

const (
	FOREGROUND_BLUE uint16 = 1 << iota
	FOREGROUND_GREEN
	FOREGROUND_RED
	FOREGROUND_INTENSITY
	BACKGROUND_BLUE
	BACKGROUND_GREEN
	BACKGROUND_RED
	BACKGROUND_INTENSITY
	COMMON_LVB_LEADING_BYTE
	COMMON_LVB_TRAILING_BYTE
	COMMON_LVB_GRID_HORIZONTAL
	COMMON_LVB_GRID_LVERTICAL
	COMMON_LVB_GRID_RVERTICAL
	_
	COMMON_LVB_REVERSE_VIDEO
	COMMON_LVB_UNDERSCORE
	COMMON_LVB_SBCSDBCS = COMMON_LVB_LEADING_BYTE | COMMON_LVB_TRAILING_BYTE
)

const (
	KEY_EVENT uint16 = 1 << iota
	MOUSE_EVENT
	WINDOW_BUFFER_SIZE_EVENT
	MENU_EVENT
	FOCUS_EVENT
)

package shellhost

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
)

const MAX_CONSOLE_COLUMNS = 9999
const MAX_CONSOLE_ROWS = 9999
const MAX_EXPECTED_BUFFER_SIZE = 1024
const STILL_ACTIVE = 259
const MAX_CTRL_SEQ_LEN = int(7)
const MIN_CTRL_SEQ_LEN = int(6)
const WM_APPEXIT = 0x0400 + 1

var (
	CREATE_NEW_CONSOLE = uint32(0x00000010)
)

const LF_FACESIZE = 32

func Start_with_pty(command string, connection ssh.Channel) error {
	var (
		si             windows.StartupInfo
		pi             windows.ProcessInformation
		sa             windows.SecurityAttributes
		childProcessId uint32
	)

	SendClearScreen(connection)

	windows.GetStartupInfo(&inputSi)

	sa.InheritHandle = 1

	events := make(chan Event)

	hEventHook, err := win.SetWinEventHook(win.EVENT_CONSOLE_CARET, win.EVENT_CONSOLE_END_APPLICATION, 0, ConsoleEventProc(events), 0, 0, win.WINEVENT_OUTOFCONTEXT)
	if err != nil {
		return err
	}
	defer win.UnhookWinEvent(hEventHook)

	si.Cb = uint32(unsafe.Sizeof(si))

	windows.SetHandleInformation(windows.Handle(os.Stdin.Fd()), windows.HANDLE_FLAG_INHERIT, 0)

	cmd, err := exec.LookPath("cmd.exe")
	if err != nil {
		return err
	}

	err = windows.CreateProcess(nil, windows.StringToUTF16Ptr(fmt.Sprintf("\"%s\" /c \"%s\"", cmd, command)), nil, nil, true, CREATE_NEW_CONSOLE, nil, nil, &si, &pi)
	if err != nil {
		return err
	}
	defer func() {
		windows.TerminateProcess(pi.Process, 0)

		windows.CloseHandle(pi.Process)
		windows.CloseHandle(pi.Thread)
	}()

	childProcessId = pi.ProcessId

	err = FreeConsole()
	if err != nil {
		log.Fatal("Freeconsole: ", err)
	}

	windows.SleepEx(20, false)

	for !AttachConsole(pi.ProcessId) {
		var exitCode uint32

		err = windows.GetExitCodeProcess(pi.Process, &exitCode)
		if err != nil && exitCode != STILL_ACTIVE {
			log.Println("Waiting on child process to exist failed")
			os.Exit(1)
		}

		windows.SleepEx(100, false)
	}

	path, err := syscall.UTF16PtrFromString("CONIN$")
	if err != nil {
		log.Fatal("Getting handle: ", err)
	}
	childIn, err := windows.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		log.Fatal("Getting handle2: ", err)
	}
	defer windows.CloseHandle(childIn)

	path, err = windows.UTF16PtrFromString("CONOUT$")
	if err != nil {
		log.Fatal("Getting handle3: ", err)

	}
	childOut, err := windows.CreateFile(path, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		log.Fatal("Getting handle3: ", err)

	}
	defer windows.CloseHandle(childOut)

	u := windows.GetCurrentThreadId()

	go func() {
		windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		err = PostThreadMessage(uint32(u), WM_APPEXIT, 0, 0)
		log.Println("Child process has now died, rip", err)
	}()

	go ProcessEvents(events, childProcessId, childOut, connection)
	go ProcessPipes(connection, childIn, pi.ThreadId)

	SizeWindow(childOut)

	var consoleInfo windows.ConsoleScreenBufferInfo
	windows.GetConsoleScreenBufferInfo(childOut, &consoleInfo)

	var msg Msg
	for GetMessage(&msg, 0, 0, 0) == nil {
		if msg.Message == WM_APPEXIT {
			break
		}

		err = TranslateMessage(&msg)
		if err != nil {
			panic(err)
		}
		err = DispatchMessage(&msg)
		if err != nil {
			panic(err)
		}

	}

	return nil
}

var inputSi windows.StartupInfo

func SizeWindow(hInput windows.Handle) {

	/* Set the default font to Consolas */
	var matchingFont CONSOLE_FONT_INFOEX
	matchingFont.cbSize = uint32(unsafe.Sizeof(matchingFont))
	matchingFont.nFont = 0
	matchingFont.dwFontSize.X = 0
	matchingFont.dwFontSize.Y = 16
	matchingFont.FontFamily = win.FF_DONTCARE
	matchingFont.FontWeight = win.FW_NORMAL

	fontName, err := windows.UTF16FromString("Consolas")
	if err != nil {
		log.Fatal("FONT name: ", err)
	}

	if copy(matchingFont.FaceName[:], fontName) > 32 {
		log.Fatal("Wrote more bytes than I wanted to")
	}

	err = SetCurrentConsoleFont(hInput, true, &matchingFont)
	if err != nil {
		log.Fatal("Me?", err)
	}

	/* This information is the live screen  */
	var consoleInfo windows.ConsoleScreenBufferInfo

	err = windows.GetConsoleScreenBufferInfo(hInput, &consoleInfo)
	if err != nil {
		log.Fatal(err)
	}

	/* Get the largest size we can size the console window to */
	coordScreen := GetLargestConsoleWindowSize(hInput)

	/* Define the new console window size and scroll position */
	if inputSi.XCountChars == 0 || inputSi.YCountChars == 0 {
		inputSi.XCountChars = 80
		inputSi.YCountChars = 25
	}
	var srWindowRect windows.SmallRect
	srWindowRect.Right = min(int16(inputSi.XCountChars), coordScreen.X) - 1
	srWindowRect.Bottom = min(int16(inputSi.YCountChars), coordScreen.Y) - 1
	srWindowRect.Left = 0
	srWindowRect.Top = 0

	/* Define the new console buffer history to be the maximum possible */
	coordScreen.X = srWindowRect.Right + 1 /* buffer width must be equ window width */
	coordScreen.Y = 9999

	if SetConsoleWindowInfo(hInput, true, &srWindowRect) != nil {
		SetConsoleScreenBufferSize(hInput, coordScreen)

	} else if SetConsoleScreenBufferSize(hInput, coordScreen) != nil {
		SetConsoleWindowInfo(hInput, true, &srWindowRect)
	}

	windows.GetConsoleScreenBufferInfo(hInput, &consoleInfo)
}

func min(x, y int16) int16 {
	if x < y {
		return x
	}

	return y
}

func ProcessPipes(connection ssh.Channel, childIn windows.Handle, threadId uint32) {
	buf := make([]byte, 128)
	for {
		n, err := connection.Read(buf)
		if err != nil {
			break
		}

		if n > 0 {
			ProcessIncomingKeys(buf[:n], childIn)
		}

		bStartup = false

	}

	//Need to do the whole "Tell everything to die", here

}

func ProcessIncomingKeys(buffer []byte, childIn windows.Handle) {

	ESC_SEQ := []byte("\x1b")

	buffer_length := len(buffer)

	for i := 0; i < buffer_length; {

		found, key := CheckKeyTranslations(buffer)
		if found {
			SendKeyStroke(childIn, key.vk, int16(key.out), uint32(key.ctrlState))
			i += len(key.in)
			continue
		}

		remainingUnprocessed := buffer_length - i
		if remainingUnprocessed >= MAX_CTRL_SEQ_LEN && ProcessModifierKeySequence(buffer[i:i+MAX_CTRL_SEQ_LEN], childIn) != 0 {
			i += MAX_CTRL_SEQ_LEN
			continue
		}

		if remainingUnprocessed >= MIN_CTRL_SEQ_LEN && ProcessModifierKeySequence(buffer[i:i+MIN_CTRL_SEQ_LEN], childIn) != 0 {
			i += MIN_CTRL_SEQ_LEN
			continue
		}

		if bytes.Equal(buffer[i:i+len(ESC_SEQ)-1], ESC_SEQ) {
			p := buffer[len(ESC_SEQ):]
			/* Alt sequence */
			ok, key := CheckKeyTranslations(p)
			if ok && (key.ctrlState&uint32(LEFT_ALT_PRESSED)) == 0 {
				wcha := windows.StringToUTF16(string(key.out))
				SendKeyStroke(childIn, key.vk, int16(wcha[0]), uint32(key.ctrlState|uint32(LEFT_ALT_PRESSED)))
				i += len(ESC_SEQ) + len(key.in)
				continue
			}

			SendKeyStroke(childIn, win.VK_ESCAPE, int16('\x1b'), 0)
			i += len(ESC_SEQ)
			continue
		}

		if string(buffer[i:]) == "\x03" {
			GenerateConsoleCtrlEvent(0)
		} else {
			cha, err := windows.UTF16FromString(string(buffer[i]))
			if err != nil {
				i++
				continue
			}
			SendKeyStroke(childIn, 0, int16(cha[0]), 0)
		}

		i++
	}

}

func CheckKeyTranslations(buf []byte) (bool, key_translation) {
	for j := 0; j < len(keys); j++ {
		if len(buf) >= len(keys[j].in) && bytes.Contains(buf, []byte(keys[j].in)) {
			return true, keys[j]
		}
	}

	return false, key_translation{}
}

func ProcessModifierKeySequence(buf []byte, childIn windows.Handle) int {
	if len(buf) < MIN_CTRL_SEQ_LEN {
		return 0
	}

	buf_len := len(buf)

	modifier_key, err := strconv.Atoi(string(buf[:buf_len-2]))
	if err != nil {
		return 1
	}

	if (modifier_key < 2) && (modifier_key > 7) {
		return 0
	}

	vkey := 0
	/* Decode special keys when pressed ALT/CTRL/SHIFT key */
	if buf[0] == '\033' && buf[1] == '[' && buf[buf_len-3] == ';' {
		if vkey == 0 {
			if buf[buf_len-1] == '~' {
				/* VK_DELETE, VK_PGDN, VK_PGUP */
				if buf_len == 6 {
					vkey = GetVirtualKeyByMask('[', buf[2:], 1, '~')
				}

				/* VK_F1 ... VK_F12 */
				if buf_len == 7 {
					vkey = GetVirtualKeyByMask('[', buf[2:], 2, '~')
				}
			} else {
				/* VK_LEFT, VK_RIGHT, VK_UP, VK_DOWN */
				if buf_len == 6 && buf[2] == '1' {
					vkey = GetVirtualKeyByMask('[', buf[5:], 1, 0)
				}

				/* VK_F1 ... VK_F4 */
				if buf_len == 6 && buf[2] == '1' && IsAlpha(buf[5]) {
					vkey = GetVirtualKeyByMask('O', buf[5:], 1, 0)
				}
			}
		}

		if vkey != 0 {
			switch modifier_key {
			case 2:
				SendKeyStroke(childIn, vkey, 0, uint32(SHIFT_PRESSED))
				break
			case 3:
				SendKeyStroke(childIn, vkey, 0, uint32(LEFT_ALT_PRESSED))
				break
			case 4:
				SendKeyStroke(childIn, vkey, 0, uint32(SHIFT_PRESSED|LEFT_ALT_PRESSED))
				break
			case 5:
				SendKeyStroke(childIn, vkey, 0, uint32(LEFT_CTRL_PRESSED))
				break
			case 6:
				SendKeyStroke(childIn, vkey, 0, uint32(SHIFT_PRESSED|LEFT_CTRL_PRESSED))
				break
			case 7:
				SendKeyStroke(childIn, vkey, 0, uint32(LEFT_CTRL_PRESSED|LEFT_ALT_PRESSED))
				break
			}
		}

	}

	return vkey
}

func IsAlpha(s byte) bool {
	return (s > 'a' || s < 'z') && (s > 'A' || s < 'Z')
}

func FindKeyTransByMask(prefix byte, value []byte, vlen int, suffix byte) key_translation {

	for _, k := range keys {
		if len(k.in) < vlen+2 {
			continue
		}
		if k.in[0] != '\033' {
			continue
		}
		if k.in[1] != prefix {
			continue
		}

		if k.in[vlen+2] != suffix {
			continue
		}

		if vlen <= 1 && value[0] == k.in[2] {
			return k
		}

		if vlen > 1 && bytes.Equal([]byte(k.in[:2][:vlen]), value[:vlen]) {
			return k
		}
	}

	return key_translation{}
}

func GetVirtualKeyByMask(prefix byte, value []byte, vlen int, suffix byte) int {
	pk := FindKeyTransByMask(prefix, value, vlen, suffix)

	return pk.vk
}

func SendKeyStrokeEx(hInput windows.Handle, vKey uint16, character uint16, ctrlState uint32, keyDown bool) {
	var ir InputRecord

	ir.EventType = uint16(KEY_EVENT)
	ir.KeyEvent.KeyDown = int32(toInt(keyDown))
	ir.KeyEvent.RepeatCount = 1
	ir.KeyEvent.VirtualKeyCode = vKey
	ir.KeyEvent.VirtualScanCode = uint16(MapVirtualKey(uint32(vKey), 0))
	ir.KeyEvent.ControlKeyState = ctrlState
	ir.KeyEvent.UnicodeChar = character

	_, err := WriteConsoleInput(hInput, &ir, 1)
	if err != nil {
		panic(err)
	}
}

func SendKeyStroke(hInput windows.Handle, keyStroke int, character int16, ctrlState uint32) {
	SendKeyStrokeEx(hInput, uint16(keyStroke), uint16(character), uint32(ctrlState), true)
	SendKeyStrokeEx(hInput, uint16(keyStroke), uint16(character), uint32(ctrlState), false)
}

type Event struct {
	Event  uint32
	Hwnd   win.HWND
	Object int32
	Child  int32
}

func ProcessEvents(queue <-chan Event, childProcessId uint32, childOutput windows.Handle, connection ssh.Channel) uint32 {
	for event := range queue {
		if event.Event < win.EVENT_CONSOLE_CARET || event.Event > win.EVENT_CONSOLE_LAYOUT {
			continue
		}

		var eventProcessId uint32
		win.GetWindowThreadProcessId(event.Hwnd, &eventProcessId)

		if eventProcessId != childProcessId {
			continue
		}

		var consoleInfo windows.ConsoleScreenBufferInfo
		err := windows.GetConsoleScreenBufferInfo(childOutput, &consoleInfo)
		if err != nil {
			log.Fatal(err)
		}

		switch event.Event {
		case win.EVENT_CONSOLE_CARET:
			co := windows.Coord{X: int16(win.LOWORD(uint32(event.Child))), Y: int16(win.HIWORD(uint32(event.Child)))}

			lastX = co.X
			lastY = co.Y

			if lastX == 0 && lastY > currentLine {
				CalculateAndSetCursor(connection, lastX, lastY, true)
			} else {
				SendSetCursor(connection, int(lastX+1), int(lastY+1))
			}
		case win.EVENT_CONSOLE_UPDATE_REGION:

			var readRect windows.SmallRect
			readRect.Top = int16(win.HIWORD(uint32(event.Object)))
			readRect.Left = int16(win.LOWORD(uint32(event.Object)))
			readRect.Bottom = int16(win.HIWORD(uint32(event.Child)))

			readRect.Right = int16(win.LOWORD(uint32(event.Child)))
			if readRect.Right < consoleInfo.Window.Right {
				readRect.Right = consoleInfo.Window.Right
			}

			if !bStartup && (readRect.Top == consoleInfo.Window.Top) {
				isClearCommand := (consoleInfo.Size.X == readRect.Right+1) && (consoleInfo.Size.Y == readRect.Bottom+1)

				if isClearCommand {
					SendClearScreen(connection)
					ViewPortY = 0
					lastViewPortY = 0

					continue
				}
			}

			var coordBufSize windows.Coord
			coordBufSize.Y = readRect.Bottom - readRect.Top + 1
			coordBufSize.X = readRect.Right - readRect.Left + 1

			if coordBufSize.X < 0 || coordBufSize.X > MAX_CONSOLE_COLUMNS ||
				coordBufSize.Y < 0 || coordBufSize.Y > MAX_CONSOLE_ROWS {
				continue
			}

			bufferSize := coordBufSize.X * coordBufSize.Y
			if bufferSize > MAX_EXPECTED_BUFFER_SIZE {
				if !bStartup {
					SendClearScreen(connection)
					ViewPortY = 0
					lastViewPortY = 0
				}
				continue
			}

			var coordBufCoord windows.Coord
			var buf []CharInfo = make([]CharInfo, bufferSize)

			err = ReadConsoleOutput(childOutput, buf, coordBufSize, coordBufCoord, &readRect)
			if err != nil {
				continue
			}

			/* Set cursor location based on the reported location from the message */
			CalculateAndSetCursor(connection, readRect.Left, readRect.Top, true)

			// /* Send the entire block */
			SendBuffer(connection, buf)
			lastViewPortY = ViewPortY
			lastLineLength = readRect.Left

		case win.EVENT_CONSOLE_UPDATE_SIMPLE:
			wX := win.LOWORD(uint32(event.Object))
			wY := win.HIWORD(uint32(event.Object))

			var readRect windows.SmallRect
			readRect.Top = int16(wY)
			readRect.Bottom = int16(wY)
			readRect.Left = int16(wX)
			readRect.Right = int16(consoleInfo.Window.Right)

			/* Set cursor location based on the reported location from the message */
			CalculateAndSetCursor(connection, int16(wX), int16(wY), true)

			var coordBufSize windows.Coord

			coordBufSize.Y = readRect.Bottom - readRect.Top + 1
			coordBufSize.X = readRect.Right - readRect.Left + 1
			bufferSize := coordBufSize.X * coordBufSize.Y
			if bufferSize > MAX_EXPECTED_BUFFER_SIZE {
				continue
			}

			/* The top left destination cell of the temporary buffer is row 0, col 0 */

			var coordBufCoord windows.Coord

			/* Copy the block from the screen buffer to the temp. buffer */
			buf := make([]CharInfo, bufferSize)
			err = ReadConsoleOutput(childOutput, buf, coordBufSize, coordBufCoord, &readRect)
			if err != nil {
				panic(err)
			}

			SendBuffer(connection, buf)

		case win.EVENT_CONSOLE_UPDATE_SCROLL:

			vd := (event.Child)
			vn := (vd * vd) / 2 //abs

			if vd > 0 {
				if ViewPortY > 0 {
					ViewPortY -= uint(vn)
				}
			} else {
				ViewPortY += uint(vn)
			}

		case win.EVENT_CONSOLE_LAYOUT:

			if consoleInfo.MaximumWindowSize.X == consoleInfo.Size.X &&
				consoleInfo.MaximumWindowSize.Y == consoleInfo.Size.Y &&
				(consoleInfo.CursorPosition.X == 0 && consoleInfo.CursorPosition.Y == 0) {
				/* Screen has switched to fullscreen mode */
				SendClearScreen(connection)
				savedViewPortY = ViewPortY
				savedLastViewPortY = lastViewPortY
				ViewPortY = 0
				lastViewPortY = 0
				bFullScreen = true
			} else {
				/* Leave full screen mode if applicable */
				if bFullScreen {
					SendClearScreen(connection)
					ViewPortY = savedViewPortY
					lastViewPortY = savedLastViewPortY
					bFullScreen = false
				}
			}
			break
		}

	}

	return 0
}

var bStartup = true
var bFullScreen = false
var lastX, lastY, currentLine, lastLineLength int16
var ViewPortY, lastViewPortY, savedViewPortY, savedLastViewPortY uint

func CalculateAndSetCursor(connection ssh.Channel, x, y int16, scroll bool) {
	if scroll && y > currentLine {
		for n := currentLine; n < y; n++ {
			SendLF(connection)
		}
	}

	SendSetCursor(connection, int(x+1), int(y+1))
	currentLine = y
}

func SendBuffer(connection ssh.Channel, buffer []CharInfo) {

	for _, c := range buffer {
		SendCharacter(connection, c.Attributes, (c.UnicodeChar))
	}
}

func SendCharacter(connection ssh.Channel, attributes uint16, char uint16) {
	if char == 0 {
		return
	}

	/* Handle the foreground intensity */
	forgroundIntensity := 0
	if attributes&(FOREGROUND_INTENSITY) != 0 {
		forgroundIntensity = 1
	}

	backgroundIntensity := 39
	/* Handle the background intensity */
	if attributes&(BACKGROUND_INTENSITY) != 0 {
		backgroundIntensity = 1
	}

	/* Handle the underline */
	underline := 24
	if attributes&(COMMON_LVB_UNDERSCORE) != 0 {
		underline = 4
	}

	/* Handle reverse video */
	reverseVideo := 27
	if attributes&(COMMON_LVB_REVERSE_VIDEO) != 0 {
		reverseVideo = 7
	}

	/* Add foreground and backgroundcolors to buffer. */
	foregroundColor := 30 +
		4*toInt((attributes&(FOREGROUND_BLUE)) != 0) +
		2*toInt((attributes&(FOREGROUND_GREEN)) != 0) +
		1*toInt((attributes&(FOREGROUND_RED)) != 0)

	backgroundColor := 40 +
		4*toInt((attributes&(BACKGROUND_BLUE)) != 0) +
		2*toInt((attributes&(BACKGROUND_GREEN)) != 0) +
		1*toInt((attributes&(BACKGROUND_RED)) != 0)

	terminalControl := fmt.Sprintf("\033[%d;%d;%d;%d;%d;%dm", forgroundIntensity, backgroundIntensity, underline, reverseVideo, foregroundColor, backgroundColor)

	if attributes != 0 {
		connection.Write([]byte(terminalControl))
	}

	connection.Write([]byte(windows.UTF16ToString([]uint16{char})))

}

func toInt(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}

func SendSetCursor(connection ssh.Channel, X, Y int) {
	connection.Write([]byte(fmt.Sprintf("\033[%d;%dH", Y, X)))
}

func SendLF(connection ssh.Channel) {
	connection.Write([]byte("\n"))
}

func SendClearScreen(connection ssh.Channel) {
	connection.Write([]byte("\033[2J"))
}

func ConsoleEventProc(eventsQueue chan<- Event) win.WINEVENTPROC {
	return func(hWinEventHook win.HWINEVENTHOOK, event uint32, hwnd win.HWND, idObject int32, idChild int32, idEventThread uint32, dwmsEventTime uint32) uintptr {

		go func() {
			eventsQueue <- Event{event, hwnd, idObject, idChild}
		}()
		return 0
	}
}

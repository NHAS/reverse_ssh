// +build windows

package shellhost

import (
	"github.com/lxn/win"
)

type key_translation struct {
	in         string
	vk         int
	out        byte
	in_key_len int
	ctrlState  uint32
}

var (
	VK_A = 0x41
	VK_B = 0x42
	VK_C = 0x43
	VK_D = 0x44
	VK_E = 0x45
	VK_F = 0x46
	VK_G = 0x47
	VK_H = 0x48
	VK_I = 0x49
	VK_J = 0x4A
	VK_K = 0x4B
	VK_L = 0x4C
	VK_M = 0x4D
	VK_N = 0x4E
	VK_O = 0x4F
	VK_P = 0x50
	VK_Q = 0x51
	VK_R = 0x52
	VK_S = 0x53
	VK_T = 0x54
	VK_U = 0x55
	VK_V = 0x56
	VK_W = 0x57
	VK_X = 0x58
	VK_Y = 0x59
	VK_Z = 0x5A
	VK_0 = 0x30
	VK_1 = 0x31
	VK_2 = 0x32
	VK_3 = 0x33
	VK_4 = 0x34
	VK_5 = 0x35
	VK_6 = 0x36
	VK_7 = 0x37
	VK_8 = 0x38
	VK_9 = 0x39

	SHIFT_PRESSED     = 0x0010
	LEFT_ALT_PRESSED  = 0x0002
	LEFT_CTRL_PRESSED = 0x0004
)

var keys = []key_translation{
	{"\r", win.VK_RETURN, '\r', 0, 0},
	{"\n", win.VK_RETURN, '\r', 0, 0},
	{"\b", win.VK_BACK, '\b', 0, 0},
	{"\x7f", win.VK_BACK, '\b', 0, 0},
	{"\t", win.VK_TAB, '\t', 0, 0},
	{"\x1b[A", win.VK_UP, 0, 0, 0},
	{"\x1b[B", win.VK_DOWN, 0, 0, 0},
	{"\x1b[C", win.VK_RIGHT, 0, 0, 0},
	{"\x1b[D", win.VK_LEFT, 0, 0, 0},
	{"\x1b[F", win.VK_END, 0, 0, 0},  /* KeyPad END */
	{"\x1b[H", win.VK_HOME, 0, 0, 0}, /* KeyPad HOME */
	{"\x1b[Z", win.VK_TAB, '\t', 0, uint32(SHIFT_PRESSED)},
	{"\x1b[1~", win.VK_HOME, 0, 0, 0},
	{"\x1b[2~", win.VK_INSERT, 0, 0, 0},
	{"\x1b[3~", win.VK_DELETE, 0, 0, 0},
	{"\x1b[4~", win.VK_END, 0, 0, 0},
	{"\x1b[5~", win.VK_PRIOR, 0, 0, 0},
	{"\x1b[6~", win.VK_NEXT, 0, 0, 0},
	{"\x1b[11~", win.VK_F1, 0, 0, 0},
	{"\x1b[12~", win.VK_F2, 0, 0, 0},
	{"\x1b[13~", win.VK_F3, 0, 0, 0},
	{"\x1b[14~", win.VK_F4, 0, 0, 0},
	{"\x1b[15~", win.VK_F5, 0, 0, 0},
	{"\x1b[17~", win.VK_F6, 0, 0, 0},
	{"\x1b[18~", win.VK_F7, 0, 0, 0},
	{"\x1b[19~", win.VK_F8, 0, 0, 0},
	{"\x1b[20~", win.VK_F9, 0, 0, 0},
	{"\x1b[21~", win.VK_F10, 0, 0, 0},
	{"\x1b[23~", win.VK_F11, 0, 0, 0},
	{"\x1b[24~", win.VK_F12, 0, 0, 0},
	{"\x1bOA", win.VK_UP, 0, 0, 0},
	{"\x1bOB", win.VK_DOWN, 0, 0, 0},
	{"\x1bOC", win.VK_RIGHT, 0, 0, 0},
	{"\x1bOD", win.VK_LEFT, 0, 0, 0},
	{"\x1bOF", win.VK_END, 0, 0, 0},  /* KeyPad END */
	{"\x1bOH", win.VK_HOME, 0, 0, 0}, /* KeyPad HOME */
	{"\x1bOP", win.VK_F1, 0, 0, 0},
	{"\x1bOQ", win.VK_F2, 0, 0, 0},
	{"\x1bOR", win.VK_F3, 0, 0, 0},
	{"\x1bOS", win.VK_F4, 0, 0, 0},
	{"\x01", VK_A, '\x01', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x02", VK_B, '\x02', 0, uint32(LEFT_CTRL_PRESSED)},
	//{ "\x3",         VK_C,   '\x03' , 0 , uint32(LEFT_CTRL_PRESSED)}, /* Control + C is handled differently */
	{"\x04", VK_D, '\x04', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x05", VK_E, '\x05', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x06", VK_F, '\x06', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x07", VK_G, '\x07', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x08", VK_H, '\x08', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x09", VK_I, '\x09', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x0A", VK_J, '\x0A', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x0B", VK_K, '\x0B', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x0C", VK_L, '\x0C', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x0D", VK_M, '\x0D', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x0E", VK_N, '\x0E', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x0F", VK_O, '\x0F', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x10", VK_P, '\x10', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x11", VK_Q, '\x11', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x12", VK_R, '\x12', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x13", VK_S, '\x13', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x14", VK_T, '\x14', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x15", VK_U, '\x15', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x16", VK_V, '\x16', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x17", VK_W, '\x17', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x18", VK_X, '\x18', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x19", VK_Y, '\x19', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\x1A", VK_Z, '\x1A', 0, uint32(LEFT_CTRL_PRESSED)},
	{"\033a", VK_A, 'a', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033b", VK_B, 'b', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033c", VK_C, 'c', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033d", VK_D, 'd', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033e", VK_E, 'e', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033f", VK_F, 'f', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033g", VK_G, 'g', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033h", VK_H, 'h', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033i", VK_I, 'i', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033j", VK_J, 'j', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033k", VK_K, 'k', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033l", VK_L, 'l', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033m", VK_M, 'm', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033n", VK_N, 'n', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033o", VK_O, 'o', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033p", VK_P, 'p', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033q", VK_Q, 'q', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033r", VK_R, 'r', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033s", VK_S, 's', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033t", VK_T, 't', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033u", VK_U, 'u', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033v", VK_V, 'v', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033w", VK_W, 'w', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033x", VK_X, 'x', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033y", VK_Y, 'y', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033z", VK_Z, 'z', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0330", VK_0, '0', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0331", VK_1, '1', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0332", VK_2, '2', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0333", VK_3, '3', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0334", VK_4, '4', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0335", VK_5, '5', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0336", VK_6, '6', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0337", VK_7, '7', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0338", VK_8, '8', 0, uint32(LEFT_ALT_PRESSED)},
	{"\0339", VK_9, '9', 0, uint32(LEFT_ALT_PRESSED)},
	{"\033!", VK_1, '!', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033@", VK_2, '@', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033#", VK_3, '#', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033$", VK_4, '$', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033%", VK_5, '%', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033^", VK_6, '^', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033&", VK_7, '&', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033*", VK_8, '*', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033(", VK_9, '(', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
	{"\033)", VK_0, ')', 0, uint32(LEFT_ALT_PRESSED) | uint32(SHIFT_PRESSED)},
}

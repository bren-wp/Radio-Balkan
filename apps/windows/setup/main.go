//go:build windows

package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

//go:embed RadioBalkan-Portable.exe
var appBytes []byte

//go:embed app.ico
var setupIconBytes []byte

const (
	appName              = "Radio Balkan"
	cls                  = "RadioBalkanSetupWindow"
	appWindowClass       = "RadioBalkanNativeWindow"
	legacyAppWindowClass = "RadioHrvatskaNativeWindow"
	WS_OVERLAPPED        = 0x00000000
	WS_CAPTION           = 0x00C00000
	WS_SYSMENU           = 0x00080000
	WS_MINIMIZEBOX       = 0x00020000
	WS_VISIBLE           = 0x10000000
	WM_CREATE            = 1
	WM_DESTROY           = 2
	WM_PAINT             = 0x000F
	WM_ERASEBKGND        = 0x0014
	WM_CLOSE             = 0x0010
	WM_KEYDOWN           = 0x0100
	WM_LBUTTONDOWN       = 0x0201
	WM_APP               = 0x8000
	VK_TAB               = 0x09
	VK_RETURN            = 0x0D
	VK_ESCAPE            = 0x1B
	VK_SPACE             = 0x20
	VK_LEFT              = 0x25
	VK_UP                = 0x26
	VK_RIGHT             = 0x27
	VK_DOWN              = 0x28
	IDC_ARROW            = 32512
	DT_LEFT              = 0
	DT_CENTER            = 1
	DT_VCENTER           = 4
	DT_SINGLELINE        = 0x20
	DT_WORDBREAK         = 0x10
	TRANSPARENT          = 1
	SRCCOPY              = 0x00CC0020
	MB_OK                = 0
	MB_ICONINFORMATION   = 0x40
	MB_ICONWARNING       = 0x30
	MB_YESNO             = 4
	IDYES                = 6
	SW_SHOW              = 5

	DWMWA_USE_IMMERSIVE_DARK_MODE  = 20
	DWMWA_WINDOW_CORNER_PREFERENCE = 33
	DWMWA_BORDER_COLOR             = 34
	DWMWA_CAPTION_COLOR            = 35
	DWMWA_TEXT_COLOR               = 36
	DWMWA_SYSTEMBACKDROP_TYPE      = 38
	DWMWCP_ROUND                   = 2
	DWMSBT_MAINWINDOW              = 2
)

var appVersion = "0.0.41"

type WNDCLASS struct {
	Style                                    uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground syscall.Handle
	LpszMenuName, LpszClassName              *uint16
}
type POINT struct{ X, Y int32 }
type MSG struct {
	Hwnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type RECT struct{ Left, Top, Right, Bottom int32 }
type PAINTSTRUCT struct {
	Hdc                  syscall.Handle
	FErase               int32
	RcPaint              RECT
	FRestore, FIncUpdate int32
	RgbReserved          [32]byte
}

type installerState struct {
	mu                         sync.RWMutex
	hwnd                       syscall.Handle
	desktop, runAfter, startup bool
	installing, done           bool
	focus                      int
	status                     string
	errorText                  string
	progress                   int
	bg, panel                  syscall.Handle
	font, bold, title          syscall.Handle
	icon                       syscall.Handle
}

var st installerState

func setInstallStatus(status string, progress int) {
	st.mu.Lock()
	st.status = status
	if progress >= 0 {
		st.progress = progress
	}
	st.mu.Unlock()
	invalidate()
}
func setInstallFlags(installing, done bool) {
	st.mu.Lock()
	st.installing = installing
	st.done = done
	st.mu.Unlock()
	invalidate()
}

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	dwmapi                 = syscall.NewLazyDLL("dwmapi.dll")
	regClass               = user32.NewProc("RegisterClassW")
	createWin              = user32.NewProc("CreateWindowExW")
	defProc                = user32.NewProc("DefWindowProcW")
	showWin                = user32.NewProc("ShowWindow")
	updateWin              = user32.NewProc("UpdateWindow")
	getMsg                 = user32.NewProc("GetMessageW")
	transMsg               = user32.NewProc("TranslateMessage")
	dispMsg                = user32.NewProc("DispatchMessageW")
	postQuit               = user32.NewProc("PostQuitMessage")
	beginPaint             = user32.NewProc("BeginPaint")
	endPaint               = user32.NewProc("EndPaint")
	getClient              = user32.NewProc("GetClientRect")
	fillRect               = user32.NewProc("FillRect")
	invRect                = user32.NewProc("InvalidateRect")
	loadCursor             = user32.NewProc("LoadCursorW")
	messageBoxP            = user32.NewProc("MessageBoxW")
	createIconP            = user32.NewProc("CreateIconFromResourceEx")
	destroyIconP           = user32.NewProc("DestroyIcon")
	findWindowP            = user32.NewProc("FindWindowW")
	postMessageP           = user32.NewProc("PostMessageW")
	drawText               = user32.NewProc("DrawTextW")
	setBkMode              = gdi32.NewProc("SetBkMode")
	setTextColor           = gdi32.NewProc("SetTextColor")
	createBrushP           = gdi32.NewProc("CreateSolidBrush")
	createFontP            = gdi32.NewProc("CreateFontW")
	selectObj              = gdi32.NewProc("SelectObject")
	deleteObj              = gdi32.NewProc("DeleteObject")
	createPen              = gdi32.NewProc("CreatePen")
	roundRect              = gdi32.NewProc("RoundRect")
	ellipseP               = gdi32.NewProc("Ellipse")
	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	deleteDC               = gdi32.NewProc("DeleteDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	bitBlt                 = gdi32.NewProc("BitBlt")
	dwmAttr                = dwmapi.NewProc("DwmSetWindowAttribute")
)

func u16(s string) *uint16 {
	s = strings.ReplaceAll(s, "\x00", " ")
	p, err := syscall.UTF16PtrFromString(s)
	if err == nil && p != nil {
		return p
	}
	p, _ = syscall.UTF16PtrFromString("")
	return p
}
func color(r, g, b byte) uint32      { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }
func rgb(r, g, b byte) uintptr       { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func inside(x, y int32, r RECT) bool { return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom }

var desktopRect = RECT{52, 222, 76, 246}
var runRect = RECT{52, 260, 76, 284}
var startupRect = RECT{52, 298, 76, 322}
var desktopHitRect = RECT{46, 214, 554, 253}
var runHitRect = RECT{46, 252, 554, 291}
var startupHitRect = RECT{46, 290, 554, 329}
var installRect = RECT{342, 376, 558, 426}
var cancelRect = RECT{230, 376, 330, 426}

const installerFocusCount = 5

func main() {
	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "--uninstall") {
		uninstall()
		return
	}

	// Win32 owns a window and its message queue on the thread that created it.
	// Keep the Setup window and GetMessage loop on that same Windows thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initGDI()
	defer cleanup()
	st.desktop = true
	st.runAfter = true
	st.startup = false
	st.focus = 4
	st.status = "Spremno za instalaciju"
	if fileExists(targetExe()) || fileExists(legacyTargetExe()) {
		st.status = "Pronađena je postojeća verzija · spremno za ažuriranje"
	}
	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	cur, _, _ := loadCursor.Call(0, IDC_ARROW)
	wc := WNDCLASS{LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hInst), HIcon: st.icon, HCursor: syscall.Handle(cur), HbrBackground: st.bg, LpszClassName: u16(cls)}
	if r, _, _ := regClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return
	}
	h, _, _ := createWin.Call(0, uintptr(unsafe.Pointer(u16(cls))), uintptr(unsafe.Pointer(u16("Radio Balkan Setup"))), WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE, 260, 120, 620, 500, 0, 0, hInst, 0)
	if h == 0 {
		return
	}
	st.hwnd = syscall.Handle(h)
	dark(st.hwnd)
	showWin.Call(h, SW_SHOW)
	updateWin.Call(h)
	var m MSG
	for {
		r, _, _ := getMsg.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		transMsg.Call(uintptr(unsafe.Pointer(&m)))
		dispMsg.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(hwnd syscall.Handle, message uint32, w, l uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			_ = writeSetupLog(fmt.Sprintf("panic msg=0x%X: %v\n%s", message, r, debug.Stack()))
			st.mu.Lock()
			st.status = "Pojavila se greška prikaza · možeš pokušati ponovno"
			st.mu.Unlock()
			ret = 0
		}
	}()
	return wndProcCore(hwnd, message, w, l)
}

func wndProcCore(hwnd syscall.Handle, message uint32, w, l uintptr) uintptr {
	switch message {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		paint(hwnd)
		return 0
	case WM_LBUTTONDOWN:
		x := int32(int16(uint16(l & 0xffff)))
		y := int32(int16(uint16((l >> 16) & 0xffff)))
		click(x, y)
		return 0
	case WM_KEYDOWN:
		handleKey(w)
		return 0
	case WM_APP + 1:
		st.mu.Lock()
		errText := st.errorText
		st.errorText = ""
		st.mu.Unlock()
		if errText != "" {
			msg(st.hwnd, "Greška", errText, MB_ICONWARNING)
		}
		return 0
	case WM_CLOSE:
		st.mu.RLock()
		installing := st.installing
		st.mu.RUnlock()
		if !installing {
			postQuit.Call(0)
		}
		return 0
	case WM_DESTROY:
		postQuit.Call(0)
		return 0
	}
	r, _, _ := defProc.Call(uintptr(hwnd), uintptr(message), w, l)
	return r
}

func initGDI() {
	st.icon = createIconFromICO(setupIconBytes, 64)
	st.bg = brush(color(25, 20, 17))
	st.panel = brush(color(31, 25, 22))
	st.font = font(16, 400, "Segoe UI Variable Text")
	st.bold = font(17, 700, "Segoe UI Variable Text")
	st.title = font(29, 700, "Segoe UI Variable Display")
}
func cleanup() {
	for _, h := range []syscall.Handle{st.bg, st.panel, st.font, st.bold, st.title} {
		if h != 0 {
			deleteObj.Call(uintptr(h))
		}
	}
	if st.icon != 0 {
		destroyIconP.Call(uintptr(st.icon))
	}
}
func createIconFromICO(data []byte, want int) syscall.Handle {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return 0
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	bestOff, bestSize, bestDiff := 0, 0, 1<<30
	for i := 0; i < count; i++ {
		p := 6 + i*16
		if p+16 > len(data) {
			break
		}
		w := int(data[p])
		if w == 0 {
			w = 256
		}
		h := int(data[p+1])
		if h == 0 {
			h = 256
		}
		d := w - want
		if d < 0 {
			d = -d
		}
		hd := h - want
		if hd < 0 {
			hd = -hd
		}
		d += hd
		sz := int(binary.LittleEndian.Uint32(data[p+8 : p+12]))
		off := int(binary.LittleEndian.Uint32(data[p+12 : p+16]))
		if off >= 0 && sz > 0 && off+sz <= len(data) && d < bestDiff {
			bestDiff = d
			bestOff = off
			bestSize = sz
		}
	}
	if bestSize == 0 {
		return 0
	}
	r, _, _ := createIconP.Call(uintptr(unsafe.Pointer(&data[bestOff])), uintptr(bestSize), 1, 0x00030000, uintptr(want), uintptr(want), 0)
	return syscall.Handle(r)
}

func brush(c uint32) syscall.Handle {
	r, _, _ := createBrushP.Call(uintptr(c))
	return syscall.Handle(r)
}
func font(h, w int32, f string) syscall.Handle {
	r, _, _ := createFontP.Call(uintptr(int32(-h)), 0, 0, 0, uintptr(w), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16(f))))
	return syscall.Handle(r)
}
func setupDwmInt32(hwnd syscall.Handle, attribute uintptr, value int32) {
	if hwnd == 0 {
		return
	}
	dwmAttr.Call(uintptr(hwnd), attribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}
func setupDwmColor(hwnd syscall.Handle, attribute uintptr, value uint32) {
	if hwnd == 0 {
		return
	}
	dwmAttr.Call(uintptr(hwnd), attribute, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}
func dark(hwnd syscall.Handle) {
	setupDwmInt32(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, 1)
	setupDwmInt32(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_ROUND)
	setupDwmInt32(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, DWMSBT_MAINWINDOW)
	setupDwmColor(hwnd, DWMWA_BORDER_COLOR, color(62, 51, 44))
	setupDwmColor(hwnd, DWMWA_CAPTION_COLOR, color(25, 20, 17))
	setupDwmColor(hwnd, DWMWA_TEXT_COLOR, color(242, 237, 232))
}
func invalidate() {
	if st.hwnd != 0 {
		invRect.Call(uintptr(st.hwnd), 0, 1)
	}
}

func paint(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdcRaw, _, _ := beginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdcRaw == 0 {
		return
	}
	hdc := syscall.Handle(hdcRaw)
	defer endPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	var cr RECT
	getClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&cr)))
	width, height := cr.Right-cr.Left, cr.Bottom-cr.Top
	if width <= 0 || height <= 0 {
		return
	}
	memRaw, _, _ := createCompatibleDC.Call(uintptr(hdc))
	if memRaw == 0 {
		paintClient(hdc, cr)
		return
	}
	memDC := syscall.Handle(memRaw)
	defer deleteDC.Call(uintptr(memDC))
	bmpRaw, _, _ := createCompatibleBitmap.Call(uintptr(hdc), uintptr(width), uintptr(height))
	if bmpRaw == 0 {
		paintClient(hdc, cr)
		return
	}
	bmp := syscall.Handle(bmpRaw)
	oldBmp, _, _ := selectObj.Call(uintptr(memDC), uintptr(bmp))
	paintClient(memDC, cr)
	bitBlt.Call(uintptr(hdc), 0, 0, uintptr(width), uintptr(height), uintptr(memDC), 0, 0, SRCCOPY)
	if oldBmp != 0 {
		selectObj.Call(uintptr(memDC), oldBmp)
	}
	deleteObj.Call(uintptr(bmp))
}

func paintClient(hdc syscall.Handle, cr RECT) {
	st.mu.RLock()
	desktop, runAfter, startup := st.desktop, st.runAfter, st.startup
	installing, done := st.installing, st.done
	focus := st.focus
	status, progress := st.status, st.progress
	st.mu.RUnlock()
	fillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&cr)), uintptr(st.bg))
	drawPanel(hdc, 32, 28, cr.Right-32, cr.Bottom-28)
	selectFont(hdc, st.title)
	circle(hdc, 54, 48, 94, 88, color(235, 83, 35))
	selectFont(hdc, st.bold)
	txt(hdc, "▶", 62, 49, 87, 87, rgb(255, 255, 255), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, st.title)
	txt(hdc, "Radio Balkan", 108, 48, 550, 88, rgb(247, 244, 241), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, st.font)
	txt(hdc, "Radio uživo iz Hrvatske i regije · verzija "+appVersion, 108, 88, 550, 118, rgb(172, 159, 151), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	txt(hdc, "Slušaj omiljene stanice iz Hrvatske, Bosne i Hercegovine, Srbije, Slovenije, Sjeverne Makedonije, Albanije i ostatka Balkana. Instalacija ne traži administratorska prava.", 54, 132, 548, 194, rgb(205, 196, 189), DT_LEFT|DT_WORDBREAK)
	checkbox(hdc, desktopRect, desktop, "Kreiraj prečac na radnoj površini", !installing && focus == 0)
	checkbox(hdc, runRect, runAfter, "Pokreni Radio Balkan nakon instalacije", !installing && focus == 1)
	checkbox(hdc, startupRect, startup, "Pokreni Radio Balkan zajedno s Windowsom", !installing && focus == 2)
	selectFont(hdc, st.font)
	txt(hdc, "Lokacija: "+installDir(), 54, 336, 550, 358, rgb(133, 122, 115), DT_LEFT|DT_SINGLELINE)
	if installing || done {
		progressBar(hdc, RECT{54, 358, 548, 366}, progress)
	}
	button(hdc, cancelRect, "Odustani", false, !installing && focus == 3)
	label := "Instaliraj"
	if fileExists(targetExe()) {
		label = "Ažuriraj"
	}
	if installing {
		label = "Instaliram…"
	}
	if done {
		label = "Završeno"
	}
	button(hdc, installRect, label, true, !installing && focus == 4)
	txt(hdc, status, 54, 434, 550, 462, rgb(184, 169, 158), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func drawPanel(hdc syscall.Handle, l, t, r, b int32) {
	rounded(hdc, l, t, r, b, 18, color(31, 25, 22), color(58, 45, 39))
}
func circle(hdc syscall.Handle, l, t, r, b int32, fill uint32) {
	br := brush(fill)
	pen, _, _ := createPen.Call(0, 1, rgb(byte(fill&0xff), byte(fill>>8&0xff), byte(fill>>16&0xff)))
	ob, _, _ := selectObj.Call(uintptr(hdc), uintptr(br))
	op, _, _ := selectObj.Call(uintptr(hdc), pen)
	ellipseP.Call(uintptr(hdc), uintptr(l), uintptr(t), uintptr(r), uintptr(b))
	selectObj.Call(uintptr(hdc), ob)
	selectObj.Call(uintptr(hdc), op)
	deleteObj.Call(uintptr(br))
	deleteObj.Call(pen)
}
func rounded(hdc syscall.Handle, l, t, r, b, rad int32, fill, border uint32) {
	br := brush(fill)
	pen, _, _ := createPen.Call(0, 1, rgb(byte(border&0xff), byte(border>>8&0xff), byte(border>>16&0xff)))
	ob, _, _ := selectObj.Call(uintptr(hdc), uintptr(br))
	op, _, _ := selectObj.Call(uintptr(hdc), pen)
	roundRect.Call(uintptr(hdc), uintptr(l), uintptr(t), uintptr(r), uintptr(b), uintptr(rad), uintptr(rad))
	selectObj.Call(uintptr(hdc), ob)
	selectObj.Call(uintptr(hdc), op)
	deleteObj.Call(uintptr(br))
	deleteObj.Call(pen)
}
func button(hdc syscall.Handle, r RECT, label string, primary, focused bool) {
	fill := color(43, 36, 32)
	border := color(71, 57, 49)
	c := rgb(231, 224, 219)
	if primary {
		fill = color(235, 83, 35)
		border = color(235, 83, 35)
		c = rgb(255, 255, 255)
	}
	if focused {
		border = color(255, 177, 61)
	}
	rounded(hdc, r.Left, r.Top, r.Right, r.Bottom, 11, fill, border)
	selectFont(hdc, st.bold)
	txt(hdc, label, r.Left, r.Top, r.Right, r.Bottom, c, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}
func progressBar(hdc syscall.Handle, r RECT, pct int) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	rounded(hdc, r.Left, r.Top, r.Right, r.Bottom, 6, color(43, 36, 32), color(71, 57, 49))
	w := int32((int64(r.Right-r.Left-2) * int64(pct)) / 100)
	if w > 0 {
		rounded(hdc, r.Left+1, r.Top+1, r.Left+1+w, r.Bottom-1, 5, color(235, 83, 35), color(235, 83, 35))
	}
}
func checkbox(hdc syscall.Handle, r RECT, on bool, label string, focused bool) {
	border := color(90, 70, 59)
	if focused {
		border = color(255, 177, 61)
	}
	rounded(hdc, r.Left, r.Top, r.Right, r.Bottom, 6, color(43, 36, 32), border)
	if on {
		selectFont(hdc, st.bold)
		txt(hdc, "✓", r.Left, r.Top-1, r.Right, r.Bottom+1, rgb(255, 151, 74), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	selectFont(hdc, st.font)
	txt(hdc, label, r.Right+12, r.Top-3, 548, r.Bottom+4, rgb(221, 214, 208), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
}
func selectFont(hdc syscall.Handle, h syscall.Handle) { selectObj.Call(uintptr(hdc), uintptr(h)) }
func txt(hdc syscall.Handle, s string, l, t, r, b int32, c uintptr, flags uint32) {
	rc := RECT{l, t, r, b}
	setBkMode.Call(uintptr(hdc), TRANSPARENT)
	setTextColor.Call(uintptr(hdc), c)
	drawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(u16(s))), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), uintptr(flags))
}

func nextInstallerFocus(current, delta int) int {
	if installerFocusCount <= 0 {
		return 0
	}
	next := (current + delta) % installerFocusCount
	if next < 0 {
		next += installerFocusCount
	}
	return next
}

func setInstallerFocus(index int) {
	st.mu.Lock()
	if index < 0 {
		index = 0
	}
	if index >= installerFocusCount {
		index = installerFocusCount - 1
	}
	st.focus = index
	st.mu.Unlock()
	invalidate()
}

func handleKey(key uintptr) {
	st.mu.RLock()
	installing := st.installing
	focus := st.focus
	st.mu.RUnlock()
	if installing {
		return
	}
	switch key {
	case VK_TAB, VK_RIGHT, VK_DOWN:
		setInstallerFocus(nextInstallerFocus(focus, 1))
	case VK_LEFT, VK_UP:
		setInstallerFocus(nextInstallerFocus(focus, -1))
	case VK_RETURN, VK_SPACE:
		activateInstallerControl(focus)
	case VK_ESCAPE:
		postQuit.Call(0)
	}
}

func click(x, y int32) {
	index := -1
	switch {
	case inside(x, y, desktopHitRect):
		index = 0
	case inside(x, y, runHitRect):
		index = 1
	case inside(x, y, startupHitRect):
		index = 2
	case inside(x, y, cancelRect):
		index = 3
	case inside(x, y, installRect):
		index = 4
	}
	if index < 0 {
		return
	}
	setInstallerFocus(index)
	activateInstallerControl(index)
}

func activateInstallerControl(index int) {
	st.mu.RLock()
	installing := st.installing
	done := st.done
	st.mu.RUnlock()
	if installing {
		return
	}
	switch index {
	case 0:
		st.mu.Lock()
		st.desktop = !st.desktop
		st.mu.Unlock()
		invalidate()
	case 1:
		st.mu.Lock()
		st.runAfter = !st.runAfter
		st.mu.Unlock()
		invalidate()
	case 2:
		st.mu.Lock()
		st.startup = !st.startup
		st.mu.Unlock()
		invalidate()
	case 3:
		postQuit.Call(0)
	case 4:
		if done {
			postQuit.Call(0)
			return
		}
		st.mu.Lock()
		st.installing = true
		st.status = "Pripremam datoteke…"
		st.progress = 5
		st.mu.Unlock()
		invalidate()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fail(fmt.Errorf("neočekivana greška: %v", r))
				}
			}()
			install()
		}()
	}
}

func install() {
	unlock, err := acquireInstallLock()
	if err != nil {
		fail(err)
		return
	}
	defer unlock()
	if len(appBytes) < 2 || string(appBytes[:2]) != "MZ" {
		fail(fmt.Errorf("ugrađena aplikacija nije valjan Windows executable"))
		return
	}
	setInstallStatus("Zatvaram postojeću verziju…", 10)
	closeRunningApp()
	dir := installDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		fail(err)
		return
	}
	if len(setupIconBytes) > 0 {
		if err := writeFileDurable(iconPath(), setupIconBytes, 0644); err != nil {
			_ = writeSetupLog("ikona: " + err.Error())
		}
	}
	setInstallStatus("Instaliram Radio Balkan…", 30)
	tmp := targetExe() + ".tmp"
	old := targetExe() + ".old"
	_ = os.Remove(tmp)
	_ = os.Remove(old)
	if err := writeFileDurable(tmp, appBytes, 0755); err != nil {
		fail(err)
		return
	}
	if fileExists(targetExe()) {
		if err := renameWithRetry(targetExe(), old, 12, 150*time.Millisecond); err != nil {
			_ = os.Remove(tmp)
			fail(fmt.Errorf("postojeća aplikacija je možda još pokrenuta: %w", err))
			return
		}
	}
	if err := renameWithRetry(tmp, targetExe(), 12, 150*time.Millisecond); err != nil {
		if fileExists(old) {
			_ = renameWithRetry(old, targetExe(), 6, 120*time.Millisecond)
		}
		fail(err)
		return
	}
	setInstallStatus("Provjeravam instaliranu datoteku…", 45)
	installedHash, err := fileSHA256(targetExe())
	embeddedHash := sha256.Sum256(appBytes)
	if err != nil || installedHash != embeddedHash {
		_ = os.Remove(targetExe())
		if fileExists(old) {
			_ = os.Rename(old, targetExe())
		}
		if err == nil {
			err = fmt.Errorf("SHA-256 provjera instalirane datoteke nije prošla")
		}
		fail(err)
		return
	}
	_ = os.Remove(old)
	cleanupLegacyInstall()
	setInstallStatus("Pripremam deinstalaciju…", 55)
	self, err := os.Executable()
	if err != nil {
		fail(fmt.Errorf("ne mogu odrediti putanju setup programa: %w", err))
		return
	}
	uninstallerBytes, err := os.ReadFile(self)
	if err != nil {
		fail(fmt.Errorf("ne mogu pripremiti deinstalaciju: %w", err))
		return
	}
	uninstaller := filepath.Join(dir, "Uninstall.exe")
	uTmp := uninstaller + ".tmp"
	_ = os.Remove(uTmp)
	if err := writeFileDurable(uTmp, uninstallerBytes, 0755); err != nil {
		fail(fmt.Errorf("ne mogu zapisati deinstalator: %w", err))
		return
	}
	_ = os.Remove(uninstaller)
	if err := renameWithRetry(uTmp, uninstaller, 8, 120*time.Millisecond); err != nil {
		_ = os.Remove(uTmp)
		fail(fmt.Errorf("ne mogu instalirati deinstalator: %w", err))
		return
	}
	setInstallStatus("Kreiram prečace i Windows zapis…", 75)
	st.mu.RLock()
	desktop, runAfter, startup := st.desktop, st.runAfter, st.startup
	st.mu.RUnlock()
	_ = os.Remove(filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Radio Hrvatska.lnk"))
	_ = os.Remove(filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "Radio Hrvatska.lnk"))
	if err := createShortcut(startShortcut(), targetExe()); err != nil {
		_ = writeSetupLog("Start prečac: " + err.Error())
	}
	if desktop {
		if err := createShortcut(desktopShortcut(), targetExe()); err != nil {
			_ = writeSetupLog("Desktop prečac: " + err.Error())
		}
	} else {
		_ = os.Remove(desktopShortcut())
	}
	writeUninstallRegistry()
	setStartup(startup)
	st.mu.Lock()
	st.installing = false
	st.done = true
	st.status = "Instalacija je završena."
	st.progress = 100
	st.mu.Unlock()
	invalidate()
	if runAfter {
		_ = exec.Command(targetExe()).Start()
	}
}
func renameWithRetry(src, dst string, attempts int, delay time.Duration) error {
	if attempts < 1 {
		attempts = 1
	}
	var last error
	for i := 0; i < attempts; i++ {
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			last = err
		}
		if i+1 < attempts {
			time.Sleep(delay)
		}
	}
	return last
}

func acquireInstallLock() (func(), error) {
	lockDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "RadioBalkan")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(lockDir, "install.lock")
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 10*time.Minute {
			_ = os.Remove(path)
			continue
		}
		return nil, fmt.Errorf("instalacija je već pokrenuta")
	}
	return nil, fmt.Errorf("nije moguće zaključati instalaciju")
}

func writeFileDurable(path string, data []byte, perm os.FileMode) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return f.Sync()
}

func fileSHA256(path string) ([32]byte, error) {
	var zero [32]byte
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 128*1024)
	if _, err = io.CopyBuffer(h, f, buf); err != nil {
		return zero, err
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func fail(err error) {
	if err == nil {
		return
	}
	_ = writeSetupLog(err.Error())
	st.mu.Lock()
	st.installing = false
	st.status = "Instalacija nije uspjela"
	st.errorText = err.Error()
	st.mu.Unlock()
	invalidate()
	if st.hwnd != 0 {
		postMessageP.Call(uintptr(st.hwnd), WM_APP+1, 0, 0)
	}
}
func writeSetupLog(line string) error {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "RadioBalkan", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "setup.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), line)
	return err
}

func closeRunningApp() {
	for i := 0; i < 20; i++ {
		h, _, _ := findWindowP.Call(uintptr(unsafe.Pointer(u16(appWindowClass))), 0)
		if h == 0 {
			h, _, _ = findWindowP.Call(uintptr(unsafe.Pointer(u16(legacyAppWindowClass))), 0)
		}
		if h == 0 {
			return
		}
		if i == 0 {
			postMessageP.Call(h, WM_CLOSE, 0, 0)
		}
		time.Sleep(150 * time.Millisecond)
	}
	for _, image := range []string{"RadioBalkan.exe", "RadioHrvatska.exe"} {
		c := exec.Command("taskkill.exe", "/IM", image, "/F")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
	}
	time.Sleep(250 * time.Millisecond)
}

func installDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Radio Balkan")
}
func targetExe() string { return filepath.Join(installDir(), "RadioBalkan.exe") }
func iconPath() string  { return filepath.Join(installDir(), "RadioBalkan.ico") }
func legacyInstallDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Radio Hrvatska")
}
func legacyTargetExe() string { return filepath.Join(legacyInstallDir(), "RadioHrvatska.exe") }
func cleanupLegacyInstall() {
	legacy := legacyInstallDir()
	for _, name := range []string{"RadioHrvatska.exe", "RadioHrvatska.ico", "Uninstall.exe"} {
		_ = os.Remove(filepath.Join(legacy, name))
	}
	_ = os.Remove(legacy)
}
func startShortcut() string {
	return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Radio Balkan.lnk")
}
func desktopShortcut() string {
	return filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "Radio Balkan.lnk")
}
func fileExists(p string) bool { _, e := os.Stat(p); return e == nil }
func psQuote(s string) string  { return strings.ReplaceAll(s, "'", "''") }
func createShortcut(path, target string) error {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	script := fmt.Sprintf(`$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut('%s');$s.TargetPath='%s';$s.WorkingDirectory='%s';$s.IconLocation='%s';$s.Description='Radio Balkan';$s.Save()`, psQuote(path), psQuote(target), psQuote(filepath.Dir(target)), psQuote(iconPath()))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
func setStartup(on bool) {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	for _, value := range []string{"RadioHrvatska", "RadioBalkan"} {
		c := exec.Command("reg.exe", "DELETE", key, "/v", value, "/f")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
	}
	if on {
		c := exec.Command("reg.exe", "ADD", key, "/v", "RadioBalkan", "/t", "REG_SZ", "/d", `"`+targetExe()+`"`, "/f")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
	}
}

func writeUninstallRegistry() {
	for _, oldKey := range []string{`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\RadioHrvatska`, `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\RadioBalkan`} {
		c := exec.Command("reg.exe", "DELETE", oldKey, "/f")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
	}
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\RadioBalkan`
	values := [][]string{{"/v", "DisplayName", "/t", "REG_SZ", "/d", appName, "/f"}, {"/v", "DisplayVersion", "/t", "REG_SZ", "/d", appVersion, "/f"}, {"/v", "Publisher", "/t", "REG_SZ", "/d", "Brendigo", "/f"}, {"/v", "InstallLocation", "/t", "REG_SZ", "/d", installDir(), "/f"}, {"/v", "DisplayIcon", "/t", "REG_SZ", "/d", iconPath(), "/f"}, {"/v", "UninstallString", "/t", "REG_SZ", "/d", `"` + filepath.Join(installDir(), "Uninstall.exe") + `" --uninstall`, "/f"}, {"/v", "NoModify", "/t", "REG_DWORD", "/d", "1", "/f"}, {"/v", "NoRepair", "/t", "REG_DWORD", "/d", "1", "/f"}}
	for _, v := range values {
		args := append([]string{"ADD", key}, v...)
		c := exec.Command("reg.exe", args...)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
	}
}

func uninstall() {
	if msg(0, "Radio Balkan", "Želiš li deinstalirati Radio Balkan?", MB_YESNO|MB_ICONWARNING) != IDYES {
		return
	}
	for _, image := range []string{"RadioBalkan.exe", "RadioHrvatska.exe"} {
		kill := exec.Command("taskkill.exe", "/IM", image, "/F")
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = kill.Run()
	}
	_ = os.Remove(startShortcut())
	_ = os.Remove(desktopShortcut())
	_ = os.Remove(filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Radio Hrvatska.lnk"))
	_ = os.Remove(filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "Radio Hrvatska.lnk"))
	setStartup(false)
	for _, key := range []string{`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\RadioBalkan`, `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\RadioHrvatska`} {
		c := exec.Command("reg.exe", "DELETE", key, "/f")
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = c.Run()
	}
	dir := installDir()
	cmdLine := fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & rmdir /s /q "%s"`, dir)
	c2 := exec.Command("cmd.exe", "/C", cmdLine)
	c2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = c2.Start()
	msg(0, "Radio Balkan", "Program je uklonjen.", MB_ICONINFORMATION)
	time.Sleep(200 * time.Millisecond)
}
func msg(hwnd syscall.Handle, title, text string, flags uintptr) int {
	r, _, _ := messageBoxP.Call(uintptr(hwnd), uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags)
	return int(r)
}

//go:build windows

package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unsafe"
)

//go:embed app.ico
var appIconBytes []byte

type RasterImage struct {
	W, H   int32
	Pixels []byte
}

type logoCacheEntry struct {
	Image    *RasterImage
	Loading  bool
	FailedAt time.Time
	UsedAt   time.Time
}

var (
	logoCacheMu sync.Mutex
	logoCache   = map[string]*logoCacheEntry{}
	logoSem     = make(chan struct{}, 2)
)

const (
	appName         = "Radio Balkan"
	className       = "RadioBalkanNativeWindow"
	legacyClassName = "RadioHrvatskaNativeWindow"

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_POPUP            = 0x80000000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WS_BORDER           = 0x00800000
	ES_AUTOHSCROLL      = 0x0080
	EM_SETCUEBANNER     = 0x1501

	SW_SHOW       = 5
	SW_RESTORE    = 9
	CW_USEDEFAULT = ^uint32(0x7fffffff)

	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_PAINT           = 0x000F
	WM_ERASEBKGND      = 0x0014
	WM_KEYDOWN         = 0x0100
	WM_CLOSE           = 0x0010
	WM_ACTIVATEAPP     = 0x001C
	WM_COMMAND         = 0x0111
	WM_APPCOMMAND      = 0x0319
	WM_SETICON         = 0x0080
	WM_MOUSEMOVE       = 0x0200
	WM_MOUSEWHEEL      = 0x020A
	WM_LBUTTONDOWN     = 0x0201
	WM_NCMOUSEMOVE     = 0x00A0
	WM_SIZE            = 0x0005
	WM_GETMINMAXINFO   = 0x0024
	WM_CTLCOLORSTATIC  = 0x0138
	WM_CTLCOLOREDIT    = 0x0133
	WM_CTLCOLORLISTBOX = 0x0134
	WM_APP             = 0x8000
	WM_USER            = 0x0400

	EN_CHANGE  = 0x0310
	BN_CLICKED = 0

	DT_LEFT         = 0x00000000
	DT_CENTER       = 0x00000001
	DT_RIGHT        = 0x00000002
	DT_VCENTER      = 0x00000004
	DT_SINGLELINE   = 0x00000020
	DT_END_ELLIPSIS = 0x00008000
	DT_NOPREFIX     = 0x00000800
	DT_WORDBREAK    = 0x00000010

	TRANSPARENT    = 1
	OPAQUE         = 2
	SRCCOPY        = 0x00CC0020
	DIB_RGB_COLORS = 0
	BI_RGB         = 0

	IDC_ARROW    = 32512
	COLOR_WINDOW = 5

	MF_STRING = 0

	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002

	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONERROR       = 0x00000010
	MB_YESNO           = 0x00000004
	IDYES              = 6

	SW_SHOWNORMAL = 1
	VK_RETURN     = 0x0D
	VK_ESCAPE     = 0x1B
	VK_UP         = 0x26
	VK_DOWN       = 0x28
	VK_F5         = 0x74
	VK_CONTROL    = 0x11
	VK_F          = 0x46

	APPCOMMAND_VOLUME_DOWN         = 9
	APPCOMMAND_VOLUME_UP           = 10
	APPCOMMAND_MEDIA_NEXTTRACK     = 11
	APPCOMMAND_MEDIA_PREVIOUSTRACK = 12
	APPCOMMAND_MEDIA_STOP          = 13
	APPCOMMAND_MEDIA_PLAY_PAUSE    = 14
	WS_EX_DLGMODALFRAME            = 0x00000001
)

var appVersion = "0.0.6"

type WNDCLASS struct {
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
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
type MINMAXINFO struct {
	PtReserved, PtMaxSize, PtMaxPosition, PtMinTrackSize, PtMaxTrackSize POINT
}
type PAINTSTRUCT struct {
	Hdc                  syscall.Handle
	FErase               int32
	RcPaint              RECT
	FRestore, FIncUpdate int32
	RgbReserved          [32]byte
}
type RGBQUAD struct{ Blue, Green, Red, Reserved byte }
type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]RGBQUAD
}

type RadioStation struct {
	ChangeUUID  string `json:"changeuuid"`
	StationUUID string `json:"stationuuid"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	URLResolved string `json:"url_resolved"`
	Homepage    string `json:"homepage"`
	Favicon     string `json:"favicon"`
	Tags        string `json:"tags"`
	Country     string `json:"country"`
	CountryCode string `json:"countrycode"`
	State       string `json:"state"`
	Language    string `json:"language"`
	Votes       int    `json:"votes"`
	Codec       string `json:"codec"`
	Bitrate     int    `json:"bitrate"`
	LastCheckOK int    `json:"lastcheckok"`

	Health      string `json:"-"`
	ActiveURL   string `json:"-"`
	Replaced    bool   `json:"-"`
	SearchIndex string `json:"-"`
	TagsIndex   string `json:"-"`
}

type PersistedState struct {
	Favorites         map[string]bool     `json:"favorites"`
	Replacements      map[string]string   `json:"replacements"`
	Backups           map[string][]string `json:"backups"`
	Recent            []string            `json:"recent"`
	Volume            int                 `json:"volume"`
	HealthIntervalMin int                 `json:"health_interval_min"`
	CountryCode       string              `json:"country_code"`
	Genre             string              `json:"genre"`
	Tab               string              `json:"tab"`
	WindowWidth       int                 `json:"window_width,omitempty"`
	WindowHeight      int                 `json:"window_height,omitempty"`
}

type CacheFile struct {
	SavedAt  time.Time      `json:"saved_at"`
	Stations []RadioStation `json:"stations"`
}

type HitKind int

const (
	hitNone HitKind = iota
	hitTab
	hitPlay
	hitLink
	hitWeb
	hitFavorite
	hitReplace
	hitPlayerPlay
	hitPlayerStop
	hitPlayerPrev
	hitPlayerNext
	hitRefresh
	hitCheckAll
	hitCheckStation
	hitVolumeDown
	hitVolumeUp
	hitCopyNowPlaying
	hitCountryDropdown
	hitGenreDropdown
	hitCountryChoice
	hitGenreChoice
	hitAbout
)

type HitRegion struct {
	R     RECT
	Kind  HitKind
	Index int
	Value string
}

type App struct {
	hwnd                                     syscall.Handle
	edit                                     syscall.Handle
	stations                                 []RadioStation
	filtered                                 []int
	hits                                     []HitRegion
	hoverToken                               string
	mu                                       sync.RWMutex
	stateMu                                  sync.RWMutex
	state                                    PersistedState
	repairMu                                 sync.Mutex
	repairLocks                              map[string]*repairGuard
	playTransitionMu                         sync.Mutex
	saveMu                                   sync.Mutex
	saveTimer                                *time.Timer
	stateFileMu                              sync.Mutex
	cacheFileMu                              sync.Mutex
	tab                                      string
	search                                   string
	genre                                    string
	country                                  string
	genreOptions                             []string
	countryMenuOpen                          bool
	genreMenuOpen                            bool
	countryMenuIndex                         int
	genreMenuIndex                           int
	safeMode                                 bool
	searchTimer                              *time.Timer
	searchSeq                                uint64
	scroll                                   int
	clientHeight                             int32
	current                                  int
	currentKey                               string
	playing                                  bool
	audioMu                                  sync.Mutex
	audioCmd                                 *exec.Cmd
	audioIn                                  io.WriteCloser
	audioRecovering                          bool
	lastAudioFailure                         time.Time
	loading                                  bool
	status                                   string
	http                                     *http.Client
	ctx                                      context.Context
	cancel                                   context.CancelFunc
	hFont, hFontBold, hFontSmall, hFontTitle syscall.Handle
	hIconBig, hIconSmall                     syscall.Handle
	bgBrush, panelBrush, editBrush           syscall.Handle
	nowPlaying                               string
	nowPlayingStation                        string
	healthRunning                            bool
	healthRescanRequested                    bool
	refreshRunning                           bool
	healthOK                                 int
	healthBroken                             int
	healthReplaced                           int
	lastHealth                               time.Time
	metadataSeq                              uint64
	playSeq                                  uint64
	streamSem                                chan struct{}
	done                                     chan struct{}
	closeOnce                                sync.Once
	alertMu                                  sync.Mutex
	alerts                                   []UIAlert
}

// UIAlert is queued from worker goroutines and displayed only on the UI thread.
type UIAlert struct {
	Title string
	Text  string
	Flags uintptr
}

var app App

const inputDialogClassName = "RadioBalkanInputDialog"

type inputDialogState struct {
	hwnd   syscall.Handle
	edit   syscall.Handle
	result string
	ok     bool
	done   bool
}

var inputDialogClassOnce sync.Once
var inputDialogClassErr error
var activeInputDialog *inputDialogState

type CountryDef struct {
	Code string
	Name string
}

var balkanCountries = []CountryDef{
	{"", "Sve podržane zemlje"},
	{"HR", "Hrvatska"},
	{"BA", "Bosna i Hercegovina"},
	{"RS", "Srbija"},
	{"SI", "Slovenija"},
	{"MK", "Sjeverna Makedonija"},
	{"AL", "Albanija"},
	{"ME", "Crna Gora"},
}

func isBalkanCode(code string) bool {
	if code == "" {
		return true
	}
	for _, c := range balkanCountries {
		if c.Code == code {
			return true
		}
	}
	return false
}
func countryCodeByName(name string) string {
	for _, c := range balkanCountries {
		if c.Name == name {
			return c.Code
		}
	}
	return ""
}
func countryNameByCode(code string) string {
	for _, c := range balkanCountries {
		if c.Code == code {
			return c.Name
		}
	}
	return code
}
func countryPriority(code string) int {
	code = strings.ToUpper(strings.TrimSpace(code))
	for i, c := range balkanCountries {
		if c.Code == code {
			return i
		}
	}
	return len(balkanCountries) + 1
}
func countCountries(list []RadioStation) int {
	seen := map[string]bool{}
	for _, s := range list {
		if s.CountryCode != "" {
			seen[strings.ToUpper(s.CountryCode)] = true
		}
	}
	return len(seen)
}

func stationKey(s RadioStation) string {
	if strings.TrimSpace(s.StationUUID) != "" {
		return s.StationUUID
	}
	return strings.ToUpper(strings.TrimSpace(s.CountryCode)) + "|" + normalizeName(s.Name) + "|" + strings.ToLower(strings.TrimSpace(s.URLResolved+s.URL))
}
func findStationIndexLocked(key string, fallback int) int {
	if fallback >= 0 && fallback < len(app.stations) && stationKey(app.stations[fallback]) == key {
		return fallback
	}
	for i := range app.stations {
		if stationKey(app.stations[i]) == key {
			return i
		}
	}
	return -1
}

func currentStationIndexLocked() int {
	if app.currentKey != "" {
		if idx := findStationIndexLocked(app.currentKey, app.current); idx >= 0 {
			return idx
		}
	}
	if app.current >= 0 && app.current < len(app.stations) {
		return app.current
	}
	return -1
}

func currentStationSnapshot() (RadioStation, string, int, bool) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	idx := currentStationIndexLocked()
	if idx < 0 || idx >= len(app.stations) {
		return RadioStation{}, "", -1, false
	}
	st := app.stations[idx]
	return st, stationKey(st), idx, true
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	winmm    = syscall.NewLazyDLL("winmm.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")

	procRegisterClass                 = user32.NewProc("RegisterClassW")
	procCreateWindowEx                = user32.NewProc("CreateWindowExW")
	procDefWindowProc                 = user32.NewProc("DefWindowProcW")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procUpdateWindow                  = user32.NewProc("UpdateWindow")
	procGetMessage                    = user32.NewProc("GetMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessage               = user32.NewProc("DispatchMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procBeginPaint                    = user32.NewProc("BeginPaint")
	procEndPaint                      = user32.NewProc("EndPaint")
	procGetClientRect                 = user32.NewProc("GetClientRect")
	procGetWindowRect                 = user32.NewProc("GetWindowRect")
	procEnableWindow                  = user32.NewProc("EnableWindow")
	procIsDialogMessage               = user32.NewProc("IsDialogMessageW")
	procFillRect                      = user32.NewProc("FillRect")
	procInvalidateRect                = user32.NewProc("InvalidateRect")
	procPostMessage                   = user32.NewProc("PostMessageW")
	procSendMessage                   = user32.NewProc("SendMessageW")
	procGetWindowText                 = user32.NewProc("GetWindowTextW")
	procSetWindowText                 = user32.NewProc("SetWindowTextW")
	procLoadCursor                    = user32.NewProc("LoadCursorW")
	procMessageBox                    = user32.NewProc("MessageBoxW")
	procSetBkMode                     = gdi32.NewProc("SetBkMode")
	procSetTextColor                  = gdi32.NewProc("SetTextColor")
	procCreateSolidBrush              = gdi32.NewProc("CreateSolidBrush")
	procCreateFont                    = gdi32.NewProc("CreateFontW")
	procSelectObject                  = gdi32.NewProc("SelectObject")
	procDeleteObject                  = gdi32.NewProc("DeleteObject")
	procRoundRect                     = gdi32.NewProc("RoundRect")
	procEllipse                       = gdi32.NewProc("Ellipse")
	procPolygon                       = gdi32.NewProc("Polygon")
	procCreatePen                     = gdi32.NewProc("CreatePen")
	procCreateCompatibleDC            = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC                      = gdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap        = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt                        = gdi32.NewProc("BitBlt")
	procMoveToEx                      = gdi32.NewProc("MoveToEx")
	procLineTo                        = gdi32.NewProc("LineTo")
	procStretchDIBits                 = gdi32.NewProc("StretchDIBits")
	procDrawText                      = user32.NewProc("DrawTextW")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procSetFocus                      = user32.NewProc("SetFocus")
	procGetKeyState                   = user32.NewProc("GetKeyState")
	procOpenClipboard                 = user32.NewProc("OpenClipboard")
	procEmptyClipboard                = user32.NewProc("EmptyClipboard")
	procSetClipboardData              = user32.NewProc("SetClipboardData")
	procCloseClipboard                = user32.NewProc("CloseClipboard")
	procGlobalAlloc                   = kernel32.NewProc("GlobalAlloc")
	procGlobalLock                    = kernel32.NewProc("GlobalLock")
	procGlobalUnlock                  = kernel32.NewProc("GlobalUnlock")
	procGlobalFree                    = kernel32.NewProc("GlobalFree")
	procCopyMemory                    = kernel32.NewProc("RtlMoveMemory")
	procShellExecute                  = shell32.NewProc("ShellExecuteW")
	procMciSendString                 = winmm.NewProc("mciSendStringW")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procDwmSetWindowAttribute         = dwmapi.NewProc("DwmSetWindowAttribute")
	procSetWindowTheme                = uxtheme.NewProc("SetWindowTheme")
	procFindWindow                    = user32.NewProc("FindWindowW")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procCreateIconFromResourceEx      = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon                   = user32.NewProc("DestroyIcon")
	procCreateMutex                   = kernel32.NewProc("CreateMutexW")
	procCloseHandle                   = kernel32.NewProc("CloseHandle")
)

var instanceMutex syscall.Handle
var apiOnce sync.Once
var apiBaseList []string
var logMu sync.Mutex

func color(r, g, b byte) uint32      { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }
func rgb(r, g, b byte) uintptr       { return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16) }
func loWord(v uintptr) uint16        { return uint16(v & 0xffff) }
func hiWord(v uintptr) uint16        { return uint16((v >> 16) & 0xffff) }
func signedHiWord(v uintptr) int16   { return int16(hiWord(v)) }
func inRect(x, y int32, r RECT) bool { return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom }
func u16(s string) *uint16 {
	s = strings.ReplaceAll(s, "\x00", " ")
	p, err := syscall.UTF16PtrFromString(s)
	if err == nil && p != nil {
		return p
	}
	p, _ = syscall.UTF16PtrFromString("")
	return p
}

func setStatus(v string) {
	app.mu.Lock()
	app.status = v
	app.mu.Unlock()
}
func getStatus() string {
	app.mu.RLock()
	v := app.status
	app.mu.RUnlock()
	return v
}
func safeGo(name string, fn func()) {
	if shuttingDown() {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logError(name, fmt.Errorf("panic: %v\n%s", r, debug.Stack()))
				if !shuttingDown() {
					setStatus("Aplikacija je nastavila rad nakon poteškoće")
					postUI()
				}
			}
		}()
		fn()
	}()
}
func logError(scope string, err error) {
	if err == nil {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	_ = os.MkdirAll(filepath.Join(stateDir(), "logs"), 0755)
	p := filepath.Join(stateDir(), "logs", "app.log")
	if info, e := os.Stat(p); e == nil && info.Size() > 2<<20 {
		_ = os.Remove(p + ".1")
		_ = os.Rename(p, p+".1")
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if e != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s [%s] %v\n", time.Now().Format(time.RFC3339), scope, err)
}
func safeHTTPURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 4096 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return false
	}
	if u.User != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	h := strings.TrimRight(strings.TrimSpace(strings.ToLower(u.Hostname())), ".")
	if h == "" || h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") ||
		h == "metadata.google.internal" || h == "instance-data.ec2.internal" || h == "metadata.azure.internal" {
		return false
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false
		}
		if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return false
		}
	}
	return true
}

func isValidTab(tab string) bool {
	switch tab {
	case "all", "popular", "favorites", "recent", "replaced", "broken":
		return true
	default:
		return false
	}
}

func validateState(st PersistedState, loaded bool) PersistedState {
	if st.Favorites == nil {
		st.Favorites = map[string]bool{}
	}
	if st.Replacements == nil {
		st.Replacements = map[string]string{}
	}
	if st.Backups == nil {
		st.Backups = map[string][]string{}
	}
	if !loaded || st.Volume < 0 || st.Volume > 100 {
		st.Volume = 80
	}
	if st.HealthIntervalMin < 10 || st.HealthIntervalMin > 240 {
		st.HealthIntervalMin = 30
	}
	if !isBalkanCode(st.CountryCode) {
		st.CountryCode = ""
	}
	st.Genre = strings.TrimSpace(st.Genre)
	if len([]rune(st.Genre)) > 40 {
		st.Genre = ""
	}
	if !isValidTab(st.Tab) {
		st.Tab = "all"
	}
	if st.WindowWidth < 1100 || st.WindowWidth > 2600 {
		st.WindowWidth = 1540
	}
	if st.WindowHeight < 720 || st.WindowHeight > 1600 {
		st.WindowHeight = 900
	}
	if len(st.Recent) > 40 {
		st.Recent = append([]string(nil), st.Recent[:40]...)
	}
	for key, raw := range st.Replacements {
		if key == "" || !safeHTTPURL(raw) {
			delete(st.Replacements, key)
		}
	}
	for key, list := range st.Backups {
		clean := make([]string, 0, len(list))
		seen := map[string]bool{}
		for _, raw := range list {
			raw = strings.TrimSpace(raw)
			if raw == "" || seen[raw] || !safeHTTPURL(raw) {
				continue
			}
			seen[raw] = true
			clean = append(clean, raw)
			if len(clean) >= 8 {
				break
			}
		}
		if len(clean) == 0 {
			delete(st.Backups, key)
		} else {
			st.Backups[key] = clean
		}
	}
	return st
}

func sessionMarkerPath() string { return filepath.Join(stateDir(), "session.lock") }

func detectAndMarkUncleanStartup() bool {
	_ = os.MkdirAll(stateDir(), 0755)
	unclean := false
	if info, err := os.Stat(sessionMarkerPath()); err == nil {
		// Treat only a recent marker as a crash signal; old orphaned files should
		// not keep the application permanently in safe mode.
		unclean = time.Since(info.ModTime()) < 48*time.Hour
	}
	payload := fmt.Sprintf("pid=%d\nstarted=%s\nversion=%s\n", os.Getpid(), time.Now().Format(time.RFC3339), appVersion)
	if err := os.WriteFile(sessionMarkerPath(), []byte(payload), 0644); err != nil {
		logError("session-marker", err)
	}
	return unclean
}

func captureWindowSize() {
	if app.hwnd == 0 {
		return
	}
	var r RECT
	if ok, _, _ := procGetWindowRect.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w < 1100 || w > 2600 || h < 720 || h > 1600 {
		return
	}
	app.stateMu.Lock()
	app.state.WindowWidth = w
	app.state.WindowHeight = h
	app.stateMu.Unlock()
}

func markCleanShutdown() {
	if err := os.Remove(sessionMarkerPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		logError("session-cleanup", err)
	}
}

type repairGuard struct {
	mu   sync.Mutex
	refs int
}

func acquireStationRepair(id string) func() {
	if id == "" {
		id = "_unknown"
	}
	app.repairMu.Lock()
	if app.repairLocks == nil {
		app.repairLocks = map[string]*repairGuard{}
	}
	g := app.repairLocks[id]
	if g == nil {
		g = &repairGuard{}
		app.repairLocks[id] = g
	}
	g.refs++
	app.repairMu.Unlock()
	g.mu.Lock()
	return func() {
		g.mu.Unlock()
		app.repairMu.Lock()
		g.refs--
		if g.refs <= 0 && app.repairLocks[id] == g {
			delete(app.repairLocks, id)
		}
		app.repairMu.Unlock()
	}
}

func main() {
	if n := runtime.NumCPU(); n > 4 {
		runtime.GOMAXPROCS(4)
	}
	debug.SetMemoryLimit(192 << 20)
	debug.SetGCPercent(80)
	defer func() {
		if r := recover(); r != nil {
			logError("main", fmt.Errorf("panic: %v\n%s", r, debug.Stack()))
			messageBox(0, appName, "Dogodila se neočekivana greška. Zapis o pogrešci spremljen je lokalno. Aplikaciju možeš ponovno pokrenuti.", MB_ICONERROR)
		}
	}()
	initDPI()
	if !acquireSingleInstance() {
		return
	}
	defer func() {
		if instanceMutex != 0 {
			procCloseHandle.Call(uintptr(instanceMutex))
		}
	}()
	migrateLegacyDataDir()
	transport := &http.Transport{MaxIdleConns: 16, MaxIdleConnsPerHost: 4, IdleConnTimeout: 45 * time.Second, TLSHandshakeTimeout: 7 * time.Second, ResponseHeaderTimeout: 9 * time.Second, ForceAttemptHTTP2: true}
	client := &http.Client{Timeout: 12 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return errors.New("previše preusmjeravanja")
		}
		if !safeHTTPURL(req.URL.String()) {
			return errors.New("nesigurno preusmjeravanje")
		}
		return nil
	}}
	appCtx, appCancel := context.WithCancel(context.Background())
	app = App{tab: "all", current: -1, loading: true, status: "Učitavanje radio stanica…", repairLocks: map[string]*repairGuard{}, http: client, ctx: appCtx, cancel: appCancel, streamSem: make(chan struct{}, 2), done: make(chan struct{})}
	state, stateLoaded := loadState()
	app.state = validateState(state, stateLoaded)
	app.safeMode = detectAndMarkUncleanStartup()
	app.country = app.state.CountryCode
	app.genre = app.state.Genre
	if isValidTab(app.state.Tab) {
		app.tab = app.state.Tab
	}
	if app.safeMode {
		app.status = "Nastavljam nakon prethodnog prekida rada…"
	}
	initGDI()
	defer cleanupGDI()
	if err := createMainWindow(); err != nil {
		markCleanShutdown()
		messageBox(0, "Greška", err.Error(), MB_ICONERROR)
		return
	}
	safeGo("load-stations", loadStations)
	var msg MSG
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if msg.Message == WM_KEYDOWN {
			key := uint32(msg.WParam)
			ctrl, _, _ := procGetKeyState.Call(VK_CONTROL)
			if key == VK_F && int16(ctrl&0xFFFF) < 0 {
				if app.edit != 0 {
					procSetFocus.Call(uintptr(app.edit))
				}
				continue
			}
			app.mu.RLock()
			menuOpen := app.countryMenuOpen || app.genreMenuOpen
			app.mu.RUnlock()
			if menuOpen || key == VK_F5 {
				handleKeyDown(key)
				continue
			}
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// NOTE: remaining implementation intentionally unchanged from the fetched source.

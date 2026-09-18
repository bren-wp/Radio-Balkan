//go:build windows

package main

import (
	"bufio"
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

var appVersion = "0.0.21"

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
	audioAck                                 chan string
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
	shutdownOnce                             sync.Once
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
	h := canonicalNetworkHost(u.Hostname())
	if unsafeNetworkHost(h) {
		return false
	}
	if ip := net.ParseIP(h); ip != nil && unsafeNetworkIP(ip) {
		return false
	}
	return true
}

func canonicalNetworkHost(host string) string {
	return strings.TrimRight(strings.TrimSpace(strings.ToLower(host)), ".")
}

func unsafeNetworkHost(host string) bool {
	host = canonicalNetworkHost(host)
	return host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") ||
		host == "metadata.google.internal" || host == "instance-data.ec2.internal" || host == "metadata.azure.internal"
}

func unsafeNetworkIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("neispravna mrežna adresa: %w", err)
	}
	host = canonicalNetworkHost(host)
	if unsafeNetworkHost(host) {
		return nil, errors.New("odredište lokalne mreže nije dopušteno")
	}

	dialer := &net.Dialer{Timeout: 7 * time.Second, KeepAlive: 30 * time.Second}
	if ip := net.ParseIP(host); ip != nil {
		if unsafeNetworkIP(ip) {
			return nil, errors.New("privatna ili lokalna IP adresa nije dopuštena")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, errors.New("odredište nema DNS adresu")
	}
	for _, candidate := range resolved {
		if unsafeNetworkIP(candidate.IP) {
			return nil, errors.New("DNS odredište vodi na privatnu ili lokalnu IP adresu")
		}
	}

	var lastErr error
	for _, candidate := range resolved {
		target := net.JoinHostPort(candidate.IP.String(), port)
		conn, dialErr := dialer.DialContext(ctx, network, target)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("nije moguće uspostaviti mrežnu vezu")
	}
	return nil, lastErr
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

// acquireStationRepair serializes stream repair per station without retaining one
// mutex forever for every station ever encountered. The returned function must be
// deferred immediately after a successful acquire.
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
	// Keep the desktop app responsive without allowing background catalog/network work
	// to monopolize every CPU core or grow the heap without a practical ceiling.
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
	transport := &http.Transport{DialContext: safeDialContext, MaxIdleConns: 16, MaxIdleConnsPerHost: 4, IdleConnTimeout: 45 * time.Second, TLSHandshakeTimeout: 7 * time.Second, ResponseHeaderTimeout: 9 * time.Second, ForceAttemptHTTP2: true}
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
		// A previous launch did not finish cleanly. Start conservatively and avoid
		// immediate full-network stress until the UI is responsive.
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

func acquireSingleInstance() bool {
	// Also detect pre-RadioBalkan builds so an upgrade cannot start two players.
	for _, cls := range []string{className, legacyClassName} {
		existing, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(u16(cls))), 0)
		if existing != 0 {
			procShowWindow.Call(existing, SW_RESTORE)
			procSetForegroundWindow.Call(existing)
			return false
		}
	}
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(u16("Local\\RadioBalkan.Native.SingleInstance"))))
	if h == 0 {
		return true
	}
	instanceMutex = syscall.Handle(h)
	if errno, ok := err.(syscall.Errno); ok && errno == 183 {
		existing, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(u16(className))), 0)
		if existing != 0 {
			procShowWindow.Call(existing, SW_RESTORE)
			procSetForegroundWindow.Call(existing)
		}
		procCloseHandle.Call(h)
		instanceMutex = 0
		return false
	}
	return true
}

func initDPI() {
	// Per-monitor-v2 context = -4. Fallback to legacy DPI aware.
	if procSetProcessDpiAwarenessContext.Find() == nil {
		r, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3))
		if r != 0 {
			return
		}
	}
	procSetProcessDPIAware.Call()
}

func initGDI() {
	app.hIconBig = createIconFromICO(appIconBytes, 64)
	app.hIconSmall = createIconFromICO(appIconBytes, 24)
	app.bgBrush = createBrush(color(9, 14, 20))
	app.panelBrush = createBrush(color(11, 16, 23))
	app.editBrush = createBrush(color(17, 25, 35))
	app.hFont = createFont(17, 400, "Segoe UI")
	app.hFontBold = createFont(17, 700, "Segoe UI")
	app.hFontSmall = createFont(14, 400, "Segoe UI")
	app.hFontTitle = createFont(28, 700, "Segoe UI")
}
func cleanupGDI() {
	for _, h := range []syscall.Handle{app.bgBrush, app.panelBrush, app.editBrush, app.hFont, app.hFontBold, app.hFontSmall, app.hFontTitle} {
		if h != 0 {
			procDeleteObject.Call(uintptr(h))
		}
	}
	if app.hIconBig != 0 {
		procDestroyIcon.Call(uintptr(app.hIconBig))
	}
	if app.hIconSmall != 0 && app.hIconSmall != app.hIconBig {
		procDestroyIcon.Call(uintptr(app.hIconSmall))
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
	r, _, _ := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&data[bestOff])), uintptr(bestSize), 1, 0x00030000, uintptr(want), uintptr(want), 0)
	return syscall.Handle(r)
}

func createBrush(color uint32) syscall.Handle {
	r, _, _ := procCreateSolidBrush.Call(uintptr(color))
	return syscall.Handle(r)
}
func createFont(height int32, weight int32, face string) syscall.Handle {
	r, _, _ := procCreateFont.Call(uintptr(int32(-height)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16(face))))
	return syscall.Handle(r)
}

func drawRasterImageCover(hdc syscall.Handle, l, t, r, b int32, img *RasterImage) bool {
	if img == nil || len(img.Pixels) == 0 || r <= l || b <= t {
		return false
	}
	dw, dh := r-l, b-t
	sw, sh := img.W, img.H
	if sw <= 0 || sh <= 0 {
		return false
	}
	srcX, srcY := int32(0), int32(0)
	srcW, srcH := sw, sh
	// Center-crop to fill destination while preserving the image aspect ratio.
	if int64(sw)*int64(dh) > int64(sh)*int64(dw) {
		srcW = int32(int64(sh) * int64(dw) / int64(dh))
		srcX = (sw - srcW) / 2
	} else if int64(sw)*int64(dh) < int64(sh)*int64(dw) {
		srcH = int32(int64(sw) * int64(dh) / int64(dw))
		srcY = (sh - srcH) / 2
	}
	bi := BITMAPINFO{}
	bi.Header.Size = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bi.Header.Width = sw
	bi.Header.Height = -sh // top-down DIB
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = BI_RGB
	bi.Header.SizeImage = uint32(len(img.Pixels))
	procStretchDIBits.Call(
		uintptr(hdc), uintptr(l), uintptr(t), uintptr(dw), uintptr(dh),
		uintptr(srcX), uintptr(srcY), uintptr(srcW), uintptr(srcH),
		uintptr(unsafe.Pointer(&img.Pixels[0])), uintptr(unsafe.Pointer(&bi)), DIB_RGB_COLORS, SRCCOPY,
	)
	return true
}

func stationLogoURL(s RadioStation) string {
	raw := strings.TrimSpace(s.Favicon)
	if raw != "" && safeHTTPURL(raw) {
		return raw
	}
	if safeHTTPURL(s.Homepage) {
		u, err := url.Parse(strings.TrimSpace(s.Homepage))
		if err == nil && u.Scheme != "" && u.Host != "" {
			u.Path, u.RawPath, u.RawQuery, u.Fragment = "/favicon.ico", "", "", ""
			if safeHTTPURL(u.String()) {
				return u.String()
			}
		}
	}
	return ""
}

func decodeRasterBytes(data []byte) *RasterImage {
	decode := func(payload []byte) image.Image {
		img, _, err := image.Decode(bytes.NewReader(payload))
		if err != nil {
			return nil
		}
		return img
	}
	decoded := decode(data)
	if decoded == nil && len(data) >= 22 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0 {
		count := int(binary.LittleEndian.Uint16(data[4:6]))
		bestOff, bestSize, bestArea := 0, 0, -1
		for i := 0; i < count; i++ {
			p := 6 + i*16
			if p+16 > len(data) {
				break
			}
			w, h := int(data[p]), int(data[p+1])
			if w == 0 {
				w = 256
			}
			if h == 0 {
				h = 256
			}
			sz := int(binary.LittleEndian.Uint32(data[p+8 : p+12]))
			off := int(binary.LittleEndian.Uint32(data[p+12 : p+16]))
			area := w * h
			if area > bestArea && off >= 0 && sz > 0 && off+sz <= len(data) {
				bestArea, bestOff, bestSize = area, off, sz
			}
		}
		if bestSize > 8 {
			payload := data[bestOff : bestOff+bestSize]
			if bytes.HasPrefix(payload, []byte{0x89, 0x50, 0x4e, 0x47}) {
				decoded = decode(payload)
			}
		}
	}
	if decoded == nil {
		return nil
	}
	b := decoded.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 || sw > 4096 || sh > 4096 || int64(sw)*int64(sh) > 16*1024*1024 {
		return nil
	}
	tw, th := sw, sh
	for tw > 192 || th > 192 {
		tw = maxInt(1, tw/2)
		th = maxInt(1, th/2)
	}
	pixels := make([]byte, tw*th*4)
	for y := 0; y < th; y++ {
		sy := b.Min.Y + y*sh/th
		for x := 0; x < tw; x++ {
			sx := b.Min.X + x*sw/tw
			r, g, bl, _ := decoded.At(sx, sy).RGBA()
			i := (y*tw + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = byte(bl>>8), byte(g>>8), byte(r>>8), 0
		}
	}
	return &RasterImage{W: int32(tw), H: int32(th), Pixels: pixels}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func requestStationLogo(s RadioStation) *RasterImage {
	raw := stationLogoURL(s)
	if raw == "" {
		return nil
	}
	now := time.Now()
	logoCacheMu.Lock()
	if e := logoCache[raw]; e != nil {
		e.UsedAt = now
		img, loading, failed := e.Image, e.Loading, e.FailedAt
		logoCacheMu.Unlock()
		if img != nil {
			return img
		}
		if loading || (!failed.IsZero() && now.Sub(failed) < 30*time.Minute) {
			return nil
		}
		logoCacheMu.Lock()
		e.Loading = true
		logoCacheMu.Unlock()
	} else {
		logoCache[raw] = &logoCacheEntry{Loading: true, UsedAt: now}
		trimLogoCacheLocked(48)
		logoCacheMu.Unlock()
	}
	safeGo("station-logo", func() {
		select {
		case logoSem <- struct{}{}:
		case <-app.done:
			return
		}
		defer func() { <-logoSem }()
		img := downloadStationLogo(raw)
		logoCacheMu.Lock()
		e := logoCache[raw]
		if e != nil {
			e.Loading = false
			e.Image = img
			e.UsedAt = time.Now()
			if img == nil {
				e.FailedAt = time.Now()
			} else {
				e.FailedAt = time.Time{}
			}
		}
		logoCacheMu.Unlock()
		if img != nil && !shuttingDown() {
			postUI()
		}
	})
	return nil
}

func trimLogoCacheLocked(limit int) {
	for len(logoCache) > limit {
		var oldestKey string
		var oldest time.Time
		for k, e := range logoCache {
			if e.Loading {
				continue
			}
			if oldestKey == "" || e.UsedAt.Before(oldest) {
				oldestKey, oldest = k, e.UsedAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(logoCache, oldestKey)
	}
}

func downloadStationLogo(raw string) *RasterImage {
	if !safeHTTPURL(raw) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
	req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/x-icon,*/*;q=0.1")
	resp, err := app.http.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 || !safeHTTPURL(resp.Request.URL.String()) {
		return nil
	}
	const maxLogo = 512 * 1024
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxLogo+1))
	if err != nil || len(data) == 0 || len(data) > maxLogo {
		return nil
	}
	return decodeRasterBytes(data)
}

func drawStationArtwork(hdc syscall.Handle, l, t, r, b int32, s RadioStation, slot int) bool {
	if img := requestStationLogo(s); img != nil {
		return drawRasterImageCover(hdc, l, t, r, b, img)
	}
	drawArtworkFallback(hdc, l, t, r, b, s, slot)
	return true
}

func drawArtworkFallback(hdc syscall.Handle, l, t, r, b int32, s RadioStation, slot int) {
	palette := [][2]uint32{
		{color(54, 29, 20), color(123, 71, 37)},
		{color(23, 37, 52), color(54, 83, 115)},
		{color(34, 28, 48), color(83, 61, 114)},
		{color(23, 45, 41), color(51, 101, 87)},
		{color(48, 31, 31), color(112, 61, 55)},
		{color(43, 39, 24), color(112, 94, 46)},
	}
	if slot < 0 {
		slot = 0
	}
	pair := palette[slot%len(palette)]
	drawRounded(hdc, l, t, r, b, 10, pair[0], pair[1])
	// Lightweight procedural bars keep fallback artwork distinct without embedding raster assets.
	width := r - l
	for i := int32(0); i < 5; i++ {
		x := l + 10 + i*(width-20)/5
		h := int32(12 + ((slot+int(i)*7)%5)*7)
		br := createBrush(pair[1])
		rc := RECT{x, b - 10 - h, x + 3, b - 10}
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(br))
		procDeleteObject.Call(uintptr(br))
	}
	initial := "RB"
	parts := strings.Fields(strings.TrimSpace(s.Name))
	if len(parts) > 0 {
		initial = strings.ToUpper(string([]rune(parts[0])[0]))
		if len(parts) > 1 {
			initial += strings.ToUpper(string([]rune(parts[1])[0]))
		}
	}
	selectFont(hdc, app.hFontBold)
	text(hdc, initial, l+6, t+4, r-6, b-4, rgb(244, 246, 248), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func createMainWindow() error {
	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	cur, _, _ := procLoadCursor.Call(0, IDC_ARROW)
	wc := WNDCLASS{LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hInst), HIcon: app.hIconBig, HCursor: syscall.Handle(cur), HbrBackground: app.bgBrush, LpszClassName: u16(className)}
	if r, _, e := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassW: %v", e)
	}
	app.stateMu.RLock()
	windowW, windowH := app.state.WindowWidth, app.state.WindowHeight
	app.stateMu.RUnlock()
	if windowW < 1100 {
		windowW = 1400
	}
	if windowH < 720 {
		windowH = 840
	}
	hwnd, _, e := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16(className))), uintptr(unsafe.Pointer(u16(appName))), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 50, 28, uintptr(windowW), uintptr(windowH), 0, 0, hInst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %v", e)
	}
	app.hwnd = syscall.Handle(hwnd)
	if app.hIconBig != 0 {
		procSendMessage.Call(hwnd, WM_SETICON, 1, uintptr(app.hIconBig))
	}
	if app.hIconSmall != 0 {
		procSendMessage.Call(hwnd, WM_SETICON, 0, uintptr(app.hIconSmall))
	}
	enableImmersiveDark(app.hwnd)
	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)
	return nil
}

func enableImmersiveDark(hwnd syscall.Handle) {
	// DWMWA_USE_IMMERSIVE_DARK_MODE 20 on current Win10/11.
	v := int32(1)
	procDwmSetWindowAttribute.Call(uintptr(hwnd), 20, uintptr(unsafe.Pointer(&v)), unsafe.Sizeof(v))
}

func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logError("wndproc", fmt.Errorf("panic msg=0x%X: %v\n%s", msg, r, debug.Stack()))
			setStatus("Pojavila se poteškoća u prikazu · pokušaj ponovno")
			if app.hwnd != 0 {
				procPostMessage.Call(uintptr(app.hwnd), WM_APP+1, 0, 0)
			}
			ret = 0
		}
	}()
	return wndProcCore(hwnd, msg, wParam, lParam)
}

func wndProcCore(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			var info MINMAXINFO
			procCopyMemory.Call(uintptr(unsafe.Pointer(&info)), lParam, unsafe.Sizeof(info))
			info.PtMinTrackSize.X = 1100
			info.PtMinTrackSize.Y = 720
			procCopyMemory.Call(lParam, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
		}
		return 0
	case WM_CREATE:
		app.hwnd = hwnd
		hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
		e, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("EDIT"))), uintptr(unsafe.Pointer(u16(""))), WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_AUTOHSCROLL, 0, 0, 0, 0, uintptr(hwnd), 1001, hInst, 0)
		if e == 0 {
			logError("ui-create-search", errors.New("CreateWindowExW EDIT failed"))
			return ^uintptr(0)
		}
		app.edit = syscall.Handle(e)
		procSendMessage.Call(e, 0x0030, uintptr(app.hFont), 1)
		procSendMessage.Call(e, EM_SETCUEBANNER, 1, uintptr(unsafe.Pointer(u16("Pretraži stanice, gradove i žanrove…"))))
		procSetWindowTheme.Call(e, uintptr(unsafe.Pointer(u16("DarkMode_CFD"))), 0)
		app.mu.Lock()
		app.genreOptions = []string{"Svi žanrovi"}
		app.mu.Unlock()
		return 0
	case WM_SIZE:
		layoutControls(hwnd)
		invalidate()
		return 0
	case WM_COMMAND:
		id := loWord(wParam)
		code := hiWord(wParam)
		if id == 1001 && code == EN_CHANGE {
			app.mu.Lock()
			app.search = getWindowText(app.edit)
			app.scroll = 0
			app.countryMenuOpen = false
			app.genreMenuOpen = false
			app.mu.Unlock()
			scheduleSearchFilter()
			return 0
		}
	case WM_KEYDOWN:
		handleKeyDown(uint32(wParam))
		return 0
	case WM_APPCOMMAND:
		command := int(hiWord(lParam) & 0x0FFF)
		switch command {
		case APPCOMMAND_MEDIA_NEXTTRACK:
			playAdjacent(1)
		case APPCOMMAND_MEDIA_PREVIOUSTRACK:
			playAdjacent(-1)
		case APPCOMMAND_MEDIA_STOP:
			stopCurrentPlayback()
		case APPCOMMAND_MEDIA_PLAY_PAUSE:
			toggleCurrentPlayback()
		case APPCOMMAND_VOLUME_DOWN:
			adjustVolume(-5)
		case APPCOMMAND_VOLUME_UP:
			adjustVolume(5)
		default:
			r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
			return r
		}
		return 1
	case WM_ACTIVATEAPP:
		if wParam == 0 {
			app.mu.Lock()
			app.countryMenuOpen = false
			app.genreMenuOpen = false
			app.mu.Unlock()
			invalidate()
		}
		return 0
	case WM_MOUSEMOVE:
		x := int32(int16(loWord(lParam)))
		y := int32(int16(hiWord(lParam)))
		updateHover(x, y)
		return 0
	case WM_NCMOUSEMOVE:
		if app.hoverToken != "" {
			app.hoverToken = ""
			invalidate()
		}
		return 0
	case WM_MOUSEWHEEL:
		d := int(signedHiWord(wParam))
		app.mu.Lock()
		if app.countryMenuOpen || app.genreMenuOpen {
			app.mu.Unlock()
			return 0
		}
		app.scroll -= d / 4
		clampScrollLocked()
		app.mu.Unlock()
		invalidate()
		return 0
	case WM_LBUTTONDOWN:
		x := int32(int16(loWord(lParam)))
		y := int32(int16(hiWord(lParam)))
		handleClick(x, y)
		return 0
	case WM_ERASEBKGND:
		// The complete client area is rendered in WM_PAINT using an off-screen
		// buffer, so suppress the default erase pass to avoid flicker.
		return 1
	case WM_PAINT:
		paint(hwnd)
		return 0
	case WM_CTLCOLORSTATIC, WM_CTLCOLOREDIT, WM_CTLCOLORLISTBOX:
		hdc := syscall.Handle(wParam)
		procSetTextColor.Call(uintptr(hdc), rgb(240, 236, 232))
		procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		return uintptr(app.editBrush)
	case WM_APP + 1:
		invalidate()
		return 0
	case WM_APP + 2:
		rebuildGenres()
		invalidate()
		return 0
	case WM_APP + 3:
		rebuildFilter()
		invalidate()
		return 0
	case WM_APP + 4:
		showNextAlert()
		return 0
	case WM_CLOSE:
		captureWindowSize()
		prepareShutdown()
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case WM_DESTROY:
		prepareShutdown()
		markCleanShutdown()
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

const (
	sidebarWidth int32 = 250
	playerHeight int32 = 94
	mainPad      int32 = 28
)

func headerLayout(clientRight int32) (searchL, searchR, countryL, countryR, genreL, genreR, checkL, checkR, refreshL, refreshR int32) {
	mainLeft := sidebarWidth + mainPad
	refreshR = clientRight - mainPad
	refreshL = refreshR - 46
	checkR = refreshL - 10
	checkL = checkR - 84
	genreR = checkL - 12
	genreL = genreR - 150
	countryR = genreL - 12
	countryL = countryR - 176
	searchL = mainLeft
	searchR = countryL - 12
	return
}

func layoutControls(hwnd syscall.Handle) {
	var r RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	app.mu.Lock()
	app.clientHeight = r.Bottom
	clampScrollLocked()
	app.mu.Unlock()
	searchL, searchR, _, _, _, _, _, _, _, _ := headerLayout(r.Right)
	width := searchR - searchL - 48
	if width < 100 {
		width = 100
	}
	procSetWindowPos.Call(uintptr(app.edit), 0, uintptr(searchL+43), 30, uintptr(width), 31, 0x0004)
}

func paint(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdcRaw, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdcRaw == 0 {
		return
	}
	hdc := syscall.Handle(hdcRaw)
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	var cr RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&cr)))
	width, height := cr.Right-cr.Left, cr.Bottom-cr.Top
	if width <= 0 || height <= 0 {
		return
	}
	memRaw, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
	if memRaw == 0 {
		paintClient(hdc, cr)
		return
	}
	memDC := syscall.Handle(memRaw)
	defer procDeleteDC.Call(uintptr(memDC))
	bmpRaw, _, _ := procCreateCompatibleBitmap.Call(uintptr(hdc), uintptr(width), uintptr(height))
	if bmpRaw == 0 {
		paintClient(hdc, cr)
		return
	}
	bmp := syscall.Handle(bmpRaw)
	oldBmp, _, _ := procSelectObject.Call(uintptr(memDC), uintptr(bmp))
	paintClient(memDC, cr)
	procBitBlt.Call(uintptr(hdc), 0, 0, uintptr(width), uintptr(height), uintptr(memDC), 0, 0, SRCCOPY)
	if oldBmp != 0 {
		procSelectObject.Call(uintptr(memDC), oldBmp)
	}
	procDeleteObject.Call(uintptr(bmp))
}

func paintClient(hdc syscall.Handle, cr RECT) {
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&cr)), uintptr(app.bgBrush))
	app.hits = app.hits[:0]
	drawSidebar(hdc, cr)
	drawHeader(hdc, cr)
	drawStations(hdc, cr)
	drawPlayer(hdc, cr)
	drawDropdownOverlay(hdc, cr)
}

func drawSidebar(hdc syscall.Handle, cr RECT) {
	r := RECT{0, 0, sidebarWidth, cr.Bottom - playerHeight}
	br := createBrush(color(11, 16, 23))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&r)), uintptr(br))
	procDeleteObject.Call(uintptr(br))

	// Premium waveform brand mark, matching the desktop reference.
	barX := int32(27)
	heights := []int32{18, 30, 42, 30, 18}
	for i, h := range heights {
		x := barX + int32(i)*8
		drawRounded(hdc, x, 31+(42-h)/2, x+4, 31+(42+h)/2, 2, color(255, 170, 50), color(255, 170, 50))
	}
	selectFont(hdc, app.hFontBold)
	text(hdc, "Radio Balkan", 82, 25, sidebarWidth-18, 62, rgb(247, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	app.mu.RLock()
	tab, genre := app.tab, strings.ToLower(strings.TrimSpace(app.genre))
	app.mu.RUnlock()
	y := int32(91)
	drawSidebarItem(hdc, y, "⌂", "Početna", tab == "all" && genre == "", hitTab, "all")
	y += 44
	drawSidebarItem(hdc, y, "⌕", "Pretraga", false, hitTab, "searchfocus")
	y += 44
	drawSidebarItem(hdc, y, "▦", "Pregledaj", false, hitTab, "browse")
	y += 44
	drawSidebarItem(hdc, y, "♡", "Omiljene", tab == "favorites", hitTab, "favorites")
	y += 44
	drawSidebarItem(hdc, y, "◷", "Nedavno slušano", tab == "recent", hitTab, "recent")

	y += 56
	drawSidebarLabel(hdc, "BRZI ODABIR", y)
	y += 30
	drawSidebarItem(hdc, y, "♫", "Popularne", tab == "popular", hitTab, "popular")
	y += 42
	drawSidebarItem(hdc, y, "♫", "Pop & Rock", genre == "pop", hitTab, "genre:pop")
	y += 42
	drawSidebarItem(hdc, y, "♫", "Narodna", genre == "folk", hitTab, "genre:folk")
	y += 42
	drawSidebarItem(hdc, y, "♫", "Elektronička", genre == "electronic", hitTab, "genre:electronic")
	y += 42
	drawSidebarItem(hdc, y, "♫", "Jazz", genre == "jazz", hitTab, "genre:jazz")

	toolsY := y + 60
	if cr.Bottom-playerHeight > toolsY+110 {
		drawSidebarLabel(hdc, "UPRAVLJANJE", toolsY)
		toolsY += 28
		drawSidebarItem(hdc, toolsY, "⇄", "Rezervni izvori", tab == "replaced", hitTab, "replaced")
		toolsY += 42
		drawSidebarItem(hdc, toolsY, "!", "Nedostupne", tab == "broken", hitTab, "broken")
	}

	// Warm footer quote as in the reference artwork.
	quoteTop := cr.Bottom - playerHeight - 118
	if quoteTop > y+28 {
		drawRounded(hdc, 14, quoteTop, sidebarWidth-14, quoteTop+92, 16, color(28, 24, 22), color(54, 43, 37))
		selectFont(hdc, app.hFontSmall)
		text(hdc, "Isti ljudi. Ista glazba.\nBliži nego ikad.", 28, quoteTop+18, sidebarWidth-28, quoteTop+68, rgb(235, 171, 91), DT_LEFT|DT_WORDBREAK)
		text(hdc, "♡", sidebarWidth-55, quoteTop+57, sidebarWidth-25, quoteTop+84, rgb(255, 177, 55), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
}

func drawSidebarLabel(hdc syscall.Handle, label string, y int32) {
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, 25, y, sidebarWidth-20, y+22, rgb(113, 102, 96), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
}

func drawSidebarItem(hdc syscall.Handle, y int32, icon, label string, selected bool, kind HitKind, value string) {
	l, r := int32(16), sidebarWidth-14
	if selected {
		drawRounded(hdc, l, y, r, y+36, 10, color(58, 39, 23), color(104, 67, 29))
	} else if hovered(kind, -1, value) {
		drawRounded(hdc, l, y, r, y+36, 10, color(27, 30, 36), color(63, 58, 55))
	}
	ic := rgb(137, 126, 119)
	tc := rgb(190, 181, 175)
	if selected {
		ic = rgb(255, 170, 50)
		tc = rgb(248, 242, 238)
	}
	selectFont(hdc, app.hFontSmall)
	text(hdc, icon, l+12, y, l+39, y+36, ic, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	text(hdc, label, l+46, y, r-10, y+36, tc, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	app.hits = append(app.hits, HitRegion{R: RECT{l, y, r, y + 36}, Kind: kind, Index: -1, Value: value})
}

func drawHeader(hdc syscall.Handle, cr RECT) {
	searchL, searchR, countryL, countryR, genreL, genreR, checkL, checkR, refreshL, refreshR := headerLayout(cr.Right)
	drawRounded(hdc, searchL, 20, searchR, 72, 14, color(17, 25, 35), color(43, 53, 66))
	selectFont(hdc, app.hFontSmall)
	text(hdc, "⌕", searchL+14, 20, searchL+38, 72, rgb(143, 153, 166), DT_CENTER|DT_VCENTER|DT_SINGLELINE)

	app.mu.RLock()
	countryCode, genre := app.country, app.genre
	countryOpen, genreOpen := app.countryMenuOpen, app.genreMenuOpen
	app.mu.RUnlock()
	countryLabel := countryNameByCode(countryCode)
	if countryCode == "" || countryLabel == "" {
		countryLabel = "Sve podržane zemlje"
	}
	genreLabel := genreDisplayName(genre)
	if genreLabel == "" {
		genreLabel = "Svi žanrovi"
	}
	drawCountrySelectBox(hdc, countryL, 20, countryR, 72, countryCode, countryLabel, countryOpen)
	drawSelectBox(hdc, genreL, 20, genreR, 72, genreLabel, genreOpen)
	app.hits = append(app.hits, HitRegion{R: RECT{countryL, 20, countryR, 72}, Kind: hitCountryDropdown})
	app.hits = append(app.hits, HitRegion{R: RECT{genreL, 20, genreR, 72}, Kind: hitGenreDropdown})
	app.mu.RLock()
	healthRunning := app.healthRunning
	app.mu.RUnlock()
	checkLabel := "✓ Sve"
	if healthRunning {
		checkLabel = "…"
	}
	drawIconButton(hdc, checkL, 20, checkR, 72, checkLabel, healthRunning)
	app.hits = append(app.hits, HitRegion{R: RECT{checkL, 20, checkR, 72}, Kind: hitCheckAll, Index: -1})
	drawIconButton(hdc, refreshL, 20, refreshR, 72, "↻", false)
	app.hits = append(app.hits, HitRegion{R: RECT{refreshL, 20, refreshR, 72}, Kind: hitRefresh, Index: -1})
}

func drawIconButton(hdc syscall.Handle, l, t, r, b int32, label string, accent bool) {
	fill, border := color(17, 25, 35), color(43, 53, 66)
	tc := rgb(199, 205, 214)
	if accent {
		fill, border, tc = color(255, 170, 50), color(255, 193, 91), rgb(22, 23, 26)
	}
	drawRounded(hdc, l, t, r, b, 13, fill, border)
	selectFont(hdc, app.hFontBold)
	text(hdc, label, l, t, r, b, tc, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func drawSelectBox(hdc syscall.Handle, l, t, r, b int32, label string, open bool) {
	fill, border := color(17, 25, 35), color(43, 53, 66)
	if open {
		border = color(151, 99, 41)
	}
	drawRounded(hdc, l, t, r, b, 13, fill, border)
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, l+14, t, r-32, b, rgb(225, 229, 234), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	arrow := "⌄"
	if open {
		arrow = "⌃"
	}
	text(hdc, arrow, r-28, t, r-8, b, rgb(146, 156, 168), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func drawCountrySelectBox(hdc syscall.Handle, l, t, r, b int32, code, label string, open bool) {
	fill, border := color(17, 25, 35), color(43, 53, 66)
	if open {
		border = color(151, 99, 41)
	}
	drawRounded(hdc, l, t, r, b, 13, fill, border)
	drawCountryFlag(hdc, l+12, t+18, l+38, t+34, code)
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, l+46, t, r-32, b, rgb(225, 229, 234), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	arrow := "⌄"
	if open {
		arrow = "⌃"
	}
	text(hdc, arrow, r-28, t, r-8, b, rgb(146, 156, 168), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func drawDropdownOverlay(hdc syscall.Handle, cr RECT) {
	app.mu.RLock()
	countryOpen, genreOpen := app.countryMenuOpen, app.genreMenuOpen
	selectedCountry, selectedGenre := app.country, app.genre
	countryFocus, genreFocus := app.countryMenuIndex, app.genreMenuIndex
	genres := append([]string(nil), app.genreOptions...)
	app.mu.RUnlock()
	if !countryOpen && !genreOpen {
		return
	}
	_, _, countryL, countryR, genreL, genreR, _, _, _, _ := headerLayout(cr.Right)
	if countryOpen {
		items := append([]CountryDef(nil), balkanCountries...)
		drawCountryMenu(hdc, countryL, countryR, 76, items, selectedCountry, countryFocus)
	}
	if genreOpen {
		if len(genres) == 0 {
			genres = []string{"Svi žanrovi"}
		}
		drawGenreMenu(hdc, genreL, genreR, 76, genres, selectedGenre, genreFocus)
	}
}

func drawCountryMenu(hdc syscall.Handle, l, r, top int32, items []CountryDef, selected string, focus int) {
	rowH := int32(30)
	bottom := top + int32(len(items))*rowH + 10
	drawRounded(hdc, l, top, r, bottom, 10, color(15, 21, 29), color(54, 64, 77))
	counts := countryCountsSnapshot()
	for i, item := range items {
		t := top + 5 + int32(i)*rowH
		sel := strings.EqualFold(item.Code, selected)
		foc := i == focus
		if sel || foc {
			fill, border := color(45, 34, 24), color(125, 82, 36)
			if foc && !sel {
				fill, border = color(27, 35, 45), color(69, 81, 96)
			}
			drawRounded(hdc, l+5, t, r-5, t+rowH-2, 7, fill, border)
		}
		drawCountryFlag(hdc, l+11, t+7, l+35, t+21, item.Code)
		selectFont(hdc, app.hFontSmall)
		text(hdc, item.Name, l+43, t, r-48, t+rowH-2, rgb(232, 224, 218), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		count := counts[strings.ToUpper(item.Code)]
		if item.Code == "" {
			count = counts[""]
		}
		text(hdc, fmt.Sprintf("%d", count), r-45, t, r-12, t+rowH-2, rgb(126, 113, 106), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{l + 5, t, r - 5, t + rowH - 2}, Kind: hitCountryChoice, Value: item.Code})
	}
}

func drawGenreMenu(hdc syscall.Handle, l, r, top int32, items []string, selected string, focus int) {
	if len(items) > 15 {
		items = items[:15]
	}
	rowH := int32(30)
	bottom := top + int32(len(items))*rowH + 10
	drawRounded(hdc, l, top, r, bottom, 10, color(15, 21, 29), color(54, 64, 77))
	for i, label := range items {
		t := top + 5 + int32(i)*rowH
		value := label
		if label == "Svi žanrovi" {
			value = ""
		}
		sel := strings.EqualFold(value, selected)
		foc := i == focus
		if sel || foc {
			fill, border := color(45, 34, 24), color(125, 82, 36)
			if foc && !sel {
				fill, border = color(27, 35, 45), color(69, 81, 96)
			}
			drawRounded(hdc, l+5, t, r-5, t+rowH-2, 7, fill, border)
		}
		selectFont(hdc, app.hFontSmall)
		text(hdc, label, l+12, t, r-12, t+rowH-2, rgb(232, 224, 218), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		app.hits = append(app.hits, HitRegion{R: RECT{l + 5, t, r - 5, t + rowH - 2}, Kind: hitGenreChoice, Value: value})
	}
}

func genreDisplayName(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "domaca":
		return "Domaća"
	case "pop":
		return "Pop & Rock"
	case "rock":
		return "Rock"
	case "folk":
		return "Narodna"
	case "electronic":
		return "Elektronička"
	default:
		return g
	}
}

func shouldShowPopular() bool {
	app.mu.RLock()
	defer app.mu.RUnlock()
	// The rich home composition is intentionally only used when it fits without
	// colliding with the persistent player. Smaller windows fall back to the
	// virtualized station library instead of clipping controls.
	return app.clientHeight >= 840 && app.tab == "all" && strings.TrimSpace(app.search) == "" && strings.TrimSpace(app.genre) == ""
}

func popularStations(limit int) []int {
	if limit <= 0 {
		return nil
	}
	app.mu.RLock()
	defer app.mu.RUnlock()
	country := strings.ToUpper(strings.TrimSpace(app.country))
	best := make([]int, 0, limit)
	for i, st := range app.stations {
		if country != "" && strings.ToUpper(st.CountryCode) != country {
			continue
		}
		insert := len(best)
		for j, idx := range best {
			if st.Votes > app.stations[idx].Votes {
				insert = j
				break
			}
		}
		if insert >= limit {
			continue
		}
		best = append(best, 0)
		copy(best[insert+1:], best[insert:])
		best[insert] = i
		if len(best) > limit {
			best = best[:limit]
		}
	}
	return best
}

func drawStations(hdc syscall.Handle, cr RECT) {
	mainL := sidebarWidth + mainPad
	mainR := cr.Right - mainPad
	bottom := cr.Bottom - playerHeight - 16
	showHome := shouldShowPopular()

	if showHome {
		ids := popularStations(16)
		if len(ids) > 0 {
			app.mu.RLock()
			featuredIdx := ids[0]
			var featured RadioStation
			if featuredIdx >= 0 && featuredIdx < len(app.stations) {
				featured = app.stations[featuredIdx]
			}
			totalStations := len(app.stations)
			app.mu.RUnlock()
			drawHomeHero(hdc, mainL, 82, mainR, 300, featuredIdx, featured, totalStations)
		}

		// Popular stations: six wide image cards, mirroring the production mockup.
		selectFont(hdc, app.hFontBold)
		text(hdc, "Popularne stanice", mainL, 315, mainR-120, 345, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		selectFont(hdc, app.hFontSmall)
		text(hdc, "Prikaži sve  →", mainR-130, 315, mainR, 345, rgb(157, 165, 176), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{mainR - 150, 315, mainR, 345}, Kind: hitTab, Index: -1, Value: "popular"})
		cards := ids
		if len(cards) > 1 {
			cards = cards[1:]
		}
		if len(cards) > 6 {
			cards = cards[:6]
		}
		gap := int32(12)
		cardW := (mainR - mainL - gap*5) / 6
		for i, idx := range cards {
			l := mainL + int32(i)*(cardW+gap)
			app.mu.RLock()
			if idx < 0 || idx >= len(app.stations) {
				app.mu.RUnlock()
				continue
			}
			st := app.stations[idx]
			app.mu.RUnlock()
			drawPopularCard(hdc, l, 350, l+cardW, 510, idx, st, i)
		}

		// Genre artwork row.
		selectFont(hdc, app.hFontBold)
		text(hdc, "Žanrovi", mainL, 528, mainR-120, 556, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		selectFont(hdc, app.hFontSmall)
		text(hdc, "Prikaži sve  →", mainR-130, 528, mainR, 556, rgb(157, 165, 176), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
		genres := []struct{ Label, Value string }{
			{"Pop", "pop"}, {"Rock", "rock"}, {"Elektronička", "electronic"},
			{"Jazz", "jazz"}, {"Klasična", "classical"}, {"Hip Hop", "hiphop"},
		}
		genreW := (mainR - mainL - gap*5) / 6
		for i, g := range genres {
			l := mainL + int32(i)*(genreW+gap)
			drawGenreTile(hdc, l, 562, l+genreW, 646, g.Label, g.Value, i)
		}

		// Compact regional row at the bottom of the home view.
		selectFont(hdc, app.hFontBold)
		text(hdc, "Stanice iz regije", mainL, 660, mainR-120, 688, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		selectFont(hdc, app.hFontSmall)
		text(hdc, "Prikaži sve  →", mainR-130, 660, mainR, 688, rgb(157, 165, 176), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{mainR - 150, 660, mainR, 690}, Kind: hitTab, Index: -1, Value: "all"})
		region := ids
		if len(region) > 7 {
			region = region[7:]
		}
		if len(region) > 6 {
			region = region[:6]
		}
		regW := (mainR - mainL - gap*5) / 6
		for i, idx := range region {
			app.mu.RLock()
			if idx < 0 || idx >= len(app.stations) {
				app.mu.RUnlock()
				continue
			}
			st := app.stations[idx]
			app.mu.RUnlock()
			l := mainL + int32(i)*(regW+gap)
			drawRegionCard(hdc, l, 695, l+regW, 760, idx, st, i)
		}
		return
	}

	// Library/search/filter pages use the efficient virtualized two-column grid.
	gridTop := int32(132)
	selectFont(hdc, app.hFontBold)
	title := "Sve stanice"
	app.mu.RLock()
	tab := app.tab
	genre := app.genre
	loading := app.loading
	filteredCount := len(app.filtered)
	app.mu.RUnlock()
	switch tab {
	case "popular":
		title = "Popularne stanice"
	case "favorites":
		title = "Omiljene"
	case "recent":
		title = "Nedavno slušano"
	case "replaced":
		title = "Rezervni izvori"
	case "broken":
		title = "Nedostupne stanice"
	}
	if genre != "" {
		title = "Žanr · " + genreDisplayName(genre)
	}
	text(hdc, title, mainL, 92, mainR-180, 125, rgb(246, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	selectFont(hdc, app.hFontSmall)
	text(hdc, fmt.Sprintf("%d stanica", filteredCount), mainR-170, 92, mainR, 125, rgb(133, 143, 154), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
	if loading && filteredCount == 0 {
		selectFont(hdc, app.hFontBold)
		text(hdc, "Učitavam stanice…", mainL, gridTop+30, mainR, bottom, rgb(214, 220, 226), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		return
	}
	if filteredCount == 0 {
		selectFont(hdc, app.hFontBold)
		text(hdc, "Nema stanica za odabrani prikaz.", mainL, gridTop+30, mainR, bottom, rgb(184, 191, 200), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		return
	}
	const cardH int32 = 104
	const gapY int32 = 12
	gapX := int32(14)
	colW := (mainR - mainL - gapX) / 2
	step := int(cardH + gapY)
	app.mu.RLock()
	scroll := app.scroll
	currentFilteredCount := len(app.filtered)
	startRow := scroll / step
	offset := scroll - startRow*step
	visible := make([]struct {
		idx, row, col int
		st            RadioStation
	}, 0, 18)
	startPos := startRow * 2
	for pos := startPos; pos < currentFilteredCount; pos++ {
		row := (pos / 2) - startRow
		y := gridTop + int32(row)*int32(step) - int32(offset)
		if y > bottom {
			break
		}
		idx := app.filtered[pos]
		if idx >= 0 && idx < len(app.stations) {
			visible = append(visible, struct {
				idx, row, col int
				st            RadioStation
			}{idx, row, pos % 2, app.stations[idx]})
		}
	}
	app.mu.RUnlock()
	for _, it := range visible {
		y := gridTop + int32(it.row)*int32(step) - int32(offset)
		if y+cardH < gridTop {
			continue
		}
		l := mainL + int32(it.col)*(colW+gapX)
		drawStationCard(hdc, l, y, l+colW, y+cardH, it.idx, it.st)
	}
	drawStationScrollBar(hdc, cr, gridTop, bottom, currentFilteredCount, scroll, step)
}

func drawHomeHero(hdc syscall.Handle, l, t, r, b int32, idx int, st RadioStation, total int) {
	if r-l < 620 || idx < 0 {
		return
	}
	gap := int32(16)
	infoW := int32(330)
	featureR := r - infoW - gap
	drawRounded(hdc, l, t, featureR, b, 16, color(14, 19, 26), color(67, 72, 82))
	drawFeatureBackdrop(hdc, l+2, t+2, featureR-2, b-2)
	// Opaque left readability panel, visually approximating the generated gradient.
	leftPanelR := l + (featureR-l)*46/100
	fill := RECT{l + 2, t + 2, leftPanelR, b - 2}
	br := createBrush(color(12, 17, 23))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&fill)), uintptr(br))
	procDeleteObject.Call(uintptr(br))
	selectFont(hdc, app.hFontSmall)
	text(hdc, "UŽIVO S BALKANA", l+24, t+17, leftPanelR-18, t+42, rgb(255, 177, 61), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, app.hFontTitle)
	text(hdc, trimName(st.Name), l+24, t+44, leftPanelR-16, t+83, rgb(250, 250, 251), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	selectFont(hdc, app.hFontSmall)
	text(hdc, "Glazba koja povezuje regiju.", l+24, t+84, leftPanelR-16, t+110, rgb(205, 209, 215), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCountryFlag(hdc, l+24, t+122, l+48, t+137, st.CountryCode)
	text(hdc, stationMeta(st), l+56, t+114, leftPanelR-16, t+144, rgb(185, 192, 201), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	heroKey := stationKey(st)
	app.mu.RLock()
	heroCurrent := app.currentKey == heroKey
	heroPlaying := heroCurrent && app.playing
	app.mu.RUnlock()
	heroFill := color(255, 176, 58)
	if hovered(hitPlay, idx, heroKey) {
		heroFill = color(255, 188, 78)
	}
	drawRounded(hdc, l+24, b-62, l+186, b-18, 13, heroFill, color(255, 198, 102))
	selectFont(hdc, app.hFontBold)
	heroLabel := "▶  Slušaj uživo"
	if heroPlaying {
		heroLabel = "Ⅱ  Pauziraj"
	} else if heroCurrent {
		heroLabel = "▶  Nastavi"
	}
	text(hdc, heroLabel, l+30, b-62, l+180, b-18, rgb(19, 20, 24), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{l + 24, b - 62, l + 186, b - 18}, Kind: hitPlay, Index: idx, Value: heroKey})

	// Companion info/wave card.
	drawRounded(hdc, featureR+gap, t, r, b, 16, color(22, 22, 23), color(63, 61, 61))
	drawWaveGraphic(hdc, featureR+gap+18, t+18, r-18, t+116)
	selectFont(hdc, app.hFontBold)
	text(hdc, "Balkan zvuči bolje", featureR+gap+22, t+126, r-22, t+151, rgb(246, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	text(hdc, "zajedno.", featureR+gap+22, t+150, r-22, t+176, rgb(246, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, app.hFontSmall)
	text(hdc, fmt.Sprintf("Otkrij nove stanice i priče.  %d ukupno", total), featureR+gap+22, t+178, r-20, b-15, rgb(172, 177, 185), DT_LEFT|DT_WORDBREAK)
}

func drawWaveGraphic(hdc syscall.Handle, l, t, r, b int32) {
	pen, _, _ := procCreatePen.Call(0, 1, rgb(181, 108, 37))
	old, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	for line := 0; line < 5; line++ {
		lastY := int32(0)
		for x := l; x <= r; x += 4 {
			phase := float64(x-l) / float64(r-l) * 6.28318
			y := t + (b-t)/2 + int32(float64(16+line*5)*math.Sin(phase+float64(line)*0.58))
			if x == l {
				procMoveToEx.Call(uintptr(hdc), uintptr(x), uintptr(y), 0)
			} else {
				procLineTo.Call(uintptr(hdc), uintptr(x), uintptr(y))
			}
			lastY = y
			_ = lastY
		}
	}
	procSelectObject.Call(uintptr(hdc), old)
	procDeleteObject.Call(pen)
}

func drawFeatureBackdrop(hdc syscall.Handle, l, t, r, b int32) {
	base := createBrush(color(31, 22, 18))
	rc := RECT{l, t, r, b}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(base))
	procDeleteObject.Call(uintptr(base))
	pen, _, _ := procCreatePen.Call(0, 2, rgb(126, 76, 38))
	old, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	for y := t + 18; y < b; y += 24 {
		procMoveToEx.Call(uintptr(hdc), uintptr(l+8), uintptr(y), 0)
		procLineTo.Call(uintptr(hdc), uintptr(r-8), uintptr(y+int32((y-t)/24)%3*7))
	}
	procSelectObject.Call(uintptr(hdc), old)
	procDeleteObject.Call(pen)
}

func drawGenreTile(hdc syscall.Handle, l, t, r, b int32, label, value string, slot int) {
	border := color(53, 61, 73)
	if hovered(hitTab, -1, "genre:"+value) {
		border = color(140, 88, 38)
	}
	palette := []uint32{color(74, 39, 25), color(35, 49, 68), color(37, 57, 52), color(56, 42, 69), color(67, 58, 31), color(66, 36, 42)}
	fill := palette[slot%len(palette)]
	drawRounded(hdc, l, t, r, b, 12, fill, border)
	// Decorative equalizer bars are cheaper than six embedded genre bitmaps.
	for i := int32(0); i < 7; i++ {
		h := int32(10 + ((slot+int(i)*3)%5)*8)
		x := l + 12 + i*8
		br := createBrush(color(151, 96, 43))
		rc := RECT{x, b - 34 - h, x + 3, b - 34}
		procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(br))
		procDeleteObject.Call(uintptr(br))
	}
	shade := RECT{l + 1, b - 31, r - 1, b - 1}
	br := createBrush(color(15, 17, 21))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&shade)), uintptr(br))
	procDeleteObject.Call(uintptr(br))
	selectFont(hdc, app.hFontBold)
	text(hdc, label, l+12, b-35, r-10, b-3, rgb(247, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitTab, Index: -1, Value: "genre:" + value})
}

func drawRegionCard(hdc syscall.Handle, l, t, r, b int32, idx int, s RadioStation, slot int) {
	border := color(49, 57, 68)
	if hovered(hitPlay, idx, stationKey(s)) {
		border = color(133, 88, 40)
	}
	drawRounded(hdc, l, t, r, b, 10, color(20, 25, 33), border)
	artR := l + 58
	drawStationArtwork(hdc, l+6, t+7, artR-5, b-7, s, slot)
	selectFont(hdc, app.hFontBold)
	text(hdc, trimName(s.Name), artR+5, t+6, r-8, t+31, rgb(244, 246, 248), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCountryFlag(hdc, artR+5, t+35, artR+25, t+48, s.CountryCode)
	selectFont(hdc, app.hFontSmall)
	meta := countryNameByCode(strings.ToUpper(s.CountryCode))
	if meta == "" {
		meta = s.Country
	}
	text(hdc, meta, artR+31, t+31, r-8, t+54, rgb(160, 168, 178), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitPlay, Index: idx, Value: stationKey(s)})
}

func drawStationScrollBar(hdc syscall.Handle, cr RECT, top, bottom int32, count, scroll, step int) {
	if count <= 0 || bottom <= top {
		return
	}
	rows := (count + 1) / 2
	content := rows*step - 12
	viewport := int(bottom - top)
	if content <= viewport || viewport <= 0 {
		return
	}
	maxScroll := content - viewport
	if maxScroll <= 0 {
		return
	}
	trackL := cr.Right - 17
	trackR := cr.Right - 11
	drawRounded(hdc, trackL, top, trackR, bottom, 4, color(42, 34, 30), color(42, 34, 30))
	trackH := int(bottom - top)
	thumbH := viewport * trackH / content
	if thumbH < 34 {
		thumbH = 34
	}
	thumbY := int(top) + scroll*(trackH-thumbH)/maxScroll
	drawRounded(hdc, trackL, int32(thumbY), trackR, int32(thumbY+thumbH), 4, color(118, 83, 64), color(118, 83, 64))
}

func drawPopularCard(hdc syscall.Handle, l, t, r, b int32, idx int, s RadioStation, theme int) {
	border := color(50, 58, 70)
	if hovered(hitPlay, idx, stationKey(s)) {
		border = color(133, 88, 40)
	}
	drawRounded(hdc, l, t, r, b, 12, color(18, 24, 33), border)
	imageBottom := t + 91
	drawStationArtwork(hdc, l+1, t+1, r-1, imageBottom, s, theme)
	// Bottom information panel.
	panel := RECT{l + 1, imageBottom - 1, r - 1, b - 1}
	br := createBrush(color(21, 27, 37))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&panel)), uintptr(br))
	procDeleteObject.Call(uintptr(br))
	selectFont(hdc, app.hFontBold)
	text(hdc, trimName(s.Name), l+12, imageBottom+4, r-42, imageBottom+29, rgb(247, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCountryFlag(hdc, l+12, imageBottom+34, l+32, imageBottom+47, s.CountryCode)
	selectFont(hdc, app.hFontSmall)
	meta := countryNameByCode(strings.ToUpper(s.CountryCode))
	if meta == "" {
		meta = s.Country
	}
	genre := firstTag(s.Tags)
	if genre != "" && meta != "" {
		meta += " · " + genre
	}
	text(hdc, meta, l+38, imageBottom+28, r-42, imageBottom+53, rgb(171, 179, 189), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	bitrate := ""
	if s.Bitrate > 0 {
		bitrate = fmt.Sprintf("◉  %d kbps", s.Bitrate)
	} else {
		bitrate = "◉  uživo"
	}
	text(hdc, bitrate, l+12, b-25, r-48, b-6, rgb(130, 140, 151), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	drawCircle(hdc, r-40, b-42, r-12, b-14, color(49, 56, 66), color(92, 69, 37))
	text(hdc, "▶", r-38, b-41, r-14, b-14, rgb(248, 249, 250), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitPlay, Index: idx, Value: stationKey(s)})
}

func firstTag(tags string) string {
	for _, raw := range strings.Split(tags, ",") {
		v := strings.TrimSpace(raw)
		if len([]rune(v)) >= 2 && len([]rune(v)) <= 18 {
			return v
		}
	}
	return ""
}

func drawStationCard(hdc syscall.Handle, l, t, r, b int32, idx int, s RadioStation) {
	key := stationKey(s)
	app.mu.RLock()
	selected := app.currentKey != "" && key == app.currentKey
	selectedPlaying := selected && app.playing
	app.mu.RUnlock()
	fill, border := color(17, 23, 31), color(47, 56, 68)
	if selected {
		fill, border = color(24, 28, 34), color(132, 89, 39)
	} else if hovered(hitPlay, idx, key) {
		fill, border = color(20, 27, 36), color(94, 72, 48)
	}
	drawRounded(hdc, l, t, r, b, 14, fill, border)
	artR := l + 128
	drawStationArtwork(hdc, l+1, t+1, artR, b-1, s, idx)
	selectFont(hdc, app.hFontBold)
	text(hdc, trimName(s.Name), artR+15, t+10, r-170, t+38, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCountryFlag(hdc, artR+15, t+45, artR+37, t+59, s.CountryCode)
	selectFont(hdc, app.hFontSmall)
	meta := countryNameByCode(strings.ToUpper(s.CountryCode))
	if meta == "" {
		meta = s.Country
	}
	g := firstTag(s.Tags)
	if g != "" && meta != "" {
		meta += " · " + g
	}
	text(hdc, meta, artR+44, t+38, r-145, t+65, rgb(178, 185, 194), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	hl, hc := healthText(s.Health)
	text(hdc, hl, artR+15, t+70, artR+112, t+93, hc, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	x := artR + 118
	y := t + 70
	action := func(label string, w int32, kind HitKind) {
		drawMiniAction(hdc, x, y, x+w, y+23, label)
		app.hits = append(app.hits, HitRegion{R: RECT{x, y, x + w, y + 23}, Kind: kind, Index: idx, Value: key})
		x += w + 6
	}
	action("Web", 40, hitWeb)
	action("Kopiraj", 54, hitLink)
	action("✓", 26, hitCheckStation)
	action("Izvor", 46, hitReplace)
	app.stateMu.RLock()
	fav := app.state.Favorites[key]
	app.stateMu.RUnlock()
	heart := "♡"
	if fav {
		heart = "♥"
	}
	drawIconButton(hdc, r-102, t+16, r-64, t+54, heart, fav)
	app.hits = append(app.hits, HitRegion{R: RECT{r - 102, t + 16, r - 64, t + 54}, Kind: hitFavorite, Index: idx, Value: key})
	drawCircle(hdc, r-54, t+13, r-12, t+55, color(38, 40, 46), color(111, 77, 41))
	selectFont(hdc, app.hFontBold)
	playLabel := "▶"
	if selectedPlaying {
		playLabel = "Ⅱ"
	}
	text(hdc, playLabel, r-50, t+13, r-15, t+55, rgb(247, 248, 250), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{r - 57, t + 10, r - 9, t + 58}, Kind: hitPlay, Index: idx, Value: key})
}

func drawMiniAction(hdc syscall.Handle, l, t, r, b int32, label string) {
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, l, t, r, b, rgb(151, 139, 132), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func drawPlayer(hdc syscall.Handle, cr RECT) {
	t := cr.Bottom - playerHeight
	if t < 0 {
		return
	}
	rc := RECT{0, t, cr.Right, cr.Bottom}
	br := createBrush(color(11, 16, 23))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(br))
	procDeleteObject.Call(uintptr(br))
	pen, _, _ := procCreatePen.Call(0, 1, rgb(52, 60, 72))
	old, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procMoveToEx.Call(uintptr(hdc), 0, uintptr(t), 0)
	procLineTo.Call(uintptr(hdc), uintptr(cr.Right), uintptr(t))
	procSelectObject.Call(uintptr(hdc), old)
	procDeleteObject.Call(pen)

	name := "Odaberi radio stanicu"
	meta := "Radio iz Hrvatske i regije"
	np := ""
	currentCountry := ""
	playing := false
	currentIdx := -1
	var current RadioStation
	app.mu.RLock()
	if idx := currentStationIndexLocked(); idx >= 0 && idx < len(app.stations) {
		currentIdx = idx
		current = app.stations[idx]
		name = current.Name
		meta = stationMeta(current)
		currentCountry = current.CountryCode
		if app.nowPlaying != "" && app.nowPlayingStation == stationKey(current) {
			np = app.nowPlaying
		}
	}
	playing = app.playing
	app.mu.RUnlock()
	app.stateMu.RLock()
	vol := app.state.Volume
	fav := false
	if currentIdx >= 0 {
		fav = app.state.Favorites[stationKey(current)]
	}
	app.stateMu.RUnlock()
	if np != "" {
		meta = np
	}

	// Artwork and title, matching the mockup's bottom-left now-playing block.
	artL := int32(20)
	artR := artL + 104
	if currentIdx >= 0 {
		drawStationArtwork(hdc, artL, t+12, artR, t+82, current, currentIdx)
	} else {
		drawRounded(hdc, artL, t+12, artR, t+82, 10, color(27, 34, 44), color(55, 65, 78))
	}
	selectFont(hdc, app.hFontBold)
	text(hdc, name, artR+16, t+14, 420, t+42, rgb(247, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	selectFont(hdc, app.hFontSmall)
	if currentCountry != "" {
		drawCountryFlag(hdc, artR+16, t+50, artR+38, t+64, currentCountry)
		text(hdc, meta, artR+45, t+43, 430, t+70, rgb(168, 176, 186), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	} else {
		text(hdc, meta, artR+16, t+43, 430, t+70, rgb(168, 176, 186), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	if np != "" {
		app.hits = append(app.hits, HitRegion{R: RECT{artR + 16, t + 42, 430, t + 71}, Kind: hitCopyNowPlaying, Index: -1})
	}
	if currentIdx >= 0 {
		heart := "♡"
		if fav {
			heart = "♥"
		}
		selectFont(hdc, app.hFontTitle)
		text(hdc, heart, 390, t+13, 430, t+45, rgb(255, 176, 58), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{388, t + 10, 432, t + 48}, Kind: hitFavorite, Index: currentIdx, Value: stationKey(current)})
	}

	// Center transport controls.
	cx := cr.Right / 2
	selectFont(hdc, app.hFontBold)
	text(hdc, "◀", cx-112, t+21, cx-74, t+59, rgb(193, 199, 207), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{cx - 116, t + 17, cx - 70, t + 63}, Kind: hitPlayerPrev, Index: -1})
	label := "▶"
	if playing {
		label = "Ⅱ"
	}
	drawCircle(hdc, cx-31, t+10, cx+31, t+72, color(255, 174, 52), color(255, 197, 95))
	selectFont(hdc, app.hFontTitle)
	text(hdc, label, cx-28, t+10, cx+28, t+72, rgb(20, 21, 24), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{cx - 35, t + 7, cx + 35, t + 76}, Kind: hitPlayerPlay, Index: -1})
	selectFont(hdc, app.hFontBold)
	text(hdc, "▶", cx+72, t+21, cx+110, t+59, rgb(193, 199, 207), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{cx + 68, t + 17, cx + 114, t + 63}, Kind: hitPlayerNext, Index: -1})
	// Live line underneath transport.
	lineL := cx - 150
	lineR := cx + 150
	fillRectColor(hdc, lineL, t+80, lineR, t+83, color(45, 53, 64))
	fillRectColor(hdc, lineL, t+80, cx+20, t+83, color(255, 174, 52))
	drawCircle(hdc, cx+15, t+76, cx+25, t+86, color(255, 174, 52), color(255, 174, 52))

	// Right-side equalizer and volume group.
	eqL := cr.Right - 390
	drawEqualizerBars(hdc, eqL, t+25, eqL+128, t+66, playing)
	drawSpeakerIcon(hdc, cr.Right-235, t+35, rgb(186, 193, 202))
	drawIconButton(hdc, cr.Right-200, t+27, cr.Right-168, t+59, "−", false)
	app.hits = append(app.hits, HitRegion{R: RECT{cr.Right - 200, t + 27, cr.Right - 168, t + 59}, Kind: hitVolumeDown, Index: -1})
	selectFont(hdc, app.hFontSmall)
	text(hdc, fmt.Sprintf("%d%%", vol), cr.Right-164, t+27, cr.Right-112, t+59, rgb(222, 225, 230), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	drawIconButton(hdc, cr.Right-106, t+27, cr.Right-74, t+59, "+", false)
	app.hits = append(app.hits, HitRegion{R: RECT{cr.Right - 106, t + 27, cr.Right - 74, t + 59}, Kind: hitVolumeUp, Index: -1})
}

func drawEqualizerBars(hdc syscall.Handle, l, t, r, b int32, active bool) {
	heights := []int32{12, 22, 34, 25, 39, 31, 18, 36, 27, 14, 30, 21, 10}
	gap := int32(4)
	w := (r - l - gap*int32(len(heights)-1)) / int32(len(heights))
	if w < 3 {
		w = 3
	}
	for i, h := range heights {
		if !active {
			h = h/2 + 4
		}
		x := l + int32(i)*(w+gap)
		c := color(255, 171, 46)
		if i < 3 {
			c = color(238, 91, 50)
		} else if i > 8 {
			c = color(92, 100, 111)
		}
		drawRounded(hdc, x, b-h, x+w, b, 2, c, c)
	}
}

func countryCountsSnapshot() map[string]int {
	counts := map[string]int{"": 0}
	app.mu.RLock()
	defer app.mu.RUnlock()
	for _, st := range app.stations {
		code := strings.ToUpper(strings.TrimSpace(st.CountryCode))
		if code == "" {
			continue
		}
		counts[code]++
		counts[""]++
	}
	return counts
}

func fillRectColor(hdc syscall.Handle, l, t, r, b int32, c uint32) {
	if r <= l || b <= t {
		return
	}
	rc := RECT{l, t, r, b}
	br := createBrush(c)
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), uintptr(br))
	procDeleteObject.Call(uintptr(br))
}

func drawPolygonColor(hdc syscall.Handle, pts []POINT, c uint32) {
	if len(pts) < 3 {
		return
	}
	br := createBrush(c)
	pen, _, _ := procCreatePen.Call(0, 1, rgb(byte(c&0xff), byte((c>>8)&0xff), byte((c>>16)&0xff)))
	oldB, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(br))
	oldP, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procPolygon.Call(uintptr(hdc), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	procSelectObject.Call(uintptr(hdc), oldB)
	procSelectObject.Call(uintptr(hdc), oldP)
	procDeleteObject.Call(uintptr(br))
	procDeleteObject.Call(pen)
}

func drawCountryFlag(hdc syscall.Handle, l, t, r, b int32, code string) {
	w, h := r-l, b-t
	if w < 10 || h < 7 {
		return
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	// Neutral regional badge.
	if code == "" {
		drawRounded(hdc, l, t, r, b, 3, color(37, 53, 66), color(93, 74, 56))
		drawCircle(hdc, l+w/2-3, t+h/2-3, l+w/2+3, t+h/2+3, color(235, 83, 35), color(235, 83, 35))
		return
	}
	fillRectColor(hdc, l, t, r, b, color(238, 238, 238))
	switch code {
	case "HR":
		fillRectColor(hdc, l, t, r, t+h/3, color(255, 0, 0))
		fillRectColor(hdc, l, t+h/3, r, t+2*h/3, color(255, 255, 255))
		fillRectColor(hdc, l, t+2*h/3, r, b, color(23, 23, 150))
		cx, cy := l+w/2-3, t+h/2-3
		for yy := int32(0); yy < 3; yy++ {
			for xx := int32(0); xx < 3; xx++ {
				c := color(255, 255, 255)
				if (xx+yy)%2 == 0 {
					c = color(220, 0, 0)
				}
				fillRectColor(hdc, cx+xx*2, cy+yy*2, cx+xx*2+2, cy+yy*2+2, c)
			}
		}
	case "BA":
		fillRectColor(hdc, l, t, r, b, color(0, 47, 108))
		drawPolygonColor(hdc, []POINT{{l + w/3, t + 1}, {r - 2, t + 1}, {r - 2, b - 2}}, color(255, 205, 0))
		for i := int32(0); i < 4; i++ {
			drawCircle(hdc, l+3+i*4, t+2+i*3, l+5+i*4, t+4+i*3, color(255, 255, 255), color(255, 255, 255))
		}
	case "RS":
		fillRectColor(hdc, l, t, r, t+h/3, color(198, 54, 60))
		fillRectColor(hdc, l, t+h/3, r, t+2*h/3, color(12, 64, 118))
		fillRectColor(hdc, l, t+2*h/3, r, b, color(255, 255, 255))
		drawRounded(hdc, l+4, t+3, l+9, b-3, 2, color(181, 32, 40), color(245, 202, 65))
	case "SI":
		fillRectColor(hdc, l, t, r, t+h/3, color(255, 255, 255))
		fillRectColor(hdc, l, t+h/3, r, t+2*h/3, color(0, 84, 166))
		fillRectColor(hdc, l, t+2*h/3, r, b, color(213, 43, 30))
		drawPolygonColor(hdc, []POINT{{l + 5, t + 2}, {l + 10, t + 2}, {l + 9, t + 8}, {l + 7, t + 10}, {l + 5, t + 8}}, color(0, 91, 170))
	case "MK":
		fillRectColor(hdc, l, t, r, b, color(210, 0, 30))
		cx, cy := l+w/2, t+h/2
		drawCircle(hdc, cx-3, cy-3, cx+3, cy+3, color(255, 227, 0), color(255, 227, 0))
		pen, _, _ := procCreatePen.Call(0, 1, rgb(255, 227, 0))
		old, _, _ := procSelectObject.Call(uintptr(hdc), pen)
		for _, p := range []POINT{{l, cy}, {r, cy}, {cx, t}, {cx, b}, {l, t}, {r, b}, {r, t}, {l, b}} {
			procMoveToEx.Call(uintptr(hdc), uintptr(cx), uintptr(cy), 0)
			procLineTo.Call(uintptr(hdc), uintptr(p.X), uintptr(p.Y))
		}
		procSelectObject.Call(uintptr(hdc), old)
		procDeleteObject.Call(pen)
	case "AL":
		fillRectColor(hdc, l, t, r, b, color(218, 18, 26))
		cx, cy := l+w/2, t+h/2
		drawPolygonColor(hdc, []POINT{{cx, cy - 5}, {cx + 4, cy - 1}, {cx + 2, cy + 1}, {cx + 5, cy + 4}, {cx, cy + 2}, {cx - 5, cy + 4}, {cx - 2, cy + 1}, {cx - 4, cy - 1}}, color(18, 18, 18))
	case "ME":
		fillRectColor(hdc, l, t, r, b, color(196, 26, 45))
		fillRectColor(hdc, l, t, r, t+1, color(218, 180, 55))
		fillRectColor(hdc, l, b-1, r, b, color(218, 180, 55))
		fillRectColor(hdc, l, t, l+1, b, color(218, 180, 55))
		fillRectColor(hdc, r-1, t, r, b, color(218, 180, 55))
		drawCircle(hdc, l+w/2-2, t+h/2-2, l+w/2+2, t+h/2+2, color(218, 180, 55), color(218, 180, 55))
	default:
		drawRounded(hdc, l, t, r, b, 2, color(56, 66, 78), color(89, 101, 113))
		selectFont(hdc, app.hFontSmall)
		text(hdc, code, l, t, r, b, rgb(240, 240, 240), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	// subtle outline for contrast on dark UI
	pen, _, _ := procCreatePen.Call(0, 1, rgb(95, 84, 77))
	old, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procMoveToEx.Call(uintptr(hdc), uintptr(l), uintptr(t), 0)
	procLineTo.Call(uintptr(hdc), uintptr(r-1), uintptr(t))
	procLineTo.Call(uintptr(hdc), uintptr(r-1), uintptr(b-1))
	procLineTo.Call(uintptr(hdc), uintptr(l), uintptr(b-1))
	procLineTo.Call(uintptr(hdc), uintptr(l), uintptr(t))
	procSelectObject.Call(uintptr(hdc), old)
	procDeleteObject.Call(pen)
}

func drawSpeakerIcon(hdc syscall.Handle, x, y int32, c uintptr) {
	pen, _, _ := procCreatePen.Call(0, 2, c)
	old, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procMoveToEx.Call(uintptr(hdc), uintptr(x), uintptr(y+3), 0)
	procLineTo.Call(uintptr(hdc), uintptr(x+5), uintptr(y+3))
	procLineTo.Call(uintptr(hdc), uintptr(x+10), uintptr(y))
	procLineTo.Call(uintptr(hdc), uintptr(x+10), uintptr(y+12))
	procLineTo.Call(uintptr(hdc), uintptr(x+5), uintptr(y+9))
	procLineTo.Call(uintptr(hdc), uintptr(x), uintptr(y+9))
	procLineTo.Call(uintptr(hdc), uintptr(x), uintptr(y+3))
	procMoveToEx.Call(uintptr(hdc), uintptr(x+13), uintptr(y+3), 0)
	procLineTo.Call(uintptr(hdc), uintptr(x+16), uintptr(y+6))
	procLineTo.Call(uintptr(hdc), uintptr(x+13), uintptr(y+9))
	procSelectObject.Call(uintptr(hdc), old)
	procDeleteObject.Call(pen)
}

func drawCircle(hdc syscall.Handle, l, t, r, b int32, fill, border uint32) {
	br := createBrush(fill)
	pen, _, _ := procCreatePen.Call(0, 1, rgb(byte(border&0xff), byte((border>>8)&0xff), byte((border>>16)&0xff)))
	oldB, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(br))
	oldP, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procEllipse.Call(uintptr(hdc), uintptr(l), uintptr(t), uintptr(r), uintptr(b))
	procSelectObject.Call(uintptr(hdc), oldB)
	procSelectObject.Call(uintptr(hdc), oldP)
	procDeleteObject.Call(uintptr(br))
	procDeleteObject.Call(pen)
}

func drawRounded(hdc syscall.Handle, l, t, r, b, rad int32, fill, border uint32) {
	br := createBrush(fill)
	pen, _, _ := procCreatePen.Call(0, 1, rgb(byte(border&0xff), byte((border>>8)&0xff), byte((border>>16)&0xff)))
	oldB, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(br))
	oldP, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procRoundRect.Call(uintptr(hdc), uintptr(l), uintptr(t), uintptr(r), uintptr(b), uintptr(rad), uintptr(rad))
	procSelectObject.Call(uintptr(hdc), oldB)
	procSelectObject.Call(uintptr(hdc), oldP)
	procDeleteObject.Call(uintptr(br))
	procDeleteObject.Call(pen)
}
func drawButton(hdc syscall.Handle, l, t, r, b int32, label string, primary bool) {
	fill := uint32(0x2c221c)
	border := uint32(0x49372d)
	tc := rgb(235, 229, 224)
	if primary {
		fill = 0x3b2a1e
		border = 0x8c4a20
		tc = rgb(255, 173, 103)
	}
	drawRounded(hdc, l, t, r, b, 10, fill, border)
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, l+8, t, r-8, b, tc, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}
func drawSmallButton(hdc syscall.Handle, l, t, r, b int32, label string, accent bool) {
	drawButton(hdc, l, t, r, b, label, accent)
}
func drawPill(hdc syscall.Handle, l, t, r, b int32, label string, selected bool) {
	fill := uint32(0x211a16)
	border := uint32(0x3d2f27)
	tc := rgb(181, 170, 162)
	if selected {
		fill = 0x39251a
		border = 0x75411f
		tc = rgb(255, 165, 94)
	}
	drawRounded(hdc, l, t, r, b, 18, fill, border)
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, l+12, t, r-12, b, tc, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}
func textButtonWidth(s string) int { return 34 + len([]rune(s))*8 }
func selectFont(hdc syscall.Handle, h syscall.Handle) {
	procSelectObject.Call(uintptr(hdc), uintptr(h))
}
func text(hdc syscall.Handle, s string, l, t, r, b int32, color uintptr, flags uint32) {
	rc := RECT{l, t, r, b}
	procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	procSetTextColor.Call(uintptr(hdc), color)
	procDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(u16(s))), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), uintptr(flags))
}
func textRect(hdc syscall.Handle, s string, rc RECT, color uintptr, flags uint32) {
	text(hdc, s, rc.Left, rc.Top, rc.Right, rc.Bottom, color, flags)
}

func stationIndexFromHit(h HitRegion) int {
	app.mu.RLock()
	defer app.mu.RUnlock()
	if h.Value != "" {
		return findStationIndexLocked(h.Value, h.Index)
	}
	if h.Index >= 0 && h.Index < len(app.stations) {
		return h.Index
	}
	return -1
}

func hitToken(kind HitKind, index int, value string) string {
	return strconv.Itoa(int(kind)) + ":" + strconv.Itoa(index) + ":" + value
}

func hovered(kind HitKind, index int, value string) bool {
	return app.hoverToken != "" && app.hoverToken == hitToken(kind, index, value)
}

func updateHover(x, y int32) {
	token := ""
	for i := len(app.hits) - 1; i >= 0; i-- {
		h := app.hits[i]
		if inRect(x, y, h.R) {
			token = hitToken(h.Kind, h.Index, h.Value)
			break
		}
	}
	if token != app.hoverToken {
		app.hoverToken = token
		invalidate()
	}
}

func countryIndexLocked(code string) int {
	for i, c := range balkanCountries {
		if strings.EqualFold(c.Code, code) {
			return i
		}
	}
	return 0
}

func genreIndexLocked(genre string) int {
	if len(app.genreOptions) == 0 {
		return 0
	}
	for i, label := range app.genreOptions {
		value := label
		if label == "Svi žanrovi" {
			value = ""
		}
		if strings.EqualFold(value, genre) {
			return i
		}
	}
	return 0
}

func handleKeyDown(key uint32) {
	app.mu.RLock()
	countryOpen := app.countryMenuOpen
	genreOpen := app.genreMenuOpen
	app.mu.RUnlock()
	if !countryOpen && !genreOpen {
		if key == VK_F5 {
			safeGo("refresh-hotkey", refreshAll)
		}
		return
	}
	if key == VK_ESCAPE {
		app.mu.Lock()
		app.countryMenuOpen = false
		app.genreMenuOpen = false
		app.mu.Unlock()
		invalidate()
		return
	}
	if countryOpen {
		app.mu.Lock()
		max := len(balkanCountries) - 1
		switch key {
		case VK_UP:
			if app.countryMenuIndex > 0 {
				app.countryMenuIndex--
			}
		case VK_DOWN:
			if app.countryMenuIndex < max {
				app.countryMenuIndex++
			}
		case VK_RETURN:
			idx := app.countryMenuIndex
			app.mu.Unlock()
			if idx >= 0 && idx < len(balkanCountries) {
				selectCountry(balkanCountries[idx].Code)
			}
			return
		}
		app.mu.Unlock()
		invalidate()
		return
	}
	if genreOpen {
		app.mu.Lock()
		limit := len(app.genreOptions)
		if limit > 15 {
			limit = 15
		}
		switch key {
		case VK_UP:
			if app.genreMenuIndex > 0 {
				app.genreMenuIndex--
			}
		case VK_DOWN:
			if app.genreMenuIndex+1 < limit {
				app.genreMenuIndex++
			}
		case VK_RETURN:
			idx := app.genreMenuIndex
			var value string
			if idx >= 0 && idx < limit {
				value = app.genreOptions[idx]
				if value == "Svi žanrovi" {
					value = ""
				}
			}
			app.mu.Unlock()
			selectGenre(value)
			return
		}
		app.mu.Unlock()
		invalidate()
	}
}

func toggleCurrentPlayback() {
	app.mu.RLock()
	current, playing := currentStationIndexLocked(), app.playing
	currentKey := app.currentKey
	if currentKey == "" && current >= 0 && current < len(app.stations) {
		currentKey = stationKey(app.stations[current])
	}
	app.mu.RUnlock()
	if current < 0 {
		return
	}
	if playing {
		if err := audioPause(); err != nil {
			logError("audio-pause", err)
		}
		app.mu.Lock()
		app.playing = false
		app.metadataSeq++
		app.mu.Unlock()
		setStatus("Pauzirano")
		invalidate()
		return
	}
	if err := audioResume(); err != nil {
		logError("audio-resume", err)
		playStationByKey(currentKey, current)
		return
	}
	app.mu.Lock()
	actual := findStationIndexLocked(currentKey, current)
	if actual < 0 || actual >= len(app.stations) {
		app.mu.Unlock()
		return
	}
	current = actual
	st := app.stations[current]
	key := stationKey(st)
	app.playing = true
	app.nowPlayingStation = key
	app.metadataSeq++
	seq := app.metadataSeq
	app.mu.Unlock()
	stream := effectiveURL(st)
	setStatus("Uživo · " + st.Name)
	safeGo("watchdog-resume-"+key, func() { playbackWatchdog(current, key) })
	if stream != "" {
		safeGo("metadata-resume-"+key, func() { metadataLoop(seq, current, key, stream) })
	}
	invalidate()
}

func stopCurrentPlayback() {
	audioStop()
	app.mu.Lock()
	app.playing = false
	app.metadataSeq++
	app.playSeq++
	app.nowPlaying = ""
	app.nowPlayingStation = ""
	app.mu.Unlock()
	setStatus("Zaustavljeno")
	invalidate()
}

func playAdjacent(delta int) {
	if delta == 0 {
		return
	}
	app.mu.RLock()
	current := currentStationIndexLocked()
	filtered := append([]int(nil), app.filtered...)
	stationCount := len(app.stations)
	app.mu.RUnlock()
	if stationCount == 0 {
		return
	}
	if len(filtered) > 0 {
		pos := -1
		for i, idx := range filtered {
			if idx == current {
				pos = i
				break
			}
		}
		if pos < 0 {
			if delta > 0 {
				pos = -1
			} else {
				pos = 0
			}
		}
		next := (pos + delta + len(filtered)) % len(filtered)
		if next >= 0 && next < len(filtered) {
			playStation(filtered[next])
			return
		}
	}
	next := current + delta
	if next < 0 {
		next = stationCount - 1
	}
	if next >= stationCount {
		next = 0
	}
	playStation(next)
}

func handleClick(x, y int32) {
	app.mu.RLock()
	menuOpen := app.countryMenuOpen || app.genreMenuOpen
	app.mu.RUnlock()

	for i := len(app.hits) - 1; i >= 0; i-- {
		h := app.hits[i]
		if !inRect(x, y, h.R) {
			continue
		}
		if menuOpen && h.Kind != hitCountryChoice && h.Kind != hitGenreChoice && h.Kind != hitCountryDropdown && h.Kind != hitGenreDropdown {
			app.mu.Lock()
			app.countryMenuOpen = false
			app.genreMenuOpen = false
			app.mu.Unlock()
			invalidate()
			return
		}
		switch h.Kind {
		case hitCountryDropdown:
			app.mu.Lock()
			app.countryMenuOpen = !app.countryMenuOpen
			app.genreMenuOpen = false
			app.countryMenuIndex = countryIndexLocked(app.country)
			app.mu.Unlock()
			invalidate()
		case hitGenreDropdown:
			app.mu.Lock()
			app.genreMenuOpen = !app.genreMenuOpen
			app.countryMenuOpen = false
			app.genreMenuIndex = genreIndexLocked(app.genre)
			app.mu.Unlock()
			invalidate()
		case hitCountryChoice:
			selectCountry(h.Value)
		case hitGenreChoice:
			selectGenre(h.Value)
		case hitTab:
			if h.Value == "searchfocus" {
				procSetFocus.Call(uintptr(app.edit))
				break
			}
			if h.Value == "browse" {
				app.mu.Lock()
				app.countryMenuOpen = true
				app.genreMenuOpen = false
				app.countryMenuIndex = countryIndexLocked(app.country)
				app.mu.Unlock()
				invalidate()
				break
			}
			if strings.HasPrefix(h.Value, "genre:") {
				g := strings.TrimPrefix(h.Value, "genre:")
				app.mu.Lock()
				app.tab = "all"
				app.genre = g
				app.scroll = 0
				app.countryMenuOpen = false
				app.genreMenuOpen = false
				app.mu.Unlock()
				app.stateMu.Lock()
				app.state.Tab = "all"
				app.state.Genre = g
				app.stateMu.Unlock()
				rebuildGenres()
				rebuildFilter()
				scheduleStateSave()
				invalidate()
				break
			}
			app.mu.Lock()
			app.tab = h.Value
			if h.Value == "all" {
				app.genre = ""
			}
			app.scroll = 0
			app.countryMenuOpen = false
			app.genreMenuOpen = false
			app.mu.Unlock()
			app.stateMu.Lock()
			app.state.Tab = h.Value
			if h.Value == "all" {
				app.state.Genre = ""
			}
			app.stateMu.Unlock()
			scheduleStateSave()
			rebuildGenres()
			rebuildFilter()
			invalidate()
		case hitPlay:
			if idx := stationIndexFromHit(h); idx >= 0 {
				activateStation(idx)
			}
		case hitLink:
			if idx := stationIndexFromHit(h); idx >= 0 {
				copyStationLink(idx)
			}
		case hitWeb:
			if idx := stationIndexFromHit(h); idx >= 0 {
				openStationWeb(idx)
			}
		case hitFavorite:
			if idx := stationIndexFromHit(h); idx >= 0 {
				toggleFavorite(idx)
			}
		case hitReplace:
			if idx := stationIndexFromHit(h); idx >= 0 {
				replaceStation(idx)
			}
		case hitCheckStation:
			if idx := stationIndexFromHit(h); idx >= 0 {
				checkStationNow(idx)
			}
		case hitVolumeDown:
			adjustVolume(-5)
		case hitVolumeUp:
			adjustVolume(5)
		case hitCopyNowPlaying:
			copyNowPlaying()
		case hitPlayerPrev:
			playAdjacent(-1)
		case hitPlayerNext:
			playAdjacent(1)
		case hitPlayerPlay:
			toggleCurrentPlayback()
		case hitPlayerStop:
			stopCurrentPlayback()
		case hitCheckAll:
			setStatus("Provjeravam dostupnost svih stanica…")
			postUI()
			safeGo("manual-health", healthCheckAll)
		case hitAbout:
			messageBox(hwndOrZero(), "Radio Balkan", "Radio Balkan "+appVersion+"\n\nRadio stanice samo iz Hrvatske, Bosne i Hercegovine, Srbije, Slovenije, Sjeverne Makedonije, Albanije i Crne Gore.\nFavoriti, povijest slušanja, automatska provjera dostupnosti i zamjenski izvori rade lokalno na tvojem računalu.", MB_ICONINFORMATION)
		case hitRefresh:
			app.mu.Lock()
			app.countryMenuOpen = false
			app.genreMenuOpen = false
			app.mu.Unlock()
			safeGo("refresh-catalog", refreshAll)
		}
		return
	}
	if menuOpen {
		app.mu.Lock()
		app.countryMenuOpen = false
		app.genreMenuOpen = false
		app.mu.Unlock()
		invalidate()
	}
}

func hwndOrZero() syscall.Handle { return app.hwnd }

func selectCountry(code string) {
	if !isBalkanCode(code) {
		code = ""
	}
	app.mu.Lock()
	app.country = code
	app.scroll = 0
	app.countryMenuOpen = false
	app.genreMenuOpen = false
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.CountryCode = code
	app.stateMu.Unlock()
	scheduleStateSave()
	rebuildGenres()
	rebuildFilter()
	invalidate()
}

func selectGenre(genre string) {
	genre = strings.TrimSpace(genre)
	app.mu.Lock()
	app.genre = genre
	app.scroll = 0
	app.countryMenuOpen = false
	app.genreMenuOpen = false
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.Genre = genre
	app.stateMu.Unlock()
	scheduleStateSave()
	rebuildFilter()
	invalidate()
}

func activateStation(idx int) {
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	key := stationKey(app.stations[idx])
	same := app.currentKey != "" && app.currentKey == key
	app.mu.RUnlock()
	if same {
		toggleCurrentPlayback()
		return
	}
	playStation(idx)
}

func playStationByKey(key string, fallback int) {
	app.mu.RLock()
	idx := findStationIndexLocked(key, fallback)
	app.mu.RUnlock()
	if idx >= 0 {
		playStation(idx)
	}
}

func playStation(idx int) {
	app.mu.Lock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.Unlock()
		return
	}
	s := app.stations[idx]
	key := stationKey(s)
	app.playSeq++
	reqSeq := app.playSeq
	app.mu.Unlock()
	setStatus("Otvaram: " + s.Name)
	invalidate()
	safeGo("play-"+key, func() {
		final, ok := ensureStreamKey(idx, key)
		if !ok {
			app.mu.RLock()
			currentReq := app.playSeq == reqSeq
			app.mu.RUnlock()
			if currentReq {
				setStatus("Stanica trenutno nije dostupna: " + s.Name)
				postUI()
				queueAlert("Stanica nije dostupna", "Za ovu stanicu trenutno nije pronađen dostupan izvor.", MB_ICONWARNING)
			}
			return
		}
		app.playTransitionMu.Lock()
		app.mu.RLock()
		if app.playSeq != reqSeq {
			app.mu.RUnlock()
			app.playTransitionMu.Unlock()
			return
		}
		app.mu.RUnlock()
		if err := audioPlay(final); err != nil {
			app.playTransitionMu.Unlock()
			logError("audio-play", err)
			app.mu.RLock()
			currentReq := app.playSeq == reqSeq
			app.mu.RUnlock()
			if currentReq {
				setStatus("Reprodukcija nije uspjela")
				postUI()
				queueAlert("Reprodukcija", "Ovu stanicu trenutno nije moguće reproducirati. Pokušaj ponovno ili odaberi drugi izvor.", MB_ICONWARNING)
			}
			return
		}
		app.mu.Lock()
		if app.playSeq != reqSeq {
			app.mu.Unlock()
			audioStop()
			app.playTransitionMu.Unlock()
			return
		}
		app.playTransitionMu.Unlock()
		actual := findStationIndexLocked(key, idx)
		if actual < 0 {
			app.playing = false
			app.mu.Unlock()
			return
		}
		idx = actual
		s = app.stations[idx]
		app.current = idx
		app.currentKey = key
		app.playing = true
		app.audioRecovering = false
		app.nowPlaying = ""
		app.nowPlayingStation = key
		app.metadataSeq++
		seq := app.metadataSeq
		app.mu.Unlock()
		addRecentLocked(key)
		scheduleStateSave()
		rebuildFilter()
		setStatus("Uživo · " + s.Name)
		postUI()
		safeGo("watchdog-"+key, func() { playbackWatchdog(idx, key) })
		safeGo("metadata-"+key, func() { metadataLoop(seq, idx, key, final) })
	})
}
func playbackWatchdog(idx int, stationID string) {
	failures := 0
	for {
		timer := time.NewTimer(45 * time.Second)
		select {
		case <-timer.C:
		case <-app.done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
		app.mu.RLock()
		actual := findStationIndexLocked(stationID, idx)
		active := app.playing && actual >= 0 && app.currentKey == stationID
		var currentStation RadioStation
		if active {
			idx = actual
			currentStation = app.stations[idx]
		}
		app.mu.RUnlock()
		if !active {
			return
		}
		u := effectiveURL(currentStation)
		if _, ok := checkStream(u); ok {
			failures = 0
			continue
		}
		failures++
		if failures < 2 {
			continue
		}
		setStatus("Veza je prekinuta · pokušavam ponovno…")
		postUI()
		if repaired, ok := ensureStreamKey(idx, stationID); ok {
			if err := audioPlay(repaired); err == nil {
				failures = 0
				rebuildFilter()
				setStatus("Veza je obnovljena · reprodukcija je nastavljena")
				postUI()
				continue
			}
		}
		app.mu.Lock()
		app.playing = false
		app.mu.Unlock()
		setStatus("Stanica trenutno nije dostupna")
		postUI()
		return
	}
}
func ensureStream(idx int) (string, bool) {
	return ensureStreamKey(idx, "")
}

func ensureStreamKey(idx int, expectedKey string) (string, bool) {
	app.mu.RLock()
	if expectedKey != "" {
		idx = findStationIndexLocked(expectedKey, idx)
	}
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return "", false
	}
	s := app.stations[idx]
	app.mu.RUnlock()
	key := stationKey(s)
	if expectedKey != "" && key != expectedKey {
		return "", false
	}
	releaseRepair := acquireStationRepair(key)
	defer releaseRepair()
	app.mu.RLock()
	if actual := findStationIndexLocked(key, idx); actual >= 0 {
		idx = actual
		s = app.stations[idx]
	}
	app.mu.RUnlock()
	candidates := []string{}
	app.stateMu.RLock()
	if r := app.state.Replacements[key]; r != "" {
		candidates = append(candidates, r)
	}
	candidates = append(candidates, app.state.Backups[key]...)
	app.stateMu.RUnlock()
	candidates = append(candidates, s.ActiveURL, s.URLResolved, s.URL)
	for _, c := range uniqueStrings(candidates) {
		if resolved, ok := checkStream(c); ok {
			updateStationURL(idx, key, resolved, c != s.URLResolved && c != s.URL)
			return resolved, true
		}
	}
	if s.StationUUID != "" {
		if one, err := fetchStationByUUID(s.StationUUID); err == nil && one != nil {
			for _, c := range uniqueStrings([]string{one.URLResolved, one.URL}) {
				if resolved, ok := checkStream(c); ok {
					rememberReplacement(idx, key, resolved)
					return resolved, true
				}
			}
		}
	}
	if s.Name != "" {
		if list, err := searchStationsByName(s.Name, s.CountryCode); err == nil {
			checked := 0
			for _, alt := range list {
				if !sameStation(s, alt) {
					continue
				}
				for _, c := range uniqueStrings([]string{alt.URLResolved, alt.URL}) {
					if checked >= 20 {
						break
					}
					checked++
					if resolved, ok := checkStream(c); ok {
						rememberReplacement(idx, key, resolved)
						return resolved, true
					}
				}
				if checked >= 20 {
					break
				}
			}
		}
	}
	if s.Homepage != "" {
		candidates := discoverStreamCandidates(s.Homepage)
		if len(candidates) > 16 {
			candidates = candidates[:16]
		}
		for _, c := range candidates {
			if resolved, ok := checkStream(c); ok {
				rememberReplacement(idx, key, resolved)
				return resolved, true
			}
		}
	}
	app.mu.Lock()
	if actual := findStationIndexLocked(key, idx); actual >= 0 {
		app.stations[actual].Health = "broken"
	}
	app.mu.Unlock()
	postUI()
	return "", false
}
func updateStationURL(idx int, key, u string, replaced bool) {
	app.mu.Lock()
	if actual := findStationIndexLocked(key, idx); actual >= 0 {
		app.stations[actual].ActiveURL = u
		app.stations[actual].Health = "ok"
		if replaced {
			app.stations[actual].Replaced = true
		}
	}
	app.mu.Unlock()
}
func rememberReplacement(idx int, key, u string) {
	app.mu.Lock()
	actual := findStationIndexLocked(key, idx)
	if actual >= 0 {
		app.stations[actual].ActiveURL = u
		app.stations[actual].Health = "ok"
		app.stations[actual].Replaced = true
	}
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.Replacements[key] = u
	app.state.Backups[key] = prependUnique(app.state.Backups[key], u, 8)
	app.stateMu.Unlock()
	scheduleStateSave()
}
func copyStationLink(idx int) {
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	s := app.stations[idx]
	app.mu.RUnlock()
	u := effectiveURL(s)
	if u == "" {
		setStatus("Ova stanica nema dostupan izvor za reprodukciju")
		postUI()
		return
	}
	if copyClipboard(u) == nil {
		setStatus("Poveznica za reprodukciju je kopirana")
	} else {
		setStatus("Nije moguće kopirati link")
	}
	invalidate()
}
func openStationWeb(idx int) {
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	u := strings.TrimSpace(app.stations[idx].Homepage)
	app.mu.RUnlock()
	if u == "" {
		setStatus("Ova stanica nema upisanu web stranicu")
		postUI()
		return
	}
	shellOpen(u)
}
func toggleFavorite(idx int) {
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	key := stationKey(app.stations[idx])
	app.mu.RUnlock()
	app.stateMu.Lock()
	app.state.Favorites[key] = !app.state.Favorites[key]
	app.stateMu.Unlock()
	scheduleStateSave()
	rebuildFilter()
	invalidate()
}
func replaceStation(idx int) {
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	s := app.stations[idx]
	app.mu.RUnlock()
	key := stationKey(s)
	def := effectiveURL(s)
	value, ok := inputDialog(app.hwnd, "Promijeni izvor", "Unesi novu http/https poveznicu za reprodukciju za:\n"+s.Name+"\n\nPrazno polje vraća automatski odabir.", def)
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		clearReplacement(idx, key)
		setStatus("Vraćen automatski odabir · " + s.Name)
		postUI()
		return
	}
	setStatus("Provjeravam novi izvor…")
	invalidate()
	safeGo("manual-replace-"+key, func() {
		resolved, valid := checkStream(value)
		if !valid {
			queueAlert("Neispravan URL", "Novi izvor nije dostupan.", MB_ICONWARNING)
			setStatus("Zamjena nije spremljena")
			postUI()
			return
		}
		rememberReplacement(idx, key, resolved)
		rebuildFilter()
		setStatus("Izvor promijenjen · " + s.Name)
		postUI()
	})
}
func checkStationNow(idx int) {
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	s := app.stations[idx]
	app.mu.RUnlock()
	setStatus("Provjeravam: " + s.Name)
	postUI()
	safeGo("check-"+stationKey(s), func() {
		if _, ok := ensureStreamKey(idx, stationKey(s)); ok {
			setStatus("Stanica je dostupna · " + s.Name)
		} else {
			setStatus("Stanica nije dostupna · " + s.Name)
		}
		rebuildFilter()
		postUI()
	})
}
func adjustVolume(delta int) {
	app.stateMu.Lock()
	app.state.Volume += delta
	if app.state.Volume < 0 {
		app.state.Volume = 0
	}
	if app.state.Volume > 100 {
		app.state.Volume = 100
	}
	v := app.state.Volume
	app.stateMu.Unlock()
	app.mu.RLock()
	playing := app.playing
	app.mu.RUnlock()
	if playing {
		audioSetVolume(v)
	}
	scheduleStateSave()
	setStatus(fmt.Sprintf("Glasnoća %d%%", v))
	invalidate()
}
func copyNowPlaying() {
	app.mu.RLock()
	v := app.nowPlaying
	app.mu.RUnlock()
	if v == "" {
		return
	}
	if copyClipboard(v) == nil {
		setStatus("Sada svira kopirano")
	} else {
		setStatus("Nije moguće kopirati Sada svira")
	}
	invalidate()
}

func metadataLoop(seq uint64, idx int, stationID, raw string) {
	for {
		app.mu.RLock()
		actual := findStationIndexLocked(stationID, idx)
		active := app.playing && actual >= 0 && app.currentKey == stationID && app.metadataSeq == seq && app.nowPlayingStation == stationID
		app.mu.RUnlock()
		if !active {
			return
		}
		idx = actual
		if title, ok := fetchIcyTitle(raw); ok && title != "" {
			app.mu.Lock()
			if app.metadataSeq == seq && app.currentKey == stationID {
				app.nowPlaying = title
			}
			app.mu.Unlock()
			postUI()
		}
		for i := 0; i < 9; i++ {
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-timer.C:
			case <-app.done:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
			app.mu.RLock()
			actual = findStationIndexLocked(stationID, idx)
			active = app.playing && actual >= 0 && app.currentKey == stationID && app.metadataSeq == seq
			app.mu.RUnlock()
			if !active {
				return
			}
			idx = actual
		}
	}
}
func fetchIcyTitle(raw string) (string, bool) {
	if strings.Contains(strings.ToLower(raw), ".m3u8") || !safeHTTPURL(raw) {
		return "", false
	}
	ctx, cancel := context.WithTimeout(appContext(), 11*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
	req.Header.Set("Icy-MetaData", "1")
	resp, err := app.http.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	mi, _ := strconv.Atoi(resp.Header.Get("icy-metaint"))
	if mi <= 0 || mi > 4<<20 {
		return "", false
	}
	for n := 0; n < 4; n++ {
		if _, err = io.CopyN(io.Discard, resp.Body, int64(mi)); err != nil {
			return "", false
		}
		var lb [1]byte
		if _, err = io.ReadFull(resp.Body, lb[:]); err != nil {
			return "", false
		}
		ml := int(lb[0]) * 16
		if ml == 0 {
			continue
		}
		if ml > 64<<10 {
			return "", false
		}
		buf := make([]byte, ml)
		if _, err = io.ReadFull(resp.Body, buf); err != nil {
			return "", false
		}
		meta := strings.TrimRight(string(buf), "\x00")
		if title := parseStreamTitle(meta); title != "" {
			return title, true
		}
	}
	return "", false
}
func parseStreamTitle(meta string) string {
	const key = "StreamTitle='"
	i := strings.Index(meta, key)
	if i < 0 {
		return ""
	}
	s := meta[i+len(key):]
	if j := strings.Index(s, "';"); j >= 0 {
		s = s[:j]
	} else if j := strings.Index(s, "'"); j >= 0 {
		s = s[:j]
	}
	s = strings.TrimSpace(strings.ReplaceAll(s, "\\'", "'"))
	if len([]rune(s)) > 180 {
		s = string([]rune(s)[:180])
	}
	return s
}

func periodicHealthLoop() {
	for {
		app.stateMu.RLock()
		m := app.state.HealthIntervalMin
		app.stateMu.RUnlock()
		if m < 10 {
			m = 30
		}
		timer := time.NewTimer(time.Duration(m) * time.Minute)
		select {
		case <-timer.C:
		case <-app.done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
		app.mu.RLock()
		n := len(app.stations)
		running := app.healthRunning
		app.mu.RUnlock()
		if n > 0 && !running && !shuttingDown() {
			healthCheckQuick(64)
		}
	}
}

func clearReplacement(idx int, expectedKey string) {
	key := expectedKey
	app.mu.RLock()
	if key != "" {
		idx = findStationIndexLocked(key, idx)
	}
	if idx >= 0 && idx < len(app.stations) {
		key = stationKey(app.stations[idx])
	}
	app.mu.RUnlock()
	if key == "" {
		return
	}
	app.stateMu.Lock()
	delete(app.state.Replacements, key)
	delete(app.state.Backups, key)
	app.stateMu.Unlock()
	app.mu.Lock()
	if actual := findStationIndexLocked(key, idx); actual >= 0 {
		st := &app.stations[actual]
		st.Replaced = false
		if st.URLResolved != "" {
			st.ActiveURL = st.URLResolved
		} else {
			st.ActiveURL = st.URL
		}
		if st.LastCheckOK == 1 {
			st.Health = "ok"
		} else {
			st.Health = "unknown"
		}
	}
	app.mu.Unlock()
	scheduleStateSave()
	rebuildFilter()
	postUI()
}

func mergeRuntimeStationState(oldList, newList []RadioStation) {
	if len(oldList) == 0 || len(newList) == 0 {
		return
	}
	type runtimeState struct {
		Health    string
		ActiveURL string
		Replaced  bool
	}
	m := make(map[string]runtimeState, len(oldList))
	for _, st := range oldList {
		key := stationKey(st)
		if key == "" {
			continue
		}
		m[key] = runtimeState{Health: st.Health, ActiveURL: st.ActiveURL, Replaced: st.Replaced}
	}
	for i := range newList {
		key := stationKey(newList[i])
		prev, ok := m[key]
		if !ok {
			continue
		}
		if prev.Health != "" && prev.Health != "unknown" {
			newList[i].Health = prev.Health
		}
		if prev.ActiveURL != "" && (prev.Replaced || newList[i].ActiveURL == "") {
			newList[i].ActiveURL = prev.ActiveURL
			newList[i].Replaced = prev.Replaced
		}
	}
}

func refreshAll() {
	app.mu.Lock()
	if app.loading {
		app.mu.Unlock()
		setStatus("Popis stanica se već osvježava…")
		postUI()
		return
	}
	if app.refreshRunning {
		app.mu.Unlock()
		setStatus("Osvježavanje je već u tijeku…")
		postUI()
		return
	}
	app.refreshRunning = true
	app.mu.Unlock()
	defer func() { app.mu.Lock(); app.refreshRunning = false; app.mu.Unlock() }()
	setStatus("Osvježavam popis stanica…")
	postUI()
	app.mu.RLock()
	currentKey := app.currentKey
	wasPlaying := app.playing
	if currentKey == "" {
		if idx := currentStationIndexLocked(); idx >= 0 {
			currentKey = stationKey(app.stations[idx])
		}
	}
	oldStations := append([]RadioStation(nil), app.stations...)
	app.mu.RUnlock()
	list, err := fetchBalkanStations()
	if err != nil {
		logError("refresh-catalog", err)
		setStatus("Osvježavanje nije uspjelo · zadržan je postojeći popis")
		postUI()
		return
	}
	prepareStations(list)
	mergeRuntimeStationState(oldStations, list)
	keepPlaying := false
	app.mu.Lock()
	app.stations = list
	app.loading = false
	app.scroll = 0
	app.current = -1
	app.currentKey = ""
	if currentKey != "" {
		if i := findStationIndexLocked(currentKey, -1); i >= 0 {
			app.current = i
			app.currentKey = currentKey
			app.playing = wasPlaying
			keepPlaying = wasPlaying
		}
	}
	app.mu.Unlock()
	if wasPlaying && !keepPlaying {
		audioStop()
		app.mu.Lock()
		app.currentKey = ""
		app.mu.Unlock()
	}
	saveCache(list)
	postGenres()
	rebuildFilter()
	setStatus(fmt.Sprintf("Učitano %d radio stanica", len(list)))
	postUI()
	safeGo("health-refresh", func() { healthCheckQuick(56) })
}
func loadStations() {
	cached := loadCache()
	if len(cached) > 0 {
		prepareStations(cached)
		app.mu.Lock()
		app.stations = cached
		app.loading = false
		app.mu.Unlock()
		postGenres()
		rebuildFilter()
		setStatus(fmt.Sprintf("Prikazan spremljeni popis · osvježavam %d stanica…", len(cached)))
		postUI()
	}

	list, err := fetchBalkanStations()
	if err != nil {
		logError("load-catalog", err)
		if len(cached) == 0 {
			app.mu.Lock()
			app.loading = false
			app.mu.Unlock()
			setStatus("Trenutno nije moguće učitati popis stanica")
			postUI()
			return
		}
		setStatus("Nema mreže · koristi se zadnji spremljeni popis")
		postUI()
		if !app.safeMode {
			safeGo("health-startup-cache", func() { healthCheckQuick(32) })
		}
		safeGo("health-periodic", periodicHealthLoop)
		return
	}

	prepareStations(list)
	app.mu.Lock()
	app.stations = list
	app.loading = false
	app.scroll = 0
	app.mu.Unlock()
	saveCache(list)
	postGenres()
	rebuildFilter()
	setStatus(fmt.Sprintf("Učitano %d stanica iz %d država", len(list), countCountries(list)))
	postUI()
	if !app.safeMode {
		safeGo("health-startup", func() { healthCheckQuick(48) })
	}
	safeGo("health-periodic", periodicHealthLoop)
}
func prepareStations(list []RadioStation) {
	app.stateMu.RLock()
	defer app.stateMu.RUnlock()
	for i := range list {
		list[i].CountryCode = strings.ToUpper(strings.TrimSpace(list[i].CountryCode))
		list[i].SearchIndex = foldText(strings.TrimSpace(list[i].Name + " " + list[i].Tags + " " + list[i].State + " " + list[i].Country + " " + list[i].Language))
		list[i].TagsIndex = foldText(strings.TrimSpace(list[i].Tags))
		key := stationKey(list[i])
		if r := app.state.Replacements[key]; r != "" {
			list[i].ActiveURL = r
			list[i].Replaced = true
		} else if list[i].URLResolved != "" {
			list[i].ActiveURL = list[i].URLResolved
		} else {
			list[i].ActiveURL = list[i].URL
		}
		if list[i].LastCheckOK == 1 {
			list[i].Health = "ok"
		} else {
			list[i].Health = "unknown"
		}
	}
}
func healthCheckOne(key string) (result int) {
	defer func() {
		if r := recover(); r != nil {
			logError("health-one-"+key, fmt.Errorf("panic: %v\n%s", r, debug.Stack()))
			result = 3
		}
	}()
	if shuttingDown() {
		return 0
	}
	app.mu.RLock()
	idx := findStationIndexLocked(key, -1)
	var st RadioStation
	if idx >= 0 {
		st = app.stations[idx]
	}
	app.mu.RUnlock()
	if idx < 0 {
		return 0
	}
	before := effectiveURL(st)
	resolved, ok := ensureStreamKey(idx, key)
	if !ok {
		app.mu.Lock()
		if actual := findStationIndexLocked(key, idx); actual >= 0 {
			app.stations[actual].Health = "broken"
		}
		app.mu.Unlock()
		return 3
	}
	app.mu.RLock()
	actual := findStationIndexLocked(key, idx)
	replaced := actual >= 0 && app.stations[actual].Replaced
	app.mu.RUnlock()
	if replaced || (before != "" && !strings.EqualFold(strings.TrimSpace(before), strings.TrimSpace(resolved))) {
		return 2
	}
	return 1
}

func healthCheckAll() { healthCheckWithLimit(0, true) }

func scheduleStartupHealth(limit int) {
	if limit < 1 {
		limit = 24
	}
	if app.safeMode || shuttingDown() {
		return
	}
	safeGo("health-startup-delay", func() {
		timer := time.NewTimer(8 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-app.done:
			return
		}
		if !shuttingDown() {
			healthCheckQuick(limit)
		}
	})
}

func healthCheckQuick(limit int) {
	if limit < 1 {
		limit = 48
	}
	healthCheckWithLimit(limit, false)
}

func healthCheckWithLimit(limit int, queueRescan bool) {
	if shuttingDown() {
		return
	}
	app.mu.Lock()
	if app.healthRunning {
		if queueRescan {
			app.healthRescanRequested = true
		}
		app.mu.Unlock()
		return
	}
	if len(app.stations) == 0 {
		app.mu.Unlock()
		return
	}
	app.healthRunning = true
	stationSnapshot := append([]RadioStation(nil), app.stations...)
	app.mu.Unlock()

	app.stateMu.RLock()
	favorites := make(map[string]bool, len(app.state.Favorites))
	for k, v := range app.state.Favorites {
		favorites[k] = v
	}
	recent := append([]string(nil), app.state.Recent...)
	app.stateMu.RUnlock()
	recentOrder := make(map[string]int, len(recent))
	for i, k := range recent {
		recentOrder[k] = i
	}
	type priorityKey struct {
		key      string
		priority int
		votes    int
	}
	items := make([]priorityKey, 0, len(stationSnapshot))
	for _, st := range stationSnapshot {
		key := stationKey(st)
		p := 2
		if favorites[key] {
			p = 0
		} else if _, ok := recentOrder[key]; ok {
			p = 1
		}
		items = append(items, priorityKey{key: key, priority: p, votes: st.Votes})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority != items[j].priority {
			return items[i].priority < items[j].priority
		}
		if items[i].priority == 1 {
			return recentOrder[items[i].key] < recentOrder[items[j].key]
		}
		return items[i].votes > items[j].votes
	})
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.key)
	}
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}

	defer func() {
		app.mu.Lock()
		again := queueRescan && app.healthRescanRequested && !shuttingDown()
		app.healthRescanRequested = false
		app.healthRunning = false
		app.lastHealth = time.Now()
		app.mu.Unlock()
		if again {
			safeGo("health-rescan", healthCheckAll)
		}
	}()
	jobs := make(chan string, 12)
	var wg sync.WaitGroup
	workers := 2
	if len(keys) < workers {
		workers = len(keys)
	}
	var doneCount, okc, repc, broken int64
	var mx sync.Mutex
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-app.done:
					return
				case key, ok := <-jobs:
					if !ok {
						return
					}
					r := healthCheckOne(key)
					mx.Lock()
					if r != 0 {
						doneCount++
					}
					switch r {
					case 1:
						okc++
					case 2:
						repc++
					case 3:
						broken++
					}
					d, o, rp, b := doneCount, okc, repc, broken
					mx.Unlock()
					if d > 0 && (d%50 == 0 || d == int64(len(keys))) && !shuttingDown() {
						setStatus(fmt.Sprintf("Provjera %d/%d · dostupno %d · alternativno %d · nedostupno %d", d, len(keys), o, rp, b))
						postUI()
					}
				}
			}
		}()
	}
	for _, key := range keys {
		select {
		case <-app.done:
			close(jobs)
			wg.Wait()
			return
		case jobs <- key:
		}
	}
	close(jobs)
	wg.Wait()
	if shuttingDown() {
		return
	}
	app.mu.Lock()
	app.healthOK = int(okc)
	app.healthReplaced = int(repc)
	app.healthBroken = int(broken)
	app.mu.Unlock()
	rebuildFilter()
	setStatus(fmt.Sprintf("Provjera završena · dostupno %d · alternativno %d · nedostupno %d", okc, repc, broken))
	postUI()
}

func fetchBalkanStations() ([]RadioStation, error) {
	// Production catalog is restricted to the seven supported countries.
	// Every batch is validated against the seven supported countries before it can
	// enter memory or the persistent cache.
	type result struct {
		list []RadioStation
		err  error
		code string
	}
	codes := make([]string, 0, len(balkanCountries)-1)
	for _, c := range balkanCountries {
		if c.Code != "" {
			codes = append(codes, c.Code)
		}
	}
	out := make(chan result, len(codes))
	sem := make(chan struct{}, 3)
	for _, code := range codes {
		code := code
		go func() {
			acquired := false
			defer func() {
				if acquired {
					<-sem
				}
				if r := recover(); r != nil {
					err := fmt.Errorf("panic kataloga %s: %v", code, r)
					logError("catalog-"+code, err)
					out <- result{nil, err, code}
				}
			}()
			select {
			case sem <- struct{}{}:
				acquired = true
			case <-app.done:
				out <- result{nil, context.Canceled, code}
				return
			}
			list, err := fetchCountryStations(code)
			out <- result{list, err, code}
		}()
	}

	all := make([]RadioStation, 0, 5000)
	failures := []string{}
	for range codes {
		r := <-out
		if r.err != nil {
			failures = append(failures, r.code)
			continue
		}
		all = append(all, r.list...)
	}
	if len(all) == 0 {
		if cached := loadCache(); len(cached) > 0 {
			return cached, nil
		}
		return nil, fmt.Errorf("nije dohvaćena nijedna radio stanica: %v", failures)
	}
	if len(failures) > 0 {
		failed := make(map[string]bool, len(failures))
		for _, code := range failures {
			failed[strings.ToUpper(code)] = true
		}
		if cached := loadCache(); len(cached) > 0 {
			for _, st := range cached {
				if failed[strings.ToUpper(strings.TrimSpace(st.CountryCode))] {
					all = append(all, st)
				}
			}
		}
		logError("catalog-partial", fmt.Errorf("neke države nisu osvježene; korišten je spremljeni katalog: %v", failures))
	}
	all = dedupeStations(filterSupportedStations(all))
	// A transient mirror response must never erase a healthy regional cache.
	if len(all) < 600 {
		if cached := loadCache(); len(cached) > 0 {
			all = dedupeStations(append(all, cached...))
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		pi, pj := countryPriority(all[i].CountryCode), countryPriority(all[j].CountryCode)
		if pi == pj {
			if all[i].Votes == all[j].Votes {
				return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
			}
			return all[i].Votes > all[j].Votes
		}
		return pi < pj
	})
	return trimStationCatalog(filterSupportedStations(all), 7000), nil
}

func filterSupportedStations(in []RadioStation) []RadioStation {
	if len(in) == 0 {
		return nil
	}
	out := make([]RadioStation, 0, len(in))
	for _, st := range in {
		code := strings.ToUpper(strings.TrimSpace(st.CountryCode))
		if code == "" || !isBalkanCode(code) {
			continue
		}
		st.CountryCode = code
		if st.Country == "" {
			st.Country = countryNameByCode(code)
		}
		out = append(out, st)
	}
	return out
}

func trimStationCatalog(in []RadioStation, max int) []RadioStation {
	in = filterSupportedStations(in)
	if max <= 0 || len(in) <= max {
		return in
	}
	out := make([]RadioStation, max)
	copy(out, in[:max])
	return out
}

func fetchCountryStations(code string) ([]RadioStation, error) {
	const pageSize = 250
	// The selected seven countries currently fit well below this bound, while the
	// guard protects against a malformed mirror returning an endless paginated set.
	const maxPerCountry = 1800
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || !isBalkanCode(code) {
		return nil, errors.New("nepodržana država")
	}
	var last error
	for _, base := range apiBases() {
		combined := make([]RadioStation, 0, 900)
		for offset := 0; offset < maxPerCountry; offset += pageSize {
			select {
			case <-app.done:
				return nil, context.Canceled
			default:
			}
			p := "/json/stations/search?countrycode=" + url.QueryEscape(code) +
				"&hidebroken=true&order=votes&reverse=true&limit=" + strconv.Itoa(pageSize) +
				"&offset=" + strconv.Itoa(offset)
			var page []RadioStation
			if err := getJSON(base+p, &page); err != nil {
				last = err
				break
			}
			if len(page) == 0 {
				break
			}
			accepted := 0
			for _, st := range page {
				cc := strings.ToUpper(strings.TrimSpace(st.CountryCode))
				if cc == "" {
					cc = code
				}
				if cc != code {
					continue
				}
				st.CountryCode = code
				if st.Country == "" {
					st.Country = countryNameByCode(code)
				}
				combined = append(combined, st)
				accepted++
			}
			if len(page) < pageSize {
				break
			}
			// A full page with zero matching country records means the mirror ignored
			// the filter; stop instead of accepting unrelated stations.
			if accepted == 0 {
				break
			}
		}
		if len(combined) == 0 {
			p := "/json/stations/bycountrycodeexact/" + url.PathEscape(code) + "?hidebroken=true&order=votes&reverse=true&limit=1800"
			var page []RadioStation
			if err := getJSON(base+p, &page); err != nil {
				last = err
				continue
			}
			for _, st := range page {
				cc := strings.ToUpper(strings.TrimSpace(st.CountryCode))
				if cc == "" {
					cc = code
				}
				if cc != code {
					continue
				}
				st.CountryCode = code
				if st.Country == "" {
					st.Country = countryNameByCode(code)
				}
				combined = append(combined, st)
			}
		}
		if len(combined) > 0 {
			combined = dedupeStations(filterSupportedStations(combined))
			sort.SliceStable(combined, func(i, j int) bool {
				if combined[i].Votes == combined[j].Votes {
					return strings.ToLower(combined[i].Name) < strings.ToLower(combined[j].Name)
				}
				return combined[i].Votes > combined[j].Votes
			})
			return combined, nil
		}
	}
	if last == nil {
		last = errors.New("prazan katalog")
	}
	return nil, last
}

func fetchStationByUUID(id string) (*RadioStation, error) {
	for _, base := range apiBases() {
		var list []RadioStation
		if err := getJSON(base+"/json/stations/byuuid/"+url.PathEscape(id), &list); err == nil {
			for i := range list {
				code := strings.ToUpper(strings.TrimSpace(list[i].CountryCode))
				if code != "" && isBalkanCode(code) {
					list[i].CountryCode = code
					return &list[i], nil
				}
			}
		}
	}
	return nil, errors.New("nije pronađeno")
}
func searchStationsByName(name, countryCode string) ([]RadioStation, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("prazan naziv")
	}
	q := url.QueryEscape(name)
	cc := url.QueryEscape(strings.ToUpper(strings.TrimSpace(countryCode)))
	paths := []string{"/json/stations/search?name=" + q + "&hidebroken=false&limit=50"}
	if cc != "" {
		paths = []string{"/json/stations/search?name=" + q + "&countrycode=" + cc + "&hidebroken=false&limit=50", paths[0]}
	}
	var last error
	for _, base := range apiBases() {
		for _, p := range paths {
			var list []RadioStation
			if err := getJSON(base+p, &list); err == nil && len(list) > 0 {
				filtered := make([]RadioStation, 0, len(list))
				for _, st := range list {
					code := strings.ToUpper(strings.TrimSpace(st.CountryCode))
					if code == "" || !isBalkanCode(code) {
						continue
					}
					if countryCode != "" && !strings.EqualFold(code, strings.TrimSpace(countryCode)) {
						continue
					}
					st.CountryCode = code
					filtered = append(filtered, st)
				}
				if len(filtered) > 0 {
					return filtered, nil
				}
			} else if err != nil {
				last = err
			}
		}
	}
	if last == nil {
		last = errors.New("nije pronađeno")
	}
	return nil, last
}
func apiBases() []string {
	apiOnce.Do(func() {
		type server struct {
			Name string `json:"name"`
		}
		fallback := []string{"https://de1.api.radio-browser.info", "https://de2.api.radio-browser.info", "https://at1.api.radio-browser.info", "https://nl1.api.radio-browser.info"}
		ctx, cancel := context.WithTimeout(appContext(), 6*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", "https://all.api.radio-browser.info/json/servers", nil)
		if err == nil {
			req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
			if resp, e := app.http.Do(req); e == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 310 {
					var servers []server
					if json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&servers) == nil {
						for _, sv := range servers {
							base := "https://" + strings.TrimSpace(sv.Name)
							if safeHTTPURL(base) {
								apiBaseList = append(apiBaseList, strings.TrimRight(base, "/"))
								if len(apiBaseList) >= 8 {
									break
								}
							}
						}
					}
				}
			}
		}
		if len(apiBaseList) == 0 {
			apiBaseList = fallback
		}
	})
	return append([]string(nil), apiBaseList...)
}
func getJSON(u string, v any) error {
	if !safeHTTPURL(u) {
		return errors.New("neispravan API URL")
	}
	ctx, cancel := context.WithTimeout(appContext(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
	req.Header.Set("Accept", "application/json")
	resp, err := app.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 310 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 12<<20))
	if err = dec.Decode(v); err != nil {
		return fmt.Errorf("JSON: %w", err)
	}
	return nil
}
func checkStream(raw string) (string, bool) {
	if shuttingDown() {
		return "", false
	}
	if app.streamSem != nil {
		select {
		case app.streamSem <- struct{}{}:
			defer func() { <-app.streamSem }()
		case <-app.done:
			return "", false
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || !safeHTTPURL(raw) {
		return "", false
	}
	if looksPlaylist(raw) {
		if u, ok := resolvePlaylist(raw); ok {
			raw = u
		}
	}
	if !safeHTTPURL(raw) {
		return "", false
	}
	ctx, cancel := context.WithTimeout(appContext(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
	req.Header.Set("Icy-MetaData", "1")
	req.Header.Set("Accept", "audio/*,application/ogg,application/vnd.apple.mpegurl,application/x-mpegURL,*/*;q=0.5")
	resp, err := app.http.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", false
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") && !strings.Contains(strings.ToLower(raw), ".m3u8") {
		return "", false
	}
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if n <= 0 {
		if err != nil {
			return "", false
		}
		return "", false
	}
	final := resp.Request.URL.String()
	if !safeHTTPURL(final) {
		return "", false
	}
	return final, true
}
func looksPlaylist(u string) bool {
	l := strings.ToLower(strings.Split(u, "?")[0])
	return strings.HasSuffix(l, ".m3u") || strings.HasSuffix(l, ".pls") || strings.HasSuffix(l, ".asx")
}
func resolvePlaylist(raw string) (string, bool) {
	if !safeHTTPURL(raw) {
		return "", false
	}
	ctx, cancel := context.WithTimeout(appContext(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
	resp, err := app.http.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", false
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return "", false
	}
	text := html.UnescapeString(string(b))
	base, _ := url.Parse(resp.Request.URL.String())
	try := func(line string) (string, bool) {
		line = strings.Trim(strings.TrimSpace(line), "\\\"' ")
		if line == "" {
			return "", false
		}
		if i := strings.Index(line, "="); i >= 0 && (strings.HasPrefix(strings.ToLower(line), "file") || strings.Contains(strings.ToLower(line[:i]), "ref")) {
			line = strings.Trim(strings.TrimSpace(line[i+1:]), "\\\"'")
		}
		u, err := url.Parse(line)
		if err != nil {
			return "", false
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if safeHTTPURL(u.String()) {
			return u.String(), true
		}
		return "", false
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if u, ok := try(line); ok {
			return u, true
		}
	}
	re := regexp.MustCompile(`(?i)https?://[^\\s"'<>]+`)
	for _, m := range re.FindAllString(text, 40) {
		if u, ok := try(m); ok {
			return u, true
		}
	}
	return "", false
}
func discoverStreamCandidates(homepage string) []string {
	if !safeHTTPURL(homepage) {
		return nil
	}
	ctx, cancel := context.WithTimeout(appContext(), 9*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", homepage, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "RadioBalkan/"+appVersion)
	resp, err := app.http.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil
	}
	body := strings.ReplaceAll(html.UnescapeString(string(b)), `\/`, `/`)
	base, _ := url.Parse(resp.Request.URL.String())
	absRe := regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
	relRe := regexp.MustCompile(`(?i)(?:href|src)\s*=\s*["']([^"']+)["']`)
	out := []string{}
	add := func(raw string) {
		raw = strings.TrimSpace(strings.Trim(raw, "()[]{}.,;"))
		if raw == "" {
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			return
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		v := u.String()
		lv := strings.ToLower(v)
		if !(strings.Contains(lv, ".mp3") || strings.Contains(lv, ".aac") || strings.Contains(lv, ".ogg") || strings.Contains(lv, ".opus") || strings.Contains(lv, ".m3u") || strings.Contains(lv, ".pls") || strings.Contains(lv, ".asx") || strings.Contains(lv, "stream") || strings.Contains(lv, "listen") || strings.Contains(lv, "icecast") || strings.Contains(lv, "shoutcast")) {
			return
		}
		if strings.HasPrefix(lv, "http://") || strings.HasPrefix(lv, "https://") {
			out = append(out, v)
		}
	}
	for _, m := range absRe.FindAllString(body, 80) {
		add(m)
	}
	for _, m := range relRe.FindAllStringSubmatch(body, 80) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	out = uniqueStrings(out)
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func sameStation(a, b RadioStation) bool {
	if a.StationUUID != "" && a.StationUUID == b.StationUUID {
		return true
	}
	if a.CountryCode != "" && b.CountryCode != "" && !strings.EqualFold(a.CountryCode, b.CountryCode) {
		return false
	}
	an := normalizeName(a.Name)
	bn := normalizeName(b.Name)
	if an == "" || bn == "" {
		return false
	}
	if an == bn || strings.Contains(an, bn) || strings.Contains(bn, an) {
		if host(a.Homepage) == "" || host(b.Homepage) == "" || host(a.Homepage) == host(b.Homepage) {
			return true
		}
	}
	return false
}
func foldText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"č", "c", "ć", "c", "š", "s", "ž", "z", "đ", "d",
		"ë", "e", "ç", "c", "ș", "s", "ş", "s", "ț", "t",
		"ă", "a", "â", "a", "î", "i", "ı", "i", "ğ", "g",
		"ö", "o", "ü", "u", "é", "e", "è", "e", "á", "a", "í", "i",
	)
	return replacer.Replace(s)
}

func normalizeName(s string) string {
	s = foldText(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func host(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
func cleanedStationName(raw string) string {
	fields := strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, raw))
	name := strings.TrimSpace(strings.Join(fields, " "))
	if len([]rune(name)) > 180 {
		name = string([]rune(name)[:180])
	}
	return name
}

func stationIdentity(s RadioStation) string {
	name := normalizeName(s.Name)
	cc := strings.ToUpper(strings.TrimSpace(s.CountryCode))
	if name == "" {
		return ""
	}
	if h := host(s.Homepage); h != "" {
		return cc + "|" + name + "|home:" + h
	}
	for _, raw := range []string{s.URLResolved, s.URL} {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Hostname() == "" {
			continue
		}
		path := strings.TrimRight(strings.ToLower(u.EscapedPath()), "/")
		return cc + "|" + name + "|stream:" + strings.ToLower(u.Hostname()) + path
	}
	return cc + "|" + name
}

func stationQuality(s RadioStation) int {
	q := s.Votes
	if s.LastCheckOK == 1 {
		q += 1000000
	}
	if safeHTTPURL(s.Favicon) {
		q += 20000
	}
	if safeHTTPURL(s.Homepage) {
		q += 10000
	}
	if safeHTTPURL(s.URLResolved) {
		q += 5000
	}
	if s.Bitrate > 0 {
		q += minInt(s.Bitrate, 512)
	}
	return q
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mergeStationRecord(a, b RadioStation) RadioStation {
	primary, other := a, b
	if stationQuality(b) > stationQuality(a) {
		primary, other = b, a
	}
	if primary.Name == "" {
		primary.Name = other.Name
	}
	if primary.StationUUID == "" {
		primary.StationUUID = other.StationUUID
	}
	if primary.Homepage == "" {
		primary.Homepage = other.Homepage
	}
	if primary.Favicon == "" {
		primary.Favicon = other.Favicon
	}
	if primary.Tags == "" {
		primary.Tags = other.Tags
	}
	if primary.URL == "" {
		primary.URL = other.URL
	}
	if primary.URLResolved == "" {
		primary.URLResolved = other.URLResolved
	}
	if primary.State == "" {
		primary.State = other.State
	}
	if primary.Language == "" {
		primary.Language = other.Language
	}
	if primary.Country == "" {
		primary.Country = other.Country
	}
	if primary.CountryCode == "" {
		primary.CountryCode = other.CountryCode
	}
	if primary.Codec == "" {
		primary.Codec = other.Codec
	}
	if primary.Bitrate == 0 {
		primary.Bitrate = other.Bitrate
	}
	if primary.Votes < other.Votes {
		primary.Votes = other.Votes
	}
	if primary.LastCheckOK < other.LastCheckOK {
		primary.LastCheckOK = other.LastCheckOK
	}
	return primary
}

func dedupeStations(in []RadioStation) []RadioStation {
	byUUID := make(map[string]int, len(in))
	byIdentity := make(map[string]int, len(in))
	out := make([]RadioStation, 0, len(in))
	for _, s := range in {
		s.Name = cleanedStationName(s.Name)
		if s.Name == "" {
			continue
		}
		if strings.TrimSpace(s.URL) == "" && strings.TrimSpace(s.URLResolved) == "" && strings.TrimSpace(s.Homepage) == "" {
			continue
		}
		// Keep the complete supported-country catalog. Real station branding is
		// loaded from the catalog favicon or the station's official website when
		// available; the UI falls back to neutral local artwork rather than inventing
		// a fake station logo.
		uuid := strings.TrimSpace(s.StationUUID)
		identity := stationIdentity(s)
		pos := -1
		if uuid != "" {
			if v, ok := byUUID[uuid]; ok {
				pos = v
			}
		}
		if pos < 0 && identity != "" {
			if v, ok := byIdentity[identity]; ok {
				pos = v
			}
		}
		if pos >= 0 {
			out[pos] = mergeStationRecord(out[pos], s)
			merged := out[pos]
			if merged.StationUUID != "" {
				byUUID[merged.StationUUID] = pos
			}
			if id := stationIdentity(merged); id != "" {
				byIdentity[id] = pos
			}
			continue
		}
		pos = len(out)
		out = append(out, s)
		if uuid != "" {
			byUUID[uuid] = pos
		}
		if identity != "" {
			byIdentity[identity] = pos
		}
	}
	return out
}

func isQuickGenre(g string) bool {
	g = strings.ToLower(strings.TrimSpace(g))
	return g == "domaca" || g == "pop" || g == "rock" || g == "folk" || g == "electronic"
}

func matchesGenre(tags, genre string) bool {
	tags = foldText(tags)
	g := foldText(genre)
	switch g {
	case "domaca":
		return strings.Contains(tags, "domac") || strings.Contains(tags, "croatian") || strings.Contains(tags, "hrvats") || strings.Contains(tags, "balkan") || strings.Contains(tags, "ex yu") || strings.Contains(tags, "ex-yu")
	case "pop":
		return strings.Contains(tags, "pop") || strings.Contains(tags, "rock") || strings.Contains(tags, "indie") || strings.Contains(tags, "alternative")
	case "folk":
		return strings.Contains(tags, "folk") || strings.Contains(tags, "narod") || strings.Contains(tags, "sevd") || strings.Contains(tags, "turbo") || strings.Contains(tags, "etno") || strings.Contains(tags, "krajisk")
	case "electronic":
		return strings.Contains(tags, "electronic") || strings.Contains(tags, "dance") || strings.Contains(tags, "house") || strings.Contains(tags, "techno") || strings.Contains(tags, "edm") || strings.Contains(tags, "trance") || strings.Contains(tags, "club")
	default:
		return g != "" && strings.Contains(tags, g)
	}
}

func rebuildFilter() {
	defer func() {
		if r := recover(); r != nil {
			logError("rebuild-filter", fmt.Errorf("panic: %v\n%s", r, debug.Stack()))
		}
	}()
	app.stateMu.RLock()
	recent := append([]string(nil), app.state.Recent...)
	favs := make(map[string]bool, len(app.state.Favorites))
	for k, v := range app.state.Favorites {
		favs[k] = v
	}
	reps := make(map[string]string, len(app.state.Replacements))
	for k, v := range app.state.Replacements {
		reps[k] = v
	}
	app.stateMu.RUnlock()
	recentSet := map[string]int{}
	for i, id := range recent {
		recentSet[id] = i
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	q := foldText(app.search)
	g := foldText(app.genre)
	country := strings.ToUpper(strings.TrimSpace(app.country))
	ids := make([]int, 0, len(app.stations))
	for i, st := range app.stations {
		key := stationKey(st)
		hay := st.SearchIndex
		if hay == "" {
			hay = foldText(st.Name + " " + st.Tags + " " + st.State + " " + st.Country + " " + st.Language)
		}
		if q != "" && !strings.Contains(hay, q) {
			continue
		}
		tags := st.TagsIndex
		if tags == "" {
			tags = foldText(st.Tags)
		}
		if g != "" && !matchesGenre(tags, g) {
			continue
		}
		if country != "" && strings.ToUpper(st.CountryCode) != country {
			continue
		}
		switch app.tab {
		case "popular":
			if st.Votes < 5 {
				continue
			}
		case "favorites":
			if !favs[key] {
				continue
			}
		case "recent":
			if _, ok := recentSet[key]; !ok {
				continue
			}
		case "replaced":
			if !st.Replaced && reps[key] == "" {
				continue
			}
		case "broken":
			if st.Health != "broken" {
				continue
			}
		}
		ids = append(ids, i)
	}
	if app.tab == "popular" {
		sort.SliceStable(ids, func(i, j int) bool { return app.stations[ids[i]].Votes > app.stations[ids[j]].Votes })
	}
	if app.tab == "recent" {
		sort.SliceStable(ids, func(i, j int) bool {
			return recentSet[stationKey(app.stations[ids[i]])] < recentSet[stationKey(app.stations[ids[j]])]
		})
	}
	app.filtered = ids
	clampScrollLocked()
}
func rebuildGenres() {
	defer func() {
		if r := recover(); r != nil {
			logError("rebuild-genres", fmt.Errorf("panic: %v\n%s", r, debug.Stack()))
		}
	}()
	set := map[string]int{}
	app.mu.RLock()
	country := strings.ToUpper(strings.TrimSpace(app.country))
	desired := app.genre
	for _, st := range app.stations {
		if country != "" && strings.ToUpper(st.CountryCode) != country {
			continue
		}
		for _, t := range strings.Split(st.Tags, ",") {
			t = strings.TrimSpace(t)
			n := len([]rune(t))
			if n >= 3 && n <= 22 {
				set[t]++
			}
		}
	}
	app.mu.RUnlock()
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(set))
	for k, v := range set {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].v == arr[j].v {
			return strings.ToLower(arr[i].k) < strings.ToLower(arr[j].k)
		}
		return arr[i].v > arr[j].v
	})
	options := []string{"Svi žanrovi"}
	for i := 0; i < len(arr) && i < 12; i++ {
		options = append(options, arr[i].k)
	}
	found := desired == "" || isQuickGenre(desired)
	if !found {
		for _, option := range options[1:] {
			if strings.EqualFold(option, desired) {
				found = true
				break
			}
		}
	}
	app.mu.Lock()
	app.genreOptions = options
	if !found {
		app.genre = ""
		app.scroll = 0
	}
	app.mu.Unlock()
	if !found {
		app.stateMu.Lock()
		app.state.Genre = ""
		app.stateMu.Unlock()
		scheduleStateSave()
		rebuildFilter()
	}
}

func clampScroll() { app.mu.Lock(); defer app.mu.Unlock(); clampScrollLocked() }
func clampScrollLocked() {
	showPopular := app.tab == "all" && strings.TrimSpace(app.search) == "" && strings.TrimSpace(app.genre) == ""
	gridTop := 128
	if showPopular {
		gridTop = 438
	}
	viewport := int(app.clientHeight) - int(playerHeight) - 18 - gridTop
	if viewport < 100 {
		viewport = 100
	}
	rows := (len(app.filtered) + 1) / 2
	content := rows*110 - 12
	max := content - viewport
	if max < 0 {
		max = 0
	}
	if app.scroll > max {
		app.scroll = max
	}
	if app.scroll < 0 {
		app.scroll = 0
	}
}

func stationMeta(s RadioStation) string {
	parts := []string{}
	country := countryNameByCode(strings.ToUpper(s.CountryCode))
	if country == "" {
		country = s.Country
	}
	if s.State != "" && country != "" {
		parts = append(parts, s.State+", "+country)
	} else if country != "" {
		parts = append(parts, country)
	} else if s.State != "" {
		parts = append(parts, s.State)
	}
	if s.Codec != "" {
		parts = append(parts, strings.ToUpper(s.Codec))
	}
	if s.Bitrate > 0 {
		parts = append(parts, strconv.Itoa(s.Bitrate)+" kbps")
	}
	if s.Tags != "" {
		tg := strings.TrimSpace(strings.Split(s.Tags, ",")[0])
		if tg != "" {
			parts = append(parts, tg)
		}
	}
	return strings.Join(parts, " · ")
}
func healthText(h string) (string, uintptr) {
	switch h {
	case "ok":
		return "● Dostupno", rgb(90, 205, 128)
	case "broken":
		return "● Nedostupno", rgb(235, 89, 78)
	default:
		return "● Nije provjereno", rgb(189, 155, 79)
	}
}
func trimName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Radio stanica"
	}
	return s
}
func effectiveURL(s RadioStation) string {
	key := stationKey(s)
	app.stateMu.RLock()
	r := app.state.Replacements[key]
	app.stateMu.RUnlock()
	if r != "" {
		return r
	}
	if s.ActiveURL != "" {
		return s.ActiveURL
	}
	if s.URLResolved != "" {
		return s.URLResolved
	}
	return s.URL
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func prependUnique(in []string, s string, max int) []string {
	out := []string{s}
	for _, x := range in {
		if x != s && x != "" {
			out = append(out, x)
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}
func addRecentLocked(id string) {
	if id == "" {
		return
	}
	app.stateMu.Lock()
	defer app.stateMu.Unlock()
	out := []string{id}
	for _, x := range app.state.Recent {
		if x != id {
			out = append(out, x)
		}
	}
	if len(out) > 40 {
		out = out[:40]
	}
	app.state.Recent = out
}
func startAudioEngineLocked() error {
	if app.audioCmd != nil && app.audioCmd.Process != nil {
		return nil
	}
	script := `$ErrorActionPreference='Stop'; Add-Type -AssemblyName PresentationCore; $p=New-Object System.Windows.Media.MediaPlayer; while(($line=[Console]::In.ReadLine()) -ne $null){ try { $sp=$line.Split(' ',3); switch($sp[0]){ 'PLAY' { $u=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($sp[1])); $v=[double]::Parse($sp[2],[Globalization.CultureInfo]::InvariantCulture); $p.Stop(); $p.Close(); $p.Open([Uri]$u); $p.Volume=$v; $p.Play() } 'PAUSE' { $p.Pause() } 'RESUME' { $p.Play() } 'STOP' { $p.Stop(); $p.Close() } 'VOLUME' { $p.Volume=[double]::Parse($sp[1],[Globalization.CultureInfo]::InvariantCulture) } default { throw 'unknown command' } }; [Console]::Out.WriteLine('OK'); [Console]::Out.Flush() } catch { [Console]::Out.WriteLine('ERR'); [Console]::Out.Flush() } }`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		_ = in.Close()
		return err
	}
	cmd.Stderr = nil
	if err = cmd.Start(); err != nil {
		_ = in.Close()
		return err
	}
	ack := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			select {
			case ack <- strings.TrimSpace(scanner.Text()):
			case <-app.done:
				close(ack)
				return
			}
		}
		close(ack)
	}()
	app.audioCmd = cmd
	app.audioIn = in
	app.audioAck = ack
	go func(c *exec.Cmd) {
		_ = c.Wait()
		app.audioMu.Lock()
		sameEngine := app.audioCmd == c
		if sameEngine {
			app.audioCmd = nil
			app.audioIn = nil
			app.audioAck = nil
		}
		app.audioMu.Unlock()
		if !sameEngine || shuttingDown() {
			return
		}
		app.mu.Lock()
		wasPlaying := app.playing
		idx := currentStationIndexLocked()
		key := app.currentKey
		if key == "" && idx >= 0 && idx < len(app.stations) {
			key = stationKey(app.stations[idx])
		}
		now := time.Now()
		allowRecover := wasPlaying && idx >= 0 && !app.audioRecovering && (app.lastAudioFailure.IsZero() || now.Sub(app.lastAudioFailure) > 20*time.Second)
		app.playing = false
		if allowRecover {
			app.audioRecovering = true
			app.lastAudioFailure = now
		}
		app.mu.Unlock()
		if !wasPlaying {
			return
		}
		if !allowRecover {
			setStatus("Reprodukcija je prekinuta · klikni Play za ponovni pokušaj")
			postUI()
			return
		}
		setStatus("Reprodukcija je prekinuta · ponovno povezujem…")
		postUI()
		safeGo("audio-recovery", func() {
			timer := time.NewTimer(1200 * time.Millisecond)
			select {
			case <-timer.C:
			case <-app.done:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
			app.mu.Lock()
			app.audioRecovering = false
			app.mu.Unlock()
			playStationByKey(key, idx)
		})
	}(cmd)
	return nil
}
func waitAudioAckLocked() error {
	if app.audioAck == nil {
		return errors.New("audio engine nema kanal potvrde")
	}
	select {
	case reply, ok := <-app.audioAck:
		if !ok {
			return errors.New("audio engine je prekinut")
		}
		if reply != "OK" {
			return errors.New("audio engine nije prihvatio naredbu")
		}
		return nil
	case <-time.After(1500 * time.Millisecond):
		return errors.New("audio engine nije odgovorio na vrijeme")
	case <-app.done:
		return context.Canceled
	}
}

func audioSend(line string) error {
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	if err := startAudioEngineLocked(); err != nil {
		return err
	}
	if app.audioIn == nil {
		return errors.New("audio engine nije dostupan")
	}
	if _, err := io.WriteString(app.audioIn, line+"\n"); err != nil {
		return err
	}
	return waitAudioAckLocked()
}
func audioSendExisting(line string) error {
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	if app.audioIn == nil {
		return errors.New("audio engine nije pokrenut")
	}
	if _, err := io.WriteString(app.audioIn, line+"\n"); err != nil {
		return err
	}
	return waitAudioAckLocked()
}
func audioShutdown() {
	app.audioMu.Lock()
	in := app.audioIn
	cmd := app.audioCmd
	app.audioIn = nil
	app.audioCmd = nil
	app.audioAck = nil
	app.audioMu.Unlock()
	if in != nil {
		_, _ = io.WriteString(in, "STOP\n")
		_ = in.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	audioStopMCI()
}

func audioPlay(raw string) error {
	app.stateMu.RLock()
	volume := app.state.Volume
	app.stateMu.RUnlock()
	vol := float64(volume) / 100.0
	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}
	enc := base64.StdEncoding.EncodeToString([]byte(raw))
	if err := audioSend(fmt.Sprintf("PLAY %s %.2f", enc, vol)); err == nil {
		return nil
	} else {
		logError("audio-engine", err)
	}
	audioStopMCI()
	cmd := fmt.Sprintf("open \"%s\" type mpegvideo alias radio", strings.ReplaceAll(raw, "\"", ""))
	if e := mci(cmd); e != nil {
		return e
	}
	if e := mci("play radio"); e != nil {
		audioStopMCI()
		return e
	}
	_ = mci(fmt.Sprintf("setaudio radio volume to %d", volume*10))
	return nil
}
func audioPause() error {
	if err := audioSendExisting("PAUSE"); err == nil {
		return nil
	}
	return mci("pause radio")
}
func audioResume() error {
	if err := audioSendExisting("RESUME"); err == nil {
		return nil
	}
	return mci("resume radio")
}
func audioSetVolume(v int) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	if audioSendExisting(fmt.Sprintf("VOLUME %.2f", float64(v)/100.0)) == nil {
		return
	}
	_ = mci(fmt.Sprintf("setaudio radio volume to %d", v*10))
}
func audioStop()    { _ = audioSendExisting("STOP"); audioStopMCI() }
func audioStopMCI() { _ = mci("stop radio"); _ = mci("close radio") }
func mci(cmd string) error {
	buf := make([]uint16, 256)
	r, _, _ := procMciSendString.Call(uintptr(unsafe.Pointer(u16(cmd))), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if r != 0 {
		return fmt.Errorf("MCI greška %d", r)
	}
	return nil
}

func shellOpen(target string) {
	if !safeHTTPURL(target) {
		setStatus("Web adresa nije dostupna ili nije sigurna")
		postUI()
		return
	}
	r, _, _ := procShellExecute.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(target))), 0, 0, SW_SHOWNORMAL)
	if r <= 32 {
		setStatus("Nije moguće otvoriti web stranicu")
		postUI()
	}
}
func copyClipboard(s string) error {
	if r, _, _ := procOpenClipboard.Call(uintptr(app.hwnd)); r == 0 {
		return errors.New("clipboard")
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	data := syscall.StringToUTF16(s)
	size := uintptr(len(data) * 2)
	h, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if h == 0 {
		return errors.New("alloc")
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return errors.New("lock")
	}
	procCopyMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), size)
	procGlobalUnlock.Call(h)
	if r, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, h); r == 0 {
		procGlobalFree.Call(h)
		return errors.New("set")
	}
	return nil
}

func getWindowText(hwnd syscall.Handle) string {
	if hwnd == 0 {
		return ""
	}
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n >= uintptr(len(buf)) {
		n = uintptr(len(buf) - 1)
	}
	return syscall.UTF16ToString(buf[:int(n)])
}
func invalidate() {
	if app.hwnd != 0 && !shuttingDown() {
		procInvalidateRect.Call(uintptr(app.hwnd), 0, 1)
	}
}
func postUI() {
	if app.hwnd != 0 && !shuttingDown() {
		procPostMessage.Call(uintptr(app.hwnd), WM_APP+1, 0, 0)
	}
}
func postGenres() {
	if app.hwnd != 0 && !shuttingDown() {
		procPostMessage.Call(uintptr(app.hwnd), WM_APP+2, 0, 0)
	}
}
func postFilterUI() {
	if app.hwnd != 0 && !shuttingDown() {
		procPostMessage.Call(uintptr(app.hwnd), WM_APP+3, 0, 0)
	}
}
func messageBox(hwnd syscall.Handle, title, msg string, flags uintptr) int {
	r, _, _ := procMessageBox.Call(uintptr(hwnd), uintptr(unsafe.Pointer(u16(msg))), uintptr(unsafe.Pointer(u16(title))), flags|MB_OK)
	return int(r)
}

func appContext() context.Context {
	if app.ctx != nil {
		return app.ctx
	}
	return context.Background()
}

func shuttingDown() bool {
	if app.done == nil {
		return false
	}
	select {
	case <-app.done:
		return true
	default:
		return false
	}
}

func signalShutdown() {
	app.closeOnce.Do(func() {
		if app.cancel != nil {
			app.cancel()
		}
		if app.done != nil {
			close(app.done)
		}
		app.saveMu.Lock()
		if app.saveTimer != nil {
			app.saveTimer.Stop()
		}
		app.saveMu.Unlock()
		app.mu.Lock()
		if app.searchTimer != nil {
			app.searchTimer.Stop()
		}
		app.mu.Unlock()
	})
}

func prepareShutdown() {
	app.shutdownOnce.Do(func() {
		signalShutdown()
		saveState()
		audioShutdown()
	})
}

func scheduleSearchFilter() {
	app.mu.Lock()
	app.searchSeq++
	seq := app.searchSeq
	if app.searchTimer != nil {
		app.searchTimer.Stop()
	}
	app.searchTimer = time.AfterFunc(140*time.Millisecond, func() {
		if shuttingDown() {
			return
		}
		app.mu.RLock()
		current := app.searchSeq
		app.mu.RUnlock()
		if current == seq && app.hwnd != 0 {
			procPostMessage.Call(uintptr(app.hwnd), WM_APP+3, 0, 0)
		}
	})
	app.mu.Unlock()
}

func queueAlert(title, text string, flags uintptr) {
	if shuttingDown() {
		return
	}
	app.alertMu.Lock()
	if len(app.alerts) >= 8 {
		app.alerts = app.alerts[1:]
	}
	app.alerts = append(app.alerts, UIAlert{Title: title, Text: text, Flags: flags})
	app.alertMu.Unlock()
	if app.hwnd != 0 {
		procPostMessage.Call(uintptr(app.hwnd), WM_APP+4, 0, 0)
	}
}

func showNextAlert() {
	app.alertMu.Lock()
	if len(app.alerts) == 0 {
		app.alertMu.Unlock()
		return
	}
	a := app.alerts[0]
	app.alerts = app.alerts[1:]
	more := len(app.alerts) > 0
	app.alertMu.Unlock()
	messageBox(app.hwnd, a.Title, a.Text, a.Flags)
	if more && app.hwnd != 0 {
		procPostMessage.Call(uintptr(app.hwnd), WM_APP+4, 0, 0)
	}
}

func stateBaseDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return base
}
func stateDir() string       { return filepath.Join(stateBaseDir(), "RadioBalkan") }
func legacyStateDir() string { return filepath.Join(stateBaseDir(), "RadioHrvatska") }
func migrateLegacyDataDir() {
	newDir, oldDir := stateDir(), legacyStateDir()
	if newDir == oldDir {
		return
	}
	if _, err := os.Stat(newDir); err == nil {
		return
	}
	if _, err := os.Stat(oldDir); err != nil {
		return
	}
	if err := os.Rename(oldDir, newDir); err == nil {
		return
	}
	_ = os.MkdirAll(newDir, 0755)
	for _, name := range []string{"state.json", "state.json.bak", "stations-cache.json", "stations-cache.json.bak"} {
		b, err := os.ReadFile(filepath.Join(oldDir, name))
		if err == nil {
			_ = os.WriteFile(filepath.Join(newDir, name), b, 0644)
		}
	}
}
func statePath() string       { return filepath.Join(stateDir(), "state.json") }
func cachePath() string       { return filepath.Join(stateDir(), "stations-cache.json") }
func cacheBackupPath() string { return cachePath() + ".bak" }

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
func loadState() (PersistedState, bool) {
	paths := []string{statePath(), statePath() + ".bak"}
	for i, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var st PersistedState
		if err = json.Unmarshal(b, &st); err == nil {
			if i == 1 {
				logError("load-state", errors.New("primarna state datoteka nije bila dostupna; učitan backup"))
			}
			return st, true
		}
		if i == 0 {
			_ = os.MkdirAll(stateDir(), 0755)
			corrupt := statePath() + ".corrupt-" + time.Now().Format("20060102-150405")
			_ = os.Rename(statePath(), corrupt)
			logError("load-state", err)
		}
	}
	return PersistedState{}, false
}

func scheduleStateSave() {
	if shuttingDown() {
		return
	}
	app.saveMu.Lock()
	if app.saveTimer != nil {
		app.saveTimer.Stop()
	}
	app.saveTimer = time.AfterFunc(750*time.Millisecond, func() {
		defer func() {
			if r := recover(); r != nil {
				logError("state-save-timer", fmt.Errorf("panic: %v", r))
			}
		}()
		if shuttingDown() {
			return
		}
		saveState()
	})
	app.saveMu.Unlock()
}

func saveState() {
	app.stateFileMu.Lock()
	defer app.stateFileMu.Unlock()
	app.stateMu.RLock()
	b, err := json.MarshalIndent(app.state, "", "  ")
	app.stateMu.RUnlock()
	if err != nil {
		logError("save-state-marshal", err)
		return
	}
	if err = os.MkdirAll(stateDir(), 0755); err != nil {
		logError("save-state-dir", err)
		return
	}
	tmp := statePath() + ".tmp"
	bak := statePath() + ".bak"
	if err = writeFileDurable(tmp, b, 0644); err != nil {
		_ = os.Remove(tmp)
		logError("save-state-write", err)
		return
	}
	_ = os.Remove(bak)
	hadPrimary := false
	if _, statErr := os.Stat(statePath()); statErr == nil {
		if err = os.Rename(statePath(), bak); err != nil {
			_ = os.Remove(tmp)
			logError("save-state-backup", err)
			return
		}
		hadPrimary = true
	}
	if err = os.Rename(tmp, statePath()); err != nil {
		if hadPrimary {
			_ = os.Rename(bak, statePath())
		}
		_ = os.Remove(tmp)
		logError("save-state-commit", err)
		return
	}
}

func saveCache(list []RadioStation) {
	list = trimStationCatalog(dedupeStations(filterSupportedStations(list)), 7000)
	app.cacheFileMu.Lock()
	defer app.cacheFileMu.Unlock()
	if err := os.MkdirAll(stateDir(), 0755); err != nil {
		logError("cache-dir", err)
		return
	}
	tmp := cachePath() + ".tmp"
	bak := cacheBackupPath()
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logError("cache-write", err)
		return
	}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	err = enc.Encode(CacheFile{time.Now(), list})
	if syncErr := f.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		logError("cache-write", err)
		return
	}
	_ = os.Remove(bak)
	hadPrimary := false
	if _, statErr := os.Stat(cachePath()); statErr == nil {
		if err = os.Rename(cachePath(), bak); err != nil {
			_ = os.Remove(tmp)
			logError("cache-backup", err)
			return
		}
		hadPrimary = true
	}
	if err = os.Rename(tmp, cachePath()); err != nil {
		if hadPrimary {
			_ = os.Rename(bak, cachePath())
		}
		_ = os.Remove(tmp)
		logError("cache-commit", err)
	}
}

func loadCache() []RadioStation {
	app.cacheFileMu.Lock()
	defer app.cacheFileMu.Unlock()
	for i, path := range []string{cachePath(), cacheBackupPath()} {
		if info, statErr := os.Stat(path); statErr == nil && info.Size() > 32<<20 {
			logError("load-cache", fmt.Errorf("spremljeni katalog je prevelik: %d B", info.Size()))
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		var c CacheFile
		dec := json.NewDecoder(io.LimitReader(f, 32<<20))
		err = dec.Decode(&c)
		_ = f.Close()
		if err != nil {
			if i == 0 {
				corrupt := cachePath() + ".corrupt-" + time.Now().Format("20060102-150405")
				_ = os.Rename(cachePath(), corrupt)
			}
			logError("load-cache", err)
			continue
		}
		if len(c.Stations) == 0 {
			continue
		}
		if i == 1 {
			logError("load-cache", errors.New("primarni katalog nije bio dostupan; učitan backup"))
		}
		return trimStationCatalog(dedupeStations(filterSupportedStations(c.Stations)), 7000)
	}
	return nil
}
func ensureInputDialogClass() error {
	inputDialogClassOnce.Do(func() {
		hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
		cur, _, _ := procLoadCursor.Call(0, IDC_ARROW)
		wc := WNDCLASS{
			LpfnWndProc:   syscall.NewCallback(inputDialogWndProc),
			HInstance:     syscall.Handle(hInst),
			HIcon:         app.hIconSmall,
			HCursor:       syscall.Handle(cur),
			HbrBackground: app.panelBrush,
			LpszClassName: u16(inputDialogClassName),
		}
		if r, _, e := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			inputDialogClassErr = fmt.Errorf("RegisterClassW input dialog: %v", e)
		}
	})
	return inputDialogClassErr
}

func inputDialogWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			logError("input-dialog-wndproc", fmt.Errorf("panic msg=0x%X: %v\n%s", msg, r, debug.Stack()))
			ret = 0
		}
	}()
	switch msg {
	case WM_COMMAND:
		id := loWord(wParam)
		if id == 2101 {
			finishInputDialog(true)
			return 0
		}
		if id == 2102 {
			finishInputDialog(false)
			return 0
		}
	case WM_CLOSE:
		finishInputDialog(false)
		return 0
	case WM_DESTROY:
		if activeInputDialog != nil {
			activeInputDialog.done = true
		}
		return 0
	case WM_CTLCOLORSTATIC, WM_CTLCOLOREDIT:
		hdc := syscall.Handle(wParam)
		procSetTextColor.Call(uintptr(hdc), rgb(240, 236, 232))
		procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		if msg == WM_CTLCOLOREDIT {
			return uintptr(app.editBrush)
		}
		return uintptr(app.panelBrush)
	}
	r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func finishInputDialog(ok bool) {
	st := activeInputDialog
	if st == nil || st.done {
		return
	}
	if ok && st.edit != 0 {
		st.result = strings.TrimSpace(getWindowText(st.edit))
		st.ok = st.result != ""
	} else {
		st.ok = false
	}
	st.done = true
	if st.hwnd != 0 {
		procDestroyWindow.Call(uintptr(st.hwnd))
	}
}

func inputDialog(parent syscall.Handle, title, prompt, def string) (string, bool) {
	if err := ensureInputDialogClass(); err != nil {
		logError("input-dialog-class", err)
		return inputDialogPowerShell(title, prompt, def)
	}
	if activeInputDialog != nil {
		return "", false
	}
	st := &inputDialogState{}
	activeInputDialog = st
	defer func() { activeInputDialog = nil }()

	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	x, y := int32(360), int32(220)
	if parent != 0 {
		var pr RECT
		if r, _, _ := procGetWindowRect.Call(uintptr(parent), uintptr(unsafe.Pointer(&pr))); r != 0 {
			x = pr.Left + (pr.Right-pr.Left-620)/2
			y = pr.Top + (pr.Bottom-pr.Top-285)/2
		}
	}
	h, _, e := procCreateWindowEx.Call(
		WS_EX_DLGMODALFRAME,
		uintptr(unsafe.Pointer(u16(inputDialogClassName))),
		uintptr(unsafe.Pointer(u16(title))),
		WS_POPUP|WS_CAPTION|WS_SYSMENU|WS_VISIBLE,
		uintptr(x), uintptr(y), 620, 285,
		uintptr(parent), 0, hInst, 0,
	)
	if h == 0 {
		logError("input-dialog-create", fmt.Errorf("CreateWindowExW: %v", e))
		return inputDialogPowerShell(title, prompt, def)
	}
	st.hwnd = syscall.Handle(h)
	enableImmersiveDark(st.hwnd)

	label, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("STATIC"))), uintptr(unsafe.Pointer(u16(prompt))), WS_CHILD|WS_VISIBLE, 24, 22, 566, 92, h, 0, hInst, 0)
	edit, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("EDIT"))), uintptr(unsafe.Pointer(u16(def))), WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 24, 122, 566, 34, h, 2201, hInst, 0)
	cancelBtn, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("BUTTON"))), uintptr(unsafe.Pointer(u16("Odustani"))), WS_CHILD|WS_VISIBLE|WS_TABSTOP, 382, 180, 96, 38, h, 2102, hInst, 0)
	okBtn, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("BUTTON"))), uintptr(unsafe.Pointer(u16("Spremi"))), WS_CHILD|WS_VISIBLE|WS_TABSTOP, 488, 180, 102, 38, h, 2101, hInst, 0)
	if edit == 0 || label == 0 || cancelBtn == 0 || okBtn == 0 {
		logError("input-dialog-controls", errors.New("nije moguće izraditi sve kontrole dijaloga"))
		procDestroyWindow.Call(h)
		return inputDialogPowerShell(title, prompt, def)
	}
	st.edit = syscall.Handle(edit)
	for _, ch := range []uintptr{label, edit, cancelBtn, okBtn} {
		procSendMessage.Call(ch, 0x0030, uintptr(app.hFontSmall), 1)
		procSetWindowTheme.Call(ch, uintptr(unsafe.Pointer(u16("DarkMode_CFD"))), 0)
	}
	procSetFocus.Call(edit)
	procEnableWindow.Call(uintptr(parent), 0)
	defer func() {
		if parent != 0 {
			procEnableWindow.Call(uintptr(parent), 1)
			procSetForegroundWindow.Call(uintptr(parent))
		}
	}()

	var m MSG
	for !st.done {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			if int32(r) == 0 {
				procPostQuitMessage.Call(0)
			}
			st.done = true
			break
		}
		if m.Message == WM_KEYDOWN {
			if m.WParam == VK_RETURN {
				finishInputDialog(true)
				continue
			}
			if m.WParam == VK_ESCAPE {
				finishInputDialog(false)
				continue
			}
		}
		if r, _, _ := procIsDialogMessage.Call(h, uintptr(unsafe.Pointer(&m))); r != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	return st.result, st.ok
}

func inputDialogPowerShell(title, prompt, def string) (string, bool) {
	ps := `Add-Type -AssemblyName Microsoft.VisualBasic; $v=[Microsoft.VisualBasic.Interaction]::InputBox($args[0],$args[1],$args[2]); [Console]::OutputEncoding=[Text.Encoding]::UTF8; Write-Output $v`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps, prompt, title, def)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		logError("input-dialog-fallback", err)
		return "", false
	}
	v := strings.TrimSpace(string(bytes.TrimSpace(out)))
	if v == "" {
		return "", false
	}
	return v, true
}

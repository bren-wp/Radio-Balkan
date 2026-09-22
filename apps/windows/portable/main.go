//go:build windows

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
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
	ES_PASSWORD         = 0x0020
	EM_SETCUEBANNER     = 0x1501
	EM_SETSEL           = 0x00B1

	SW_SHOW       = 5
	SW_RESTORE    = 9
	CW_USEDEFAULT = ^uint32(0x7fffffff)

	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_PAINT           = 0x000F
	WM_ERASEBKGND      = 0x0014
	WM_KEYDOWN         = 0x0100
	WM_CHAR            = 0x0102
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

	EN_CHANGE  = 0x0300
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

var appVersion = "0.0.38"

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
	ChangeUUID        string `json:"changeuuid"`
	StationUUID       string `json:"stationuuid"`
	Name              string `json:"name"`
	URL               string `json:"url"`
	URLResolved       string `json:"url_resolved"`
	Homepage          string `json:"homepage"`
	Favicon           string `json:"favicon"`
	Tags              string `json:"tags"`
	Country           string `json:"country"`
	CountryCode       string `json:"countrycode"`
	SourceCountryCode string `json:"sourcecountrycode,omitempty"`
	State             string `json:"state"`
	Language          string `json:"language"`
	Votes             int    `json:"votes"`
	Codec             string `json:"codec"`
	Bitrate           int    `json:"bitrate"`
	LastCheckOK       int    `json:"lastcheckok"`

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
	hitAdmin
	hitStationDetails
	hitStationBack
	hitBrendigo
)

type HitRegion struct {
	R     RECT
	Kind  HitKind
	Index int
	Value string
}

type HomeDiscoveryCache struct {
	Revision     uint64
	Columns      int
	Valid        bool
	CroatiaCount int
	Popular      []int
	Croatia      []int
	Bosnia       []int
	Serbia       []int
	Balkan       []int
	Folk         []int
	PopRock      []int
}

type audioBackendKind uint8

const (
	audioBackendNone audioBackendKind = iota
	audioBackendWPF
	audioBackendMCI
)

type App struct {
	hwnd                                     syscall.Handle
	edit                                     syscall.Handle
	stations                                 []RadioStation
	filtered                                 []int
	hits                                     []HitRegion
	catalogRevision                          uint64
	homeCacheMu                              sync.Mutex
	homeCache                                HomeDiscoveryCache
	hoverToken                               string
	detailKey                                string
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
	adminMode                                bool
	adminFailures                            int
	adminLockedUntil                         time.Time
	searchTimer                              *time.Timer
	searchSeq                                uint64
	scroll                                   int
	clientWidth                              int32
	clientHeight                             int32
	ciPaintSeq                               uint64
	current                                  int
	currentKey                               string
	playing                                  bool
	audioStopped                             bool
	audioBackend                             audioBackendKind
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

const adminPasswordIterations = 120000

var adminPasswordSalt = [16]byte{0xc6, 0xd7, 0x9a, 0xca, 0xaf, 0xb5, 0x2b, 0xb8, 0xba, 0xc2, 0x78, 0x31, 0x3e, 0x84, 0xcc, 0xf7}
var adminPasswordDigest = [32]byte{0x6d, 0x31, 0x9a, 0xde, 0x7c, 0x2c, 0x0f, 0x33, 0x3d, 0x1d, 0x52, 0x0e, 0xaf, 0x58, 0x20, 0x34, 0xa8, 0x0c, 0xab, 0xbc, 0x4e, 0x2f, 0x03, 0x17, 0x52, 0x7b, 0x68, 0x57, 0x6b, 0x63, 0x1f, 0x80}

func deriveAdminPasswordKey(password string) [32]byte {
	var derived [32]byte
	var block [4]byte
	binary.BigEndian.PutUint32(block[:], 1)

	mac := hmac.New(sha256.New, []byte(password))
	_, _ = mac.Write(adminPasswordSalt[:])
	_, _ = mac.Write(block[:])
	u := mac.Sum(nil)
	copy(derived[:], u)

	for iteration := 1; iteration < adminPasswordIterations; iteration++ {
		mac.Reset()
		_, _ = mac.Write(u)
		u = mac.Sum(nil)
		for i := range derived {
			derived[i] ^= u[i]
		}
	}
	return derived
}

func adminCredentialsValid(username, password string) bool {
	if !strings.EqualFold(strings.TrimSpace(username), "brendigo") {
		return false
	}
	derived := deriveAdminPasswordKey(password)
	return subtle.ConstantTimeCompare(derived[:], adminPasswordDigest[:]) == 1
}

func adminModeEnabled() bool {
	app.mu.RLock()
	enabled := app.adminMode
	app.mu.RUnlock()
	return enabled
}

func requireAdmin() bool {
	if adminModeEnabled() {
		return true
	}
	setStatus("Ova opcija dostupna je samo administratoru")
	invalidate()
	return false
}

func logoutAdmin() {
	app.mu.Lock()
	app.adminMode = false
	if app.tab == "replaced" || app.tab == "broken" {
		app.tab = "all"
		app.scroll = 0
	}
	app.mu.Unlock()
	app.stateMu.Lock()
	if app.state.Tab == "replaced" || app.state.Tab == "broken" {
		app.state.Tab = "all"
	}
	app.stateMu.Unlock()
	scheduleStateSave()
	rebuildFilter()
	setStatus("Administrator je odjavljen")
	invalidate()
}

func showAdminLogin() {
	app.mu.RLock()
	lockedUntil := app.adminLockedUntil
	app.mu.RUnlock()
	if time.Now().Before(lockedUntil) {
		remaining := time.Until(lockedUntil).Round(time.Second)
		setStatus("Admin prijava privremeno je zaključana · " + remaining.String())
		invalidate()
		return
	}
	username, ok := inputDialog(app.hwnd, "Admin prijava", "Korisničko ime", "")
	if !ok {
		return
	}
	password, ok := passwordDialog(app.hwnd, "Admin prijava", "Lozinka")
	if !ok {
		return
	}
	if adminCredentialsValid(username, password) {
		app.mu.Lock()
		app.adminMode = true
		app.adminFailures = 0
		app.adminLockedUntil = time.Time{}
		app.mu.Unlock()
		setStatus("Admin način rada · brendigo")
		invalidate()
		return
	}
	app.mu.Lock()
	app.adminFailures++
	if app.adminFailures >= 5 {
		app.adminFailures = 0
		app.adminLockedUntil = time.Now().Add(30 * time.Second)
	}
	locked := !app.adminLockedUntil.IsZero() && time.Now().Before(app.adminLockedUntil)
	app.mu.Unlock()
	if locked {
		setStatus("Previše neuspjelih prijava · pokušaj ponovno za 30 s")
	} else {
		setStatus("Neispravno korisničko ime ili lozinka")
	}
	invalidate()
}

func toggleAdminSession() {
	if adminModeEnabled() {
		logoutAdmin()
		return
	}
	showAdminLogin()
}

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
	{"BG", "Bugarska"},
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
func isSelectableCatalogCode(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	return code == "" || isRegionalCatalogCode(code) || isSupplementalCatalogCode(code)
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

func currentAudioBackend() audioBackendKind {
	app.mu.RLock()
	backend := app.audioBackend
	app.mu.RUnlock()
	return backend
}

func setAudioBackend(backend audioBackendKind) {
	app.mu.Lock()
	app.audioBackend = backend
	app.mu.Unlock()
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
	procMciGetErrorString             = winmm.NewProc("mciGetErrorStringW")
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
func runtimeTestTrace(scope string) {
	if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") != "1" {
		return
	}
	logError("runtime-test", errors.New(scope))
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

func safeHTTPURLForConnection(raw string) bool {
	if !safeHTTPURL(raw) {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return false
	}
	host := canonicalNetworkHost(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		return !unsafeNetworkIP(ip)
	}
	ctx, cancel := context.WithTimeout(appContext(), 1500*time.Millisecond)
	defer cancel()
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(resolved) == 0 {
		return false
	}
	for _, candidate := range resolved {
		if unsafeNetworkIP(candidate.IP) {
			return false
		}
	}
	return true
}

func streamCandidateForPlayback(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if !safeHTTPURLForConnection(raw) {
		return "", false
	}
	if looksPlaylist(raw) {
		resolved, ok := resolvePlaylist(raw)
		if !ok || !safeHTTPURLForConnection(resolved) {
			return "", false
		}
		return resolved, true
	}
	// Radio Browser's url_resolved is already a direct stream URL. Avoid opening
	// a second GET connection before the real media decoder; many Icecast/Shoutcast
	// servers behave differently for probes or limit concurrent listeners per client.
	return raw, true
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
	case "all", "popular", "countries", "genres", "favorites", "recent", "replaced", "broken":
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
	if !loaded {
		st.CountryCode = "HR"
	} else if !isSelectableCatalogCode(st.CountryCode) {
		st.CountryCode = ""
	}
	st.Genre = strings.TrimSpace(st.Genre)
	if len([]rune(st.Genre)) > 40 {
		st.Genre = ""
	}
	if !isValidTab(st.Tab) {
		st.Tab = "all"
	}
	if st.WindowWidth < 1024 || st.WindowWidth > 2600 {
		st.WindowWidth = 1240
	}
	if st.WindowHeight < 680 || st.WindowHeight > 1600 {
		st.WindowHeight = 760
	}
	// Keep restored desktop windows compact. Older builds could persist a
	// maximized/full-screen rectangle and reopen much larger than necessary.
	if st.WindowWidth > 1500 {
		st.WindowWidth = 1420
	}
	if st.WindowHeight > 900 {
		st.WindowHeight = 860
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
	if w < 1024 || w > 2600 || h < 680 || h > 1600 {
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
	// A Win32 window and its message queue belong to the OS thread that creates them.
	// Keep createMainWindow, GetMessage and WndProc dispatch on one Windows thread;
	// otherwise Go may migrate this goroutine and leave the UI waiting on the wrong queue.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

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
	if app.tab == "replaced" || app.tab == "broken" {
		app.tab = "all"
		app.state.Tab = "all"
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
	// Initialize audio lazily on the first playback command. A backend startup
	// failure must never delay or block navigation, search, or any UI button.
	if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") == "1" {
		safeGo("ci-runtime-catalog", seedCIRuntimeCatalog)
	} else {
		safeGo("load-stations", loadStations)
	}
	scheduleCIRuntimeSmokeClose()
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

func scheduleCIRuntimeSmokeClose() {
	if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") != "1" {
		return
	}
	enabled := false
	for _, arg := range os.Args[1:] {
		if arg == "--ci-runtime-smoke" {
			enabled = true
			break
		}
	}
	if !enabled {
		return
	}
	safeGo("ci-runtime-smoke-suite", func() {
		runCIInputSmoke()
		runCIAudioSmoke()
	})
	safeGo("ci-runtime-smoke-close", func() {
		timer := time.NewTimer(24 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-app.done:
			return
		}
		if app.hwnd != 0 && !shuttingDown() {
			runtimeTestTrace("ci-smoke-post-wm-close")
			procPostMessage.Call(uintptr(app.hwnd), WM_CLOSE, 0, 0)
			watchdog := time.NewTimer(4 * time.Second)
			defer watchdog.Stop()
			select {
			case <-app.done:
				return
			case <-watchdog.C:
				buf := make([]byte, 512<<10)
				n := runtime.Stack(buf, true)
				logError("runtime-test-stacks", errors.New(string(buf[:n])))
			}
		}
	})
}

func seedCIRuntimeCatalog() {
	if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") != "1" {
		return
	}
	codes := []string{"HR", "BA", "RS", "SI", "MK", "AL", "ME", "BG"}
	list := make([]RadioStation, 0, 6000)
	for i := 0; i < 6000; i++ {
		code := codes[i%len(codes)]
		tags := "pop,regional"
		if i%3 == 0 {
			tags = "folk,narodna"
		}
		list = append(list, RadioStation{
			StationUUID: fmt.Sprintf("ci-station-%05d", i),
			Name:        fmt.Sprintf("CI Radio %05d", i),
			URLResolved: fmt.Sprintf("https://example.com/radio/%05d.mp3", i),
			Country:     countryNameByCode(code),
			CountryCode: code,
			Tags:        tags,
			Votes:       100000 - i,
			LastCheckOK: 1,
		})
	}
	prepareStations(list)
	app.mu.Lock()
	app.stations = list
	app.loading = false
	app.scroll = 0
	app.catalogRevision++
	app.mu.Unlock()
	rebuildGenres()
	rebuildFilter()
	setStatus(fmt.Sprintf("CI katalog spreman · %d stanica", len(list)))
	postUI()
}

func runCIInputSmoke() {
	if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") != "1" {
		return
	}
	token := strings.TrimSpace(os.Getenv("RADIO_BALKAN_RUNTIME_TOKEN"))
	waitFor := func(timeout time.Duration, predicate func() bool) bool {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if predicate() {
				return true
			}
			select {
			case <-time.After(40 * time.Millisecond):
			case <-app.done:
				return false
			}
		}
		return predicate()
	}
	postClick := func(x, y int32) {
		packed := uintptr(uint32(uint16(x)) | uint32(uint16(y))<<16)
		procPostMessage.Call(uintptr(app.hwnd), WM_LBUTTONDOWN, 0, packed)
	}
	forcePaint := func(timeout time.Duration) bool {
		app.mu.RLock()
		before := app.ciPaintSeq
		app.mu.RUnlock()
		procPostMessage.Call(uintptr(app.hwnd), WM_APP+5, 0, 0)
		return waitFor(timeout, func() bool {
			app.mu.RLock()
			done := app.ciPaintSeq > before
			app.mu.RUnlock()
			return done
		})
	}

	if !waitFor(4*time.Second, func() bool {
		app.mu.RLock()
		ready := app.hwnd != 0 && app.clientWidth >= 1024 && app.clientHeight > playerHeight && !app.loading && len(app.stations) >= 6000
		app.mu.RUnlock()
		return ready
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=window-ready")
		return
	}

	// Exercise the real Win32 mouse-message -> hit-region -> state path.
	postClick(100, 153) // Top
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "popular"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=top")
		return
	}

	// Favorite must travel through the actual rendered library-card hit region.
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=favorite-paint")
		return
	}
	app.mu.RLock()
	favoriteIdx := -1
	widthForFavorite := app.clientWidth
	if len(app.filtered) > 0 {
		favoriteIdx = app.filtered[0]
	}
	favoriteKey := ""
	if favoriteIdx >= 0 && favoriteIdx < len(app.stations) {
		favoriteKey = stationKey(app.stations[favoriteIdx])
	}
	app.mu.RUnlock()
	if favoriteKey == "" {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=favorite-station")
		return
	}
	app.stateMu.RLock()
	favoriteBefore := app.state.Favorites[favoriteKey]
	app.stateMu.RUnlock()
	favoriteMainL := sidebarWidth + mainPad
	favoriteMainR := widthForFavorite - mainPad
	favoriteColW := (favoriteMainR - favoriteMainL - 14) / 2
	postClick(favoriteMainL+favoriteColW-83, 167)
	if !waitFor(700*time.Millisecond, func() bool {
		app.stateMu.RLock()
		changed := app.state.Favorites[favoriteKey] != favoriteBefore
		app.stateMu.RUnlock()
		return changed
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=favorite")
		return
	}
	postClick(100, 197) // Zemlje
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "countries"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=countries")
		return
	}
	postClick(100, 241) // Žanrovi
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "genres" && !app.countryMenuOpen && !app.genreMenuOpen
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=genres")
		return
	}
	postClick(100, 109) // Početna
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "all" && app.country == "HR"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=home")
		return
	}

	app.mu.RLock()
	widthForHeader := app.clientWidth
	app.mu.RUnlock()
	_, _, countryL, countryR, genreL, genreR, _, _, _, _ := headerLayout(widthForHeader)
	postClick((countryL+countryR)/2, 46)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "countries" && !app.countryMenuOpen && !app.genreMenuOpen
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=header-countries")
		return
	}
	postClick((genreL+genreR)/2, 46)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "genres" && !app.countryMenuOpen && !app.genreMenuOpen
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=header-genres")
		return
	}
	postClick(100, 109)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "all" && app.country == "HR"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=home-after-header")
		return
	}

	// Resize the actual top-level window, wait for WM_SIZE + repaint, then
	// continue through the real hit regions. This catches stale pre-resize hit
	// boxes and search-control geometry that can make a visually-correct UI dead.
	app.mu.RLock()
	beforeResizeW, beforeResizeH := app.clientWidth, app.clientHeight
	app.mu.RUnlock()
	resized, _, _ := procSetWindowPos.Call(uintptr(app.hwnd), 0, 0, 0, 1100, 720, 0x0016) // NOMOVE|NOZORDER|NOACTIVATE
	if resized == 0 || !waitFor(time.Second, func() bool {
		app.mu.RLock()
		changed := app.clientWidth != beforeResizeW || app.clientHeight != beforeResizeH
		valid := app.clientWidth >= 1024 && app.clientHeight > playerHeight
		app.mu.RUnlock()
		return changed && valid
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=resize")
		return
	}
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=resize-paint")
		return
	}
	postClick(100, 197)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "countries"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=resize-click")
		return
	}
	postClick(100, 109)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "all" && app.country == "HR"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=resize-home")
		return
	}

	// Exercise the native EDIT -> WM_COMMAND/EN_CHANGE path, then ensure sidebar
	// navigation still works while the child search control owns focus.
	if app.edit == 0 {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=search-hwnd")
		return
	}
	procSetFocus.Call(uintptr(app.edit))
	for _, ch := range "CI Radio 00001" {
		procPostMessage.Call(uintptr(app.edit), WM_CHAR, uintptr(ch), 0)
	}
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.search == "CI Radio 00001"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=search-change")
		return
	}
	procSendMessage.Call(uintptr(app.edit), EM_SETSEL, 0, ^uintptr(0))
	procPostMessage.Call(uintptr(app.edit), WM_CHAR, 0x08, 0) // Backspace
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.search == ""
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=search-clear")
		return
	}
	procSetFocus.Call(uintptr(app.edit))
	postClick(100, 153)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "popular"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=search-focus-sidebar")
		return
	}
	postClick(100, 109)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "all" && app.country == "HR"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=home-after-search")
		return
	}

	app.mu.Lock()
	app.genre = "rock"
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.Genre = "rock"
	app.stateMu.Unlock()
	postClick(100, 285) // Dijaspora must also clear any stale genre filter
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.country == diasporaCatalogCode && app.genre == ""
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=diaspora")
		return
	}
	postClick(100, 329) // Strano
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.country == foreignCatalogCode
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=foreign")
		return
	}
	postClick(100, 109)
	if !waitFor(time.Second, func() bool {
		app.mu.RLock()
		ok := app.tab == "all" && app.country == "HR"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=home-after-supplemental")
		return
	}
	// Force the 6000-station home to complete an actual WM_PAINT on the UI
	// thread. The acknowledgement is incremented only after UpdateWindow returns,
	// so this cannot pass before the expensive repaint has really completed.
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=post-load-render")
		return
	}
	postClick(100, 153)
	if !waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		ok := app.tab == "popular"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=post-load-click")
		return
	}
	postClick(100, 109)
	if !waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		ok := app.tab == "all" && app.country == "HR"
		app.mu.RUnlock()
		return ok
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=home-after-render")
		return
	}

	// Exercise the station card itself, its in-app Back action, then the real
	// rendered Play hit-region. None of these may open a secondary window.
	app.mu.RLock()
	widthForCard := app.clientWidth
	beforePlaySeq := app.playSeq
	app.mu.RUnlock()
	mainL := sidebarWidth + mainPad
	mainR := widthForCard - mainPad
	columns := homeGridColumns(mainR - mainL)
	cardW := (mainR - mainL - 8*int32(columns-1)) / int32(columns)
	firstPlayX := mainL + cardW - 22
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=station-card-paint")
		return
	}
	postClick(mainL+80, 200)
	if !waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		opened := app.detailKey != ""
		app.mu.RUnlock()
		return opened
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=station-details")
		return
	}
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=station-details-paint")
		return
	}
	postClick(mainL+70, 110)
	if !waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		closed := app.detailKey == ""
		app.mu.RUnlock()
		return closed
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=station-back")
		return
	}
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=station-play-paint")
		return
	}
	postClick(firstPlayX, 200)
	if !waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		advanced := app.playSeq > beforePlaySeq
		app.mu.RUnlock()
		return advanced
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=station-play")
		return
	}
	// Cancel the synthetic internet request before the independent local-WAV
	// audio smoke starts; request-aware playback must observe this sequence bump.
	app.mu.Lock()
	app.playSeq++
	if len(app.stations) == 0 {
		app.mu.Unlock()
		runtimeTestTrace("input-smoke-fail token=" + token + " step=stop-station")
		return
	}
	app.current = 0
	app.currentKey = stationKey(app.stations[0])
	app.playing = true
	app.audioStopped = false
	beforeStopSeq := app.playSeq
	app.audioBackend = audioBackendNone
	app.mu.Unlock()
	if !forcePaint(700 * time.Millisecond) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=stop-paint")
		return
	}
	app.mu.RLock()
	stopWidth, stopHeight := app.clientWidth, app.clientHeight
	app.mu.RUnlock()
	stopCX := playerTransportCenter(stopWidth)
	postClick(stopCX+68, stopHeight-playerHeight+45)
	if !waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		stopped := app.audioStopped && !app.playing && app.playSeq > beforeStopSeq
		app.mu.RUnlock()
		return stopped
	}) {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=player-stop")
		return
	}

	// Reproduce the production failure mode deliberately: hold the backend lock,
	// click volume, then click navigation. Neither click may block the UI thread.
	app.audioMu.Lock()
	app.mu.Lock()
	app.audioBackend = audioBackendWPF
	app.mu.Unlock()
	app.stateMu.RLock()
	beforeVolume := app.state.Volume
	app.stateMu.RUnlock()
	app.mu.RLock()
	width, height := app.clientWidth, app.clientHeight
	app.mu.RUnlock()
	postClick(width-90, height-playerHeight+43) // volume +
	volumeChanged := waitFor(700*time.Millisecond, func() bool {
		app.stateMu.RLock()
		changed := app.state.Volume != beforeVolume
		app.stateMu.RUnlock()
		return changed
	})
	postClick(100, 197) // navigation must still work while audioMu is held
	navigationChanged := waitFor(700*time.Millisecond, func() bool {
		app.mu.RLock()
		ok := app.tab == "countries"
		app.mu.RUnlock()
		return ok
	})
	app.audioMu.Unlock()
	time.Sleep(120 * time.Millisecond)
	setAudioBackend(audioBackendNone)
	if !volumeChanged {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=volume-blocked")
		return
	}
	if !navigationChanged {
		runtimeTestTrace("input-smoke-fail token=" + token + " step=navigation-blocked")
		return
	}
	runtimeTestTrace("input-smoke-ok token=" + token)
}

func runCIAudioSmoke() {
	if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") != "1" {
		return
	}
	token := strings.TrimSpace(os.Getenv("RADIO_BALKAN_RUNTIME_TOKEN"))
	wav := makeCISmokeWAV()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logError("runtime-test-audio", err)
		return
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/tone.wav", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(wav)
	})
	server := &http.Server{Handler: mux}
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if serveErr := server.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logError("runtime-test-audio-server", serveErr)
		}
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = server.Shutdown(ctx)
		cancel()
		select {
		case <-serveDone:
		case <-time.After(time.Second):
		}
	}()

	streamURL := "http://" + ln.Addr().String() + "/tone.wav"
	encoded := base64.StdEncoding.EncodeToString([]byte(streamURL))
	if err := audioSend(audioPlayCommand(0, encoded, 0.0)); err != nil {
		if strings.Contains(strings.ToUpper(err.Error()), "0XC00D11BA") {
			runtimeTestTrace("audio-smoke-no-device token=" + token)
			return
		}
		logError("runtime-test-audio", err)
		return
	}
	_ = audioSendExisting("STOP")
	runtimeTestTrace("audio-smoke-ok token=" + token)
}

func makeCISmokeWAV() []byte {
	const sampleRate = 8000
	const seconds = 1
	const channels = 1
	const bitsPerSample = 16
	dataSize := sampleRate * seconds * channels * (bitsPerSample / 8)
	out := make([]byte, 44+dataSize)
	copy(out[0:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(36+dataSize))
	copy(out[8:12], "WAVE")
	copy(out[12:16], "fmt ")
	binary.LittleEndian.PutUint32(out[16:20], 16)
	binary.LittleEndian.PutUint16(out[20:22], 1)
	binary.LittleEndian.PutUint16(out[22:24], channels)
	binary.LittleEndian.PutUint32(out[24:28], sampleRate)
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	binary.LittleEndian.PutUint32(out[28:32], uint32(byteRate))
	blockAlign := channels * (bitsPerSample / 8)
	binary.LittleEndian.PutUint16(out[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:36], bitsPerSample)
	copy(out[36:40], "data")
	binary.LittleEndian.PutUint32(out[40:44], uint32(dataSize))
	return out
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
	if windowW < 1024 {
		windowW = 1240
	}
	if windowH < 680 {
		windowH = 760
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
			info.PtMinTrackSize.X = 1024
			info.PtMinTrackSize.Y = 680
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
		if id == 1001 && code == EN_CHANGE && lParam == uintptr(app.edit) {
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
	case WM_APP + 5:
		if os.Getenv("RADIO_BALKAN_RUNTIME_TEST") == "1" {
			procInvalidateRect.Call(uintptr(hwnd), 0, 1)
			procUpdateWindow.Call(uintptr(hwnd))
			app.mu.Lock()
			app.ciPaintSeq++
			app.mu.Unlock()
		}
		return 0
	case WM_CLOSE:
		runtimeTestTrace("wm-close-enter")
		captureWindowSize()
		runtimeTestTrace("wm-close-before-shutdown")
		prepareShutdown()
		runtimeTestTrace("wm-close-after-shutdown")
		procDestroyWindow.Call(uintptr(hwnd))
		runtimeTestTrace("wm-close-after-destroy-window")
		return 0
	case WM_DESTROY:
		runtimeTestTrace("wm-destroy-enter")
		prepareShutdown()
		runtimeTestTrace("wm-destroy-after-shutdown")
		markCleanShutdown()
		runtimeTestTrace("wm-destroy-before-post-quit")
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
	app.clientWidth = r.Right
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
	text(hdc, "Radio Balkan", 82, 25, sidebarWidth-18, 55, rgb(247, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	admin := adminModeEnabled()
	adminLabel := "Admin prijava"
	adminIcon := "♙"
	adminValue := "login"
	if admin {
		adminLabel = "brendigo · odjava"
		adminIcon = "♛"
		adminValue = "logout"
	}
	selectFont(hdc, app.hFontSmall)
	adminColor := rgb(150, 139, 132)
	if admin {
		adminColor = rgb(255, 177, 55)
	}
	text(hdc, adminIcon+"  "+adminLabel, 82, 52, sidebarWidth-18, 82, adminColor, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	app.hits = append(app.hits, HitRegion{R: RECT{76, 52, sidebarWidth - 14, 84}, Kind: hitAdmin, Index: -1, Value: adminValue})

	app.mu.RLock()
	tab, genre, country := app.tab, strings.ToLower(strings.TrimSpace(app.genre)), strings.ToUpper(strings.TrimSpace(app.country))
	app.mu.RUnlock()
	y := int32(91)
	drawSidebarItem(hdc, y, "⌂", "Početna", tab == "all" && genre == "" && country == "HR", hitTab, "all")
	y += 44
	drawSidebarItem(hdc, y, "★", "Top", tab == "popular", hitTab, "popular")
	y += 44
	drawSidebarItem(hdc, y, "◉", "Zemlje", tab == "countries", hitTab, "countries")
	y += 44
	drawSidebarItem(hdc, y, "♫", "Žanrovi", tab == "genres", hitTab, "genres")
	y += 44
	drawSidebarItem(hdc, y, "◎", "Dijaspora", country == diasporaCatalogCode, hitCountryChoice, diasporaCatalogCode)
	y += 44
	drawSidebarItem(hdc, y, "◌", "Strano", country == foreignCatalogCode, hitCountryChoice, foreignCatalogCode)

	y += 54
	drawSidebarLabel(hdc, "BIBLIOTEKA", y)
	y += 28
	drawSidebarItem(hdc, y, "♡", "Omiljene", tab == "favorites", hitTab, "favorites")
	y += 42
	drawSidebarItem(hdc, y, "◷", "Nedavno", tab == "recent", hitTab, "recent")

	toolsY := y + 60
	if admin && cr.Bottom-playerHeight > toolsY+110 {
		drawSidebarLabel(hdc, "UPRAVLJANJE", toolsY)
		toolsY += 28
		drawSidebarItem(hdc, toolsY, "⇄", "Rezervni izvori", tab == "replaced", hitTab, "replaced")
		toolsY += 42
		drawSidebarItem(hdc, toolsY, "!", "Nedostupne", tab == "broken", hitTab, "broken")
	}

	// Warm footer quote remains decorative and may collapse on compact heights.
	quoteTop := cr.Bottom - playerHeight - 118
	if quoteTop > y+28 {
		drawRounded(hdc, 14, quoteTop, sidebarWidth-14, quoteTop+92, 16, color(28, 24, 22), color(54, 43, 37))
		selectFont(hdc, app.hFontSmall)
		text(hdc, "Isti ljudi. Ista glazba.\nBliži nego ikad.", 28, quoteTop+18, sidebarWidth-28, quoteTop+68, rgb(235, 171, 91), DT_LEFT|DT_WORDBREAK)
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
	countryCode, genre, tab := app.country, app.genre, app.tab
	app.mu.RUnlock()
	countryLabel := countryNameByCode(countryCode)
	if countryCode == "" || countryLabel == "" {
		countryLabel = "Sve zemlje"
	}
	countryButton := "Zemlje"
	if countryCode != "" && tab != "countries" {
		countryButton = "Zemlje · " + countryLabel
	}
	genreButton := "Žanrovi"
	if label := genreDisplayName(genre); label != "" && tab != "genres" {
		genreButton = "Žanrovi · " + label
	}
	drawBrowseButton(hdc, countryL, 20, countryR, 72, countryButton, tab == "countries")
	drawBrowseButton(hdc, genreL, 20, genreR, 72, genreButton, tab == "genres")
	app.hits = append(app.hits, HitRegion{R: RECT{countryL, 20, countryR, 72}, Kind: hitTab, Index: -1, Value: "countries"})
	app.hits = append(app.hits, HitRegion{R: RECT{genreL, 20, genreR, 72}, Kind: hitTab, Index: -1, Value: "genres"})
	if adminModeEnabled() {
		app.mu.RLock()
		healthRunning := app.healthRunning
		app.mu.RUnlock()
		checkLabel := "✓ Sve"
		if healthRunning {
			checkLabel = "…"
		}
		drawIconButton(hdc, checkL, 20, checkR, 72, checkLabel, healthRunning)
		app.hits = append(app.hits, HitRegion{R: RECT{checkL, 20, checkR, 72}, Kind: hitCheckAll, Index: -1})
	}
	drawIconButton(hdc, refreshL, 20, refreshR, 72, "↻", false)
	app.hits = append(app.hits, HitRegion{R: RECT{refreshL, 20, refreshR, 72}, Kind: hitRefresh, Index: -1})
}

func drawBrowseButton(hdc syscall.Handle, l, t, r, b int32, label string, selected bool) {
	fill, border := color(17, 25, 35), color(43, 53, 66)
	tc := rgb(225, 229, 234)
	if selected {
		fill, border, tc = color(58, 39, 23), color(132, 87, 38), rgb(255, 190, 92)
	}
	drawRounded(hdc, l, t, r, b, 13, fill, border)
	selectFont(hdc, app.hFontSmall)
	text(hdc, label, l+14, t, r-14, b, tc, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
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

type browseGenreDef struct {
	Value string
	Label string
}

var browseGenres = []browseGenreDef{
	{"", "Svi žanrovi"},
	{"domaca", "Domaća / regionalna"},
	{"pop", "Pop & Rock"},
	{"folk", "Narodna / Folk"},
	{"electronic", "Elektronička"},
	{"jazz", "Jazz"},
	{"classical", "Klasična"},
	{"news", "Vijesti & Talk"},
	{"hits", "Hits / Top 40"},
	{"oldies", "Oldies"},
}

func browseGridColumns(width int32) int {
	switch {
	case width >= 1080:
		return 4
	case width >= 780:
		return 3
	default:
		return 2
	}
}

func browsePageMaxScroll(clientHeight, contentWidth int32, tab string) int {
	columns := browseGridColumns(contentWidth)
	if columns < 1 {
		columns = 1
	}
	itemCount := 0
	cardHeight := int32(78)
	switch tab {
	case "countries":
		itemCount = len(regionalCountryDefs())
		cardHeight = 82
	case "genres":
		itemCount = len(browseGenres)
	default:
		return 0
	}
	rows := (itemCount + columns - 1) / columns
	if rows <= 0 {
		return 0
	}
	const gap int32 = 12
	contentHeight := int32(rows)*cardHeight + int32(rows-1)*gap
	visibleHeight := clientHeight - playerHeight - 16 - 164
	if visibleHeight < 80 {
		visibleHeight = 80
	}
	max := int(contentHeight - visibleHeight)
	if max < 0 {
		return 0
	}
	return max
}

func drawBrowseScrollBar(hdc syscall.Handle, cr RECT, maxScroll, scroll int) {
	if maxScroll <= 0 {
		return
	}
	top := int32(164)
	bottom := cr.Bottom - playerHeight - 16
	if bottom <= top {
		return
	}
	trackL := cr.Right - 18
	trackR := cr.Right - 12
	drawRounded(hdc, trackL, top, trackR, bottom, 4, color(42, 34, 30), color(42, 34, 30))
	trackH := int(bottom - top)
	thumbH := trackH * trackH / (trackH + maxScroll)
	if thumbH < 34 {
		thumbH = 34
	}
	thumbY := int(top)
	if maxScroll > 0 && trackH > thumbH {
		thumbY += scroll * (trackH - thumbH) / maxScroll
	}
	drawRounded(hdc, trackL, int32(thumbY), trackR, int32(thumbY+thumbH), 4, color(118, 83, 64), color(118, 83, 64))
}

func regionalCountryDefs() []CountryDef {
	items := make([]CountryDef, 0, len(regionalCatalogCodes))
	for _, item := range balkanCountries {
		if isRegionalCatalogCode(item.Code) {
			items = append(items, item)
		}
	}
	return items
}

func genreCountsSnapshot() map[string]int {
	app.mu.RLock()
	defer app.mu.RUnlock()
	counts := make(map[string]int, len(browseGenres))
	counts[""] = len(app.stations)
	for _, station := range app.stations {
		tags := station.TagsIndex
		if tags == "" {
			tags = foldText(station.Tags)
		}
		for _, item := range browseGenres {
			if item.Value != "" && matchesGenre(tags, item.Value) {
				counts[item.Value]++
			}
		}
	}
	return counts
}

func drawCountryBrowsePage(hdc syscall.Handle, cr RECT) {
	mainL := sidebarWidth + mainPad
	mainR := cr.Right - mainPad
	width := mainR - mainL
	columns := browseGridColumns(width)
	items := regionalCountryDefs()
	counts := countryCountsSnapshot()
	app.mu.RLock()
	scroll := app.scroll
	app.mu.RUnlock()
	bottom := cr.Bottom - playerHeight - 16

	selectFont(hdc, app.hFontTitle)
	text(hdc, "Zemlje", mainL, 92, mainR, 126, rgb(248, 249, 251), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, app.hFontSmall)
	text(hdc, "Odaberi zemlju i pregledaj cijeli dostupni katalog radio stanica.", mainL, 122, mainR, 151, rgb(151, 160, 172), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

	gap := int32(12)
	cardH := int32(82)
	cardW := (width - gap*int32(columns-1)) / int32(columns)
	for i, item := range items {
		row, col := i/columns, i%columns
		l := mainL + int32(col)*(cardW+gap)
		t := int32(164-scroll) + int32(row)*(cardH+gap)
		r := l + cardW
		b := t + cardH
		if b < 154 || t > bottom || b > bottom {
			continue
		}
		fill, border := color(16, 23, 31), color(45, 56, 69)
		if hovered(hitCountryChoice, -1, item.Code) {
			fill, border = color(31, 31, 33), color(132, 87, 38)
		}
		drawRounded(hdc, l, t, r, b, 13, fill, border)
		if item.Code == "" {
			selectFont(hdc, app.hFontTitle)
			text(hdc, "◎", l+16, t, l+54, b, rgb(255, 174, 55), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		} else {
			drawCountryFlag(hdc, l+16, t+28, l+48, t+48, item.Code)
		}
		selectFont(hdc, app.hFontBold)
		text(hdc, item.Name, l+62, t+13, r-54, t+43, rgb(241, 244, 247), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		count := counts[strings.ToUpper(item.Code)]
		selectFont(hdc, app.hFontSmall)
		text(hdc, fmt.Sprintf("%d stanica", count), l+62, t+41, r-54, t+68, rgb(145, 155, 168), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		text(hdc, "›", r-40, t, r-14, b, rgb(255, 174, 55), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitTab, Index: -1, Value: "country:" + item.Code})
	}
	drawBrowseScrollBar(hdc, cr, browsePageMaxScroll(cr.Bottom, width, "countries"), scroll)
}

func drawGenreBrowsePage(hdc syscall.Handle, cr RECT) {
	mainL := sidebarWidth + mainPad
	mainR := cr.Right - mainPad
	width := mainR - mainL
	columns := browseGridColumns(width)
	counts := genreCountsSnapshot()
	app.mu.RLock()
	scroll := app.scroll
	app.mu.RUnlock()
	bottom := cr.Bottom - playerHeight - 16

	selectFont(hdc, app.hFontTitle)
	text(hdc, "Žanrovi", mainL, 92, mainR, 126, rgb(248, 249, 251), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, app.hFontSmall)
	text(hdc, "Pregledaj stanice po glazbi i programu bez dodatnih skočnih izbornika.", mainL, 122, mainR, 151, rgb(151, 160, 172), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

	gap := int32(12)
	cardH := int32(78)
	cardW := (width - gap*int32(columns-1)) / int32(columns)
	for i, item := range browseGenres {
		row, col := i/columns, i%columns
		l := mainL + int32(col)*(cardW+gap)
		t := int32(164-scroll) + int32(row)*(cardH+gap)
		r := l + cardW
		b := t + cardH
		if b < 154 || t > bottom || b > bottom {
			continue
		}
		fill, border := color(16, 23, 31), color(45, 56, 69)
		if hovered(hitGenreChoice, -1, item.Value) {
			fill, border = color(31, 31, 33), color(132, 87, 38)
		}
		drawRounded(hdc, l, t, r, b, 13, fill, border)
		selectFont(hdc, app.hFontTitle)
		text(hdc, "♫", l+14, t, l+54, b, rgb(255, 174, 55), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		selectFont(hdc, app.hFontBold)
		text(hdc, item.Label, l+62, t+10, r-52, t+40, rgb(241, 244, 247), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		selectFont(hdc, app.hFontSmall)
		text(hdc, fmt.Sprintf("%d stanica", counts[item.Value]), l+62, t+38, r-52, t+65, rgb(145, 155, 168), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		text(hdc, "›", r-40, t, r-14, b, rgb(255, 174, 55), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitTab, Index: -1, Value: "genre:" + item.Value})
	}
	drawBrowseScrollBar(hdc, cr, browsePageMaxScroll(cr.Bottom, width, "genres"), scroll)
}

func rankedStationIndices(stations []RadioStation, limit int, excluded map[int]struct{}, predicate func(RadioStation) bool) []int {
	if limit <= 0 {
		return nil
	}
	best := make([]int, 0, limit)
	better := func(left, right RadioStation) bool {
		if left.Votes != right.Votes {
			return left.Votes > right.Votes
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	}
	for idx, station := range stations {
		if _, skip := excluded[idx]; skip {
			continue
		}
		if predicate != nil && !predicate(station) {
			continue
		}
		insert := len(best)
		for pos, current := range best {
			if better(station, stations[current]) {
				insert = pos
				break
			}
		}
		if insert >= limit {
			continue
		}
		best = append(best, 0)
		copy(best[insert+1:], best[insert:])
		best[insert] = idx
		if len(best) > limit {
			best = best[:limit]
		}
	}
	return best
}

func buildHomeDiscoverySnapshot(stations []RadioStation, columns int, revision uint64) HomeDiscoveryCache {
	if columns < 3 {
		columns = 3
	}
	if columns > 6 {
		columns = 6
	}
	out := HomeDiscoveryCache{Revision: revision, Columns: columns, Valid: true}
	for _, station := range stations {
		if strings.EqualFold(strings.TrimSpace(station.CountryCode), "HR") {
			out.CroatiaCount++
		}
	}
	excluded := make(map[int]struct{}, columns*10)
	out.Popular = rankedStationIndices(stations, columns, excluded, func(st RadioStation) bool {
		return strings.EqualFold(strings.TrimSpace(st.CountryCode), "HR")
	})
	addExcluded(excluded, out.Popular)
	out.Croatia = rankedStationIndices(stations, columns*3, excluded, func(st RadioStation) bool {
		return strings.EqualFold(strings.TrimSpace(st.CountryCode), "HR")
	})
	addExcluded(excluded, out.Croatia)
	out.Bosnia = rankedStationIndices(stations, columns, excluded, func(st RadioStation) bool {
		return strings.EqualFold(strings.TrimSpace(st.CountryCode), "BA")
	})
	addExcluded(excluded, out.Bosnia)
	out.Serbia = rankedStationIndices(stations, columns, excluded, func(st RadioStation) bool {
		return strings.EqualFold(strings.TrimSpace(st.CountryCode), "RS")
	})
	addExcluded(excluded, out.Serbia)
	out.Balkan = rankedStationIndices(stations, columns*2, excluded, func(st RadioStation) bool {
		code := strings.ToUpper(strings.TrimSpace(st.CountryCode))
		return code != "HR" && code != "BA" && code != "RS" && isRegionalCatalogCode(code)
	})
	addExcluded(excluded, out.Balkan)
	out.Folk = rankedStationIndices(stations, columns, excluded, func(st RadioStation) bool {
		tags := st.TagsIndex
		if tags == "" {
			tags = foldText(st.Tags)
		}
		return isRegionalCatalogCode(st.CountryCode) && matchesGenre(tags, "folk")
	})
	addExcluded(excluded, out.Folk)
	out.PopRock = rankedStationIndices(stations, columns, excluded, func(st RadioStation) bool {
		tags := st.TagsIndex
		if tags == "" {
			tags = foldText(st.Tags)
		}
		return isRegionalCatalogCode(st.CountryCode) && matchesGenre(tags, "pop")
	})
	return out
}

func cachedHomeDiscovery(columns int) HomeDiscoveryCache {
	if columns < 3 {
		columns = 3
	}
	if columns > 6 {
		columns = 6
	}
	for attempt := 0; attempt < 2; attempt++ {
		app.mu.RLock()
		revision := app.catalogRevision
		app.mu.RUnlock()
		app.homeCacheMu.Lock()
		if app.homeCache.Valid && app.homeCache.Revision == revision && app.homeCache.Columns == columns {
			cached := app.homeCache
			app.homeCacheMu.Unlock()
			return cached
		}
		app.homeCacheMu.Unlock()

		app.mu.RLock()
		revision = app.catalogRevision
		stations := append([]RadioStation(nil), app.stations...)
		app.mu.RUnlock()
		built := buildHomeDiscoverySnapshot(stations, columns, revision)
		app.mu.RLock()
		stillCurrent := app.catalogRevision == revision
		app.mu.RUnlock()
		if !stillCurrent {
			continue
		}
		app.homeCacheMu.Lock()
		app.homeCache = built
		app.homeCacheMu.Unlock()
		return built
	}
	return HomeDiscoveryCache{Columns: columns}
}

func discoveryStations(limit int, excluded map[int]struct{}, predicate func(RadioStation) bool) []int {
	if limit <= 0 {
		return nil
	}
	app.mu.RLock()
	items := make([]int, 0, limit*3)
	for idx, station := range app.stations {
		if _, skip := excluded[idx]; skip {
			continue
		}
		if predicate != nil && !predicate(station) {
			continue
		}
		items = append(items, idx)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := app.stations[items[i]], app.stations[items[j]]
		if left.Votes != right.Votes {
			return left.Votes > right.Votes
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	app.mu.RUnlock()
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func addExcluded(excluded map[int]struct{}, ids []int) {
	for _, idx := range ids {
		excluded[idx] = struct{}{}
	}
}

func homeCatalogContentHeight() int {
	// Compact intro plus country-led discovery sections with ten dense card rows.
	// Keep this in sync with drawStations so the final rows remain reachable
	// even at the minimum supported client height.
	return 980
}

func homeCatalogEnabled(clientHeight int32, tab, search, genre, country string) bool {
	return clientHeight >= 600 &&
		tab == "all" &&
		strings.TrimSpace(search) == "" &&
		strings.TrimSpace(genre) == "" &&
		strings.EqualFold(strings.TrimSpace(country), "HR")
}

func shouldShowPopular() bool {
	app.mu.RLock()
	defer app.mu.RUnlock()
	// WM_GETMINMAXINFO constrains the outer window, while clientHeight excludes
	// title-bar/frame chrome. Keep this threshold below the outer 680px minimum
	// and keep the catalog content above the persistent player.
	return homeCatalogEnabled(app.clientHeight, app.tab, app.search, app.genre, app.country)
}

func homeGridColumns(width int32) int {
	switch {
	case width >= 1380:
		return 6
	case width >= 1120:
		return 5
	case width >= 840:
		return 4
	default:
		return 3
	}
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
	app.mu.RLock()
	detailOpen := strings.TrimSpace(app.detailKey) != ""
	tab := app.tab
	app.mu.RUnlock()
	if detailOpen {
		drawStationDetailPage(hdc, cr)
		return
	}
	if tab == "countries" {
		drawCountryBrowsePage(hdc, cr)
		return
	}
	if tab == "genres" {
		drawGenreBrowsePage(hdc, cr)
		return
	}

	mainL := sidebarWidth + mainPad
	mainR := cr.Right - mainPad
	bottom := cr.Bottom - playerHeight - 16
	showHome := shouldShowPopular()

	if showHome {
		width := mainR - mainL
		columns := homeGridColumns(width)
		home := cachedHomeDiscovery(columns)
		popular := home.Popular
		croatia := home.Croatia
		bosnia := home.Bosnia
		serbia := home.Serbia
		balkan := home.Balkan
		folk := home.Folk
		popRock := home.PopRock
		croatiaCount := home.CroatiaCount

		app.mu.RLock()
		scroll := app.scroll
		app.mu.RUnlock()

		contentTop := int32(92 - scroll)
		if contentTop >= 92 && contentTop+46 <= bottom {
			drawRounded(hdc, mainL, contentTop, mainR, contentTop+46, 14, color(17, 23, 31), color(63, 52, 43))
			selectFont(hdc, app.hFontBold)
			text(hdc, "Radio Balkan · Hrvatska", mainL+18, contentTop+2, mainR-180, contentTop+23, rgb(248, 249, 251), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			selectFont(hdc, app.hFontSmall)
			text(hdc, "Najslušanije i regionalne postaje na jednom mjestu.", mainL+18, contentTop+22, mainR-180, contentTop+43, rgb(163, 172, 183), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			text(hdc, fmt.Sprintf("%d hrvatskih stanica", croatiaCount), mainR-170, contentTop+2, mainR-18, contentTop+43, rgb(255, 177, 55), DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		}

		gapX := int32(8)
		gapY := int32(8)
		cardH := int32(60)
		cardW := (width - gapX*int32(columns-1)) / int32(columns)
		drawRows := func(ids []int, startY int32, maxRows int) int32 {
			if maxRows <= 0 {
				return startY
			}
			for pos, idx := range ids {
				row, col := pos/columns, pos%columns
				if row >= maxRows {
					break
				}
				y := startY + int32(row)*(cardH+gapY)
				if y < 92 || y+cardH > bottom {
					continue
				}
				app.mu.RLock()
				if idx < 0 || idx >= len(app.stations) {
					app.mu.RUnlock()
					continue
				}
				st := app.stations[idx]
				app.mu.RUnlock()
				l := mainL + int32(col)*(cardW+gapX)
				drawRegionCard(hdc, l, y, l+cardW, y+cardH, idx, st, pos)
			}
			rows := (len(ids) + columns - 1) / columns
			if rows > maxRows {
				rows = maxRows
			}
			return startY + int32(rows)*(cardH+gapY)
		}
		drawSection := func(title, action string, actionKind HitKind, actionValue string, ids []int, y int32, rows int) int32 {
			if len(ids) == 0 {
				return y
			}
			if y >= 92 && y+24 <= bottom {
				selectFont(hdc, app.hFontBold)
				text(hdc, title, mainL, y, mainR-150, y+24, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
				if action != "" {
					selectFont(hdc, app.hFontSmall)
					text(hdc, action+"  →", mainR-150, y, mainR, y+24, rgb(157, 165, 176), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
					app.hits = append(app.hits, HitRegion{R: RECT{mainR - 160, y - 2, mainR, y + 26}, Kind: actionKind, Index: -1, Value: actionValue})
				}
			}
			next := drawRows(ids, y+26, rows)
			return next + 10
		}

		y := contentTop + 52
		y = drawSection("Popularno u Hrvatskoj", "Top", hitTab, "popular", popular, y, 1)
		y = drawSection("Hrvatska", "Sve hrvatske", hitTab, "all", croatia, y, 3)
		y = drawSection("Bosna i Hercegovina", "Sve zemlje", hitTab, "country:BA", bosnia, y, 1)
		y = drawSection("Srbija", "Sve zemlje", hitTab, "country:RS", serbia, y, 1)
		y = drawSection("Ostatak Balkana", "Zemlje", hitTab, "countries", balkan, y, 2)
		y = drawSection("Narodna / Folk", "Žanrovi", hitTab, "genre:folk", folk, y, 1)
		_ = drawSection("Pop & Rock", "Žanrovi", hitTab, "genre:pop", popRock, y, 1)
		return
	}
	// Library/search/filter pages use the efficient virtualized two-column grid.
	gridTop := int32(132)
	selectFont(hdc, app.hFontBold)
	title := "Sve stanice"
	app.mu.RLock()
	tab = app.tab
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

func drawRegionCard(hdc syscall.Handle, l, t, r, b int32, idx int, s RadioStation, slot int) {
	key := stationKey(s)
	app.mu.RLock()
	selected := app.currentKey != "" && app.currentKey == key
	selectedPlaying := selected && app.playing
	app.mu.RUnlock()
	border := color(49, 57, 68)
	fill := color(20, 25, 33)
	if selected {
		border = color(175, 112, 45)
		fill = color(28, 27, 29)
	} else if hovered(hitPlay, idx, key) || hovered(hitStationDetails, idx, key) {
		border = color(133, 88, 40)
	}
	drawRounded(hdc, l, t, r, b, 10, fill, border)
	artR := l + 58
	drawStationArtwork(hdc, l+6, t+7, artR-5, b-7, s, slot)
	selectFont(hdc, app.hFontBold)
	text(hdc, trimName(s.Name), artR+5, t+6, r-40, t+31, rgb(244, 246, 248), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCountryFlag(hdc, artR+5, t+35, artR+25, t+48, stationFlagCode(s))
	selectFont(hdc, app.hFontSmall)
	meta := stationAreaLabel(s)
	text(hdc, meta, artR+31, t+31, r-40, t+54, rgb(160, 168, 178), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCircle(hdc, r-34, t+18, r-10, t+42, color(45, 49, 57), color(105, 74, 40))
	playLabel := "▶"
	if selectedPlaying {
		playLabel = "Ⅱ"
	}
	text(hdc, playLabel, r-32, t+18, r-12, t+42, rgb(246, 248, 250), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitStationDetails, Index: idx, Value: key})
	app.hits = append(app.hits, HitRegion{R: RECT{r - 38, t + 14, r - 6, t + 46}, Kind: hitPlay, Index: idx, Value: key})
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

func firstPublicTag(tags string) string {
	for _, raw := range strings.Split(tags, ",") {
		v := strings.TrimSpace(raw)
		if v == "" || strings.EqualFold(v, "dijaspora") {
			continue
		}
		if len([]rune(v)) >= 2 && len([]rune(v)) <= 32 {
			return v
		}
	}
	return ""
}

func stationPublicDescription(s RadioStation) string {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		name = "Ova radio stanica"
	}
	area := stationAreaLabel(s)
	if strings.TrimSpace(area) == "" {
		area = "Radio Balkan kataloga"
	}
	description := name + " je radio stanica iz područja " + area + ". "
	if genre := firstPublicTag(s.Tags); genre != "" {
		description += "Istaknuta kategorija: " + genre + ". "
	}
	if language := strings.TrimSpace(s.Language); language != "" {
		description += "Jezik programa: " + language + ". "
	}
	description += "Ako se veza sa stanicom privremeno prekine, Radio Balkan automatski pokušava ponovno uspostaviti reprodukciju."
	return description
}

func stationPublicFacts(s RadioStation) string {
	facts := make([]string, 0, 6)
	if language := strings.TrimSpace(s.Language); language != "" {
		facts = append(facts, "Jezik: "+language)
	}
	if strings.EqualFold(strings.TrimSpace(s.Health), "ok") || s.LastCheckOK == 1 {
		facts = append(facts, "Dostupnost: potvrđena")
	} else {
		facts = append(facts, "Dostupnost: provjera pri reprodukciji")
	}
	if state := strings.TrimSpace(s.State); state != "" && !strings.EqualFold(state, strings.TrimSpace(s.Country)) {
		facts = append(facts, state)
	}
	if codec := strings.TrimSpace(s.Codec); codec != "" {
		facts = append(facts, strings.ToUpper(codec))
	}
	if s.Bitrate > 0 {
		facts = append(facts, fmt.Sprintf("%d kbps", s.Bitrate))
	}
	tags := make([]string, 0, 5)
	for _, raw := range strings.Split(s.Tags, ",") {
		tag := strings.TrimSpace(raw)
		if tag == "" || strings.EqualFold(tag, "dijaspora") {
			continue
		}
		duplicate := false
		for _, existing := range tags {
			if strings.EqualFold(existing, tag) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			tags = append(tags, tag)
		}
		if len(tags) >= 5 {
			break
		}
	}
	if len(tags) > 0 {
		facts = append(facts, "Kategorije: "+strings.Join(tags, ", "))
	}
	if len(facts) == 0 {
		return "Radio uživo"
	}
	return strings.Join(facts, " · ")
}

func similarStationIndices(source RadioStation, sourceIdx, limit int) []int {
	if limit <= 0 {
		return nil
	}
	genre := strings.ToLower(firstPublicTag(source.Tags))
	sourceCountry := stationComparisonCountry(source)
	type candidate struct {
		idx, score, votes int
	}
	app.mu.RLock()
	items := make([]candidate, 0, len(app.stations))
	for idx, station := range app.stations {
		if idx == sourceIdx || stationKey(station) == stationKey(source) {
			continue
		}
		score := 0
		if sourceCountry != "" && stationComparisonCountry(station) == sourceCountry {
			score += 6
		}
		if genre != "" && strings.Contains(strings.ToLower(station.Tags), genre) {
			score += 4
		}
		if score >= 4 {
			items = append(items, candidate{idx: idx, score: score, votes: station.Votes})
		}
	}
	app.mu.RUnlock()
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].votes > items[j].votes
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		out = append(out, item.idx)
	}
	return out
}

func drawStationDetailPage(hdc syscall.Handle, cr RECT) {
	mainL := sidebarWidth + mainPad
	mainR := cr.Right - mainPad
	bottom := cr.Bottom - playerHeight - 14

	app.mu.RLock()
	idx := findStationIndexLocked(app.detailKey, -1)
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		selectFont(hdc, app.hFontBold)
		text(hdc, "Stanica više nije dostupna u katalogu.", mainL, 150, mainR, bottom, rgb(190, 198, 207), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
		app.hits = append(app.hits, HitRegion{R: RECT{mainL, 92, mainL + 150, 128}, Kind: hitStationBack, Index: -1})
		return
	}
	station := app.stations[idx]
	app.mu.RUnlock()

	key := stationKey(station)
	app.stateMu.RLock()
	favorite := app.state.Favorites[key]
	app.stateMu.RUnlock()

	drawRounded(hdc, mainL, 92, mainL+148, 128, 10, color(17, 24, 33), color(52, 65, 79))
	selectFont(hdc, app.hFontSmall)
	text(hdc, "←  Sve stanice", mainL+10, 92, mainL+138, 128, rgb(221, 226, 232), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{mainL, 92, mainL + 148, 128}, Kind: hitStationBack, Index: -1})

	heroTop := int32(142)
	heroBottom := int32(318)
	drawRounded(hdc, mainL, heroTop, mainR, heroBottom, 18, color(18, 23, 31), color(88, 59, 37))
	drawStationArtwork(hdc, mainL+20, heroTop+20, mainL+144, heroBottom-20, station, 0)

	copyL := mainL + 166
	selectFont(hdc, app.hFontSmall)
	text(hdc, "●  UŽIVO", copyL, heroTop+19, mainR-220, heroTop+43, rgb(255, 177, 55), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, app.hFontTitle)
	text(hdc, trimName(station.Name), copyL, heroTop+46, mainR-210, heroTop+88, rgb(247, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	selectFont(hdc, app.hFontSmall)
	meta := stationAreaLabel(station)
	if genre := firstPublicTag(station.Tags); genre != "" {
		meta += " · " + genre
	}
	text(hdc, meta, copyL, heroTop+91, mainR-220, heroTop+121, rgb(183, 192, 203), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

	playL := mainR - 198
	app.mu.RLock()
	detailCurrent := app.currentKey == key
	detailPlaying := detailCurrent && app.playing
	app.mu.RUnlock()
	playLabel := "▶  Slušaj uživo"
	if detailPlaying {
		playLabel = "Ⅱ  Pauziraj"
	} else if detailCurrent {
		playLabel = "▶  Nastavi"
	}
	drawRounded(hdc, playL, heroTop+58, mainR-20, heroTop+106, 13, color(255, 177, 55), color(255, 204, 116))
	selectFont(hdc, app.hFontBold)
	text(hdc, playLabel, playL+8, heroTop+58, mainR-28, heroTop+106, rgb(29, 20, 11), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{playL, heroTop + 58, mainR - 20, heroTop + 106}, Kind: hitPlay, Index: idx, Value: key})

	favLabel := "♡  Dodaj u omiljene"
	if favorite {
		favLabel = "♥  Omiljena"
	}
	drawRounded(hdc, playL, heroTop+114, mainR-20, heroTop+154, 11, color(20, 27, 36), color(68, 58, 48))
	selectFont(hdc, app.hFontSmall)
	text(hdc, favLabel, playL+8, heroTop+114, mainR-28, heroTop+154, rgb(223, 194, 154), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{playL, heroTop + 114, mainR - 20, heroTop + 154}, Kind: hitFavorite, Index: idx, Value: key})

	contentTop := int32(336)
	drawRounded(hdc, mainL, contentTop, mainR, contentTop+158, 15, color(15, 22, 31), color(45, 56, 69))
	selectFont(hdc, app.hFontBold)
	text(hdc, "O radio stanici", mainL+18, contentTop+12, mainR-18, contentTop+40, rgb(246, 248, 250), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	selectFont(hdc, app.hFontSmall)
	text(hdc, stationPublicDescription(station), mainL+18, contentTop+45, mainR-18, contentTop+112, rgb(215, 221, 229), DT_LEFT|DT_WORDBREAK)
	drawRounded(hdc, mainL+18, contentTop+119, mainR-18, contentTop+148, 8, color(12, 18, 25), color(43, 54, 67))
	text(hdc, stationPublicFacts(station), mainL+29, contentTop+119, mainR-29, contentTop+148, rgb(154, 166, 179), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

	similarTop := contentTop + 176
	if similarTop+98 < bottom {
		selectFont(hdc, app.hFontBold)
		text(hdc, "Slične stanice", mainL, similarTop, mainR, similarTop+28, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		ids := similarStationIndices(station, idx, 4)
		if len(ids) == 0 {
			selectFont(hdc, app.hFontSmall)
			text(hdc, "Nema dovoljno sličnih stanica za ovaj prikaz.", mainL, similarTop+32, mainR, similarTop+78, rgb(132, 143, 155), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
			return
		}
		gap := int32(10)
		cardW := (mainR - mainL - gap*int32(len(ids)-1)) / int32(len(ids))
		for i, candidateIdx := range ids {
			app.mu.RLock()
			if candidateIdx < 0 || candidateIdx >= len(app.stations) {
				app.mu.RUnlock()
				continue
			}
			candidate := app.stations[candidateIdx]
			app.mu.RUnlock()
			l := mainL + int32(i)*(cardW+gap)
			drawRegionCard(hdc, l, similarTop+34, l+cardW, similarTop+96, candidateIdx, candidate, i)
		}
	}
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
	} else if hovered(hitPlay, idx, key) || hovered(hitStationDetails, idx, key) {
		fill, border = color(20, 27, 36), color(94, 72, 48)
	}
	drawRounded(hdc, l, t, r, b, 14, fill, border)
	app.hits = append(app.hits, HitRegion{R: RECT{l, t, r, b}, Kind: hitStationDetails, Index: idx, Value: key})
	artR := l + 128
	drawStationArtwork(hdc, l+1, t+1, artR, b-1, s, idx)
	selectFont(hdc, app.hFontBold)
	text(hdc, trimName(s.Name), artR+15, t+10, r-170, t+38, rgb(245, 247, 249), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	drawCountryFlag(hdc, artR+15, t+45, artR+37, t+59, stationFlagCode(s))
	selectFont(hdc, app.hFontSmall)
	meta := stationAreaLabel(s)
	g := firstPublicTag(s.Tags)
	if g != "" && meta != "" {
		meta += " · " + g
	}
	text(hdc, meta, artR+44, t+38, r-145, t+65, rgb(178, 185, 194), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	if adminModeEnabled() {
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
		action("Izvor", 46, hitReplace)
		action("✓", 26, hitCheckStation)
	}
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

func playerTransportCenter(width int32) int32 {
	center := width / 2
	if width < 1180 && center < 554 {
		center = 554
	}
	return center
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
	stopped := true
	currentIdx := -1
	canNavigate := false
	var current RadioStation
	app.mu.RLock()
	if idx := currentStationIndexLocked(); idx >= 0 && idx < len(app.stations) {
		currentIdx = idx
		current = app.stations[idx]
		name = current.Name
		meta = stationMeta(current)
		currentCountry = stationFlagCode(current)
		if app.nowPlaying != "" && app.nowPlayingStation == stationKey(current) {
			np = app.nowPlaying
		}
	}
	playing = app.playing
	stopped = app.audioStopped
	canNavigate = canNavigateStations(currentIdx, len(app.stations), len(app.filtered))
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
	if currentIdx >= 0 {
		app.hits = append(app.hits, HitRegion{R: RECT{artL, t + 8, 382, t + 86}, Kind: hitStationDetails, Index: currentIdx, Value: stationKey(current)})
	}
	if np != "" {
		app.hits = append(app.hits, HitRegion{R: RECT{artR + 16, t + 42, 382, t + 71}, Kind: hitCopyNowPlaying, Index: -1})
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

	// Center transport controls without overlapping the favorite action on
	// compact windows.
	cx := playerTransportCenter(cr.Right)
	compactPlayer := cr.Right < 1180
	canStop := canStopPlayback(currentIdx, stopped)
	selectFont(hdc, app.hFontBold)
	prevColor := rgb(193, 199, 207)
	if !canNavigate {
		prevColor = rgb(91, 98, 108)
	}
	text(hdc, "◀", cx-112, t+21, cx-74, t+59, prevColor, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	if canNavigate {
		app.hits = append(app.hits, HitRegion{R: RECT{cx - 116, t + 17, cx - 70, t + 63}, Kind: hitPlayerPrev, Index: -1})
	}
	label := "▶"
	if playing {
		label = "Ⅱ"
	}
	drawCircle(hdc, cx-31, t+10, cx+31, t+72, color(255, 174, 52), color(255, 197, 95))
	selectFont(hdc, app.hFontTitle)
	text(hdc, label, cx-28, t+10, cx+28, t+72, rgb(20, 21, 24), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{cx - 35, t + 7, cx + 35, t + 76}, Kind: hitPlayerPlay, Index: -1})
	if canStop {
		drawIconButton(hdc, cx+48, t+25, cx+88, t+65, "■", false)
		app.hits = append(app.hits, HitRegion{R: RECT{cx + 44, t + 21, cx + 92, t + 69}, Kind: hitPlayerStop, Index: -1})
	} else {
		drawRounded(hdc, cx+48, t+25, cx+88, t+65, 13, color(14, 20, 28), color(33, 41, 51))
		selectFont(hdc, app.hFontBold)
		text(hdc, "■", cx+48, t+25, cx+88, t+65, rgb(91, 98, 108), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	selectFont(hdc, app.hFontBold)
	nextColor := rgb(193, 199, 207)
	if !canNavigate {
		nextColor = rgb(91, 98, 108)
	}
	text(hdc, "▶", cx+112, t+21, cx+150, t+59, nextColor, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	if canNavigate {
		app.hits = append(app.hits, HitRegion{R: RECT{cx + 108, t + 17, cx + 154, t + 63}, Kind: hitPlayerNext, Index: -1})
	}
	// Live line underneath transport.
	lineL := cx - 150
	lineR := cx + 150
	fillRectColor(hdc, lineL, t+80, lineR, t+83, color(45, 53, 64))
	fillRectColor(hdc, lineL, t+80, cx+20, t+83, color(255, 174, 52))
	drawCircle(hdc, cx+15, t+76, cx+25, t+86, color(255, 174, 52), color(255, 174, 52))

	// Right-side equalizer and volume group. Decorative bars collapse first
	// on compact widths so transport controls retain independent hit areas.
	if !compactPlayer {
		eqL := cr.Right - 390
		drawEqualizerBars(hdc, eqL, t+25, eqL+128, t+66, playing)
	}
	drawSpeakerIcon(hdc, cr.Right-235, t+35, rgb(186, 193, 202))
	if canAdjustVolume(vol, -5) {
		drawIconButton(hdc, cr.Right-200, t+27, cr.Right-168, t+59, "−", false)
		app.hits = append(app.hits, HitRegion{R: RECT{cr.Right - 200, t + 27, cr.Right - 168, t + 59}, Kind: hitVolumeDown, Index: -1})
	} else {
		drawRounded(hdc, cr.Right-200, t+27, cr.Right-168, t+59, 13, color(14, 20, 28), color(33, 41, 51))
		selectFont(hdc, app.hFontBold)
		text(hdc, "−", cr.Right-200, t+27, cr.Right-168, t+59, rgb(91, 98, 108), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	selectFont(hdc, app.hFontSmall)
	text(hdc, fmt.Sprintf("%d%%", vol), cr.Right-164, t+27, cr.Right-112, t+59, rgb(222, 225, 230), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	if canAdjustVolume(vol, 5) {
		drawIconButton(hdc, cr.Right-106, t+27, cr.Right-74, t+59, "+", false)
		app.hits = append(app.hits, HitRegion{R: RECT{cr.Right - 106, t + 27, cr.Right - 74, t + 59}, Kind: hitVolumeUp, Index: -1})
	} else {
		drawRounded(hdc, cr.Right-106, t+27, cr.Right-74, t+59, 13, color(14, 20, 28), color(33, 41, 51))
		selectFont(hdc, app.hFontBold)
		text(hdc, "+", cr.Right-106, t+27, cr.Right-74, t+59, rgb(91, 98, 108), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}

	// Production attribution: only the Brendigo word is interactive.
	selectFont(hdc, app.hFontSmall)
	text(hdc, "Built with", cr.Right-244, t+66, cr.Right-188, t+90, rgb(106, 116, 128), DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
	text(hdc, "Brendigo", cr.Right-182, t+66, cr.Right-112, t+90, rgb(218, 164, 91), DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	app.hits = append(app.hits, HitRegion{R: RECT{cr.Right - 184, t + 66, cr.Right - 110, t + 91}, Kind: hitBrendigo, Index: -1, Value: "https://brendigo.com/"})
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
	case "BG":
		fillRectColor(hdc, l, t, r, t+h/3, color(255, 255, 255))
		fillRectColor(hdc, l, t+h/3, r, t+2*h/3, color(0, 150, 110))
		fillRectColor(hdc, l, t+2*h/3, r, b, color(214, 38, 18))
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

type playbackToggleAction uint8

const (
	playbackToggleNone playbackToggleAction = iota
	playbackTogglePause
	playbackToggleResume
	playbackToggleReconnect
)

func decidePlaybackToggle(current int, playing, stopped bool) playbackToggleAction {
	if current < 0 {
		return playbackToggleNone
	}
	if playing {
		return playbackTogglePause
	}
	if stopped {
		return playbackToggleReconnect
	}
	return playbackToggleResume
}

func defaultPlaybackIndexLocked() int {
	if len(app.filtered) > 0 {
		idx := app.filtered[0]
		if idx >= 0 && idx < len(app.stations) {
			return idx
		}
	}
	if len(app.stations) > 0 {
		return 0
	}
	return -1
}

func toggleCurrentPlayback() {
	app.mu.RLock()
	current, playing, stopped := currentStationIndexLocked(), app.playing, app.audioStopped
	currentKey := app.currentKey
	defaultIndex := defaultPlaybackIndexLocked()
	if currentKey == "" && current >= 0 && current < len(app.stations) {
		currentKey = stationKey(app.stations[current])
	}
	app.mu.RUnlock()
	action := decidePlaybackToggle(current, playing, stopped)
	if action == playbackToggleNone {
		if defaultIndex >= 0 {
			playStation(defaultIndex)
		}
		return
	}
	if action == playbackTogglePause {
		app.mu.Lock()
		key := app.currentKey
		app.playing = false
		app.metadataSeq++
		app.mu.Unlock()
		setStatus("Pauzirano")
		invalidate()
		if currentAudioBackend() != audioBackendNone {
			safeGo("audio-pause-control", func() {
				if err := audioPause(); err != nil {
					logError("audio-pause", err)
					// If pause cannot be delivered to the active backend, stop
					// playback so audible and visible state cannot diverge.
					audioStop()
					app.mu.Lock()
					if app.currentKey == key {
						app.audioStopped = true
						app.playing = false
						app.metadataSeq++
					}
					app.mu.Unlock()
					setStatus("Reprodukcija je zaustavljena")
					postUI()
				}
			})
		}
		return
	}
	if action == playbackToggleReconnect {
		playStationByKey(currentKey, current)
		return
	}
	if currentAudioBackend() == audioBackendNone {
		playStationByKey(currentKey, current)
		return
	}
	app.mu.Lock()
	app.playSeq++
	reqSeq := app.playSeq
	app.mu.Unlock()
	setStatus("Nastavljam reprodukciju…")
	invalidate()
	safeGo("audio-resume-control", func() {
		if err := audioResume(); err != nil {
			logError("audio-resume", err)
			app.mu.RLock()
			stillCurrent := app.playSeq == reqSeq && !app.audioStopped
			app.mu.RUnlock()
			if stillCurrent {
				playStationByKey(currentKey, current)
			}
			return
		}
		// MediaFailed can arrive while the RESUME command is in flight. The
		// runtime failure handler clears the backend; reconnect immediately
		// instead of marking a dead MediaPlayer as "Uživo".
		if currentAudioBackend() == audioBackendNone {
			app.mu.RLock()
			stillCurrent := app.playSeq == reqSeq && !app.audioStopped
			app.mu.RUnlock()
			if stillCurrent {
				playStationByKey(currentKey, current)
			}
			return
		}
		app.mu.Lock()
		if app.playSeq != reqSeq || app.audioStopped {
			app.mu.Unlock()
			return
		}
		actual := findStationIndexLocked(currentKey, current)
		if actual < 0 || actual >= len(app.stations) {
			app.mu.Unlock()
			return
		}
		current = actual
		st := app.stations[current]
		key := stationKey(st)
		app.playing = true
		app.audioStopped = false
		app.nowPlayingStation = key
		app.metadataSeq++
		seq := app.metadataSeq
		app.mu.Unlock()
		stream := effectiveURL(st)
		setStatus("Uživo · " + st.Name)
		postUI()
		safeGo("watchdog-resume-"+key, func() { playbackWatchdog(current, key) })
		if stream != "" {
			safeGo("metadata-resume-"+key, func() { metadataLoop(seq, current, key, stream) })
		}
	})
}

func canStopPlayback(current int, stopped bool) bool {
	return current >= 0 && !stopped
}

func canAdjustVolume(volume, delta int) bool {
	if delta < 0 {
		return volume > 0
	}
	if delta > 0 {
		return volume < 100
	}
	return false
}

func canNavigateStations(current, stationCount, filteredCount int) bool {
	if current < 0 {
		return false
	}
	count := stationCount
	if filteredCount > 0 {
		count = filteredCount
	}
	return count > 1
}

func stopCurrentPlayback() {
	app.mu.RLock()
	canStop := canStopPlayback(currentStationIndexLocked(), app.audioStopped)
	app.mu.RUnlock()
	if !canStop {
		return
	}
	app.mu.Lock()
	app.playing = false
	app.audioStopped = true
	app.metadataSeq++
	app.playSeq++
	stopSeq := app.playSeq
	backend := app.audioBackend
	app.nowPlaying = ""
	app.nowPlayingStation = ""
	app.mu.Unlock()
	setStatus("Zaustavljeno")
	invalidate()
	if backend != audioBackendNone {
		safeGo("audio-stop-control", func() { audioStopForRequest(stopSeq) })
	}
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
	if !canNavigateStations(current, stationCount, len(filtered)) {
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

func selectTabValue(value string) {
	if strings.HasPrefix(value, "country:") {
		code := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(value, "country:")))
		if !isRegionalCatalogCode(code) {
			return
		}
		app.mu.Lock()
		app.detailKey = ""
		app.tab = "all"
		app.country = code
		app.genre = ""
		app.scroll = 0
		app.countryMenuOpen = false
		app.genreMenuOpen = false
		app.mu.Unlock()
		app.stateMu.Lock()
		app.state.Tab = "all"
		app.state.CountryCode = code
		app.state.Genre = ""
		app.stateMu.Unlock()
		rebuildGenres()
		rebuildFilter()
		scheduleStateSave()
		invalidate()
		return
	}
	if strings.HasPrefix(value, "genre:") {
		g := strings.TrimSpace(strings.TrimPrefix(value, "genre:"))
		app.mu.Lock()
		app.detailKey = ""
		app.tab = "all"
		app.country = ""
		app.genre = g
		app.scroll = 0
		app.countryMenuOpen = false
		app.genreMenuOpen = false
		app.mu.Unlock()
		app.stateMu.Lock()
		app.state.Tab = "all"
		app.state.CountryCode = ""
		app.state.Genre = g
		app.stateMu.Unlock()
		rebuildGenres()
		rebuildFilter()
		scheduleStateSave()
		invalidate()
		return
	}
	app.mu.Lock()
	app.detailKey = ""
	app.tab = value
	app.country = ""
	if value == "all" {
		app.country = "HR"
	}
	app.genre = ""
	app.scroll = 0
	app.countryMenuOpen = false
	app.genreMenuOpen = false
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.Tab = value
	app.state.CountryCode = ""
	if value == "all" {
		app.state.CountryCode = "HR"
	}
	app.state.Genre = ""
	app.stateMu.Unlock()
	scheduleStateSave()
	rebuildGenres()
	rebuildFilter()
	invalidate()
}

func handleCoreClickFallback(x, y int32) bool {
	app.mu.RLock()
	width, height := app.clientWidth, app.clientHeight
	app.mu.RUnlock()
	if width <= 0 || height <= 0 {
		return false
	}

	if x >= 16 && x <= sidebarWidth-14 {
		switch {
		case y >= 91 && y <= 127:
			selectTabValue("all")
			return true
		case y >= 135 && y <= 171:
			selectTabValue("popular")
			return true
		case y >= 179 && y <= 215:
			selectTabValue("countries")
			return true
		case y >= 223 && y <= 259:
			selectTabValue("genres")
			return true
		case y >= 267 && y <= 303:
			selectCountry(diasporaCatalogCode)
			return true
		case y >= 311 && y <= 347:
			selectCountry(foreignCatalogCode)
			return true
		case y >= 437 && y <= 473:
			selectTabValue("favorites")
			return true
		case y >= 479 && y <= 515:
			selectTabValue("recent")
			return true
		}
	}

	_, _, countryL, countryR, genreL, genreR, _, _, refreshL, refreshR := headerLayout(width)
	if y >= 20 && y <= 72 {
		switch {
		case x >= countryL && x <= countryR:
			selectTabValue("countries")
			return true
		case x >= genreL && x <= genreR:
			selectTabValue("genres")
			return true
		case x >= refreshL && x <= refreshR:
			safeGo("refresh-catalog", refreshAll)
			return true
		}
	}

	playerTop := height - playerHeight
	if y >= playerTop && y <= height {
		cx := playerTransportCenter(width)
		switch {
		case x >= cx-116 && x <= cx-70 && y >= playerTop+17 && y <= playerTop+63:
			playAdjacent(-1)
			return true
		case x >= cx-35 && x <= cx+35 && y >= playerTop+7 && y <= playerTop+76:
			toggleCurrentPlayback()
			return true
		case x >= cx+44 && x <= cx+92 && y >= playerTop+21 && y <= playerTop+69:
			stopCurrentPlayback()
			return true
		case x >= cx+108 && x <= cx+154 && y >= playerTop+17 && y <= playerTop+63:
			playAdjacent(1)
			return true
		case x >= width-200 && x <= width-168 && y >= playerTop+27 && y <= playerTop+59:
			adjustVolume(-5)
			return true
		case x >= width-106 && x <= width-74 && y >= playerTop+27 && y <= playerTop+59:
			adjustVolume(5)
			return true
		}
	}
	return false
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
			selectTabValue("countries")
		case hitGenreDropdown:
			selectTabValue("genres")
		case hitCountryChoice:
			selectCountry(h.Value)
		case hitGenreChoice:
			selectGenre(h.Value)
		case hitTab:
			selectTabValue(h.Value)
		case hitPlay:
			if idx := stationIndexFromHit(h); idx >= 0 {
				activateStation(idx)
			}
		case hitLink:
			if requireAdmin() {
				if idx := stationIndexFromHit(h); idx >= 0 {
					copyStationLink(idx)
				}
			}
		case hitWeb:
			if requireAdmin() {
				if idx := stationIndexFromHit(h); idx >= 0 {
					openStationWeb(idx)
				}
			}
		case hitFavorite:
			if idx := stationIndexFromHit(h); idx >= 0 {
				toggleFavorite(idx)
			}
		case hitReplace:
			if requireAdmin() {
				if idx := stationIndexFromHit(h); idx >= 0 {
					replaceStation(idx)
				}
			}
		case hitCheckStation:
			if requireAdmin() {
				if idx := stationIndexFromHit(h); idx >= 0 {
					checkStationNow(idx)
				}
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
			if requireAdmin() {
				setStatus("Provjeravam dostupnost svih stanica…")
				postUI()
				safeGo("manual-health", healthCheckAll)
			}
		case hitAbout:
			messageBox(hwndOrZero(), "Radio Balkan", "Radio Balkan "+appVersion+"\n\nRadio iz Hrvatske i regije, posebna kategorija Dijaspora te odabrane strane postaje.\nFavoriti i povijest slušanja rade lokalno na tvojem računalu. Napredne kontrole reprodukcije dostupne su samo u Admin načinu rada.", MB_ICONINFORMATION)
		case hitAdmin:
			toggleAdminSession()
		case hitStationDetails:
			if idx := stationIndexFromHit(h); idx >= 0 {
				openStationDetails(idx)
			}
		case hitStationBack:
			closeStationDetails()
		case hitBrendigo:
			shellOpen("https://brendigo.com/")
		case hitRefresh:
			app.mu.Lock()
			app.countryMenuOpen = false
			app.genreMenuOpen = false
			app.mu.Unlock()
			safeGo("refresh-catalog", refreshAll)
		}
		return
	}
	if !menuOpen && handleCoreClickFallback(x, y) {
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
	code = strings.ToUpper(strings.TrimSpace(code))
	if !isSelectableCatalogCode(code) {
		code = ""
	}
	app.mu.Lock()
	app.detailKey = ""
	app.tab = "all"
	app.country = code
	app.genre = ""
	app.scroll = 0
	app.countryMenuOpen = false
	app.genreMenuOpen = false
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.Tab = "all"
	app.state.CountryCode = code
	app.state.Genre = ""
	app.stateMu.Unlock()
	scheduleStateSave()
	rebuildGenres()
	rebuildFilter()
	invalidate()
}

func selectGenre(genre string) {
	genre = strings.TrimSpace(genre)
	app.mu.Lock()
	app.detailKey = ""
	app.tab = "all"
	app.genre = genre
	app.scroll = 0
	app.countryMenuOpen = false
	app.genreMenuOpen = false
	app.mu.Unlock()
	app.stateMu.Lock()
	app.state.Tab = "all"
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
		if err := audioPlayRequest(final, reqSeq); err != nil {
			if errors.Is(err, errPlayRequestSuperseded) {
				app.playTransitionMu.Unlock()
				return
			}
			logError("audio-play-primary", err)
			setStatus("Veza sa stanicom nije uspjela · pokušavam ponovno…")
			postUI()
			alternate, altErr := tryAlternatePlayback(idx, key, final, reqSeq)
			if altErr != nil {
				app.playTransitionMu.Unlock()
				if errors.Is(altErr, errPlayRequestSuperseded) {
					return
				}
				logError("audio-play-alternate", altErr)
				app.mu.RLock()
				currentReq := app.playSeq == reqSeq
				app.mu.RUnlock()
				if currentReq {
					setStatus("Stanica trenutno nije dostupna")
					postUI()
				}
				return
			}
			final = alternate
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
		app.audioStopped = false
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
func playbackBackendNeedsRecovery(active bool, backend audioBackendKind) bool {
	return active && backend == audioBackendNone
}

func playbackWatchdog(idx int, stationID string) {
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
		backend := app.audioBackend
		if actual >= 0 {
			idx = actual
		}
		app.mu.RUnlock()
		if !active {
			return
		}

		// The player backend is the source of truth. Never issue a second HTTP GET
		// against an already-playing Icecast/Shoutcast/HLS stream: many healthy
		// stations reject probes or limit concurrent listeners.
		switch backend {
		case audioBackendWPF:
			// WPF forwards MediaFailed/MediaEnded asynchronously from MediaPlayer.
			continue
		case audioBackendMCI:
			mode, err := mciQueryExisting("status radio mode")
			if err == nil && mciModeHealthy(mode) {
				continue
			}
			handleAudioBackendFailure(audioBackendMCI, "MCI playback stopped")
			return
		case audioBackendNone:
			if playbackBackendNeedsRecovery(active, backend) {
				handleAudioBackendFailure(audioBackendNone, "playback backend missing")
				return
			}
		}
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
	// Prefer the fresh catalog's resolved/direct URL before persisted automatic
	// backups from older releases. Stale state must never make every station feel
	// dead after an upgrade; remembered sources remain available as fallbacks.
	candidates := []string{s.URLResolved, s.URL, s.ActiveURL}
	app.stateMu.RLock()
	if r := app.state.Replacements[key]; r != "" {
		candidates = append(candidates, r)
	}
	candidates = append(candidates, app.state.Backups[key]...)
	app.stateMu.RUnlock()
	for _, c := range uniqueStrings(candidates) {
		if resolved, ok := streamCandidateForPlayback(c); ok {
			updateStationURL(idx, key, resolved, c != s.URLResolved && c != s.URL)
			return resolved, true
		}
	}
	if s.StationUUID != "" {
		if one, err := fetchStationByUUID(s.StationUUID, s.CountryCode, s.SourceCountryCode); err == nil && one != nil {
			for _, c := range uniqueStrings([]string{one.URLResolved, one.URL}) {
				if resolved, ok := streamCandidateForPlayback(c); ok {
					rememberReplacement(idx, key, resolved)
					return resolved, true
				}
			}
		}
	}
	if s.Name != "" {
		if list, err := searchStationsByName(s.Name, s.CountryCode, s.SourceCountryCode); err == nil {
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
					if resolved, ok := streamCandidateForPlayback(c); ok {
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
func tryAlternatePlayback(idx int, key, failedURL string, reqSeq uint64) (string, error) {
	if !playRequestStillCurrent(reqSeq) {
		return "", errPlayRequestSuperseded
	}
	app.mu.RLock()
	actual := findStationIndexLocked(key, idx)
	if actual < 0 || actual >= len(app.stations) {
		app.mu.RUnlock()
		return "", errors.New("stanica više nije u katalogu")
	}
	idx = actual
	station := app.stations[idx]
	app.mu.RUnlock()

	candidates := make([]string, 0, 24)
	app.stateMu.RLock()
	candidates = append(candidates, app.state.Backups[key]...)
	app.stateMu.RUnlock()
	candidates = append(candidates, station.URLResolved, station.URL)

	if station.StationUUID != "" {
		if !playRequestStillCurrent(reqSeq) {
			return "", errPlayRequestSuperseded
		}
		if refreshed, err := fetchStationByUUID(station.StationUUID, station.CountryCode, station.SourceCountryCode); err == nil && refreshed != nil {
			candidates = append(candidates, refreshed.URLResolved, refreshed.URL)
		}
	}
	if station.Name != "" {
		if !playRequestStillCurrent(reqSeq) {
			return "", errPlayRequestSuperseded
		}
		if alternatives, err := searchStationsByName(station.Name, station.CountryCode, station.SourceCountryCode); err == nil {
			for _, alt := range alternatives {
				if sameStation(station, alt) {
					candidates = append(candidates, alt.URLResolved, alt.URL)
				}
				if len(candidates) >= 24 {
					break
				}
			}
		}
	}

	var lastErr error
	tried := 0
	for _, candidate := range uniqueStrings(candidates) {
		if !playRequestStillCurrent(reqSeq) {
			return "", errPlayRequestSuperseded
		}
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(failedURL)) {
			continue
		}
		resolved, ok := streamCandidateForPlayback(candidate)
		if !ok || strings.EqualFold(strings.TrimSpace(resolved), strings.TrimSpace(failedURL)) {
			continue
		}
		tried++
		if err := audioPlayRequest(resolved, reqSeq); err == nil {
			rememberReplacement(idx, key, resolved)
			return resolved, nil
		} else {
			if errors.Is(err, errPlayRequestSuperseded) {
				return "", err
			}
			lastErr = err
			logError("audio-play-candidate", err)
		}
		if tried >= 3 {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("nije pronađen drugi kompatibilan izvor")
	}
	return "", lastErr
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
	if !requireAdmin() {
		return
	}
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	s := app.stations[idx]
	app.mu.RUnlock()
	u := effectiveURL(s)
	if u == "" {
		setStatus("Stanica trenutno nije dostupna")
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
func openStationDetails(idx int) {
	app.mu.Lock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.Unlock()
		return
	}
	app.detailKey = stationKey(app.stations[idx])
	app.countryMenuOpen = false
	app.genreMenuOpen = false
	app.mu.Unlock()
	invalidate()
}

func closeStationDetails() {
	app.mu.Lock()
	app.detailKey = ""
	app.mu.Unlock()
	invalidate()
}

func openStationWeb(idx int) {
	if !requireAdmin() {
		return
	}
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
func shouldRestartAfterSourceChange(currentKey, changedKey string, playing bool) bool {
	return playing && currentKey != "" && currentKey == changedKey
}

func restartCurrentStationIfPlaying(key string, fallback int) bool {
	app.mu.RLock()
	restart := shouldRestartAfterSourceChange(app.currentKey, key, app.playing)
	app.mu.RUnlock()
	if !restart {
		return false
	}
	playStationByKey(key, fallback)
	return true
}

func replaceStation(idx int) {
	if !requireAdmin() {
		return
	}
	app.mu.RLock()
	if idx < 0 || idx >= len(app.stations) {
		app.mu.RUnlock()
		return
	}
	s := app.stations[idx]
	app.mu.RUnlock()
	key := stationKey(s)
	def := effectiveURL(s)
	value, ok := inputDialog(app.hwnd, "Promijeni poveznicu", "Unesi novu http/https poveznicu za reprodukciju za:\n"+s.Name+"\n\nPrazno polje vraća automatski odabir.", def)
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	if value == "" {
		clearReplacement(idx, key)
		if restartCurrentStationIfPlaying(key, idx) {
			setStatus("Vraćen automatski odabir · ponovno povezujem")
		} else {
			setStatus("Vraćen automatski odabir · " + s.Name)
		}
		postUI()
		return
	}
	setStatus("Provjeravam novu poveznicu…")
	invalidate()
	safeGo("manual-replace-"+key, func() {
		resolved, valid := checkStream(value)
		if !valid {
			queueAlert("Neispravan URL", "Nova poveznica nije dostupna.", MB_ICONWARNING)
			setStatus("Zamjena nije spremljena")
			postUI()
			return
		}
		rememberReplacement(idx, key, resolved)
		rebuildFilter()
		if restartCurrentStationIfPlaying(key, idx) {
			setStatus("Poveznica je promijenjena · ponovno povezujem")
		} else {
			setStatus("Poveznica je promijenjena · " + s.Name)
		}
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
	scheduleStateSave()
	setStatus(fmt.Sprintf("Glasnoća %d%%", v))
	invalidate()
	if currentAudioBackend() != audioBackendNone {
		safeGo("audio-volume-control", func() { audioSetVolume(v) })
	}
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
	keepCurrent := false
	stopSeq := uint64(0)
	stopBackend := audioBackendNone
	app.mu.Lock()
	// Re-read the active selection after the network refresh. A station click,
	// Stop, Next or Play that happened while the catalog request was in flight
	// must win over the stale pre-refresh snapshot.
	currentKey := app.currentKey
	if currentKey == "" {
		if idx := currentStationIndexLocked(); idx >= 0 {
			currentKey = stationKey(app.stations[idx])
		}
	}
	wasPlaying := app.playing
	app.stations = list
	app.catalogRevision++
	app.loading = false
	app.scroll = 0
	app.current = -1
	app.currentKey = ""
	if currentKey != "" {
		if i := findStationIndexLocked(currentKey, -1); i >= 0 {
			app.current = i
			app.currentKey = currentKey
			app.playing = wasPlaying
			keepCurrent = true
		}
	}
	if currentKey != "" && !keepCurrent {
		app.playing = false
		app.audioStopped = true
		app.metadataSeq++
		app.playSeq++
		stopSeq = app.playSeq
		stopBackend = app.audioBackend
		app.nowPlaying = ""
		app.nowPlayingStation = ""
	}
	app.mu.Unlock()
	if stopBackend != audioBackendNone {
		safeGo("audio-stop-refresh", func() { audioStopForRequest(stopSeq) })
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
		app.catalogRevision++
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
			scheduleStartupHealth(20)
		}
		safeGo("health-periodic", periodicHealthLoop)
		return
	}

	prepareStations(list)
	app.mu.Lock()
	app.stations = list
	app.catalogRevision++
	app.loading = false
	app.scroll = 0
	app.mu.Unlock()
	saveCache(list)
	postGenres()
	rebuildFilter()
	setStatus(fmt.Sprintf("Učitano %d stanica iz %d država", len(list), countCountries(list)))
	postUI()
	if !app.safeMode {
		scheduleStartupHealth(24)
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
					if d > 0 && (d%50 == 0 || d == int64(len(keys))) && !shuttingDown() && adminModeEnabled() {
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
	if adminModeEnabled() {
		setStatus(fmt.Sprintf("Provjera završena · dostupno %d · alternativno %d · nedostupno %d", okc, repc, broken))
		postUI()
	}
}

func regionalFetchCodes() []string {
	codes := make([]string, 0, len(regionalCatalogCodes))
	for _, country := range balkanCountries {
		if isRegionalCatalogCode(country.Code) {
			codes = append(codes, strings.ToUpper(strings.TrimSpace(country.Code)))
		}
	}
	return codes
}

func preserveSupplementalStations(regional, previous []RadioStation) []RadioStation {
	regionalOnly := make([]RadioStation, 0, len(regional))
	for _, station := range regional {
		if isRegionalCatalogCode(station.CountryCode) {
			regionalOnly = append(regionalOnly, station)
		}
	}
	regionalOnly = dedupeStations(regionalOnly)
	if len(regionalOnly) > regionalCatalogLimit {
		regionalOnly = append([]RadioStation(nil), regionalOnly[:regionalCatalogLimit]...)
	}

	diaspora := make([]RadioStation, 0, diasporaCatalogLimit)
	foreign := make([]RadioStation, 0, foreignCatalogLimit)
	for _, station := range previous {
		switch {
		case isDiasporaCatalogCode(station.CountryCode):
			diaspora = append(diaspora, station)
		case isForeignCatalogCode(station.CountryCode):
			foreign = append(foreign, station)
		}
	}
	diaspora = sortAndCapSupplemental(diaspora, diasporaCatalogLimit)
	foreign = sortAndCapSupplemental(foreign, foreignCatalogLimit)

	combined := make([]RadioStation, 0, len(regionalOnly)+len(diaspora)+len(foreign))
	combined = append(combined, regionalOnly...)
	combined = append(combined, diaspora...)
	combined = append(combined, foreign...)
	return trimStationCatalog(dedupeStations(combined), regionalCatalogLimit+supplementalCatalogLimit)
}

func fetchBalkanStations() ([]RadioStation, error) {
	// Production catalog is restricted to the supported regional Balkan countries.
	// Application-level groups such as DIA/INT must never be sent as ISO country
	// filters to Radio Browser.
	type result struct {
		list []RadioStation
		err  error
		code string
	}
	codes := regionalFetchCodes()
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
			for _, station := range cached {
				if isRegionalCatalogCode(station.CountryCode) {
					all = append(all, station)
				}
			}
			all = dedupeStations(all)
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
	regional := trimStationCatalog(filterSupportedStations(all), regionalCatalogLimit)
	app.mu.RLock()
	previous := append([]RadioStation(nil), app.stations...)
	app.mu.RUnlock()
	if len(previous) == 0 {
		previous = loadCache()
	}
	return preserveSupplementalStations(regional, previous), nil
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
	if code == "" || !isRegionalCatalogCode(code) {
		return nil, errors.New("nepodržana regionalna država")
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

func fetchStationByUUID(id, requestedCode, sourceCountryCode string) (*RadioStation, error) {
	for _, base := range apiBases() {
		var list []RadioStation
		if err := getJSON(base+"/json/stations/byuuid/"+url.PathEscape(id), &list); err == nil {
			for i := range list {
				actual := strings.ToUpper(strings.TrimSpace(list[i].CountryCode))
				if !matchesCatalogCountry(requestedCode, sourceCountryCode, actual) {
					continue
				}
				if isSupplementalCatalogCode(requestedCode) {
					list[i].SourceCountryCode = actual
					list[i].CountryCode = strings.ToUpper(strings.TrimSpace(requestedCode))
				} else {
					list[i].CountryCode = actual
				}
				return &list[i], nil
			}
		}
	}
	return nil, errors.New("nije pronađeno")
}
func searchStationsByName(name, requestedCode, sourceCountryCode string) ([]RadioStation, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("prazan naziv")
	}
	q := url.QueryEscape(name)
	queryCountry := strings.ToUpper(strings.TrimSpace(requestedCode))
	if isSupplementalCatalogCode(queryCountry) {
		queryCountry = strings.ToUpper(strings.TrimSpace(sourceCountryCode))
	}
	paths := []string{"/json/stations/search?name=" + q + "&hidebroken=false&limit=50"}
	if queryCountry != "" {
		paths = []string{"/json/stations/search?name=" + q + "&countrycode=" + url.QueryEscape(queryCountry) + "&hidebroken=false&limit=50", paths[0]}
	}
	var last error
	for _, base := range apiBases() {
		for _, p := range paths {
			var list []RadioStation
			if err := getJSON(base+p, &list); err == nil && len(list) > 0 {
				filtered := make([]RadioStation, 0, len(list))
				for _, st := range list {
					actual := strings.ToUpper(strings.TrimSpace(st.CountryCode))
					if !matchesCatalogCountry(requestedCode, sourceCountryCode, actual) {
						continue
					}
					if isSupplementalCatalogCode(requestedCode) {
						st.SourceCountryCode = actual
						st.CountryCode = strings.ToUpper(strings.TrimSpace(requestedCode))
					} else {
						st.CountryCode = actual
					}
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
	ac := stationComparisonCountry(a)
	bc := stationComparisonCountry(b)
	if ac != "" && bc != "" && !strings.EqualFold(ac, bc) {
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
	if primary.SourceCountryCode == "" {
		primary.SourceCountryCode = other.SourceCountryCode
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
	if isDiasporaCatalogCode(a.CountryCode) || isDiasporaCatalogCode(b.CountryCode) {
		primary.CountryCode = diasporaCatalogCode
		if primary.SourceCountryCode == "" {
			if a.SourceCountryCode != "" {
				primary.SourceCountryCode = a.SourceCountryCode
			} else {
				primary.SourceCountryCode = b.SourceCountryCode
			}
		}
		if !containsFoldedTag(primary.Tags, "dijaspora") {
			if strings.TrimSpace(primary.Tags) == "" {
				primary.Tags = "dijaspora"
			} else {
				primary.Tags = primary.Tags + ",dijaspora"
			}
		}
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
	case "news":
		return strings.Contains(tags, "news") || strings.Contains(tags, "talk") || strings.Contains(tags, "vijesti")
	case "hits":
		return strings.Contains(tags, "hits") || strings.Contains(tags, "top 40") || strings.Contains(tags, "top40")
	case "oldies":
		return strings.Contains(tags, "oldies") || strings.Contains(tags, "retro") || strings.Contains(tags, "evergreen")
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
	if homeCatalogEnabled(app.clientHeight, app.tab, app.search, app.genre, app.country) {
		viewport := int(app.clientHeight) - int(playerHeight) - 16 - 92
		if viewport < 100 {
			viewport = 100
		}
		max := homeCatalogContentHeight() - viewport
		if max < 0 {
			max = 0
		}
		if app.scroll > max {
			app.scroll = max
		}
		if app.scroll < 0 {
			app.scroll = 0
		}
		return
	}
	if app.tab == "countries" || app.tab == "genres" {
		contentWidth := app.clientWidth - sidebarWidth - mainPad*2
		max := browsePageMaxScroll(app.clientHeight, contentWidth, app.tab)
		if app.scroll > max {
			app.scroll = max
		}
		if app.scroll < 0 {
			app.scroll = 0
		}
		return
	}
	gridTop := 132
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

func stationFlagCode(s RadioStation) string {
	source := strings.ToUpper(strings.TrimSpace(s.SourceCountryCode))
	if source != "" {
		return source
	}
	if isSupplementalCatalogCode(s.CountryCode) {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(s.CountryCode))
}

func stationAreaLabel(s RadioStation) string {
	code := strings.ToUpper(strings.TrimSpace(s.CountryCode))
	country := strings.TrimSpace(s.Country)
	switch {
	case isDiasporaCatalogCode(code):
		if country != "" {
			return "Dijaspora · " + country
		}
		return "Dijaspora"
	case isForeignCatalogCode(code):
		if country != "" {
			return "Strano · " + country
		}
		return "Strano"
	}
	label := countryNameByCode(code)
	if label == code && country != "" {
		return country
	}
	if label != "" {
		return label
	}
	return country
}

func stationComparisonCountry(s RadioStation) string {
	if isSupplementalCatalogCode(s.CountryCode) && strings.TrimSpace(s.SourceCountryCode) != "" {
		return strings.ToUpper(strings.TrimSpace(s.SourceCountryCode))
	}
	return strings.ToUpper(strings.TrimSpace(s.CountryCode))
}

func matchesCatalogCountry(requestedCode, sourceCode, actualCode string) bool {
	requested := strings.ToUpper(strings.TrimSpace(requestedCode))
	source := strings.ToUpper(strings.TrimSpace(sourceCode))
	actual := strings.ToUpper(strings.TrimSpace(actualCode))
	if actual == "" {
		return false
	}
	if isSupplementalCatalogCode(requested) {
		if isRegionalCatalogCode(actual) {
			return false
		}
		return source == "" || source == actual
	}
	return isRegionalCatalogCode(requested) && requested == actual
}

func stationMeta(s RadioStation) string {
	parts := []string{}
	country := stationAreaLabel(s)
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

const audioEngineStartupTimeout = 20 * time.Second

func startAudioEngineLocked() error {
	return startAudioEngineLockedForRequest(0)
}

func startAudioEngineLockedForRequest(reqSeq uint64) error {
	if app.audioCmd != nil && app.audioCmd.Process != nil {
		return nil
	}
	script := `$ErrorActionPreference='Stop'
try {
Add-Type -AssemblyName PresentationCore
Add-Type -AssemblyName WindowsBase
$source=@'
using System;
using System.Threading;
using System.Windows;
using System.Windows.Media;
using System.Windows.Threading;

public sealed class RadioBalkanMediaHost : IDisposable
{
    private Thread thread;
    private Dispatcher dispatcher;
    private MediaPlayer player;
    private readonly ManualResetEventSlim ready = new ManualResetEventSlim(false);
    private bool disposed;
    private bool currentOpened;
    private string currentToken = "";
    private bool runtimeArmed;
    private string pendingRuntimeFailure = "";

    public RadioBalkanMediaHost()
    {
        thread = new Thread(ThreadMain);
        thread.IsBackground = true;
        thread.Name = "RadioBalkanMedia";
        thread.SetApartmentState(ApartmentState.STA);
        thread.Start();
        if (!ready.Wait(8000))
            throw new TimeoutException("media dispatcher startup timeout");
    }

    private void ThreadMain()
    {
        dispatcher = Dispatcher.CurrentDispatcher;
        ready.Set();
        Dispatcher.Run();
    }

    private void EmitRuntimeFailure(string token, string message)
    {
        try
        {
            var text = String.IsNullOrWhiteSpace(message) ? "media failed" : message;
            var encoded = Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes(text));
            Console.Out.WriteLine("EVENT FAILED " + token + " " + encoded);
            Console.Out.Flush();
        }
        catch { }
    }

    private void NotifyRuntimeFailure(MediaPlayer sourcePlayer, string token, string message)
    {
        // Each Play owns a distinct MediaPlayer instance and request token.
        // Delayed events from a closed previous stream cannot fail the new stream.
        if (disposed || sourcePlayer == null || !Object.ReferenceEquals(player, sourcePlayer)) return;
        if (!String.Equals(currentToken, token, StringComparison.Ordinal) || !currentOpened) return;
        currentOpened = false;
        var text = String.IsNullOrWhiteSpace(message) ? "media failed" : message;
        if (!runtimeArmed)
        {
            pendingRuntimeFailure = text;
            return;
        }
        EmitRuntimeFailure(token, text);
    }

    public bool Play(string uri, double volume, int timeoutMs, string token, out string error)
    {
        error = "";
        if (disposed || dispatcher == null)
        {
            error = "media host unavailable";
            return false;
        }

        var done = new ManualResetEventSlim(false);
        var opened = false;
        var failure = "";
        MediaPlayer next = null;
        EventHandler onOpened = null;
        EventHandler<ExceptionEventArgs> onFailed = null;
        EventHandler onEnded = null;

        dispatcher.BeginInvoke(new Action(() =>
        {
            try
            {
                var previous = player;
                player = null;
                currentOpened = false;
                currentToken = token ?? "";
                runtimeArmed = false;
                pendingRuntimeFailure = "";
                if (previous != null)
                {
                    try { previous.Stop(); } catch { }
                    try { previous.Close(); } catch { }
                }

                next = new MediaPlayer();
                player = next;
                onOpened = (sender, args) =>
                {
                    if (!Object.ReferenceEquals(player, next) || !String.Equals(currentToken, token, StringComparison.Ordinal)) return;
                    currentOpened = true;
                    opened = true;
                    done.Set();
                };
                onFailed = (sender, args) =>
                {
                    var message = args != null && args.ErrorException != null ? args.ErrorException.Message : "media failed";
                    if (!opened)
                    {
                        failure = message;
                        done.Set();
                        return;
                    }
                    NotifyRuntimeFailure(next, token, message);
                };
                onEnded = (sender, args) =>
                {
                    if (opened) NotifyRuntimeFailure(next, token, "media ended");
                };
                next.MediaOpened += onOpened;
                next.MediaFailed += onFailed;
                next.MediaEnded += onEnded;
                next.Volume = Math.Max(0.0, Math.Min(1.0, volume));
                next.Open(new Uri(uri, UriKind.Absolute));
                next.Play();
            }
            catch (Exception ex)
            {
                failure = ex.Message;
                done.Set();
            }
        }), DispatcherPriority.Send);

        if (!done.Wait(timeoutMs))
            failure = "media open timeout";

        try
        {
            dispatcher.Invoke(new Action(() =>
            {
                if ((!opened || !String.IsNullOrEmpty(failure)) && next != null)
                {
                    if (onOpened != null) next.MediaOpened -= onOpened;
                    if (onFailed != null) next.MediaFailed -= onFailed;
                    if (onEnded != null) next.MediaEnded -= onEnded;
                    if (Object.ReferenceEquals(player, next))
                    {
                        player = null;
                        currentOpened = false;
                        currentToken = "";
                        runtimeArmed = false;
                        pendingRuntimeFailure = "";
                    }
                    try { next.Stop(); } catch { }
                    try { next.Close(); } catch { }
                }
            }), DispatcherPriority.Send);
        }
        catch (Exception ex)
        {
            if (String.IsNullOrEmpty(failure)) failure = ex.Message;
        }

        error = failure;
        done.Dispose();
        return opened && String.IsNullOrEmpty(failure);
    }

    public void Arm(string token)
    {
        if (disposed || dispatcher == null) return;
        dispatcher.Invoke(new Action(() =>
        {
            if (player == null || !String.Equals(currentToken, token, StringComparison.Ordinal)) return;
            runtimeArmed = true;
            if (!String.IsNullOrEmpty(pendingRuntimeFailure))
            {
                var pending = pendingRuntimeFailure;
                pendingRuntimeFailure = "";
                EmitRuntimeFailure(token, pending);
            }
        }), DispatcherPriority.Send);
    }

    public void Pause() { Invoke(() => { if (player != null) player.Pause(); }); }
    public void Resume() { Invoke(() => { if (player != null) player.Play(); }); }
    public void Stop()
    {
        Invoke(() =>
        {
            currentOpened = false;
            currentToken = "";
            runtimeArmed = false;
            pendingRuntimeFailure = "";
            var current = player;
            player = null;
            if (current != null)
            {
                try { current.Stop(); } catch { }
                try { current.Close(); } catch { }
            }
        });
    }
    public void SetVolume(double value) { Invoke(() => { if (player != null) player.Volume = Math.Max(0.0, Math.Min(1.0, value)); }); }

    private void Invoke(Action action)
    {
        if (disposed || dispatcher == null) return;
        dispatcher.Invoke(action, DispatcherPriority.Send);
    }

    public void Dispose()
    {
        if (disposed) return;
        disposed = true;
        if (dispatcher != null)
        {
            try
            {
                dispatcher.Invoke(new Action(() =>
                {
                    currentOpened = false;
                    currentToken = "";
                    runtimeArmed = false;
                    pendingRuntimeFailure = "";
                    var current = player;
                    player = null;
                    if (current != null)
                    {
                        try { current.Stop(); } catch { }
                        try { current.Close(); } catch { }
                    }
                }), DispatcherPriority.Send);
            }
            catch { }
            try { dispatcher.BeginInvokeShutdown(DispatcherPriority.Send); } catch { }
        }
        if (thread != null && thread.IsAlive) thread.Join(2000);
        ready.Dispose();
    }
}
'@
$refs=@([System.Windows.Media.MediaPlayer].Assembly.Location,[System.Windows.Threading.Dispatcher].Assembly.Location)
Add-Type -TypeDefinition $source -ReferencedAssemblies $refs
$p=[RadioBalkanMediaHost]::new()
[Console]::Out.WriteLine('READY')
[Console]::Out.Flush()
} catch {
  $m=$_.Exception.ToString()
  $encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($m))
  [Console]::Out.WriteLine('BOOTERR '+$encoded)
  [Console]::Out.Flush()
  exit 2
}
while(($line=[Console]::In.ReadLine()) -ne $null){
  try {
    $sp=$line.Split(' ',4)
    $armToken=''
    switch($sp[0]){
      'PLAY' {
        $armToken=$sp[1]
        $u=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($sp[2]))
        $v=[double]::Parse($sp[3],[Globalization.CultureInfo]::InvariantCulture)
        $err=''
        if(-not $p.Play($u,$v,7000,$armToken,[ref]$err)){ throw $err }
      }
      'PAUSE' { $p.Pause() }
      'RESUME' { $p.Resume() }
      'STOP' { $p.Stop() }
      'VOLUME' {
        $v=[double]::Parse($sp[1],[Globalization.CultureInfo]::InvariantCulture)
        $p.SetVolume($v)
      }
      'PING' { }
      default { throw 'unknown command' }
    }
    [Console]::Out.WriteLine('OK')
    [Console]::Out.Flush()
    if($sp[0] -eq 'PLAY'){ $p.Arm($armToken) }
  } catch {
    $m=$_.Exception.Message
    if([String]::IsNullOrWhiteSpace($m)){ $m='audio command failed' }
    $encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($m))
    [Console]::Out.WriteLine('ERR '+$encoded)
    [Console]::Out.Flush()
  }
}
try { $p.Dispose() } catch {}`
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
	var audioStderr bytes.Buffer
	cmd.Stderr = &audioStderr
	if err = cmd.Start(); err != nil {
		_ = in.Close()
		return err
	}
	ack := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if eventSeq, detail, ok := parseAudioRuntimeFailure(line); ok {
				safeGo("audio-runtime-failure", func() {
					handleAudioBackendFailureForRequest(audioBackendWPF, eventSeq, detail)
				})
				continue
			}
			select {
			case ack <- line:
			case <-app.done:
				close(ack)
				return
			}
		}
		close(ack)
	}()
	startupTimer := time.NewTimer(audioEngineStartupTimeout)
	defer startupTimer.Stop()
	var supersedeTick <-chan time.Time
	var supersedeTicker *time.Ticker
	if reqSeq != 0 {
		supersedeTicker = time.NewTicker(50 * time.Millisecond)
		supersedeTick = supersedeTicker.C
		defer supersedeTicker.Stop()
	}

startupReady:
	for {
		select {
		case ready, ok := <-ack:
			if !ok || ready != "READY" {
				detail := strings.TrimSpace(audioStderr.String())
				if strings.HasPrefix(ready, "BOOTERR ") {
					if decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(ready, "BOOTERR "))); decodeErr == nil {
						detail = strings.TrimSpace(string(decoded))
					}
				}
				_ = in.Close()
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				if detail != "" {
					return fmt.Errorf("audio engine inicijalizacija: %s", detail)
				}
				return errors.New("audio engine se nije ispravno inicijalizirao")
			}
			break startupReady
		case <-supersedeTick:
			if !playRequestStillCurrent(reqSeq) {
				_ = in.Close()
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				return errPlayRequestSuperseded
			}
		case <-startupTimer.C:
			_ = in.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return errors.New("audio engine startup timeout")
		case <-app.done:
			_ = in.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return context.Canceled
		}
	}
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
		if app.audioBackend == audioBackendWPF {
			app.audioBackend = audioBackendNone
		}
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

func parseAudioRuntimeFailure(line string) (uint64, string, bool) {
	const prefix = "EVENT FAILED "
	if !strings.HasPrefix(line, prefix) {
		return 0, "", false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	parts := strings.SplitN(payload, " ", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	reqSeq, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, "", false
	}
	encoded := strings.TrimSpace(parts[1])
	if encoded == "" {
		return reqSeq, "media failed", true
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return reqSeq, "media failed", true
	}
	detail := strings.TrimSpace(string(decoded))
	if detail == "" {
		detail = "media failed"
	}
	return reqSeq, detail, true
}

func handleAudioBackendFailure(expected audioBackendKind, detail string) {
	handleAudioBackendFailureForRequest(expected, 0, detail)
}

func handleAudioBackendFailureForRequest(expected audioBackendKind, reqSeq uint64, detail string) {
	if shuttingDown() {
		return
	}
	if reqSeq != 0 {
		// A runtime event can be read immediately after the PLAY ACK while the
		// command goroutine still owns audioMu and has not committed WPF backend
		// ownership yet. Wait briefly for that commit, but reject the event as
		// soon as a newer playback generation wins.
		deadline := time.Now().Add(500 * time.Millisecond)
		for {
			app.mu.RLock()
			currentSeq := app.playSeq
			backend := app.audioBackend
			app.mu.RUnlock()
			if currentSeq != reqSeq {
				return
			}
			if backend == expected {
				break
			}
			if backend != audioBackendNone || time.Now().After(deadline) {
				return
			}
			select {
			case <-time.After(10 * time.Millisecond):
			case <-app.done:
				return
			}
		}
	}
	if strings.TrimSpace(detail) != "" {
		logError("audio-runtime-failure", errors.New(detail))
	}

	app.mu.Lock()
	if app.audioBackend != expected || (reqSeq != 0 && app.playSeq != reqSeq) {
		app.mu.Unlock()
		return
	}
	wasPlaying := app.playing
	wasStopped := app.audioStopped
	idx := currentStationIndexLocked()
	key := app.currentKey
	if key == "" && idx >= 0 && idx < len(app.stations) {
		key = stationKey(app.stations[idx])
	}
	if idx < 0 || key == "" {
		app.playing = false
		app.audioBackend = audioBackendNone
		app.metadataSeq++
		app.mu.Unlock()
		setStatus("Stanica trenutno nije dostupna")
		postUI()
		return
	}

	// A backend can fail while the UI is paused. Mark it unavailable but do not
	// auto-restart audio behind the user's back; the next Play/Resume will see
	// audioBackendNone and reconnect the selected station.
	if !wasPlaying {
		app.audioBackend = audioBackendNone
		app.metadataSeq++
		app.mu.Unlock()
		if !wasStopped {
			setStatus("Veza je privremeno prekinuta · pritisni Play za ponovno povezivanje")
			postUI()
		}
		return
	}

	now := time.Now()
	recoverySeq := app.playSeq
	allowRecover := !app.audioRecovering && (app.lastAudioFailure.IsZero() || now.Sub(app.lastAudioFailure) > 20*time.Second)
	app.playing = false
	app.audioBackend = audioBackendNone
	app.metadataSeq++
	if allowRecover {
		app.audioRecovering = true
		app.lastAudioFailure = now
	}
	app.mu.Unlock()

	if !allowRecover {
		setStatus("Veza je privremeno prekinuta · klikni Play za ponovni pokušaj")
		postUI()
		return
	}

	setStatus("Veza je privremeno prekinuta · pokušavam ponovno povezivanje…")
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
		// A Stop, Next or newer station click always wins over delayed recovery.
		if app.playSeq != recoverySeq || app.currentKey != key || app.audioStopped {
			app.audioRecovering = false
			app.mu.Unlock()
			return
		}
		app.audioRecovering = false
		app.mu.Unlock()
		playStationByKey(key, idx)
	})
}

func resetAudioEngineLocked() {
	in := app.audioIn
	cmd := app.audioCmd
	app.audioIn = nil
	app.audioCmd = nil
	app.audioAck = nil
	if in != nil {
		_ = in.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func audioPlayCommand(reqSeq uint64, encoded string, volume float64) string {
	return fmt.Sprintf("PLAY %d %s %.2f", reqSeq, encoded, volume)
}

func audioCommandTimeout(line string) time.Duration {
	if strings.HasPrefix(line, "PLAY ") {
		return 9 * time.Second
	}
	return 3 * time.Second
}

var errPlayRequestSuperseded = errors.New("play request superseded")

func playRequestStillCurrent(reqSeq uint64) bool {
	if reqSeq == 0 {
		return true
	}
	app.mu.RLock()
	current := app.playSeq == reqSeq
	app.mu.RUnlock()
	return current
}

func audioAckResultLocked(reply string, ok bool) error {
	if !ok {
		resetAudioEngineLocked()
		return errors.New("audio engine je prekinut")
	}
	if reply == "OK" {
		return nil
	}
	if strings.HasPrefix(reply, "ERR ") {
		if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(reply, "ERR "))); err == nil && len(decoded) > 0 {
			return fmt.Errorf("audio engine: %s", strings.TrimSpace(string(decoded)))
		}
	}
	return errors.New("audio engine nije prihvatio naredbu")
}

func waitAudioAckLocked(timeout time.Duration) error {
	return waitAudioAckLockedForRequest(timeout, 0)
}

func waitAudioAckLockedForRequest(timeout time.Duration, reqSeq uint64) error {
	if app.audioAck == nil {
		return errors.New("audio engine nema kanal potvrde")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var staleTick <-chan time.Time
	var staleTicker *time.Ticker
	if reqSeq != 0 {
		staleTicker = time.NewTicker(50 * time.Millisecond)
		staleTick = staleTicker.C
		defer staleTicker.Stop()
	}

	for {
		select {
		case reply, ok := <-app.audioAck:
			return audioAckResultLocked(reply, ok)
		case <-staleTick:
			if !playRequestStillCurrent(reqSeq) {
				// Kill the helper/channel together so a late ACK can never satisfy
				// the newer station request waiting behind this one.
				resetAudioEngineLocked()
				setAudioBackend(audioBackendNone)
				return errPlayRequestSuperseded
			}
		case <-timer.C:
			// A late ACK must never be consumed by the next command. Discard the
			// entire helper process/channel before the caller falls back or retries.
			resetAudioEngineLocked()
			return errors.New("audio engine nije odgovorio na vrijeme")
		case <-app.done:
			return context.Canceled
		}
	}
}

func audioSend(line string) error {
	return audioSendForRequest(line, 0)
}

func shouldStopMCIForAudioCommand(line string, backend audioBackendKind) bool {
	return backend == audioBackendMCI && strings.HasPrefix(strings.TrimSpace(line), "PLAY ")
}

func audioSendForRequest(line string, reqSeq uint64) error {
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	if !playRequestStillCurrent(reqSeq) {
		return errPlayRequestSuperseded
	}
	playCommand := strings.HasPrefix(strings.TrimSpace(line), "PLAY ")
	cleanupFailedPlayLocked := func() {
		if !playCommand {
			return
		}
		resetAudioEngineLocked()
		setAudioBackend(audioBackendNone)
	}
	// A newer WPF PLAY must replace an older MCI fallback stream. Otherwise the
	// two backends can remain audible at the same time after rapid station changes.
	if shouldStopMCIForAudioCommand(line, currentAudioBackend()) {
		audioStopMCI()
		setAudioBackend(audioBackendNone)
	}
	if err := startAudioEngineLockedForRequest(reqSeq); err != nil {
		cleanupFailedPlayLocked()
		return err
	}
	if !playRequestStillCurrent(reqSeq) {
		cleanupFailedPlayLocked()
		return errPlayRequestSuperseded
	}
	if app.audioIn == nil {
		cleanupFailedPlayLocked()
		return errors.New("audio engine nije dostupan")
	}
	if _, err := io.WriteString(app.audioIn, line+"\n"); err != nil {
		cleanupFailedPlayLocked()
		return err
	}
	if err := waitAudioAckLockedForRequest(audioCommandTimeout(line), reqSeq); err != nil {
		// PLAY failures are fully cleaned up while audioMu is still held. The
		// caller must never tear down backend state after this function returns.
		cleanupFailedPlayLocked()
		return err
	}
	if playCommand {
		// Commit backend ownership while audioMu is still held. A newer request can
		// advance playSeq concurrently, but it cannot enter another backend command
		// until this lock is released.
		if !playRequestStillCurrent(reqSeq) {
			cleanupFailedPlayLocked()
			return errPlayRequestSuperseded
		}
		setAudioBackend(audioBackendWPF)
	}
	return nil
}
func audioSendExisting(line string) error {
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	if app.audioIn == nil {
		return errors.New("audio engine nije pokrenut")
	}
	if _, err := io.WriteString(app.audioIn, line+"\n"); err != nil {
		resetAudioEngineLocked()
		return err
	}
	return waitAudioAckLocked(audioCommandTimeout(line))
}
func warmAudioEngine() {
	if shuttingDown() {
		return
	}
	app.audioMu.Lock()
	err := startAudioEngineLocked()
	app.audioMu.Unlock()
	if err != nil && !shuttingDown() {
		logError("audio-warmup", err)
	}
}

func audioShutdown() {
	deadline := time.Now().Add(750 * time.Millisecond)
	for !app.audioMu.TryLock() {
		if time.Now().After(deadline) {
			logError("audio-shutdown", errors.New("audio backend lock remained busy; continuing bounded shutdown"))
			setAudioBackend(audioBackendNone)
			audioStopMCI()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if app.audioIn != nil {
		_, _ = io.WriteString(app.audioIn, "STOP\n")
	}
	resetAudioEngineLocked()
	audioStopMCI()
	setAudioBackend(audioBackendNone)
	app.audioMu.Unlock()
}

func audioPlay(raw string) error {
	return audioPlayRequest(raw, 0)
}

func audioPlayRequest(raw string, reqSeq uint64) error {
	if !playRequestStillCurrent(reqSeq) {
		return errPlayRequestSuperseded
	}
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
	if err := audioSendForRequest(audioPlayCommand(reqSeq, enc, vol), reqSeq); err == nil {
		return nil
	} else {
		if errors.Is(err, errPlayRequestSuperseded) {
			// The stale request must not mutate backend state after audioMu was
			// released; a newer request may already be waiting to take ownership.
			return err
		}
		logError("audio-engine", err)
		// audioSendForRequest already cleaned the failed WPF PLAY while holding
		// audioMu; do not perform post-unlock cleanup here.
	}

	if !playRequestStillCurrent(reqSeq) {
		return errPlayRequestSuperseded
	}

	// Serialize the complete MCI fallback lifecycle with all WPF/backend commands.
	// This prevents a superseded MCI request from closing the shared "radio" alias
	// after a newer request has already started.
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	abortStaleMCI := func() bool {
		if playRequestStillCurrent(reqSeq) {
			return false
		}
		audioStopMCI()
		setAudioBackend(audioBackendNone)
		return true
	}
	if abortStaleMCI() {
		return errPlayRequestSuperseded
	}
	audioStopMCI()
	setAudioBackend(audioBackendNone)
	cleanURL := strings.ReplaceAll(raw, "\"", "")
	openCommands := []string{
		fmt.Sprintf("open \"%s\" alias radio", cleanURL),
		fmt.Sprintf("open \"%s\" type mpegvideo alias radio", cleanURL),
	}
	var openErr error
	for _, command := range openCommands {
		if abortStaleMCI() {
			return errPlayRequestSuperseded
		}
		if e := mci(command); e == nil {
			openErr = nil
			break
		} else {
			openErr = e
			audioStopMCI()
		}
	}
	if abortStaleMCI() {
		return errPlayRequestSuperseded
	}
	if openErr != nil {
		setAudioBackend(audioBackendNone)
		return openErr
	}
	if e := mci("play radio"); e != nil {
		audioStopMCI()
		setAudioBackend(audioBackendNone)
		return e
	}
	if abortStaleMCI() {
		return errPlayRequestSuperseded
	}
	_ = mci(fmt.Sprintf("setaudio radio volume to %d", volume*10))
	if abortStaleMCI() {
		return errPlayRequestSuperseded
	}
	setAudioBackend(audioBackendMCI)
	return nil
}

func mciQueryExisting(cmd string) (string, error) {
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	if currentAudioBackend() != audioBackendMCI {
		return "", errors.New("MCI backend nije aktivan")
	}
	return mciQuery(cmd)
}

func audioPause() error {
	switch currentAudioBackend() {
	case audioBackendWPF:
		return audioSendExisting("PAUSE")
	case audioBackendMCI:
		_, err := mciQueryExisting("pause radio")
		return err
	default:
		return errors.New("audio backend nije aktivan")
	}
}

func audioResume() error {
	switch currentAudioBackend() {
	case audioBackendWPF:
		return audioSendExisting("RESUME")
	case audioBackendMCI:
		_, err := mciQueryExisting("resume radio")
		return err
	default:
		return errors.New("audio backend nije aktivan")
	}
}

func audioSetVolume(v int) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	switch currentAudioBackend() {
	case audioBackendWPF:
		if err := audioSendExisting(fmt.Sprintf("VOLUME %.2f", float64(v)/100.0)); err != nil {
			logError("audio-volume", err)
			// A failed WPF control may have reset/killed the helper. Recover now
			// instead of leaving a stale WPF backend that the watchdog trusts.
			handleAudioBackendFailure(audioBackendWPF, "volume control failed")
		}
	case audioBackendMCI:
		_, _ = mciQueryExisting(fmt.Sprintf("setaudio radio volume to %d", v*10))
	}
}

func audioStopForRequest(reqSeq uint64) {
	app.audioMu.Lock()
	defer app.audioMu.Unlock()
	// Stop is dispatched asynchronously from the UI thread. If a newer Play/Next
	// already advanced playSeq before this goroutine obtained the backend lock,
	// the old Stop must not silence the newer stream.
	if !playRequestStillCurrent(reqSeq) {
		return
	}
	switch currentAudioBackend() {
	case audioBackendWPF:
		if app.audioIn != nil {
			if _, err := io.WriteString(app.audioIn, "STOP\n"); err != nil {
				resetAudioEngineLocked()
			} else {
				_ = waitAudioAckLockedForRequest(audioCommandTimeout("STOP"), reqSeq)
			}
		}
	case audioBackendMCI:
		audioStopMCI()
	default:
		return
	}
	setAudioBackend(audioBackendNone)
}

func audioStop() {
	audioStopForRequest(0)
}

func audioStopMCI() {
	_ = mci("stop radio")
	_ = mci("close radio")
}

func mci(cmd string) error {
	_, err := mciQuery(cmd)
	return err
}

func mciQuery(cmd string) (string, error) {
	buf := make([]uint16, 256)
	r, _, _ := procMciSendString.Call(uintptr(unsafe.Pointer(u16(cmd))), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if r != 0 {
		detail := make([]uint16, 256)
		ok, _, _ := procMciGetErrorString.Call(r, uintptr(unsafe.Pointer(&detail[0])), uintptr(len(detail)))
		message := strings.TrimSpace(syscall.UTF16ToString(detail))
		if ok != 0 && message != "" {
			return "", fmt.Errorf("MCI greška %d: %s", r, message)
		}
		return "", fmt.Errorf("MCI greška %d", r)
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf)), nil
}

func mciModeHealthy(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "playing", "paused", "seeking":
		return true
	default:
		return false
	}
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
	runtimeTestTrace("signal-shutdown-enter")
	app.closeOnce.Do(func() {
		if app.cancel != nil {
			app.cancel()
		}
		if app.done != nil {
			close(app.done)
		}
		runtimeTestTrace("signal-shutdown-after-cancel")
		app.saveMu.Lock()
		if app.saveTimer != nil {
			app.saveTimer.Stop()
			app.saveTimer = nil
		}
		app.saveMu.Unlock()
		// The search debounce callback checks shuttingDown() before touching UI
		// state. Do not block the UI thread acquiring app.mu during shutdown.
	})
	runtimeTestTrace("signal-shutdown-exit")
}

func prepareShutdown() {
	runtimeTestTrace("prepare-shutdown-enter")
	app.shutdownOnce.Do(func() {
		signalShutdown()
		finished := make(chan struct{})
		go func() {
			runtimeTestTrace("cleanup-goroutine-enter")
			defer func() {
				runtimeTestTrace("cleanup-goroutine-exit")
				close(finished)
			}()
			saveState()
			runtimeTestTrace("cleanup-after-save-state")
			audioShutdown()
			runtimeTestTrace("cleanup-after-audio-shutdown")
		}()
		select {
		case <-finished:
			runtimeTestTrace("prepare-cleanup-finished")
		case <-time.After(2500 * time.Millisecond):
			logError("shutdown-timeout", errors.New("finalni state/audio cleanup prekoračio je 2.5 s"))
			runtimeTestTrace("prepare-cleanup-timeout")
		}
	})
	runtimeTestTrace("prepare-shutdown-exit")
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
	list = trimStationCatalog(dedupeStations(filterSupportedStations(list)), regionalCatalogLimit+supplementalCatalogLimit)
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
		return trimStationCatalog(dedupeStations(filterSupportedStations(c.Stations)), regionalCatalogLimit+supplementalCatalogLimit)
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

func passwordDialog(parent syscall.Handle, title, prompt string) (string, bool) {
	if err := ensureInputDialogClass(); err != nil {
		logError("password-dialog-class", err)
		return "", false
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
		logError("password-dialog-create", fmt.Errorf("CreateWindowExW: %v", e))
		return "", false
	}
	st.hwnd = syscall.Handle(h)
	enableImmersiveDark(st.hwnd)

	label, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("STATIC"))), uintptr(unsafe.Pointer(u16(prompt))), WS_CHILD|WS_VISIBLE, 24, 22, 566, 92, h, 0, hInst, 0)
	edit, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("EDIT"))), uintptr(unsafe.Pointer(u16(""))), WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL|ES_PASSWORD, 24, 122, 566, 34, h, 2201, hInst, 0)
	cancelBtn, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("BUTTON"))), uintptr(unsafe.Pointer(u16("Odustani"))), WS_CHILD|WS_VISIBLE|WS_TABSTOP, 382, 180, 96, 38, h, 2102, hInst, 0)
	okBtn, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(u16("BUTTON"))), uintptr(unsafe.Pointer(u16("Prijavi se"))), WS_CHILD|WS_VISIBLE|WS_TABSTOP, 488, 180, 102, 38, h, 2101, hInst, 0)
	if edit == 0 || label == 0 || cancelBtn == 0 || okBtn == 0 {
		logError("password-dialog-controls", errors.New("nije moguće izraditi sve kontrole dijaloga"))
		procDestroyWindow.Call(h)
		return "", false
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

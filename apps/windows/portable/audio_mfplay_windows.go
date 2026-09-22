//go:build windows

package main

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	mfVersion             = 0x00020070
	coinitApartmentThread = 0x2
	coinitDisableOLE1DDE  = 0x4
)

var (
	mfplatDLL              = syscall.NewLazyDLL("mfplat.dll")
	mfplayDLL              = syscall.NewLazyDLL("mfplay.dll")
	ole32DLL               = syscall.NewLazyDLL("ole32.dll")
	oleaut32DLL             = syscall.NewLazyDLL("oleaut32.dll")
	procMFStartup          = mfplatDLL.NewProc("MFStartup")
	procMFShutdown         = mfplatDLL.NewProc("MFShutdown")
	procMFPCreateMediaPlayer = mfplayDLL.NewProc("MFPCreateMediaPlayer")
	procCoInitializeEx     = ole32DLL.NewProc("CoInitializeEx")
	procCoUninitialize     = ole32DLL.NewProc("CoUninitialize")
	procDispCallFunc        = oleaut32DLL.NewProc("DispCallFunc")
)

func hresultError(name string, hr uintptr) error {
	if int32(hr) >= 0 {
		return nil
	}
	return fmt.Errorf("%s failed: HRESULT 0x%08X", name, uint32(hr))
}

func comMethod(object unsafe.Pointer, index int) (uintptr, error) {
	if object == nil {
		return 0, errors.New("COM object is nil")
	}
	vtable := *(**[64]uintptr)(object)
	if vtable == nil {
		return 0, errors.New("COM vtable is nil")
	}
	if index < 0 || index >= len(vtable) {
		return 0, fmt.Errorf("COM method index %d is out of range", index)
	}
	method := vtable[index]
	if method == 0 {
		return 0, fmt.Errorf("COM method %d is nil", index)
	}
	return method, nil
}

func comCall(object unsafe.Pointer, index int, args ...uintptr) error {
	method, err := comMethod(object, index)
	if err != nil {
		return err
	}
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, uintptr(object))
	callArgs = append(callArgs, args...)
	hr, _, _ := syscall.SyscallN(method, callArgs...)
	return hresultError(fmt.Sprintf("COM method %d", index), hr)
}

// VARIANTARG is 24 bytes on Windows/amd64 because its value union can contain
// a two-pointer BRECORD. The scalar payload still starts at byte offset 8.
type variantArg struct {
	VT        uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	Value     uint64
	Tail      uint64
}

const (
	vtI4      = 3
	vtR4      = 4
	ccStdcall = 4
)

func comCallFloat32(object unsafe.Pointer, index int, value float32) error {
	if object == nil {
		return errors.New("COM object is nil")
	}
	arg := variantArg{VT: vtR4, Value: uint64(math.Float32bits(value))}
	argTypes := [1]uint16{vtR4}
	argPointers := [1]unsafe.Pointer{unsafe.Pointer(&arg)}
	var result variantArg

	hr, _, _ := procDispCallFunc.Call(
		uintptr(object),
		uintptr(index)*unsafe.Sizeof(uintptr(0)),
		ccStdcall,
		vtI4,
		1,
		uintptr(unsafe.Pointer(&argTypes[0])),
		uintptr(unsafe.Pointer(&argPointers[0])),
		uintptr(unsafe.Pointer(&result)),
	)
	if err := hresultError("DispCallFunc", hr); err != nil {
		return err
	}
	return hresultError(fmt.Sprintf("COM float method %d", index), uintptr(uint32(result.Value&0xffffffff)))
}

func comRelease(object unsafe.Pointer) {
	if object == nil {
		return
	}
	if method, err := comMethod(object, 2); err == nil {
		_, _, _ = syscall.SyscallN(method, uintptr(object))
	}
}

// mfplaySmokeOpen proves that Windows can open HTTP media through Media
// Foundation entirely in-process. It deliberately lives on one locked OS
// thread because both COM and Media Foundation have thread-affinity rules.
func mfplaySmokeOpen(rawURL string) error {
	if rawURL == "" {
		return errors.New("MFPlay URL is empty")
	}
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThread|coinitDisableOLE1DDE)
		if err := hresultError("CoInitializeEx", hr); err != nil {
			result <- err
			return
		}
		defer procCoUninitialize.Call()

		hr, _, _ = procMFStartup.Call(mfVersion, 0)
		if err := hresultError("MFStartup", hr); err != nil {
			result <- err
			return
		}
		defer procMFShutdown.Call()

		var player unsafe.Pointer
		hr, _, _ = procMFPCreateMediaPlayer.Call(0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&player)))
		if err := hresultError("MFPCreateMediaPlayer", hr); err != nil {
			result <- err
			return
		}
		if player == nil {
			result <- errors.New("MFPCreateMediaPlayer returned nil player")
			return
		}
		defer func() {
			_ = comCall(player, 38) // IMFPMediaPlayer::Shutdown
			comRelease(player)
		}()

		urlPtr, err := syscall.UTF16PtrFromString(rawURL)
		if err != nil {
			result <- err
			return
		}

		var item unsafe.Pointer
		createMethod, err := comMethod(player, 14) // CreateMediaItemFromURL
		if err != nil {
			result <- err
			return
		}
		hr, _, _ = syscall.SyscallN(
			createMethod,
			uintptr(player),
			uintptr(unsafe.Pointer(urlPtr)),
			1, // synchronous
			0,
			uintptr(unsafe.Pointer(&item)),
		)
		if err := hresultError("IMFPMediaPlayer.CreateMediaItemFromURL", hr); err != nil {
			result <- err
			return
		}
		if item == nil {
			result <- errors.New("CreateMediaItemFromURL returned nil item")
			return
		}
		defer comRelease(item)

		if err := comCall(player, 16, uintptr(item)); err != nil { // SetMediaItem
			result <- err
			return
		}
		if err := comCallFloat32(player, 20, 0.25); err != nil { // SetVolume
			result <- err
			return
		}
		if err := comCall(player, 3); err != nil { // Play
			result <- err
			return
		}
		if err := comCall(player, 4); err != nil { // Pause
			result <- err
			return
		}
		if err := comCall(player, 3); err != nil { // Resume
			result <- err
			return
		}
		if err := comCall(player, 5); err != nil { // Stop
			result <- err
			return
		}
		result <- nil
	}()
	return <-result
}

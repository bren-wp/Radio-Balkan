//go:build windows

package main

import (
	"errors"
	"fmt"
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
	procMFStartup          = mfplatDLL.NewProc("MFStartup")
	procMFShutdown         = mfplatDLL.NewProc("MFShutdown")
	procMFPCreateMediaPlayer = mfplayDLL.NewProc("MFPCreateMediaPlayer")
	procCoInitializeEx     = ole32DLL.NewProc("CoInitializeEx")
	procCoUninitialize     = ole32DLL.NewProc("CoUninitialize")
)

func hresultError(name string, hr uintptr) error {
	if int32(hr) >= 0 {
		return nil
	}
	return fmt.Errorf("%s failed: HRESULT 0x%08X", name, uint32(hr))
}

func comMethod(object uintptr, index uintptr) (uintptr, error) {
	if object == 0 {
		return 0, errors.New("COM object is nil")
	}
	vtable := *(*uintptr)(unsafe.Pointer(object))
	if vtable == 0 {
		return 0, errors.New("COM vtable is nil")
	}
	method := *(*uintptr)(unsafe.Pointer(vtable + index*unsafe.Sizeof(uintptr(0))))
	if method == 0 {
		return 0, fmt.Errorf("COM method %d is nil", index)
	}
	return method, nil
}

func comCall(object uintptr, index uintptr, args ...uintptr) error {
	method, err := comMethod(object, index)
	if err != nil {
		return err
	}
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, object)
	callArgs = append(callArgs, args...)
	hr, _, _ := syscall.SyscallN(method, callArgs...)
	return hresultError(fmt.Sprintf("COM method %d", index), hr)
}

func comRelease(object uintptr) {
	if object == 0 {
		return
	}
	if method, err := comMethod(object, 2); err == nil {
		_, _, _ = syscall.SyscallN(method, object)
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

		var player uintptr
		hr, _, _ = procMFPCreateMediaPlayer.Call(0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&player)))
		if err := hresultError("MFPCreateMediaPlayer", hr); err != nil {
			result <- err
			return
		}
		if player == 0 {
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

		var item uintptr
		createMethod, err := comMethod(player, 14) // CreateMediaItemFromURL
		if err != nil {
			result <- err
			return
		}
		hr, _, _ = syscall.SyscallN(
			createMethod,
			player,
			uintptr(unsafe.Pointer(urlPtr)),
			1, // synchronous
			0,
			uintptr(unsafe.Pointer(&item)),
		)
		if err := hresultError("IMFPMediaPlayer.CreateMediaItemFromURL", hr); err != nil {
			result <- err
			return
		}
		if item == 0 {
			result <- errors.New("CreateMediaItemFromURL returned nil item")
			return
		}
		defer comRelease(item)

		if err := comCall(player, 16, item); err != nil { // SetMediaItem
			result <- err
			return
		}
		volume := float32(0)
		volumeBits := *(*uint32)(unsafe.Pointer(&volume))
		if err := comCall(player, 20, uintptr(volumeBits)); err != nil { // SetVolume
			result <- err
			return
		}
		if err := comCall(player, 3); err != nil { // Play
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

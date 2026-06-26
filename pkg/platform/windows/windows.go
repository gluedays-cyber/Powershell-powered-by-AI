package windows

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	setConsoleMode         = kernel32.NewProc("SetConsoleMode")
	getStdHandle           = kernel32.NewProc("GetStdHandle")
	getConsoleMode         = kernel32.NewProc("GetConsoleMode")
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
)

type DATA_BLOB struct {
	CbData uint32
	PbData *byte
}

type WinAPI interface {
	EnableVTMode() error
	CryptUnprotectData(data []byte) ([]byte, error)
	CryptProtectData(data []byte) ([]byte, error)
}

type windowsAPI struct{}

var Current WinAPI = &windowsAPI{}

func (w *windowsAPI) EnableVTMode() error {
	const stdOutputHandle = uint32(0xfffffff5)
	handle, _, err := getStdHandle.Call(uintptr(stdOutputHandle))
	if handle == 0 {
		if err != nil && err.Error() != "The operation completed successfully." {
			return fmt.Errorf("failed to get std handle: %w", err)
		}
		return fmt.Errorf("failed to get std handle (returned NULL)")
	}

	var mode uint32
	r, _, err := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		if err != nil && err.Error() != "The operation completed successfully." {
			return fmt.Errorf("failed to get console mode: %w", err)
		}
		return fmt.Errorf("failed to get console mode (returned 0)")
	}

	mode |= 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
	r, _, err = setConsoleMode.Call(handle, uintptr(mode))
	if r == 0 {
		if err != nil && err.Error() != "The operation completed successfully." {
			return fmt.Errorf("failed to set console mode: %w", err)
		}
		return fmt.Errorf("failed to set console mode (returned 0)")
	}
	return nil
}

func (w *windowsAPI) CryptUnprotectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var in DATA_BLOB
	var out DATA_BLOB

	in.CbData = uint32(len(data))
	in.PbData = &data[0]

	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, fmt.Errorf("Windows DPAPI decryption failed: %w", err)
		}
		return nil, fmt.Errorf("Windows DPAPI decryption failed (returned 0)")
	}
	if out.PbData == nil || out.CbData == 0 {
		return nil, fmt.Errorf("Windows DPAPI decryption returned empty data or nil pointer")
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.PbData)))

	result := make([]byte, out.CbData)
	copy(result, unsafe.Slice(out.PbData, out.CbData))
	return result, nil
}

func (w *windowsAPI) CryptProtectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var in DATA_BLOB
	var out DATA_BLOB

	in.CbData = uint32(len(data))
	in.PbData = &data[0]

	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		if err != nil && err.Error() != "The operation completed successfully." {
			return nil, fmt.Errorf("Windows DPAPI encryption failed: %w", err)
		}
		return nil, fmt.Errorf("Windows DPAPI encryption failed (returned 0)")
	}
	if out.PbData == nil || out.CbData == 0 {
		return nil, fmt.Errorf("Windows DPAPI encryption returned empty data or nil pointer")
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.PbData)))

	result := make([]byte, out.CbData)
	copy(result, unsafe.Slice(out.PbData, out.CbData))
	return result, nil
}

func EnableVTMode() error {
	return Current.EnableVTMode()
}

func CryptUnprotectData(data []byte) ([]byte, error) {
	return Current.CryptUnprotectData(data)
}

func CryptProtectData(data []byte) ([]byte, error) {
	return Current.CryptProtectData(data)
}

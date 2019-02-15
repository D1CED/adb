package adb

import "github.com/pkg/errors"

// Sentinel error values used by this package
var (
	ErrPackageNotExist    = errors.New("package not exist")
	ErrAssertionViolation = errors.New("AssertionError")
	ErrParsing            = errors.New("ParseError")
	// The server was not available on the requested port.
	ErrServerNotAvailable = errors.New("ServerNotAvailable")
	// General network error communicating with the server.
	ErrNetworkIO = errors.New("NetworkError")
	// The connection to the server was reset in the middle of an operation. Server probably died.
	ErrConnectionReset = errors.New("ConnectionResetError")
	// The server returned an error message, but we couldn't parse it.
	ErrError = errors.New("ADBError")
	// The server returned a "device not found" error.
	ErrDeviceNotFound = errors.New("DeviceNotFound")
	// Tried to perform an operation on a path that doesn't exist on the device.
	ErrFileNotExist = errors.New("FileNoExistError")
)

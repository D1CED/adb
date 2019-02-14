package adb

import "github.com/pkg/errors"

var (
	ErrPackageNotExist = errors.New("package not exist")
	AssertionError     = errors.New("AssertionError")
	ParseError         = errors.New("ParseError")
	// The server was not available on the requested port.
	ServerNotAvailable = errors.New("ServerNotAvailable")
	// General network error communicating with the server.
	NetworkError = errors.New("NetworkError")
	// The connection to the server was reset in the middle of an operation. Server probably died.
	ConnectionResetError = errors.New("ConnectionResetError")
	// The server returned an error message, but we couldn't parse it.
	AdbError = errors.New("AdbError")
	// The server returned a "device not found" error.
	DeviceNotFound = errors.New("DeviceNotFound")
	// Tried to perform an operation on a path that doesn't exist on the device.
	FileNoExistError = errors.New("FileNoExistError")
)

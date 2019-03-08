package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/cheggaaa/pb"
	"github.com/pkg/errors"

	"github.com/d1ced/adb"
)

const StdIoFilename = "-"

var (
	serial = kingpin.Flag("serial", "Connect to device by serial number.").
		Short('s').
		String()

	shellCommand    = kingpin.Command("shell", "Run a shell command on the device.")
	shellCommandArg = shellCommand.Arg("command", "Command to run on device.").
			Strings()

	devicesCommand  = kingpin.Command("devices", "List devices.")
	devicesLongFlag = devicesCommand.Flag("long", "Include extra detail about devices.").
			Short('l').
			Bool()

	forwardCommand  = kingpin.Command("forward", "Forward")
	forwardListFlag = forwardCommand.Flag("list", "List forwards").
			Short('l').
			Bool()

	pullCommand      = kingpin.Command("pull", "Pull a file from the device.")
	pullProgressFlag = pullCommand.Flag("progress", "Show progress.").
				Short('p').
				Bool()
	pullRemoteArg = pullCommand.Arg("remote", "Path of source file on device.").
			Required().
			String()
	pullLocalArg = pullCommand.Arg("local", "Path of destination file. If -, will write to stdout.").
			String()

	pushCommand      = kingpin.Command("push", "Push a file to the device.")
	pushProgressFlag = pushCommand.Flag("progress", "Show progress.").
				Short('p').
				Bool()
	pushLocalArg = pushCommand.Arg("local", "Path of source file. If -, will read from stdin.").
			Required().
			String()
	pushRemoteArg = pushCommand.Arg("remote", "Path of destination file on device.").
			Required().
			String()
)

type userError struct{ error }

func (ue userError) Error() string {
	kingpin.Usage()
	return ue.error.Error()
}

func main() {
	client, err := adb.NewDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	switch kingpin.Parse() {
	case "devices":
		err = listDevices(client, *devicesLongFlag)
	case "shell":
		err = runShellCommand(client, *shellCommandArg, *serial)
	case "pull":
		err = pull(client, *pullProgressFlag, *pullRemoteArg, *pullLocalArg, *serial)
	case "push":
		err = push(client, *pushProgressFlag, *pushLocalArg, *pushRemoteArg, *serial)
	case "forward":
		err = forward(client, *forwardListFlag, *serial)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func listDevices(client *adb.Server, long bool) error {
	//client := adb.New(server)
	devices, err := client.ListDevices()
	if err != nil {
		return err
	}

	for _, device := range devices {
		if long {
			if !device.IsUSB() {
				fmt.Printf("%s\tproduct:%s model:%s device:%s\n",
					device.Serial, device.Product, device.Model, device)
			} else {
				fmt.Printf("%s\tusb:%s product:%s model:%s device:%s\n",
					device.Serial, device.USB, device.Product, device.Model, device)
			}
		} else {
			fmt.Println(device.Serial)
		}
	}

	return nil
}

func runShellCommand(client *adb.Server, commandAndArgs []string, deviceSerial string) error {
	if len(commandAndArgs) == 0 {
		return userError{fmt.Errorf("no command")}
	}

	command := commandAndArgs[0]
	var args []string

	if len(commandAndArgs) > 1 {
		args = commandAndArgs[1:]
	}

	device := client.Device(deviceSerial)
	output, err := device.Command(command, args...).Output()
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func forward(client *adb.Server, listForwards bool, deviceSerial string) error {
	device := client.Device(deviceSerial)
	fws, err := device.ForwardList()
	if err != nil {
		return err
	}
	for _, fw := range fws {
		fmt.Printf("%s %v %v\n", device, fw[0], fw[1])
	}
	return nil
}

func pull(client *adb.Server, showProgress bool, remotePath, localPath string, deviceSerial string) error {
	if remotePath == "" {
		return userError{fmt.Errorf("must specify remote file")}
	}

	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	device := client.Device(deviceSerial)

	info, err := device.Stat(remotePath)
	if _, ok := errors.Cause(err).(*os.PathError); ok {
		return fmt.Errorf("remote file does not exist: %v", remotePath)
	} else if err != nil {
		return fmt.Errorf("failed reading remote file %s: %s", remotePath, err)
	}

	remoteFile, err := device.ReadFile(remotePath)
	if err != nil {
		return fmt.Errorf("failed opening remote file %s: %v", remotePath, errors.Cause(err))
	}
	defer remoteFile.Close()

	var localFile io.WriteCloser
	if localPath == StdIoFilename {
		localFile = os.Stdout
	} else {
		localFile, err = os.Create(localPath)
		if err != nil {
			return fmt.Errorf("error opening local file %s: %v", localPath, err)
		}
	}
	defer localFile.Close()

	if err := copyWithProgressAndStats(localFile, remoteFile, int(info.Size()), showProgress); err != nil {
		return fmt.Errorf("failed pulling file: %v", err)
	}
	return nil
}

func push(client *adb.Server, showProgress bool, localPath, remotePath string, deviceSerial string) error {
	if remotePath == "" {
		return userError{fmt.Errorf("must specify remote file")}
	}

	var (
		localFile io.ReadCloser
		size      int
		perms     os.FileMode
		mtime     time.Time
	)
	if localPath == "" || localPath == StdIoFilename {
		localFile = os.Stdin
		// 0 size will hide the progress bar.
		perms = os.FileMode(0660)
		mtime = time.Time{}
	} else {
		var err error
		localFile, err = os.Open(localPath)
		if err != nil {
			return fmt.Errorf("failed opening local file %s: %v", localPath, err)
		}
		info, err := os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("failed reading local file %s: %v", localPath, err)
		}
		size = int(info.Size())
		perms = info.Mode().Perm()
		mtime = info.ModTime()
	}
	defer localFile.Close()

	device := client.Device(deviceSerial)
	writer, err := device.OpenWrite(remotePath, perms, mtime)
	if err != nil {
		return fmt.Errorf("failed opening remote file %s: %v", remotePath, err)
	}
	defer writer.Close()

	if err := copyWithProgressAndStats(writer, localFile, size, showProgress); err != nil {
		return fmt.Errorf("failed pushing file: %v", err)
	}
	return nil
}

// copyWithProgressAndStats copies src to dst.
// If showProgress is true and size is positive, a progress bar is shown.
// After copying, final stats about the transfer speed and size are shown.
// Progress and stats are printed to stderr.
func copyWithProgressAndStats(dst io.Writer, src io.Reader, size int, showProgress bool) error {
	var progress *pb.ProgressBar
	if showProgress && size > 0 {
		progress = pb.New(size)
		// Write to stderr in case dst is stdout.
		progress.Output = os.Stderr
		progress.ShowSpeed = true
		progress.ShowPercent = true
		progress.ShowTimeLeft = true
		progress.SetUnits(pb.U_BYTES)
		progress.Start()
		dst = io.MultiWriter(dst, progress)
	}

	startTime := time.Now()
	copied, err := io.Copy(dst, src)

	if progress != nil {
		progress.Finish()
	}

	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok && errno == syscall.EPIPE {
			// Pipe closed. Handle this like an EOF.
			err = nil
		}
	}
	if err != nil {
		return err
	}

	duration := time.Now().Sub(startTime)
	rate := int64(float64(copied) / duration.Seconds())
	fmt.Fprintf(os.Stderr, "%d B/s (%d bytes in %s)\n", rate, copied, duration)

	return nil
}

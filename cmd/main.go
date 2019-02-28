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
	serial = kingpin.Flag("serial",
		"Connect to device by serial number.").
		Short('s').
		String()

	shellCommand = kingpin.Command("shell",
		"Run a shell command on the device.")
	shellCommandArg = shellCommand.Arg("command",
		"Command to run on device.").
		Strings()

	devicesCommand = kingpin.Command("devices",
		"List devices.")
	devicesLongFlag = devicesCommand.Flag("long",
		"Include extra detail about devices.").
		Short('l').
		Bool()

	forwardCommand = kingpin.Command("forward",
		"Forward")
	forwardListFlag = forwardCommand.Flag("list",
		"List forwards").
		Short('l').
		Bool()

	pullCommand = kingpin.Command("pull",
		"Pull a file from the device.")
	pullProgressFlag = pullCommand.Flag("progress",
		"Show progress.").
		Short('p').
		Bool()
	pullRemoteArg = pullCommand.Arg("remote",
		"Path of source file on device.").
		Required().
		String()
	pullLocalArg = pullCommand.Arg("local",
		"Path of destination file. If -, will write to stdout.").
		String()

	pushCommand = kingpin.Command("push",
		"Push a file to the device.")
	pushProgressFlag = pushCommand.Flag("progress",
		"Show progress.").
		Short('p').
		Bool()
	pushLocalArg = pushCommand.Arg("local",
		"Path of source file. If -, will read from stdin.").
		Required().
		String()
	pushRemoteArg = pushCommand.Arg("remote",
		"Path of destination file on device.").
		Required().
		String()
)

var client *adb.Server

func main() {
	var exitCode int

	var err error
	client, err = adb.NewDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	switch kingpin.Parse() {
	case "devices":
		exitCode = listDevices(*devicesLongFlag)
	case "shell":
		exitCode = runShellCommand(*shellCommandArg, *serial)
	case "pull":
		exitCode = pull(*pullProgressFlag, *pullRemoteArg, *pullLocalArg, *serial)
	case "push":
		exitCode = push(*pushProgressFlag, *pushLocalArg, *pushRemoteArg, *serial)
	case "forward":
		exitCode = forward(*forwardListFlag, *serial)
	}

	os.Exit(exitCode)
}

func listDevices(long bool) int {
	//client := adb.New(server)
	devices, err := client.ListDevices()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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

	return 0
}

func runShellCommand(commandAndArgs []string, deviceSerial string) int {
	if len(commandAndArgs) == 0 {
		fmt.Fprintln(os.Stderr, "error: no command")
		kingpin.Usage()
		return 1
	}

	command := commandAndArgs[0]
	var args []string

	if len(commandAndArgs) > 1 {
		args = commandAndArgs[1:]
	}

	client := client.Device(deviceSerial)
	output, err := client.Command(command, args...).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Print(output)
	return 0
}

func forward(listForwards bool, deviceSerial string) int {
	client := client.Device(deviceSerial)
	fws, err := client.ForwardList()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	for _, fw := range fws {
		fmt.Printf("%v %v %v\n", client.String(), fw[0], fw[1])
	}
	return 0
}

func pull(showProgress bool, remotePath, localPath string, deviceSerial string) int {
	if remotePath == "" {
		fmt.Fprintln(os.Stderr, "error: must specify remote file")
		kingpin.Usage()
		return 1
	}

	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	client := client.Device(deviceSerial)

	info, err := client.Stat(remotePath)
	if _, ok := errors.Cause(err).(*os.PathError); ok {
		fmt.Fprintln(os.Stderr, "remote file does not exist:", remotePath)
		return 1
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error reading remote file %s: %s\n", remotePath, err)
		return 1
	}

	remoteFile, err := client.ReadFile(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, errors.Cause(err))
		return 1
	}
	defer remoteFile.Close()

	var localFile io.WriteCloser
	if localPath == StdIoFilename {
		localFile = os.Stdout
	} else {
		localFile, err = os.Create(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
			return 1
		}
	}
	defer localFile.Close()

	if err := copyWithProgressAndStats(localFile, remoteFile, int(info.Size()), showProgress); err != nil {
		fmt.Fprintln(os.Stderr, "error pulling file:", err)
		return 1
	}
	return 0
}

func push(showProgress bool, localPath, remotePath string, deviceSerial string) int {
	if remotePath == "" {
		fmt.Fprintln(os.Stderr, "error: must specify remote file")
		kingpin.Usage()
		return 1
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
			fmt.Fprintf(os.Stderr, "error opening local file %s: %s\n", localPath, err)
			return 1
		}
		info, err := os.Stat(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading local file %s: %s\n", localPath, err)
			return 1
		}
		size = int(info.Size())
		perms = info.Mode().Perm()
		mtime = info.ModTime()
	}
	defer localFile.Close()

	client := client.Device(deviceSerial)
	writer, err := client.OpenWrite(remotePath, perms, mtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening remote file %s: %s\n", remotePath, err)
		return 1
	}
	defer writer.Close()

	if err := copyWithProgressAndStats(writer, localFile, size, showProgress); err != nil {
		fmt.Fprintln(os.Stderr, "error pushing file:", err)
		return 1
	}
	return 0
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

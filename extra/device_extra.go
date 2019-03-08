package extra

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"github.com/d1ced/adb"
)

type Process struct {
	User string
	Pid  int
	Name string
}

// ListProcesses return list of Process
func ListProcesses(d *adb.Device) ([]Process, error) {
	// example output of command "ps":
	//     USER  PID  PPID  VSIZE  RSS  WCHAN     PC         NAME
	//     root    1     0    684  540  ffffffff  00000000 S /init
	//     root    2     0      0    0  ffffffff  00000000 S kthreadd
	out, err := d.Command("ps").Output()
	if err != nil {
		return nil, err
	}

	var (
		fieldNames = make([]string, 0, 4)
		pp         = make([]Process, 0, 4)
		bufrd      = bufio.NewReader(bytes.NewReader(out))
	)

	for {
		line, err := bufrd.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if fieldNames == nil {
			// as first row
			fieldNames = fields
			continue
		}
		if len(fields) != len(fieldNames)+1 {
			return nil, errors.New("unexpected format")
		}

		var process Process

		for index, name := range fieldNames {
			value := fields[index]
			switch strings.ToUpper(name) {
			case "PID":
				process.Pid, _ = strconv.Atoi(value)
			case "NAME":
				process.Name = fields[len(fields)-1]
			case "USER":
				process.User = value
			}
		}
		if process.Pid == 0 {
			continue
		}
		pp = append(pp, process)
	}
	return pp, nil
}

// KillProcessByName return if killed success
func KillProcessByName(d *adb.Device, name string, sig syscall.Signal) error {
	pp, err := ListProcesses(d)
	if err != nil {
		return err
	}
	for _, p := range pp {
		if p.Name != name {
			continue
		}
		// log.Printf("kill %s with pid: %d", p.Name, p.Pid)
		cmd := d.Command("kill", "-"+strconv.Itoa(int(sig)), strconv.Itoa(p.Pid))
		_, err := cmd.Output()
		if err != nil {
			return err
		}
		if cmd.ExitCode() != 0 {
			return errors.New("non 0 exit code")
		}
	}
	return nil
}

type PackageInfo struct {
	Name    string
	Path    string
	Version struct {
		Code int
		Name string
	}
}

var (
	rePkgPath = regexp.MustCompile(`codePath=([^\s]+)`)
	reVerCode = regexp.MustCompile(`versionCode=(\d+)`)
	reVerName = regexp.MustCompile(`versionName=([^\s]+)`)

	ErrPackageNotExist = errors.New("package does not exist")
)

// StatPackage returns PackageInfo
// If package not found, err will be ErrPackageNotExist
func StatPackage(d *adb.Device, packageName string) (PackageInfo, error) {
	out, err := d.Command("dumpsys", "package", packageName).Output()
	if err != nil {
		return PackageInfo{}, err
	}

	matches := rePkgPath.FindSubmatch(out)
	if len(matches) == 0 {
		return PackageInfo{}, ErrPackageNotExist
	}
	path := matches[1]

	matches = reVerCode.FindSubmatch(out)
	if len(matches) == 0 {
		return PackageInfo{}, ErrPackageNotExist
	}
	piVersionCode, _ := strconv.Atoi(string(matches[1]))

	matches = reVerName.FindSubmatch(out)
	if len(matches) == 0 {
		return PackageInfo{}, ErrPackageNotExist
	}
	piVersionName := matches[1]

	return PackageInfo{
		Name: packageName,
		Path: string(path),
		Version: struct {
			Code int
			Name string
		}{
			Code: piVersionCode,
			Name: string(piVersionName),
		}}, nil
}

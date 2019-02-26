package adb

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	rhttp "github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
)

type Process struct {
	User string
	Pid  int
	Name string
}

// ListProcesses return list of Process
func (d *Device) ListProcesses() ([]Process, error) {
	// example output of command "ps":
	//     USER  PID  PPID  VSIZE  RSS  WCHAN     PC         NAME
	//     root    1     0    684  540  ffffffff  00000000 S /init
	//     root    2     0      0    0  ffffffff  00000000 S kthreadd
	err := d.OpenCommand("ps")
	if err != nil {
		return nil, err
	}
	defer d.server.Close()

	var (
		fieldNames []string
		bufrd      = bufio.NewReader(d.server.conn)
		pp         = make([]Process, 0, 10)
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
func (d *Device) KillProcessByName(name string, sig syscall.Signal) error {
	pp, err := d.ListProcesses()
	if err != nil {
		return err
	}
	for _, p := range pp {
		if p.Name != name {
			continue
		}
		// log.Printf("kill %s with pid: %d", p.Name, p.Pid)
		_, i, err := d.RunCommandWithExitCode("kill", "-"+
			strconv.Itoa(int(sig)), strconv.Itoa(p.Pid))
		if err != nil {
			return err
		}
		if i != 0 {
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
)

// StatPackage returns PackageInfo
// If package not found, err will be ErrPackageNotExist
func (d *Device) StatPackage(packageName string) (PackageInfo, error) {
	out, err := d.RunCommand("dumpsys", "package", packageName)
	if err != nil {
		return PackageInfo{}, err
	}

	matches := rePkgPath.FindStringSubmatch(out)
	if len(matches) == 0 {
		return PackageInfo{}, ErrPackageNotExist
	}
	path := matches[1]

	matches = reVerCode.FindStringSubmatch(out)
	if len(matches) == 0 {
		return PackageInfo{}, ErrPackageNotExist
	}
	piVersionCode, _ := strconv.Atoi(matches[1])

	matches = reVerName.FindStringSubmatch(out)
	if len(matches) == 0 {
		return PackageInfo{}, ErrPackageNotExist
	}
	piVersionName := matches[1]

	return PackageInfo{
		Name: packageName,
		Path: path,
		Version: struct {
			Code int
			Name string
		}{
			Code: piVersionCode,
			Name: piVersionName,
		}}, nil
}

// DoSyncFile return an object, use this object can Cancel write and get Process
func (d *Device) DoSyncFile(path string, rd io.ReadCloser, size int64, perms os.FileMode) (*AsyncWriter, error) {
	dst, err := d.OpenWrite(path, perms, time.Now())
	if err != nil {
		return nil, err
	}
	awr := newAsyncWriter(d, dst, size)
	go awr.readFrom(rd)
	return awr, nil
}

func (d *Device) DoSyncLocalFile(dst string, src string, perms os.FileMode) (*AsyncWriter, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	finfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return d.DoSyncFile(dst, f, finfo.Size(), perms)
}

func (d *Device) DoSyncHTTPFile(dst string, srcUrl string, perms os.FileMode) (*AsyncWriter, error) {
	res, err := rhttp.Get(srcUrl)
	if err != nil {
		return nil, err
	}

	var length int64
	fmt.Sscanf(res.Header.Get("Content-Length"), "%d", &length)
	return d.DoSyncFile(dst, res.Body, length, perms)
}

// WriteToFile write a reader stream to device
// TODO(JMH): Fix error handling and multi-return
// See also CopyFile
func (d *Device) WriteToFile(path string, rd io.Reader, perms os.FileMode) (written int64, err error) {
	dst, err := d.OpenWrite(path, perms, time.Now())
	if err != nil {
		return 0, err
	}

	defer func() {
		dst.Close()
		if err != nil || written == 0 {
			return
		}
		// wait until write finished.
		fromTime := time.Now()
		for {
			if time.Since(fromTime) > time.Second*600 {
				err = fmt.Errorf("write file to device timeout (10min)")
				return
			}
			finfo, er := d.Stat(path)
			if er != nil && errors.Cause(er) == ErrFileNotExist {
				err = er
				return
			}
			if finfo == (DirEntry{}) {
				err = fmt.Errorf("target file %s not created", strconv.Quote(path))
				return
			}
			if finfo != (DirEntry{}) && finfo.Size == int32(written) {
				break
			}
			time.Sleep(time.Duration(200+rand.Intn(100)) * time.Millisecond)
		}
	}()

	written, err = io.Copy(dst, rd)
	return
}

// WriteHTTPToFile downloads http resource to device
func (d *Device) WriteHTTPToFile(path string, urlStr string, perms os.FileMode) (int64, error) {
	res, err := rhttp.Get(urlStr)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, errors.Errorf("http download <%s> status %v", urlStr, res.Status)
	}
	return d.WriteToFile(path, res.Body, perms)
}

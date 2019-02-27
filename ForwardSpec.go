package adb

import (
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ForwardSpec protocols
const (
	FProtocolTCP   = "tcp"
	FProtocolLocal = "local"
	FProtocolJDWP  = "jdwp"

	// TODO(jmh): check if they are still in use.
	FProtocolAbstract   = "localabstract"
	FProtocolReserved   = "localreserved"
	FProtocolFilesystem = "localfilesystem"
)

type ForwardSpec string

// Port returns -1 if the endpoint has no port.
func (f ForwardSpec) Port() int {
	fields := strings.Split(string(f), ":")
	if len(fields) < 2 {
		return -1
	}
	if fields[0] != FProtocolTCP {
		return -1
	}
	p, err := strconv.Atoi(fields[1])
	if err != nil {
		return -1
	}
	return p
}

func (f ForwardSpec) Protocol() string {
	fields := strings.Split(string(f), ":")
	if len(fields) < 1 {
		return ""
	}
	return fields[0]
}

func isForwardSpec(s string) (ForwardSpec, error) {
	fields := strings.Split(s, ":")
	if len(fields) != 2 {
		return "", errors.New("malformed forward spec")
	}
	switch fields[0] {
	case FProtocolTCP, FProtocolJDWP:
		if _, err := strconv.Atoi(fields[1]); err != nil {
			return "", errors.Errorf("malformed pid or port: %s", fields[1])
		}
		return ForwardSpec(s), nil
	case FProtocolLocal:
		return ForwardSpec(s), nil
	default:
		return "", errors.Errorf("unrecognized protoclol: %s", fields[0])
	}
}

// ForwardList returns list with struct ForwardPair
// If no device serial specified all devices's forward list will returned
// [2]ForwardSpec is {local, remote} serial is d.serial
func (d *Device) ForwardList() ([][2]ForwardSpec, error) {
	attr, err := d.requestResponseString("list-forward")
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(string(attr))
	if len(fields)%3 != 0 {
		return nil, errors.Errorf("list forward parse error")
	}
	fs := make([][2]ForwardSpec, 0, 2)
	for i := 0; i < len(fields)/3; i++ {
		var serial = fields[i*3]
		// skip other device serial forwards
		if d.serial == "host-serial:"+serial {
			continue
		}
		local, err := isForwardSpec(fields[i*3+1])
		if err != nil {
			return nil, err
		}
		remote, err := isForwardSpec(fields[i*3+2])
		if err != nil {
			return nil, err
		}
		fs = append(fs, [2]ForwardSpec{local, remote})
	}
	return fs, nil
}

// ForwardRemove specified forward
func (d *Device) ForwardRemove(local ForwardSpec) error {
	return d.send("killforward:" + string(local))
}

// ForwardRemoveAll cancel all exists forwards
func (d *Device) ForwardRemoveAll() error {
	return d.send("killforward-all")
}

// Forward remote connection to local
func (d *Device) Forward(local, remote ForwardSpec) error {
	// last one colon too?!
	err := d.send("forward:" + string(local) + ";" + string(remote))
	return errors.WithMessage(err, "Forward")
}

// ForwardToFreePort return random generated port
// If forward already exists, just return current forworded port
func (d *Device) ForwardToFreePort(remote ForwardSpec) (int, error) {
	var getFreePort = func() int {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return 0
		}
		return addr.Port
	}

	fws, err := d.ForwardList()
	if err != nil {
		return 0, err
	}
	for _, fw := range fws {
		if fw[1] == remote {
			if fw[0].Port() == -1 {
				return 0, errors.New("no local port")
			}
			return fw[0].Port(), nil
		}
	}
	port := getFreePort()
	return port, d.Forward(ForwardSpec(FProtocolTCP+strconv.Itoa(port)), remote)
}

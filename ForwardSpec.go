package adb

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ForwardSpec protocols
const (
	FProtocolTCP        = "tcp"
	FProtocolAbstract   = "localabstract"
	FProtocolReserved   = "localreserved"
	FProtocolFilesystem = "localfilesystem"
)

type ForwardSpec struct {
	Protocol   string
	PortOrName string
}

func (f ForwardSpec) String() string {
	return f.Protocol + ":" + f.PortOrName
}

// Port returns -1 if the endpoint has no port.
func (f ForwardSpec) Port() int {
	if f.Protocol != FProtocolTCP {
		return -1
	}
	p, err := strconv.Atoi(f.PortOrName)
	if err != nil {
		return -1
	}
	return p
}

func newForwardSpec(s string) (ForwardSpec, error) {
	fields := strings.Split(s, ":")
	if len(fields) != 2 {
		return ForwardSpec{}, errors.Errorf("expect string contains only one ':', str = %s", s)
	}
	return ForwardSpec{
		Protocol:   fields[0],
		PortOrName: fields[1],
	}, nil
}

// ForwardList returns list with struct ForwardPair
// If no device serial specified all devices's forward list will returned
// [2]ForwardSpec is {local, remote} serial is d.serial
func (d *Device) ForwardList() ([][2]ForwardSpec, error) {
	attr, err := d.requestResponseString("list-forward")
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(attr)
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
		local, err := newForwardSpec(fields[i*3+1])
		if err != nil {
			return nil, err
		}
		remote, err := newForwardSpec(fields[i*3+2])
		if err != nil {
			return nil, err
		}
		fs = append(fs, [2]ForwardSpec{local, remote})
	}
	return fs, nil
}

// ForwardRemove specified forward
func (d *Device) ForwardRemove(local ForwardSpec) error {
	return d.server.send("host-serial:" + d.serial + ":killforward:" + local.String())
}

// ForwardRemoveAll cancel all exists forwards
func (d *Device) ForwardRemoveAll() error {
	return d.server.send("host-serial:" + d.serial + ":killforward-all")
}

// Forward remote connection to local
func (d *Device) Forward(local, remote ForwardSpec) error {
	// last one colon too?!
	err := d.server.send(fmt.Sprintf("host-serial:%s:forward:%v;%v", d.serial, local, remote))
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
	return port, d.Forward(ForwardSpec{FProtocolTCP, strconv.Itoa(port)}, remote)
}

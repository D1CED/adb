package adb

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Cmd represents a command that can be executed on a device.
// Use Command to get an instance.
type Cmd struct {
	Path string
	Args []string

	exitCode int
	conn     net.Conn
	output   []byte
	device   *Device

	// does this work?
	env []string
}

// Command sets up a command to execute on device d. Command takes ownership of
// args.
func (d *Device) Command(cmd string, args ...string) *Cmd {
	for i, arg := range args {
		if strings.ContainsAny(arg, " \t\n\v\r") &&
			!(arg[0] == '"' && arg[len(arg)-1] == '"') {

			args[i] = `"` + arg + `"`
		}
	}
	return &Cmd{
		Path:     cmd,
		Args:     args,
		device:   d,
		exitCode: -1,
	}
}

func (c *Cmd) StartTimeout(timeout time.Time) error {
	conn, err := dial(c.device.server.address)
	if err != nil {
		return err
	}
	conn.SetDeadline(timeout)
	c.conn = conn
	buf := []byte("0000" + "host-serial:" + c.device.serial + ":shell:")
	buf = append(buf, []byte(c.Path+" "+strings.Join(c.Args, " ")+"; echo :$?")...)
	length := len(buf) - 4
	if length > 65535 {
		return errors.Errorf("message exceeds maximum length: %d", length)
	}
	copy(buf[:4], fmt.Sprintf("%04x", length))
	_, err = conn.Write(buf)
	return err
}

// Start sends command to device.
func (c *Cmd) Start() error {
	return c.StartTimeout(time.Time{})
}

// Wait waits for the servers response.
func (c *Cmd) Wait() error {
	if c.conn == nil {
		return errors.New("no command to wait for")
	}
	b, err := readBytes(c.conn)
	if err != nil {
		return err
	}
	// split off exit code
	var splitter int
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == ':' {
			splitter = i
			break
		}
	}
	c.output = b[:splitter]
	exitString := string(b[splitter+1:])
	exitCode, _ := strconv.Atoi(exitString)
	c.exitCode = exitCode
	err = c.conn.Close()
	if err != nil {
		return err
	}
	c.conn = nil
	return nil
}

// Run starts and waits for command.
func (c *Cmd) Run() error {
	err := c.Start()
	if err != nil {
		return err
	}
	return c.Wait()
}

// Output returns the output send by the server as reply. Waits if not finished.
func (c *Cmd) Output() ([]byte, error) {
	if c.output != nil {
		return c.output, nil
	}
	if c.conn != nil {
		err := c.Wait()
		if err != nil {
			return nil, err
		}
		return c.output, nil
	}
	return nil, errors.New("no command to wait for")
}

func (c *Cmd) ExitCode() int {
	return c.exitCode
}

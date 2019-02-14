package wire

import (
	"io"
	"strings"
)

// requestResponse sends msg to the server, writing the result to w.
func (c *Conn) requestResponse(msg string, w io.Writer) (int64, error) {
	_, err := c.write([]byte(msg))
	if err != nil {
		return 0, err
	}
	if err = c.status().Error; err != nil {
		return 0, err
	}
	return c.writeTo(w)
}

func (c *Conn) Version() string {
	b := &strings.Builder{}
	_, err := c.requestResponse("host:version", b)
	if err != nil {
		return ""
	}
	return b.String()
}

func (c *Conn) Kill() error {
	_, err := c.write([]byte("host:kill"))
	if err != nil {
		return err
	}
	return c.status().Error
}

func (c *Conn) Devices() error {
	b := &strings.Builder{}
	_, err := c.requestResponse("host:devices-l", b)
	return err
}

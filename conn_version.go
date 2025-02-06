package mc

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
)

var minVersion = semver.Version{Major: 1, Minor: 6, Patch: 14}

func (c *conn) version() (*semver.Version, error) {
	var (
		prefix = "VERSION "
	)
	if _, err := c.buff.Write([]byte("version\r\n")); err != nil {
		return nil, err
	}
	if err := c.buff.Flush(); err != nil {
		return nil, err
	}
	line, err := c.buff.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, prefix) {
		return nil, fmt.Errorf("memcache: unexpected response line from version: %q", string(line))
	}
	v, err := semver.Parse(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	if err != nil {
		return nil, err
	}
	{
		v.Pre = nil
		v.Build = nil
	}
	return &v, nil
}

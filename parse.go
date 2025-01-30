package mc

import (
	"bufio"
	"strings"
)

func parseResponse(buff *bufio.Reader) (*Value, error) {
	line, err := buff.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	switch line := strings.TrimSpace(string(line)); line {
	case "HD":
		return nil, nil
	case "NS":
		return nil, ErrNotStored
	case "NF":
		return nil, ErrCacheMiss
	case "EX":
		return nil, ErrCASConflict
	case "ERROR":
		return nil, ErrNonexistentCommandName
	}

	return nil, nil
}

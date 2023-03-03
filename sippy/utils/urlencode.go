package sippy_utils

import (
    "errors"
)

type UrlEncode struct {
    unrestricted    map[byte]bool
}

func NewUrlEncode(unrestricted []byte) *UrlEncode {
    self := &UrlEncode{
        unrestricted    : make(map[byte]bool),
    }
    if unrestricted != nil {
        for _, c := range unrestricted {
            self.unrestricted[c] = true
        }
    }
    return self
}

func (self *UrlEncode) Escape(qry string) string {
    buf := make([]byte, 0, len(qry))
    for _, c := range []byte(qry) {
        if self.shouldEscape(c) {
            buf = append(buf, '%', "0123456789ABCDEF"[c >> 4], "0123456789ABCDEF"[c & 0xf])
        } else {
            buf = append(buf, c)
        }
    }
    return string(buf)
}

func (self *UrlEncode) shouldEscape(c byte) bool {
    if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
        return false
    }
    _, ok := self.unrestricted[c]
    return ! ok
}

func (self *UrlEncode) Unescape(qry string) (string, error) {
    var err error
    inbuf := []byte(qry)
    outbuf := make([]byte, 0, len(inbuf))
    var ch byte

    for i := 0; i < len(inbuf); {
        c := inbuf[i]
        if c == '%' {
            bytes_left := len(inbuf) - i
            if bytes_left < 3 {
                err = errors.New("error")
                outbuf = append(outbuf, '.')
                i += bytes_left
            } else {
                ch, err = decodeHex(inbuf[i + 1:i + 3])
                outbuf = append(outbuf, ch)
                i += 3
            }
        } else {
            outbuf = append(outbuf, c)
            i += 1
        }
    }
    return string(outbuf), err
}

func decodeHex(hex []byte) (byte, error) {
    c1, ok1 := decodeNibble(hex[0])
    c2, ok2 := decodeNibble(hex[1])
    if (! ok1) || (! ok2) {
        return '.', errors.New("error")
    }
    return (c1 << 4) + c2, nil
}

func decodeNibble(c byte) (byte, bool) {
    switch {
    case c >= '0' && c <= '9':
        return c - '0', true
    case c >= 'a' && c <= 'f':
        return c - 'a' + 10, true
    case c >= 'A' && c <= 'F':
        return c - 'A' + 10, true
    }
    return 0, false
}

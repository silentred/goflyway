package proxy

import (
	. "github.com/silentred/goflyway/config"
	"github.com/silentred/goflyway/lookup"
	"github.com/silentred/goflyway/shoco"

	"bytes"
	"strings"
)

const base35table = "abcdfghijklmnopqrtuvwxyz0123456789" // use 'e' for padding, 's' for conjunction

func SplitHostPort(host string) (string, string) {
	if idx := strings.Index(host, ":"); idx > 0 {
		return strings.ToLower(host[:idx]), host[idx:] // Port has a colon ':'
	} else {
		return strings.ToLower(host), ""
	}
}

func EncryptHost(text, mark string) string {
	host, port := SplitHostPort(text)

	enc := func(in string) string {
		if *G_DisableShoco {
			return Base35Encode(AEncrypt([]byte(in)))
		} else {
			return Base35Encode(AEncrypt(shoco.Compress(in)))
		}
	}

	if lookup.IPAddressToInteger(host) != 0 {
		return enc(mark+host) + port
	}

	parts := strings.Split(host, ".")
	flag := false
	for i := len(parts) - 1; i >= 0; i-- {
		if !tlds[parts[i]] {
			parts[i] = enc(mark + parts[i])
			flag = true
			break
		}
	}

	if flag {
		return strings.Join(parts, ".") + port
	}

	return enc(mark+host) + port
}

func DecryptHost(text, mark string) string {
	host, port := SplitHostPort(text)
	parts := strings.Split(host, ".")

	for i := len(parts) - 1; i >= 0; i-- {
		if !tlds[parts[i]] {
			buf := Base35Decode(parts[i])

			if *G_DisableShoco {
				parts[i] = string(ADecrypt(buf))
			} else {
				parts[i] = shoco.Decompress(ADecrypt(buf))
			}
			if len(parts[i]) == 0 || parts[i][0] != mark[0] {
				return ""
			}

			parts[i] = parts[i][1:]
			break
		}
	}

	return strings.Join(parts, ".") + port
}

func Base35Encode(buf []byte) string {
	ret := bytes.Buffer{}
	padded := false

	if len(buf)%2 != 0 {
		buf = append(buf, 0)
		padded = true
	}

	for i := 0; i < len(buf); i += 2 {
		n := int(buf[i])<<8 + int(buf[i+1])

		ret.WriteString(base35table[n%34 : n%34+1])
		n /= 34

		ret.WriteString(base35table[n%34 : n%34+1])
		n /= 34

		if n < 34 {
			// cheers
			ret.WriteString(base35table[n : n+1])
		} else {
			m := n % 34
			ret.WriteString("s" + base35table[m:m+1])
		}
	}

	if padded {
		ret.WriteString("e")
	}

	return ret.String()
}

func Base35Decode(text string) []byte {
	ret := bytes.Buffer{}
	padded := false

	i := -1

	var _next func() (int, bool)
	_next = func() (int, bool) {
		i++

		if i >= len(text) {
			return 0, false
		}

		b := text[i]

		if b >= 'a' && b <= 'd' {
			return int(b - 'a'), true
		} else if b >= 'f' && b <= 'r' {
			return int(b-'f') + 4, true
		} else if b >= 't' && b <= 'z' {
			return int(b-'t') + 17, true
		} else if b >= '0' && b <= '9' {
			return int(b-'0') + 24, true
		} else if b == 's' {
			n, ok := _next()
			if !ok {
				return 0, false
			}
			return n + 34, true
		} else if b == 'e' {
			padded = true
		}

		return 0, false
	}

	for {
		var ok bool
		var n1, n2, n3 int

		if n1, ok = _next(); !ok {
			break
		}

		if n2, ok = _next(); !ok {
			break
		}

		if n3, ok = _next(); !ok {
			break
		}

		n := n3*34*34 + n2*34 + n1
		b1 := n / 256
		b2 := n - b1*256

		ret.WriteByte(byte(b1))
		ret.WriteByte(byte(b2))
	}

	buf := ret.Bytes()
	if padded && len(buf) > 0 {
		buf = buf[:len(buf)-1]
	}

	return buf
}

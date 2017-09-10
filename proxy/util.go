package proxy

import (
	. "github.com/silentred/goflyway/config"
	"github.com/silentred/goflyway/counter"
	"github.com/silentred/goflyway/logg"

	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	OK200   = []byte("HTTP/1.0 200 OK\r\n\r\n")
	tlsSkip = &tls.Config{InsecureSkipVerify: true}

	rkeyHeader  = "X-Request-ID"
	rkeyHeader2 = "X-Request-HTTP-ID"
	dnsHeader   = "X-Host-Lookup"

	hostHeadExtract = regexp.MustCompile(`(\S+)\.com`)
	urlExtract      = regexp.MustCompile(`\?q=(\S+)$`)
	hasPort         = regexp.MustCompile(`:\d+$`)

	base32Paddings = []string{".com", ".org", ".net", ".co", ".me", ".cc", ".edu", ".cn"}
)

func NewRand() *rand.Rand {
	var k int64 = int64(binary.BigEndian.Uint64(G_KeyBytes[:8]))
	var k2 int64

	if *G_HRCounter {
		k2 = counter.Get()
	} else {
		k2 = time.Now().UnixNano()
	}

	return rand.New(rand.NewSource(k2 ^ k))
}

func RandomKey() string {
	_rand := NewRand()
	retB := make([]byte, 16)

	for i := 0; i < 16; i++ {
		retB[i] = byte(_rand.Intn(255) + 1)
	}

	return base64.StdEncoding.EncodeToString(AEncrypt(retB))
}

func ReverseRandomKey(key string) []byte {
	if key == "" {
		return nil
	}

	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil
	}

	return ADecrypt(k)
}

func processBody(req *http.Request, enc bool) {
	var rkey string
	if enc {
		add := func(field string) {
			if x := req.Header.Get(field); x != "" {
				G_RequestDummies.Add(field, x)
			}
		}

		add("Accept-Language")
		add("User-Agent")
		add("Referer")
		add("Cache-Control")
		add("Accept-Encoding")
		add("Connection")

		rkey = RandomKey()
		SafeAddHeader(req, rkeyHeader2, rkey)
	} else {
		rkey = SafeGetHeader(req, rkeyHeader2)
	}

	for _, c := range req.Cookies() {
		if enc {
			c.Value = AEncryptString(c.Value)
		} else {
			c.Value = ADecryptString(c.Value)
		}
	}

	req.Body = ioutil.NopCloser((&IOReaderCipher{Src: req.Body, Key: ReverseRandomKey(rkey)}).Init())
}

func SafeAddHeader(req *http.Request, k, v string) {
	if orig := req.Header.Get(k); orig != "" {
		req.Header.Set(k, v+" "+orig)
	} else {
		req.Header.Add(k, v)
	}
}

func SafeGetHeader(req *http.Request, k string) string {
	v := req.Header.Get(k)
	if s := strings.Index(v, " "); s > 0 {
		req.Header.Set(k, v[s+1:])
		v = v[:s]
	}

	return v
}

func EncryptRequest(req *http.Request) string {
	req.Host = EncryptHost(req.Host, "#")
	req.URL, _ = url.Parse("http://" + req.Host + "/?q=" + AEncryptString(req.URL.String()))

	rkey := RandomKey()
	SafeAddHeader(req, rkeyHeader, rkey)
	processBody(req, true)
	return rkey
}

func DecryptRequest(req *http.Request) string {
	req.Host = DecryptHost(req.Host, "#")
	if p := urlExtract.FindStringSubmatch(req.URL.String()); len(p) > 1 {
		req.URL, _ = url.Parse(ADecryptString(p[1]))
	}

	rkey := SafeGetHeader(req, rkeyHeader)
	processBody(req, false)
	return rkey
}

func copyHeaders(dst, src http.Header) {
	for k := range dst {
		dst.Del(k)
	}
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func getAuth(r *http.Request) string {
	pa := r.Header.Get("Proxy-Authorization")
	if pa == "" {
		pa = r.Header.Get("X-Authorization")
	}

	return pa
}

func basicAuth(token string) bool {
	parts := strings.Split(token, " ")
	if len(parts) != 2 {
		return false
	}

	pa, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return false
	}

	return string(pa) == *G_Auth
}

func tryClose(b io.ReadCloser) {
	if err := b.Close(); err != nil {
		logg.W("can't close response body - ", err)
	}
}

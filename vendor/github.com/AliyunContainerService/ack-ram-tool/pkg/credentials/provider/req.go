package provider

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func shaHmac1(source, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha1.New, key)
	h.Write([]byte(source))
	signedBytes := h.Sum(nil)
	signedString := base64.StdEncoding.EncodeToString(signedBytes)
	return signedString
}

type commonRequest struct {
	Scheme         string
	Method         string
	Domain         string
	RegionId       string
	URL            string
	ReadTimeout    time.Duration
	ConnectTimeout time.Duration
	isInsecure     *bool
	BodyParams     map[string]string
	userAgent      map[string]string
	QueryParams    map[string]string
	Headers        map[string]string

	queries string
}

func newCommonRequest() *commonRequest {
	return &commonRequest{
		BodyParams:  make(map[string]string),
		QueryParams: make(map[string]string),
		Headers:     make(map[string]string),
	}
}

// BuildURL returns a url
func (request *commonRequest) BuildURL() string {
	url := fmt.Sprintf("%s://%s", strings.ToLower(request.Scheme), request.Domain)
	request.queries = "/?" + getURLFormedMap(request.QueryParams)
	return url + request.queries
}

func getURLFormedMap(source map[string]string) (urlEncoded string) {
	urlEncoder := url.Values{}
	for key, value := range source {
		urlEncoder.Add(key, value)
	}
	urlEncoded = urlEncoder.Encode()
	return
}

// BuildStringToSign returns BuildStringToSign
func (request *commonRequest) BuildStringToSign() (stringToSign string) {
	signParams := make(map[string]string)
	for key, value := range request.QueryParams {
		signParams[key] = value
	}

	for key, value := range request.BodyParams {
		signParams[key] = value
	}
	stringToSign = getURLFormedMap(signParams)
	stringToSign = strings.Replace(stringToSign, "+", "%20", -1)
	stringToSign = strings.Replace(stringToSign, "*", "%2A", -1)
	stringToSign = strings.Replace(stringToSign, "%7E", "~", -1)
	stringToSign = url.QueryEscape(stringToSign)
	stringToSign = request.Method + "&%2F&" + stringToSign
	return
}

func getTimeInFormatISO8601() (timeStr string) {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

type uuid [16]byte

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytes(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[r.Intn(len(letterBytes))]
	}
	return string(b)
}

func newUUID() uuid {
	ns := uuid{}
	safeRandom(ns[:])
	u := newFromHash(md5.New(), ns, randStringBytes(16))
	u[6] = (u[6] & 0x0f) | (byte(2) << 4)
	u[8] = u[8]&(0xff>>2) | (0x02 << 6)

	return u
}

func newFromHash(h hash.Hash, ns uuid, name string) uuid {
	u := uuid{}
	h.Write(ns[:])
	h.Write([]byte(name))
	copy(u[:], h.Sum(nil))

	return u
}

func safeRandom(dest []byte) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if _, err := r.Read(dest); err != nil {
		panic(err)
	}
}

func (u uuid) String() string {
	buf := make([]byte, 36)

	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])

	return string(buf)
}

func getUUID() (uuidHex string) {
	uuid := newUUID()
	uuidHex = hex.EncodeToString(uuid[:])
	return
}

func genDebugReqMessages(req *http.Request) []string {
	var ret []string
	ret = append(ret, fmt.Sprintf("%s %s", req.Method, req.URL.String()))
	ret = append(ret, "Request Headers:")
	for k, vs := range req.Header {
		ret = append(ret, fmt.Sprintf("    %s: %s", k, strings.Join(vs, ", ")))
	}
	return ret
}

func genDebugRespMessages(resp *http.Response) []string {
	var ret []string
	ret = append(ret, fmt.Sprintf("Response Status: %s", resp.Status))
	ret = append(ret, "Response Headers:")
	for k, vs := range resp.Header {
		ret = append(ret, fmt.Sprintf("    %s: %s", k, strings.Join(vs, ", ")))
	}
	return ret
}

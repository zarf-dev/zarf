package gguf_parser

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gpustack/gguf-parser-go/util/funcx"
	"github.com/gpustack/gguf-parser-go/util/httpx"
	"github.com/gpustack/gguf-parser-go/util/osx"
	"github.com/gpustack/gguf-parser-go/util/stringx"
)

const (
	httpHeaderWWWAuthenticate = "WWW-Authenticate"
	httpHeaderAuthorization   = "Authorization"
)

// OllamaUserAgent returns the user agent string for Ollama,
// since llama3.1, the user agent is required to be set,
// otherwise the request will be rejected by 412.
func OllamaUserAgent() string {
	return fmt.Sprintf("ollama/9.9.9 (%s %s) Go/%s", runtime.GOARCH, runtime.GOOS, runtime.Version())
}

// OllamaRegistryAuthorizeRetry returns true if the request should be retried with authorization.
//
// OllamaRegistryAuthorizeRetry leverages OllamaRegistryAuthorize to obtain an authorization token,
// and configures the request with the token.
func OllamaRegistryAuthorizeRetry(resp *http.Response, cli *http.Client) bool {
	if resp == nil || cli == nil {
		return false
	}

	if resp.StatusCode != http.StatusUnauthorized && resp.Request == nil {
		// Not unauthorized, return.
		return false
	}

	req := resp.Request
	if req.Header.Get(httpHeaderAuthorization) != "" {
		// Already authorized, return.
		return false
	}

	const tokenPrefix = "Bearer "
	authnToken := strings.TrimPrefix(resp.Header.Get(httpHeaderWWWAuthenticate), tokenPrefix)
	if authnToken == "" {
		// No authentication token, return.
		return false
	}
	authzToken := funcx.MustNoError(OllamaRegistryAuthorize(req.Context(), cli, authnToken))
	req.Header.Set(httpHeaderAuthorization, tokenPrefix+authzToken)
	return true
}

// OllamaRegistryAuthorize authorizes the request with the given authentication token,
// and returns the authorization token.
func OllamaRegistryAuthorize(ctx context.Context, cli *http.Client, authnToken string) (string, error) {
	priKey, err := OllamaSingKeyLoad()
	if err != nil {
		return "", fmt.Errorf("load sign key: %w", err)
	}

	var authzUrl string
	{
		ss := strings.Split(authnToken, ",")
		if len(ss) < 3 {
			return "", errors.New("invalid authn token")
		}

		var realm, service, scope string
		for _, s := range ss {
			sp := strings.SplitN(s, "=", 2)
			if len(sp) < 2 {
				continue
			}
			sp[1] = strings.TrimFunc(sp[1], func(r rune) bool {
				return r == '"' || r == '\''
			})
			switch sp[0] {
			case "realm":
				realm = sp[1]
			case "service":
				service = sp[1]
			case "scope":
				scope = sp[1]
			}
		}

		u, err := url.Parse(realm)
		if err != nil {
			return "", fmt.Errorf("parse realm: %w", err)
		}

		qs := u.Query()
		qs.Add("service", service)
		for _, s := range strings.Split(scope, " ") {
			qs.Add("scope", s)
		}
		qs.Add("ts", strconv.FormatInt(time.Now().Unix(), 10))
		qs.Add("nonce", stringx.RandomBase64(16))
		u.RawQuery = qs.Encode()

		authzUrl = u.String()
	}

	var authnData string
	{
		pubKey := ssh.MarshalAuthorizedKey(priKey.PublicKey())
		pubKeyp := bytes.Split(pubKey, []byte(" "))
		if len(pubKeyp) < 2 {
			return "", errors.New("malformed public key")
		}

		nc := base64.StdEncoding.EncodeToString([]byte(stringx.SumBytesBySHA256(nil)))
		py := []byte(fmt.Sprintf("%s,%s,%s", http.MethodGet, authzUrl, nc))
		sd, err := priKey.Sign(rand.Reader, py)
		if err != nil {
			return "", fmt.Errorf("signing data: %w", err)
		}
		authnData = fmt.Sprintf("%s:%s", bytes.TrimSpace(pubKeyp[1]), base64.StdEncoding.EncodeToString(sd.Blob))
	}

	req, err := httpx.NewGetRequestWithContext(ctx, authzUrl)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Add(httpHeaderAuthorization, authnData)

	var authzToken string
	err = httpx.Do(cli, req, func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code %d", resp.StatusCode)
		}
		var tok struct {
			Token string `json:"token"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&tok); err != nil {
			return err
		}
		if tok.Token == "" {
			return errors.New("empty token")
		}
		authzToken = tok.Token
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("do request %s: %w", authzUrl, err)
	}

	return authzToken, nil
}

// OllamaSingKeyLoad loads the signing key for Ollama,
// and generates a new key if not exists.
func OllamaSingKeyLoad() (ssh.Signer, error) {
	hd := filepath.Join(osx.UserHomeDir(), ".ollama")

	priKeyPath := filepath.Join(hd, "id_ed25519")
	if !osx.ExistsFile(priKeyPath) {
		// Generate key if not exists.
		pubKey, priKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}

		priKeyPem, err := ssh.MarshalPrivateKey(priKey, "")
		if err != nil {
			return nil, fmt.Errorf("marshal private key: %w", err)
		}
		priKeyBs := pem.EncodeToMemory(priKeyPem)

		sshPubKey, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			return nil, fmt.Errorf("new public key: %w", err)
		}
		pubKeyBs := ssh.MarshalAuthorizedKey(sshPubKey)

		if err = osx.WriteFile(priKeyPath, priKeyBs, 0o600); err != nil {
			return nil, fmt.Errorf("write private key: %w", err)
		}
		if err = osx.WriteFile(priKeyPath+".pub", pubKeyBs, 0o644); err != nil {
			_ = os.Remove(priKeyPath)
			return nil, fmt.Errorf("write public key: %w", err)
		}
	}

	priKeyBs, err := os.ReadFile(priKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	priKey, err := ssh.ParsePrivateKey(priKeyBs)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return priKey, nil
}

package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const defaultEncryptedFilePath = "/var/addon/token-config"

type EncryptedFileProvider struct {
	f *FileProvider
}

type EncryptedFileProviderOptions struct {
	FilePath      string
	RefreshPeriod time.Duration
	ExpiryWindow  time.Duration
	Logger        Logger
}

func NewEncryptedFileProvider(opts EncryptedFileProviderOptions) *EncryptedFileProvider {
	opts.applyDefaults()

	e := &EncryptedFileProvider{}
	e.f = NewFileProvider(opts.FilePath, parseEncryptedToken, FileProviderOptions{
		RefreshPeriod: opts.RefreshPeriod,
		ExpiryWindow:  opts.ExpiryWindow,
		Logger:        opts.Logger,
		LogPrefix:     "[EncryptedFileProvider]",
	})

	return e
}

func (e *EncryptedFileProvider) Credentials(ctx context.Context) (*Credentials, error) {
	return e.f.Credentials(ctx)
}

func (o *EncryptedFileProviderOptions) applyDefaults() {
	if o.ExpiryWindow == 0 {
		o.ExpiryWindow = defaultExpiryWindow
	}
	if o.FilePath == "" {
		o.FilePath = defaultEncryptedFilePath
	}
	if o.Logger == nil {
		o.Logger = defaultLog
	}
}

func parseEncryptedToken(data []byte) (*Credentials, error) {
	var t encryptedToken
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse data failed: %w", err)
	}
	if t.Error != nil {
		return nil, t.Error
	}

	id, err := decrypt(t.AccessKeyId, []byte(t.Keyring))
	if err != nil {
		return nil, fmt.Errorf("parse data failed: %w", err)
	}
	se, err := decrypt(t.AccessKeySecret, []byte(t.Keyring))
	if err != nil {
		return nil, fmt.Errorf("parse data failed: %w", err)
	}
	st, err := decrypt(t.SecurityToken, []byte(t.Keyring))
	if err != nil {
		return nil, fmt.Errorf("parse data failed: %w", err)
	}
	exp, err := time.Parse("2006-01-02T15:04:05Z", t.Expiration)
	if err != nil {
		return nil, fmt.Errorf("parse expiration %s failed: %w", t.Expiration, err)
	}

	return &Credentials{
		AccessKeyId:     string(id),
		AccessKeySecret: string(se),
		SecurityToken:   string(st),
		Expiration:      exp,
	}, nil
}

type encryptedToken struct {
	AccessKeyId     string `json:"access.key.id"`
	AccessKeySecret string `json:"access.key.secret"`
	SecurityToken   string `json:"security.token"`
	Expiration      string `json:"expiration"`
	Keyring         string `json:"keyring"`

	Error *encryptedTokenError `json:"error,omitempty"`
}

type encryptedTokenError struct {
	RoleName string `json:"roleName,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (e encryptedTokenError) Error() string {
	return fmt.Sprintf("assume role %s failed: %s %s", e.RoleName, e.Code, e.Message)
}

func decrypt(s string, keyring []byte) ([]byte, error) {
	cdata, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(keyring)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	iv := cdata[:blockSize]
	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(cdata)-blockSize)

	blockMode.CryptBlocks(origData, cdata[blockSize:])

	origData = pkcs5UnPadding(origData)
	return origData, nil
}

func pkcs5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

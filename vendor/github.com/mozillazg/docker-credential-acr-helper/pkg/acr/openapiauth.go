package acr

import (
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"

	"github.com/AliyunContainerService/ack-ram-tool/pkg/credentials/provider"
	"github.com/aliyun/credentials-go/credentials"
)

var defaultProfilePath = filepath.Join("~", ".alibabacloud", "credentials")

type credentialForV2SDK struct {
	*provider.CredentialForV2SDK
}

type logWrapper struct {
	logger *logrus.Logger
}

func getOpenapiAuth(logger *logrus.Logger) (credentials.Credential, error) {
	profilePath := defaultProfilePath
	if os.Getenv(credentials.ENVCredentialFile) != "" {
		profilePath = os.Getenv(credentials.ENVCredentialFile)
	}
	path, err := expandPath(profilePath)
	if err == nil {
		if _, err := os.Stat(path); err == nil {
			_ = os.Setenv(credentials.ENVCredentialFile, path)
			return credentials.NewCredential(nil)
		}
	}

	cp := provider.NewDefaultChainProvider(provider.DefaultChainProviderOptions{
		Logger: &logWrapper{logger: logger},
	})
	cred := &credentialForV2SDK{
		CredentialForV2SDK: provider.NewCredentialForV2SDK(cp, provider.CredentialForV2SDKOptions{
			CredentialRetrievalTimeout: time.Second * 30,
			Logger:                     &logWrapper{logger: logger},
		}),
	}

	return cred, err
}

func (c *credentialForV2SDK) GetCredential() (*credentials.CredentialModel, error) {
	ak, err := c.GetAccessKeyId()
	if err != nil {
		return nil, err
	}
	sk, err := c.GetAccessKeySecret()
	if err != nil {
		return nil, err
	}
	token, err := c.GetSecurityToken()
	if err != nil {
		return nil, err
	}
	return &credentials.CredentialModel{
		AccessKeyId:     ak,
		AccessKeySecret: sk,
		SecurityToken:   token,
		BearerToken:     nil,
		Type:            c.GetType(),
	}, err
}

func (l *logWrapper) Info(msg string) {
	l.logger.Debug(msg)
}

func (l *logWrapper) Debug(msg string) {
	l.logger.Debug(msg)
}

func (l *logWrapper) Error(err error, msg string) {
	l.logger.WithError(err).Error(msg)
}

func expandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return path, nil
}

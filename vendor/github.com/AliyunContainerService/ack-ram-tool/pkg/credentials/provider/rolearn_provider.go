package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type RoleArnProvider struct {
	u *Updater

	client *http.Client

	stsEndpoint string
	stsScheme   string
	sessionName string

	policy          string
	externalId      string
	durationSeconds string

	roleArn string
	cp      CredentialsProvider

	Logger Logger
}

type RoleArnProviderOptions struct {
	STSEndpoint string
	stsScheme   string
	SessionName string

	TokenDuration time.Duration
	Policy        string
	ExternalId    string

	Timeout   time.Duration
	Transport http.RoundTripper

	ExpiryWindow  time.Duration
	RefreshPeriod time.Duration
	Logger        Logger
}

func NewRoleArnProvider(cp CredentialsProvider, roleArn string, opts RoleArnProviderOptions) *RoleArnProvider {
	opts.applyDefaults()

	client := &http.Client{
		Transport: opts.Transport,
		Timeout:   opts.Timeout,
	}
	e := &RoleArnProvider{
		client:      client,
		stsEndpoint: opts.STSEndpoint,
		stsScheme:   opts.stsScheme,
		sessionName: opts.SessionName,
		policy:      opts.Policy,
		externalId:  opts.ExternalId,
		roleArn:     roleArn,
		cp:          cp,
		Logger:      opts.Logger,
	}
	if opts.TokenDuration >= time.Second*900 {
		ds := int64(opts.TokenDuration.Seconds())
		e.durationSeconds = fmt.Sprintf("%d", ds)
	}

	e.u = NewUpdater(e.getCredentials, UpdaterOptions{
		ExpiryWindow:  opts.ExpiryWindow,
		RefreshPeriod: opts.RefreshPeriod,
		Logger:        opts.Logger,
		LogPrefix:     "[RoleArnProvider]",
	})
	e.u.Start(context.TODO())

	return e
}

func (r *RoleArnProvider) Credentials(ctx context.Context) (*Credentials, error) {
	return r.u.Credentials(ctx)
}

func (r *RoleArnProvider) Stop(ctx context.Context) {
	r.u.Stop(ctx)
	if s, ok := r.cp.(Stopper); ok {
		s.Stop(ctx)
	}
}

func (r *RoleArnProvider) getCredentials(ctx context.Context) (*Credentials, error) {
	return r.assumeRole(ctx, r.roleArn)
}

type roleArnResponse struct {
	Credentials *credentialsInResponse `json:"Credentials"`
}

func (r *RoleArnProvider) assumeRole(ctx context.Context, roleArn string) (*Credentials, error) {
	cred, err := r.cp.Credentials(ctx)
	if err != nil {
		return nil, err
	}

	reqOpts := newCommonRequest()
	reqOpts.Domain = r.stsEndpoint
	reqOpts.Scheme = r.stsScheme
	reqOpts.Method = "POST"
	reqOpts.QueryParams["Timestamp"] = getTimeInFormatISO8601()
	reqOpts.QueryParams["AccessKeyId"] = cred.AccessKeyId
	reqOpts.QueryParams["Action"] = "AssumeRole"
	reqOpts.QueryParams["Format"] = "JSON"
	reqOpts.QueryParams["RoleArn"] = roleArn
	if r.durationSeconds != "" {
		reqOpts.QueryParams["DurationSeconds"] = r.durationSeconds
	}
	if r.policy != "" {
		reqOpts.BodyParams["Policy"] = r.policy
	}
	if r.externalId != "" {
		reqOpts.QueryParams["ExternalId"] = r.externalId
	}
	reqOpts.QueryParams["RoleSessionName"] = r.sessionName
	reqOpts.QueryParams["SignatureMethod"] = "HMAC-SHA1"
	reqOpts.QueryParams["SignatureVersion"] = "1.0"
	reqOpts.QueryParams["Version"] = "2015-04-01"
	reqOpts.QueryParams["SignatureNonce"] = getUUID()
	if cred.SecurityToken != "" {
		reqOpts.QueryParams["SecurityToken"] = cred.SecurityToken
	}
	signature := shaHmac1(reqOpts.BuildStringToSign(), cred.AccessKeySecret+"&")
	reqOpts.QueryParams["Signature"] = signature

	reqOpts.Headers["Accept-Encoding"] = "identity"
	reqOpts.Headers["content-type"] = "application/x-www-form-urlencoded"
	reqOpts.URL = reqOpts.BuildURL()

	req, err := http.NewRequest(reqOpts.Method, reqOpts.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range reqOpts.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", UserAgent)
	req = req.WithContext(ctx)

	if debugMode {
		for _, item := range genDebugReqMessages(req) {
			r.logger().Debug(item)
		}
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s failed: %w", req.URL, err)
	}
	defer resp.Body.Close()

	if debugMode {
		for _, item := range genDebugRespMessages(resp) {
			r.logger().Debug(item)
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var obj roleArnResponse
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if obj.Credentials == nil || obj.Credentials.AccessKeySecret == "" {
		return nil, fmt.Errorf("call AssumeRole failed, got unexpected body: %s",
			strings.ReplaceAll(string(data), "\n", " "))
	}

	exp, err := time.Parse("2006-01-02T15:04:05Z", obj.Credentials.Expiration)
	if err != nil {
		return nil, err
	}
	return &Credentials{
		AccessKeyId:     obj.Credentials.AccessKeyId,
		AccessKeySecret: obj.Credentials.AccessKeySecret,
		SecurityToken:   obj.Credentials.SecurityToken,
		Expiration:      exp,
	}, nil
}

func (r *RoleArnProvider) logger() Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return defaultLog
}

func (o *RoleArnProviderOptions) applyDefaults() {
	if o.Timeout <= 0 {
		o.Timeout = defaultClientTimeout
	}
	if o.Transport == nil {
		ts := http.DefaultTransport.(*http.Transport).Clone()
		o.Transport = ts
	}
	if o.STSEndpoint == "" {
		o.STSEndpoint = defaultSTSEndpoint
	} else {
		o.STSEndpoint = strings.TrimRight(o.STSEndpoint, "/")
	}

	if strings.HasPrefix(o.STSEndpoint, "https://") {
		o.stsScheme = "HTTPS"
		o.STSEndpoint = strings.TrimPrefix(o.STSEndpoint, "https://")
	} else if strings.HasPrefix(o.STSEndpoint, "http://") {
		o.stsScheme = "HTTP"
		o.STSEndpoint = strings.TrimPrefix(o.STSEndpoint, "http://")
	}
	if o.stsScheme == "" {
		o.stsScheme = defaultSTSScheme
	}
	o.stsScheme = strings.ToUpper(o.stsScheme)

	if o.SessionName == "" {
		o.SessionName = defaultSessionName
	}
	if o.ExpiryWindow == 0 {
		o.ExpiryWindow = defaultExpiryWindowForAssumeRole
		if o.TokenDuration > 0 && o.TokenDuration <= o.ExpiryWindow {
			o.ExpiryWindow = o.TokenDuration / 2
		}
	}
	if o.Logger == nil {
		o.Logger = defaultLog
	}
}

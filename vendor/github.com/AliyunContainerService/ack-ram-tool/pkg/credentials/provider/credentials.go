package provider

import "time"

type Credentials struct {
	AccessKeyId     string
	AccessKeySecret string
	SecurityToken   string
	Expiration      time.Time
}

func (c *Credentials) DeepCopy() *Credentials {
	if c == nil {
		return nil
	}
	return &Credentials{
		AccessKeyId:     c.AccessKeyId,
		AccessKeySecret: c.AccessKeySecret,
		SecurityToken:   c.SecurityToken,
		Expiration:      c.Expiration,
	}
}

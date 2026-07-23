package acr

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"time"

	cr2018 "github.com/alibabacloud-go/cr-20181201/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/mozillazg/docker-credential-acr-helper/pkg/version"
)

type eeClient struct {
	client *cr2018.Client
}

func newEEClient(region string, logger *logrus.Logger) (*eeClient, error) {
	cred, err := getOpenapiAuth(logger)
	if err != nil {
		return nil, err
	}
	c := &openapi.Config{
		RegionId:   tea.String(region),
		Credential: cred,
		UserAgent:  tea.String(version.UserAgent()),
	}
	client, err := cr2018.NewClient(c)
	if err != nil {
		return nil, err
	}
	return &eeClient{client: client}, nil
}

func (c *eeClient) getInstanceId(instanceName string) (string, error) {
	req := &cr2018.ListInstanceRequest{
		InstanceName: tea.String(instanceName),
	}
	resp, err := c.client.ListInstance(req)
	if err != nil {
		return "", err
	}
	if resp.Body == nil {
		return "", fmt.Errorf("get ACR EE instance id for name %q failed: %s", instanceName, resp.String())
	}
	if !tea.BoolValue(resp.Body.IsSuccess) {
		return "", fmt.Errorf("get ACR EE instance id for name %q failed: %s", instanceName, resp.Body.String())
	}
	instances := resp.Body.Instances
	for _, item := range instances {
		if tea.StringValue(item.InstanceName) == instanceName {
			return tea.StringValue(item.InstanceId), nil
		}
	}

	return "", fmt.Errorf("get ACR EE instance id for name %q failed: instance name is not found", instanceName)
}

func (c *eeClient) getCredentials(instanceId string) (*Credentials, error) {
	req := &cr2018.GetAuthorizationTokenRequest{
		InstanceId: &instanceId,
	}
	resp, err := c.client.GetAuthorizationToken(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, fmt.Errorf("get credentials failed: %s", resp.String())
	}
	if !tea.BoolValue(resp.Body.IsSuccess) {
		return nil, fmt.Errorf("get credentials failed: %s", resp.Body.String())
	}

	exp := tea.Int64Value(resp.Body.ExpireTime) / 1000
	expTime := time.Unix(exp, 0).UTC()
	cred := &Credentials{
		UserName:   tea.StringValue(resp.Body.TempUsername),
		Password:   tea.StringValue(resp.Body.AuthorizationToken),
		ExpireTime: expTime.Add(-time.Minute),
	}
	return cred, nil
}

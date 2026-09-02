package provider

import (
	"os"
	"strings"
)

const (
	regionPlaceholder        = "{region}"
	defaultSTSEndpointTPL    = "sts.{region}.aliyuncs.com"
	defaultSTSVPCEndpointTPL = "sts-vpc.{region}.aliyuncs.com"
)

// https://help.aliyun.com/zh/ram/developer-reference/api-sts-2015-04-01-endpoint
var stsEndpointsByRegion = map[string][2]string{
	"": {
		defaultSTSEndpoint,
		defaultSTSEndpoint,
	},
	"__default__": {
		defaultSTSEndpointTPL,
		defaultSTSVPCEndpointTPL,
	},
	"cn-hangzhou-finance": {
		"sts.cn-hangzhou.aliyuncs.com",
		"sts-vpc.cn-hangzhou.aliyuncs.com",
	},
}

func GetSTSEndpoint(region string, vpcNetwork bool) string {
	if v := os.Getenv(envStsEndpoint); v != "" {
		return v
	}
	endpoints, exist := stsEndpointsByRegion[region]
	if !exist {
		endpoints = stsEndpointsByRegion["__default__"]
	}
	endpoint := endpoints[0]
	if vpcNetwork {
		endpoint = endpoints[1]
	}
	return strings.ReplaceAll(endpoint, regionPlaceholder, region)
}

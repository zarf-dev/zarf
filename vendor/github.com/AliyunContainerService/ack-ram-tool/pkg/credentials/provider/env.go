package provider

import "os"

const (
	envRoleArn         = "ALIBABA_CLOUD_ROLE_ARN"
	envOidcProviderArn = "ALIBABA_CLOUD_OIDC_PROVIDER_ARN"
	envOidcTokenFile   = "ALIBABA_CLOUD_OIDC_TOKEN_FILE"

	envStsEndpoint   = "ALIBABA_CLOUD_STS_ENDPOINT"
	envStsHttpScheme = "ALIBABA_CLOUD_STS_HTTP_SCHEME"
)

// https://github.com/aliyun/credentials-go
const (
	envNewSdkAccessKeyId     = "ALIBABA_CLOUD_ACCESS_KEY_ID"
	envNewSdkAccessKeySecret = "ALIBABA_CLOUD_ACCESS_KEY_SECRET" // #nosec G101
	envNewSdkSecurityToken   = "ALIBABA_CLOUD_SECURITY_TOKEN"    // #nosec G101
	envNewSdkRoleSessionName = "ALIBABA_CLOUD_ROLE_SESSION_NAME"

	envNewSdkCredentialsURI = "ALIBABA_CLOUD_CREDENTIALS_URI" // #nosec G101

	envNewSdkCredentialFile = "ALIBABA_CLOUD_CREDENTIALS_FILE" // #nosec G101

	envRoleSessionName = envNewSdkRoleSessionName
	envCredentialsURI  = envNewSdkCredentialsURI // #nosec G101
)

// https://github.com/aliyun/alibaba-cloud-sdk-go/tree/master/sdk/auth
const (
	envOldSdkAccessKeyID       = "ALICLOUD_ACCESS_KEY"
	envOldSdkAccessKeySecret   = "ALICLOUD_SECRET_KEY"
	envOldSdkAccessKeyStsToken = "ALICLOUD_ACCESS_KEY_STS_TOKEN" // #nosec G101
	//envOldSdkRoleArn               = "ALICLOUD_ROLE_ARN"
	envOldSdkRoleSessionName = "ALICLOUD_ROLE_SESSION_NAME"
	//envOldSdkRoleSessionExpiration = "ALICLOUD_ROLE_SESSION_EXPIRATION"
	//envOldSdkPrivateKey            = "ALICLOUD_PRIVATE_KEY"
	//envOldSdkPublicKeyID           = "ALICLOUD_PUBLIC_KEY_ID"
	//envOldSdkSessionExpiration     = "ALICLOUD_SESSION_EXPIRATION"
	//envOldSdkRoleName              = "ALICLOUD_ROLE_NAME"
)

const (
	envAliyuncliAccessKeyId1 = "ALIBABACLOUD_ACCESS_KEY_ID"
	envAliyuncliAccessKeyId2 = "ALICLOUD_ACCESS_KEY_ID"
	envAliyuncliAccessKeyId3 = "ACCESS_KEY_ID"

	envAliyuncliAccessKeySecret1 = "ALIBABACLOUD_ACCESS_KEY_SECRET" // #nosec G101
	envAliyuncliAccessKeySecret2 = "ALICLOUD_ACCESS_KEY_SECRET"     // #nosec G101
	envAliyuncliAccessKeySecret3 = "ACCESS_KEY_SECRET"              // #nosec G101

	envAliyuncliStsToken1 = "ALIBABACLOUD_SECURITY_TOKEN" // #nosec G101
	envAliyuncliStsToken2 = "ALICLOUD_SECURITY_TOKEN"     // #nosec G101
	envAliyuncliStsToken3 = "SECURITY_TOKEN"              // #nosec G101

	envAliyuncliProfileName1 = "ALIBABACLOUD_PROFILE"
	envAliyuncliProfileName2 = "ALIBABA_CLOUD_PROFILE"
	envAliyuncliProfileName3 = "ALICLOUD_PROFILE"

	envAliyuncliIgnoreProfile = "ALIBABACLOUD_IGNORE_PROFILE"

	envAliyuncliProfilePath = "ALIBABACLOUD_PROFILE_PATH"
)

// https://github.com/aliyun/alibabacloud-credentials-cli
const (
	envAccAlibabaCloudAccessKeyId     = "ALIBABACLOUD_ACCESS_KEY_ID"
	envAccAlibabaCloudAccessKeySecret = "ALIBABACLOUD_ACCESS_KEY_SECRET" // #nosec G101
	envAccAlibabaCloudSecurityToken   = "ALIBABACLOUD_SECURITY_TOKEN"    // #nosec G101
)

var (
	accessKeyIdEnvs = []string{
		envNewSdkAccessKeyId,
		envOldSdkAccessKeyID,
		envAliyuncliAccessKeyId1,
		envAliyuncliAccessKeyId2,
		envAccAlibabaCloudAccessKeyId,
		//envAliyuncliAccessKeyId3,
	}

	accessKeySecretEnvs = []string{
		envNewSdkAccessKeySecret,
		envOldSdkAccessKeySecret,
		envAliyuncliAccessKeySecret1,
		envAliyuncliAccessKeySecret2,
		envAccAlibabaCloudAccessKeySecret,
		//envAliyuncliAccessKeySecret3,
	}

	securityTokenEnvs = []string{
		envNewSdkSecurityToken,
		envOldSdkAccessKeyStsToken,
		envAliyuncliStsToken1,
		envAliyuncliStsToken2,
		envAccAlibabaCloudSecurityToken,
		//envAliyuncliStsToken3,
	}

	roleArnEnvs = []string{
		envRoleArn,
	}
	oidcProviderArnEnvs = []string{
		envOidcProviderArn,
	}
	oidcTokenFileEnvs = []string{
		envOidcTokenFile,
	}
	roleSessionNameEnvs = []string{
		envNewSdkRoleSessionName,
		envOldSdkRoleSessionName,
	}

	credentialsURIEnvs = []string{
		envNewSdkCredentialsURI,
	}

	credentialFileEnvs = []string{
		envNewSdkCredentialFile,
	}

	aliyuncliProfileNameEnvs = []string{
		envAliyuncliProfileName1,
		envAliyuncliProfileName2,
		envAliyuncliProfileName3,
	}
	aliyuncliIgnoreProfileEnvs = []string{
		envAliyuncliIgnoreProfile,
	}
	aliyuncliProfilePathEnvs = []string{
		envAliyuncliProfilePath,
	}
)

func getEnvsValue(keys []string) string {
	for _, key := range keys {
		v := os.Getenv(key)
		if v != "" {
			return v
		}
	}
	return ""
}

func getRoleSessionNameFromEnv() string {
	return getEnvsValue(roleSessionNameEnvs)
}

func getStsEndpointFromEnv() string {
	return getEnvsValue([]string{envStsEndpoint})
}

func getStsHttpSchemeFromEnv() string {
	return getEnvsValue([]string{envStsHttpScheme})
}

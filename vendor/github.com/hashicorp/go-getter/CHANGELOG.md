## 1.8.10 (October 6, 2026)

BUG FIXES:

* s3: Only force path-style addressing for non-AWS custom endpoints, restoring the SDK-default virtual-hosted-style for AWS endpoints. Fixes S3 downloads with `AWS_USE_FIPS_ENDPOINT=true`, whose path-style hostnames do not resolve [[GH-676](https://github.com/hashicorp/go-getter/pull/676)]


## 1.8.9 (September 3, 2026)

IMPROVEMENTS:

* build: Updated Go to 1.26.8 [[GH-657](https://github.com/hashicorp/go-getter/pull/682)]
* security: Sanitize file permissions when getting ZIP or TAR files.


## 1.8.8 (August 12, 2026)

SECURITY:

* git: Remove `.git` from the destination dir before `git init`, even if it's a file instead of a directory [[GH-634](https://github.com/hashicorp/go-getter/pull/634)]

IMPROVEMENTS:

* build: Updated Go to 1.26.5 [[GH-657](https://github.com/hashicorp/go-getter/pull/657)]

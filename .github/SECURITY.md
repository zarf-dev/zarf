# Reporting Security Issues

## Report a Vulnerability

Report a security issue or vulnerability through GitHub Security Advisories using the confidential [Report a Vulnerability](https://github.com/zarf-dev/zarf/security/advisories) workflow. Do not disclose exploit details publicly before the team has coordinated a fix and disclosure with you.

## What to Include

Include the affected Zarf version and deployment environment, the affected package create, deploy, runtime, or state-management path, reproduction steps or a proof of concept, the impact and attack preconditions, and a suggested mitigation if known.

## When to Report a Vulnerability

- You found a vulnerability in Zarf code.
- You found an unpatched vulnerability in a Zarf dependency that affects the project.

## When Not to Report a Vulnerability

- You found a bug or malfunction that is not security-related.
- You want to add a feature to Zarf.

Use the [issue tracker](https://github.com/zarf-dev/zarf/issues) for non-security bugs and feature requests.

## Supported Versions

Security fixes are provided for the latest stable minor release of the current major version. Earlier minor releases, release candidates, and nightly builds are unsupported; affected users should upgrade to the current supported release. Maintainers MAY provide a backport when operational impact warrants it.

## Disclosure and Remediation Process

1. Maintainers acknowledge the report and assess its impact.
2. Maintainers coordinate a fix and disclosure with the reporter when appropriate.
3. Zarf publishes a fixed release, security advisory, and migration guidance as applicable.
4. Zarf pursues CVE assignment when appropriate; a CVE is not required before a fix is released.

## Contacting Us

For security discussions that cannot use GitHub Security Advisories, email the maintainers at [zarf-dev-private@googlegroups.com](mailto:zarf-dev-private@googlegroups.com).

## CVE Posture and VEX

Each Zarf release includes a **VEX (Vulnerability Exploitability eXchange) document** that records the project's assessment of known dependency findings for that release. It is not a substitute for a security advisory, a list of all vulnerabilities, or a guarantee that a finding is non-exploitable in every deployment.

Use the VEX document with [`grype`](https://github.com/anchore/grype):

```bash
grype zarf --vex zarf.cli.openvex.json
```

`zarf.cli.openvex.json` is available as a standalone asset on each [GitHub release](https://github.com/zarf-dev/zarf/releases) and is bundled inside the init package.

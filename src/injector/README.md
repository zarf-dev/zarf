Also pushed to dockerhub and can be verified/pulled via:

## Local Verification:

```
sha256sum -c sha256sum
```

## Remote download / verification using [cosign](https://github.com/sigstore/cosign)

```
cosign verify --key ../../cosign.pub defenseunicorns/zarf-injector:0.1.0

cosign verify --key ../../cosign.pub defenseunicorns/zarf-registry:0.2.0

sget --key ../../cosign.pub defenseunicorns/zarf-injector:0.1.0  > zarf-injector
sget --key ../../cosign.pub defenseunicorns/zarf-registry:0.2.0  > zarf-registry
```

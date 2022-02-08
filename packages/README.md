# Zarf Prebuilt Packages
## Prerequisites
- Add zarf binary to path `export PATH=path/to/zarf/binary:$PATH`
- Install `sha256sum` (on Mac it's `brew install coreutils`)

## Usage
executing the command `make package name=NAME_OF_PACKAGE` will build the zarf-package.tar.zst in the folder of the required package. It will also output the required yaml to add to your zarf.yaml components section.  
### Building Composable Packages example: 
```yaml
☁  packages  ⚡  make package name=flux
Created flux add the sha and path to your zarf yaml components: 
  - name: flux
    files:
      - source: "/home/ubuntu/github/zarf/packages/flux/zarf-package-flux.tar.zst"
        shasum: a8932720c6fe95e99be74c9dd1a6e8905b5c92b2b4672b3350e45739f3aad8be
        target: "/usr/local/bin/zarf-package-flux.tar.zst"
    scripts:
      after:
        - "./zarf package deploy /usr/local/bin/zarf-package-flux.tar.zst --confirm"
```

### Composition example:
```yaml
kind: ZarfPackageConfig
metadata:
  name: CompositionExample
  description: "A composed zarf package example."

components:
  - name: flux
    required: true
    files:
      - source: "/home/ubuntu/github/zarf/packages/flux/zarf-package-flux.tar.zst"
        shasum: a8932720c6fe95e99be74c9dd1a6e8905b5c92b2b4672b3350e45739f3aad8be
        target: "/usr/local/bin/zarf-package-flux.tar.zst"
    scripts:
      after:
        - "./zarf package deploy /usr/local/bin/zarf-package-flux.tar.zst --confirm"
```

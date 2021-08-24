# remove all zarf pacakges recursively
remove-packages:
	find . -type f -name 'zarf-package*' -delete

# usage: make test OS=ubuntu
test:
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

test-close:
	vagrant destroy -f

package:
	./build/zarf package create --confirm
	mv zarf*.tar.zst build

	cd build && sha256sum -b zarf* > zarf.sha256
	ls -lh build

package-mac:
	./build/zarf-mac-intel package create --confirm
	mv zarf*.tar.zst build

	cd build && sha256sum -b zarf* > zarf.sha256
	ls -lh build

build-cli:
	rm -fr build
	cd cli && $(MAKE) build
	cd cli && $(MAKE) build-mac

build-test: build-cli package

ci-release: package

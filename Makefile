MAKEFLAGS += --silent

# usage: make test OS=ubuntu
test:
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

# usage: make run-example KIND=game
run-example: build-cli
	cd examples/${KIND} ../../build/zarf package create
	mv examples/${KIND}/zarf-init.tar.zst build
	$(MAKE) test OS=ubuntu

test-close:
	vagrant destroy -f

# Temporary until integration into the cli with <URL>/index.yaml parsing 
charts:
	helm repo add docker-registry https://helm.twun.io
	helm repo add gitea https://dl.gitea.io/charts

	rm -fr charts
	mkdir -p charts
	
	helm pull docker-registry/docker-registry -d ./charts --version 1.10.1
	helm pull gitea/gitea -d ./charts --version 2.2.5

build-cli:
	rm -fr build
	cd cli && $(MAKE) build
	cd cli && $(MAKE) build-mac

build-test: charts build-cli
	./build/zarf package create --config config-utility.yaml
	mv zarf-init.tar.zst build
	cd build && sha256sum -b zarf* > zarf.sha256	

ci-release: charts
	./build/zarf package create --config config-utility.yaml
	mv zarf-init.tar.zst build

	./build/zarf package create
	mv zarf-init.tar.zst build

	cd build && sha256sum -b zarf* > zarf.sha256	
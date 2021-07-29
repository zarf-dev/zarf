# usage: make test OS=ubuntu
test:
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

test-close:
	vagrant destroy -f

# Temporary until integration into the cli with <URL>/index.yaml parsing 
charts:
	helm repo add docker-registry https://helm.twun.io
	helm repo add gitea https://dl.gitea.io/charts
	helm repo add grafana https://grafana.github.io/helm-charts

	rm -fr charts
	mkdir -p charts
	
	helm pull docker-registry/docker-registry -d ./charts --version 1.10.1
	helm pull gitea/gitea -d ./charts --version 2.2.5
	helm pull grafana/loki-stack -d ./charts --version 2.4.1

package: charts
	./build/zarf package create --confirm --config config-utility.yaml

	./build/zarf package create --confirm
	mv zarf*.tar.zst build

	cd build && sha256sum -b zarf* > zarf.sha256	
	ls -lh build

build-cli:
	rm -fr build
	cd cli && $(MAKE) build
	cd cli && $(MAKE) build-mac

build-test: build-cli package

ci-release: package
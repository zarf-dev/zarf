MAKEFLAGS += --silent

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

	rm -fr charts
	mkdir -p charts
	
	helm pull docker-registry/docker-registry -d ./charts --version 1.10.1
	helm pull gitea/gitea -d ./charts --version 2.2.5

build-cli:
	rm -fr build
	cd cli && $(MAKE) build
	
release: charts build-cli
	./build/zarf package create
	mv zarf-initialize.tar.zst build
	sha256sum -b build/zarf* > build/zarf.sha256	
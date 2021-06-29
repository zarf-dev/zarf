MAKEFLAGS += --silent

test:
	vagrant destroy -f
	vagrant up --no-color ${OS}
	echo -e "\n\n\n\033[1;93m  ✅ BUILD COMPLETE.  To access this environment, run \"vagrant ssh ${OS}\"\n\n\n"

test-destory:
	vagrant destroy -f

charts:

charts:
	helm repo add docker-registry https://helm.twun.io
	helm repo add gitea https://dl.gitea.io/charts

	mkdir -p charts
	
	helm pull docker-registry/docker-registry -d ./charts --version 1.10.1
	helm pull gitea/gitea -d ./charts --version 2.2.5

release:
	sha256sum -b bin/zarf* > bin/zarf.sha256	
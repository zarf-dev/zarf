

kill-mutants:
	kubectl delete pod -l zarf.dev/agent=patched

watch-hook:
	kubectl logs -f -n zarf -l app=agent-hook --since 1s

dev-setup:
	cd ../../../ && \
	go run src/main.go pki regenerate --host agent-hook.zarf.svc && \
	kubectl create ns zarf --dry-run=client -o yaml | kubectl apply -f - && \
	kubectl apply -f assets/manifests/agent/service.yaml && \
	kubectl create secret -n zarf tls agent-hook-tls \
		--cert "zarf-pki/zarf-server.crt" \
		--key "zarf-pki/zarf-server.key" \
		--dry-run=client -o yaml | kubectl apply -f - && \
	sed -e 's@###ZARF_AGENT_CA###@'"$$(cat zarf-pki/zarf-ca.crt | base64 | tr -d '\n')"'@g' <"assets/manifests/agent/webhook.yaml" | kubectl apply -f -

dev-build:
	# Skaffold would be totally fine for this except that it seems that M1 docker go builds are insanely slow vs native making skaffold less valuable here
	$(eval tag := $(shell date +%s))
	cd ../../../ && \
	CGO_ENABLED=0 GOOS=linux go build -o build/zarf cli/main.go && \
	docker build --tag zarf-agent:$(tag) --file Dockerfile.dev . && \
	kind load docker-image zarf-agent:$(tag) && \
	sed -e 's@###ZARF_REGISTRY###\/defenseunicorns\/zarf\-agent\:v0.15@'"zarf-agent:$(tag)"'@g' < "assets/manifests/agent/deployment.yaml" | kubectl apply -f -

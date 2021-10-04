## Zarf Simple gitops service Update

This examples shows how to package images and repos to be loaded into the gitops service.  This package does not deploy anything itself, but pushes assets to the gitops service to be consumed by the gitops engine of your choice.

### Steps to use:
1. Create a Zarf cluster as outlined in the main [README](../../README.md#2-create-the-zarf-cluster), note the git username / password output at the end
2. Follow [step 3](../../README.md#3-add-resources-to-the-zarf-cluster) using this config in this folder
3. Run `kubectl apply -k https://zarf-git-user:$(./zarf tools get-admin-password)@zarf.localhost/zarf-git-user/mirror__github.com__stefanprodan__podinfo//kustomize` to deploy podinfo into cluster from the gitops service

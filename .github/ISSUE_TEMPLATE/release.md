
📅 Target dates:

Release Manager: 

Release checklist:
🚨 Update the release version variable to the version
you are releasing. 

[ ] `export RELEASE_VERSION=v0.33.1`

[ ] `git tag -sa $RELEASE_VERSION -m "$RELEASE_VERSION" `

[ ] The tag has to be pushed to a remote `git push upstream/origin`

[ ] Merge the `goreleaser` PR in the [homebrew-tap repository](https://github.com/defenseunicorns/homebrew-tap)

[ ] Send release update in the #zarf Kubernetes Channel

[ ] Send release update in the #zarf OpenSSF channel 

[ ] Send release update in the [public Zarf Google Group](https://groups.google.com/g/zarf-dev)


TODO:
(Issues to complete, or PRs to merge before release is cut)
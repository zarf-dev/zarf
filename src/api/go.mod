module github.com/zarf-dev/zarf/src/api

go 1.22.4

replace github.com/zarf-dev/zarf => ../..

require (
	github.com/defenseunicorns/pkg/helpers/v2 v2.0.1
	github.com/invopop/jsonschema v0.12.0
	github.com/stretchr/testify v1.9.0
	github.com/zarf-dev/zarf v0.37.0
	k8s.io/apimachinery v0.30.0
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/otiai10/copy v1.14.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/utils v0.0.0-20231127182322-b307cd553661 // indirect
	oras.land/oras-go/v2 v2.5.0 // indirect
)

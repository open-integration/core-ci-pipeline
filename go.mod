module open-integration/core-ci-pipeline

go 1.13

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Shopify/ejson v1.2.1 // indirect
	github.com/aws/aws-sdk-go v1.29.10 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/docker/libkv v0.2.1 // indirect
	github.com/dustin/gojson v0.0.0-20160307161227-2e71ec9dd5ad // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gobuffalo/envy v1.9.0 // indirect
	github.com/gobuffalo/packd v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/gosimple/slug v1.9.0 // indirect
	github.com/hairyhenderson/gomplate v3.5.0+incompatible
	github.com/hairyhenderson/toml v0.3.0 // indirect
	github.com/hashicorp/consul/api v1.4.0 // indirect
	github.com/hashicorp/vault/api v1.0.4 // indirect
	github.com/inconshreveable/log15 v0.0.0-20200109203555-b30bc20e4fd1 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/open-integration/core v0.43.0
	github.com/open-integration/service-catalog/google-spreadsheet v0.10.0 // indirect
	github.com/open-integration/service-catalog/kubernetes v0.2.0
	github.com/rogpeppe/go-internal v1.5.2 // indirect
	github.com/zealic/xignore v0.3.3 // indirect
	golang.org/x/crypto v0.0.0-20200221231518-2aa609cf4a9d // indirect
	golang.org/x/net v0.0.0-20200222125558-5a598a2470a0 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sys v0.0.0-20200223170610-d5e6a3e2c0ae // indirect
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63 // indirect
	google.golang.org/grpc v1.27.1 // indirect
	gopkg.in/hairyhenderson/yaml.v2 v2.0.0-00010101000000-000000000000
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace gopkg.in/hairyhenderson/yaml.v2 => github.com/maxaudron/yaml v0.0.0-20190411130442-27c13492fe3c

replace github.com/open-integration/core => ../core
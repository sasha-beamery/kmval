# kmval
`kmval` is a `kustomize` manifest validator that uses `kustomize` and `yq` to run user-defined validations against
any type of Kubernetes object produced by building a `kustomize` `base` or `overlay` layer.

- [Installation](#installation)
  * [Go Get](#go-get)
  * [Homebrew](#homebrew)
- [Background](#background)
- [Usage](#usage)
  * [`validations.yaml`](#-validationsyaml-)
  * [Output](#output)
  * [CLI](#cli)
- [Release](#release)

## Installation
### Go Get
```bash
go get -u github.com/LGUG2Z/kmval
cd ${GOPATH}/src/github.com/LGUG2Z/kmval
make install
```

### Homebrew
```bash
brew tap LGUG2Z/tap
brew install LGUG2Z/tap/kmval
```

## Background
At [Beamery](https://beamery.com), where I work, we deploy over 70 services as various Kubernetes objects on our clusters.
Alongside mainline path-to-production environments, there are also scaled-down development and staging environments used by
delivery teams. This results in `>70 * N` Kubernetes manifest configurations to keep a handle on, which is not really possible
to do manually.

[Kustomize](https://kustomize.io) is an excellent tool that provides Kubernetes-native configuration management. Being
able to define a common base layer and patch differences such as replica numbers, resource requests etc. per environment
using Kustomize `overlay`s really helps to improve consistency, and opens up the path to running linting and quality tools
that would otherwise not be possible to use with templating which often results in invalid `yaml` being stored in version
control and templated at deploy-time.

The `kustomize build` command has an additional use as an overlay validator and can be scripted to ensure that every
base layer and overlay conforms to the semantics of `kustomize` to produce a valid list of `yaml` documents as its
output:

```bash
#!/usr/bin/env bash

kustomizatons=$(find . -name kustomization.yaml -print0 | xargs -0 -n1 dirname | sort --unique)
for kustomization in ${kustomizatons}; do
	if kustomize build "${kustomization}" -o /tmp/manifest.yaml; then
		echo "validated ${kustomization}"
	else
		echo "validation failed for ${kustomization}"
		exit 1
	fi
done
```

`kmval` takes this a step further and allows you to perform validations on specific parts of the manifests produced by
`kustomize` across multiple overlays. This can include node affinity keys, tolerations, replica numbers, resource requests...
Anything that can be looked up via a `yaml` path.

## Usage
### `validations.yaml`
Assume the following folder structure:
```text
├── api
│   ├── acl
│   │   ├── base
│   │   ├── canary
│   │   ├── production
│   │   ├── ...
│   ├── auth
│   ├── settings
│   ├── ...
├── app
└── ...
```

At each top level directory containing subdirectories of `kustomize` layers for various deployable artifacts, `kmval`
will look for a `validations.yaml` file which declares assertions against Kubernetes objects to be validated
for each `kustomize` layer.

Taking the example of the `api` top level folder and the `acl` artifact:

```yaml
common:
  Deployment:
    defined:
      spec.template.spec.tolerations: true
      spec.template.spec.affinity.nodeAffinity: true
      spec.template.spec.containers[0].resources.requests: true
      spec.template.spec.containers[0].resources.limits: true
artifacts:
  acl:
    base:
      Deployment:
        strings:
          spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0]: api
          spec.template.spec.tolerations[0].value: api
          metadata.namespace: acl
        integers:
          spec.replicas: 5
    overlays:
      canary:
        Deployment:
          integers:
            spec.replicas: 3
      production:
```

There are four basic types of validations:
* `defined`: Validate that a property is or is not set at the given `yaml` path
* `integers`: Validate that the property set at the given `yaml` path is an exact match for the given integer
* `strings`: Validate that the property set at the given `yaml` path is an exact match for the given string
* `partials`: Validate that the property set at the given `yaml` path is partial match for the given string

The `common` block declares validations for any matching Kubernetes object in any artifact added to the `validations.yaml`
file. In the example given, every `Deployment` object should have tolerations, node affinity, resource requests and resource
limits set.

The `artifacts` block declares the artifacts against which to run validations for different Kubernetes objects. `kmval`
will build their paths from the structure of the map:
* `artifacts.acl.base` will run validations against the output of `kustomize build` in `./acl/base`
* `artifacts.acl.overlays.canary` will run validations against the output of `kustomize build` in `./acl/canary`
* `artifacts.acl.overlays.production` will run validations against the output of `kustomize build` in `./acl/production`

The `base` block for the `acl` artifact declares that it should have a node affinity set for a node pool with a label
matching the value `api`, a toleration for any nodes with the taint value `api`, and that it should be deployed in the
`acl` namespace. Every validation from the base block will also be applied to any overlay defined unless it is explicitly
overridden.

In the `overlays.canary` block, for the `Deployment` object, the integer validation for the number of replicas is overridden
fom `5` replicas in the `base` layer to `3`.

The `overlays.production` layer is empty, meaning it will inherit and run all validations from the `common` block and the
`acl.base` block.

### Output
An example of the validation output based on the example `validations.yaml` file from the previous section:

```text
❯ kmval

acl/base
PASS: Deployment metadata.namespace acl
PASS: Deployment spec.replicas 5
PASS: Deployment spec.template.spec.affinity.nodeAffinity DEFINED
PASS: Deployment spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0] api
PASS: Deployment spec.template.spec.containers[0].resources.limits DEFINED
PASS: Deployment spec.template.spec.containers[0].resources.requests DEFINED
PASS: Deployment spec.template.spec.tolerations DEFINED
PASS: Deployment spec.template.spec.tolerations[0].value api

acl/canary
PASS: Deployment metadata.namespace acl
PASS: Deployment spec.template.spec.affinity.nodeAffinity DEFINED
PASS: Deployment spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0] api
PASS: Deployment spec.template.spec.containers[0].resources.limits DEFINED
PASS: Deployment spec.template.spec.containers[0].resources.requests DEFINED
PASS: Deployment spec.template.spec.tolerations DEFINED
PASS: Deployment spec.template.spec.tolerations[0].value api
FAIL: Deployment spec.replicas 3

acl/production
PASS: Deployment metadata.namespace acl
PASS: Deployment spec.replicas 5
PASS: Deployment spec.template.spec.affinity.nodeAffinity DEFINED
PASS: Deployment spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values[0] api
PASS: Deployment spec.template.spec.containers[0].resources.limits DEFINED
PASS: Deployment spec.template.spec.containers[0].resources.requests DEFINED
PASS: Deployment spec.template.spec.tolerations DEFINED
PASS: Deployment spec.template.spec.tolerations[0].value api

PASSED: acl/base
PASSED: acl/production

FAILED: acl/canary

Manifest validations failed!
```

We can see that a patch had not been created to change the number of replicas running on the presumably scaled-down `canary` environment,
so the validation for `acl/canary` reports as failed.

### CLI
`kmval` can either be run in the directory where the `validations.yaml` file to run against is kept, or a single
argument can be given defining an absolute or relative path to the directory where the `validations.yaml` file to
run against is kept. Alternatively, the `validations.yaml` filename can be overridden using the `--file` flag,
making it possible to keep validations separated in a single top level folder by area of concern.

```text
NAME:
   kmval - Kustomize Manifest Validator

USAGE:
   kmval [global options] command [command options] [arguments...]

VERSION:
   0.0.1

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --fail-fast    stop running validations after the first failure
   --file value   name of validations file (default: "validations.yaml")
   --help, -h     show help
   --version, -v  print the version
```

## Release

```
git clone https://github.com/LGUG2Z/kmval.git
cd kmval
make build_all
gh auth login
gh release upload v`cat VERSION` kmval_`cat VERSION`_*.tar.gz checksums.txt --clobber
make clean
```

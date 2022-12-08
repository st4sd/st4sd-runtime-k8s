# ST4SD Runtime K8s

## Details

Kubernetes operator for `workflows.st4sd.ibm.com`.

The documentation of the `Workflow` schema is in [docs/schema.md](docs/schema.md).

## Example

The [Workflow object below](examples/sum-numbers.yaml)  below instructs Kubernetes/OpenShift to execute the [`sum-numbers`](https://github.com/st4sd/sum-numbers/) workflow. The [`tutorial`](https://pages.ibm.com/st4sd/overview/tutorial/) explains the [FlowIR](www.github.com/st4sd/st4sd-runtime-core) implementation of the toy-example [sum-numbers](https://github.com/st4sd/sum-numbers/).


```yaml
# Assumes a full ST4SD deployment because it extracts configuration metadata from
# the `st4sd-runtime-service` ConfigMap

apiVersion: st4sd.ibm.com/v1alpha1
kind: Workflow
metadata:
  namespace: sum-numbers
spec:
  package:
    url: https://github.com/st4sd/sum-numbers/
    branch: main
```

## Quick links

- [Getting started](#getting-started)
- [Development](#development)
- [Help and Support](#help-and-support)
- [Contributing](#contributing)
- [License](#license)

## Getting started

### Requirements


- Install [Go](https://go.dev/dl/) v1.19
- [operator-sdk](https://v1-17-x.sdk.operatorframework.io/) v1.17x

## Development

- Build the binaries: `make build`
- If you want to start fresh with a different version of the operator sdk:

    ```bash
    operator-sdk init --domain ibm.com --repo github.com/st4sd/st4sd-runtime-k8s
    operator-sdk create api --group st4sd --version v1alpha1 --kind Workflow --resource --controller --namespaced=true
    ```

- To modify the workflow schema:
  - Modifying [workflow_types.go](api/v1alpha1/workflow_types.go) accordingly
  - Execute `make generate`
  - You may then build the new Custom Resource Definition (CRD): `make manifests`.
    - Use the new [Workflow CRD](config/crd/bases/st4sd.ibm.com_workflows.yaml) (e.g. `kubectly apply -f config/crd/bases/st4sd.ibm.com_workflows.yaml`)

### Installing dependencies

Install the dependencies for this project with:

```bash
go get .
```

### Developing locally

Coming soon.

### Lint and fix files

Coming soon.

## Help and Support

Please feel free to reach out to one of the maintainers listed in the [MAINTAINERS.md](MAINTAINERS.md) page.

## Contributing

We always welcome external contributions. Please see our [guidance](CONTRIBUTING.md) for details on how to do so.

## License

This project is licensed under the Apache 2.0 license. Please [see details here](LICENSE.md).

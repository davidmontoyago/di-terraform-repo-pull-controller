# di-terraform-repo-pull-controller

In a GitOps deployment model, infrastructure changes are pulled from a source repository and applied asynchronously. This differs from the traditional "push" model where changes are pushed to a repo and then applied via a CI pipeline.

This repository implements a K8s controller that manages `Repo` resources and watches for new commits on a given repo url. When a new commit revision is pushed, the controller will schedule a Job to apply the Terraform changes.

## Getting Started

```sh
# build controller
make build

# deploy controller & CRD
make deploy

# create a custom resource Repo with a valid repo Url
kubectl apply -f https://raw.githubusercontent.com/davidmontoyago/di-terraform-repo-pull-controller-sample-repo/master/deployment/repo-resource.yaml

# check jobs created through the custom resource
kubectl get jobs
```

## Controller Details

The controller makes use of the generators in [k8s.io/code-generator](https://github.com/kubernetes/code-generator)
to generate a typed client, informers, listers and deep-copy functions.

Re-generate the client:
```
make gen
```

The controller uses [client-go library](https://github.com/kubernetes/client-go/tree/master/tools/cache) extensively.
The details of interaction points of the sample controller with various mechanisms from this library are
explained [here](docs/controller-client-go.md).

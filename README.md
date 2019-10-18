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

## How It Works
The controller uses "Informers" to be notified of changes to `Repo` or `Job` resources. When a `Repo` resource is created, a `RepoPoller` goroutine will run to check the source repo for new revisions. When a new revision is found, its "Run" status will be updated to trigger the scheduling of a new Job to apply the changes. The repo "Run" status will be reconciled by `syncHandler`.

All `Repo` resource changes are processed via a work queue. From the original K8s `sample-controller` documentation:

> workqueue is a rate limited work queue. This is used to queue work to be
	processed instead of performing it as soon as a change happens. This
	means we can ensure we only process a fixed amount of resources at a
	time, and makes it easy to ensure we are never processing the same item
	simultaneously in two different workers.

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

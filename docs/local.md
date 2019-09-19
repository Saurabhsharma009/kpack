### Local Development Install

Access to a Kubernetes cluster is needed in order to install the kpack controllers.

```bash
kubectl cluster-info # ensure you have access to a cluster
./hack/apply.sh <IMAGE/NAME> # <IMAGE/NAME> is a writable and publicly accessible location 
```

### Running Tests

* To run the e2e tests, kpack must be installed and running on a cluster
* The IMAGE_REGISTRY environment variable must point at a registry with local write access 

```bash
IMAGE_REGISTRY=gcr.io/<some-project> go test ./...
```
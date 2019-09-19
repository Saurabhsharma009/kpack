# Builders

In kpack the Builder and ClusterBuilder resources are a reference to a [Cloud Native Buildpacks builder image](https://buildpacks.io/docs/using-pack/working-with-builders/). 
The builder image contains buildpacks that will be used to build images with kpack.

The builder resource tracks the buildpacks in the builder image on the registry. This enables kpack to automatically rebuild images when there are relevant buildpack updates.
These Builder resources need to be created prior to the creation of any Image Resource, because they will define what builder will be used to create these images.      

### Builders
The Builder resource is namespace scoped and can only be used by images in the same namespace.   

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: Builder
metadata:
  name: sample-builder
spec:
  image: cloudfoundry/cnb:bionic
  # imagePullSecrets: # Use these secrets if credentials are required to pull the builder
  # - name: builder-secret
```
- `name`: The name of the builder that will be used to reference by the image.
- `image`: Builder image tag.
- `updatePolicy`: Update policy of the builder. Valid options are `polling` and `external`
The major difference between the options is that `external` require a user to update the resource by applying a new
configuration. While `polling` automatically checks every 5 minutes to see if a new version of the builder image exists
- `imagePullSecrets`: This is an optional parameter that should only be used if the builder image is in a
private registry. [To create this secret please reference this link](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials)

A sample builder is available in [samples/builder](../samples/builder.yaml) 

### ClusterBuilder

The ClusterBuilder resource is cluster scoped and can only be used in any namespace.   

```yaml
apiVersion: build.pivotal.io/v1alpha1
kind: ClusterBuilder
metadata:
  name: cluster-sample-builder
spec:
  image: cloudfoundry/cnb:bionic
```
- `name`: The name of the builder that will be used to reference by the image.
- `namespace`: Namespace where the builder builder will be created
- `image`: Builder image tag.
- `updatePolicy`: Update policy of the builder. Valid options are `polling` and `external`
The major difference between the options is that `external` require a user to update the resource by applying a new
configuration. While `polling` automatically checks every 5 minutes to see if a new version of the builder image exists

> Note: ClusterBuilders do not support imagePullSecrets. Therefore the builder image must be available to kpack without credentials.

A sample cluster builder is available in [samples/cluster_builder.yaml](../samples/cluster_builder.yaml) 

### Suggested builders

The most commonly used builders are [cloudfoundry/cnb:bionic](https://hub.docker.com/r/cloudfoundry/cnb) and [cloudfoundry/cnb](https://hub.docker.com/r/cloudfoundry/cnb).
 
### Creating your own builder  

To create your own builder with custom buildpacks follow the instructions on creating them using the [pack cli](https://buildpacks.io/docs/using-pack/working-with-builders/).
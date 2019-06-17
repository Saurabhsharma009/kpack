package cnbbuild

import (
	"context"
	"fmt"

	knv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	knv1alpha1informer "github.com/knative/build/pkg/client/informers/externalversions/build/v1alpha1"
	knv1alpha1lister "github.com/knative/build/pkg/client/listers/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informer "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1lister "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	ReconcilerName = "CNBBuilds"
	Kind           = "CNBBuild"
)

type MetadataRetriever interface {
	GetBuiltImage(repoName registry.ImageRef) (registry.BuiltImage, error)
}

func NewController(opt reconciler.Options, knClient knversioned.Interface, cnbinformer v1alpha1informer.CNBBuildInformer, kninformer knv1alpha1informer.BuildInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		KNClient:          knClient,
		CNBClient:         opt.CNBClient,
		CNBLister:         cnbinformer.Lister(),
		KnLister:          kninformer.Lister(),
		MetadataRetriever: metadataRetriever,
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	cnbinformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	kninformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	KNClient          knversioned.Interface
	CNBClient         versioned.Interface
	CNBLister         v1alpha1lister.CNBBuildLister
	KnLister          knv1alpha1lister.BuildLister
	MetadataRetriever MetadataRetriever
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	build, err := c.CNBLister.CNBBuilds(namespace).Get(buildName)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	build = build.DeepCopy()

	knBuild, err := c.KnLister.Builds(namespace).Get(buildName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		knBuild, err = c.createKNBuild(namespace, build)
		if err != nil {
			return err
		}
	}

	if knBuild.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() && !build.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() {
		image, err := c.MetadataRetriever.GetBuiltImage(build)
		if err != nil {
			return err
		}

		build.Status.BuildMetadata = buildMetadataFromBuiltImage(image)
		build.Status.SHA = image.SHA
	}

	build.Status.Conditions = knBuild.Status.Conditions
	build.Status.ObservedGeneration = build.Generation

	_, err = c.CNBClient.BuildV1alpha1().CNBBuilds(namespace).UpdateStatus(build)
	if err != nil {
		return err
	}

	return nil
}

func (c *Reconciler) createKNBuild(namespace string, build *v1alpha1.CNBBuild) (*knv1alpha1.Build, error) {
	const userId = 1000
	const groupId = 1000
	const cacheDirName = "empty-dir"
	const layersDirName = "layers-dir"
	return c.KNClient.BuildV1alpha1().Builds(namespace).Create(&knv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: build.Name,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(build),
			},
		},
		Spec: knv1alpha1.BuildSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Source: &knv1alpha1.SourceSpec{
				Git: &knv1alpha1.GitSourceSpec{
					Url:      build.Spec.GitURL,
					Revision: build.Spec.GitRevision,
				},
			},
			Steps: []corev1.Container{
				{
					Name:    "prepare",
					Image:   "alpine",
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						fmt.Sprintf(`chown -R "%d:%d" "/builder/home" &&
chown -R "%d:%d" /layers &&
chown -R "%d:%d" /cache &&
chown -R "%d:%d" /workspace`,
							userId, groupId,
							userId, groupId,
							userId, groupId,
							userId, groupId,
						),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name: cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "detect",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/detector"},
					Args: []string{
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "restore",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/restorer"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name: cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "analyze",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/analyzer"},
					Args: []string{
						"-layers=/layers",
						"-helpers=false",
						"-group=/layers/group.toml",
						build.Spec.Image,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "build",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/builder"},
					Args: []string{
						"-layers=/layers",
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "export",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/exporter"},
					Args: []string{
						"-layers=/layers",
						"-helpers=false",
						"-app=/workspace",
						"-group=/layers/group.toml",
						build.Spec.Image,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "cache",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/cacher"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name: cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "Always",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name:         cacheDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: layersDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	})
}

func buildMetadataFromBuiltImage(image registry.BuiltImage) []v1alpha1.CNBBuildpackMetadata {
	buildpackMetadata := make([]v1alpha1.CNBBuildpackMetadata, 0, len(image.BuildpackMetadata))
	for _, metadata := range image.BuildpackMetadata {
		buildpackMetadata = append(buildpackMetadata, v1alpha1.CNBBuildpackMetadata{
			ID:      metadata.ID,
			Version: metadata.Version,
		})
	}
	return buildpackMetadata
}
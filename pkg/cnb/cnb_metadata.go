package cnb

import (
	"encoding/json"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/image/source/registry"
)

const BuilderMetadataLabel = "io.buildpacks.builder.metadata"

type BuildpackMetadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type BuilderImageMetadata struct {
	Buildpacks []BuildpackMetadata `json:"buildpacks"`
}

type BuilderImage struct {
	BuilderBuildpackMetadata BuilderMetadata
	Identifier               string
}

type BuilderMetadata []BuildpackMetadata

type RemoteMetadataRetriever struct {
	LifecycleImageFactory registry.RemoteImageFactory
}

func (r *RemoteMetadataRetriever) GetBuilderImage(repo registry.ImageRef) (BuilderImage, error) {
	img, err := r.LifecycleImageFactory.NewRemote(repo)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	var metadataJSON string
	metadataJSON, err = img.Label(BuilderMetadataLabel)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "builder image metadata label not present")
	}

	var metadata BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	identifier, err := img.Identifier()
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "failed to retrieve builder image SHA")
	}

	return BuilderImage{
		BuilderBuildpackMetadata: metadata.Buildpacks,
		Identifier:               identifier.String(),
	}, nil
}

func (r *RemoteMetadataRetriever) GetBuiltImage(ref registry.ImageRef) (BuiltImage, error) {
	img, err := r.LifecycleImageFactory.NewRemote(ref)
	if err != nil {
		return BuiltImage{}, err
	}

	var metadataJSON string
	metadataJSON, err = img.Label(lcyclemd.AppMetadataLabel)
	if err != nil {
		return BuiltImage{}, err
	}

	var metadata lcyclemd.AppImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuiltImage{}, err
	}

	imageCreatedAt, err := img.CreatedAt()
	if err != nil {
		return BuiltImage{}, err
	}

	identifier, err := img.Identifier()
	if err != nil {
		return BuiltImage{}, err
	}

	return BuiltImage{
		Identifier:        identifier.String(),
		CompletedAt:       imageCreatedAt,
		BuildpackMetadata: metadata.Buildpacks,
	}, nil
}

type BuiltImage struct {
	Identifier        string
	CompletedAt       time.Time
	BuildpackMetadata []lcyclemd.BuildpackMetadata
}

package oci

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sirupsen/logrus"
)

// The layer media types supported when no layer media type is
// configured. The first layer having one of them is extracted.
var supportedLayerMediaTypes = []string{
	// Produced by `flux push artifact`
	"application/vnd.cncf.flux.content.v1.tar+gzip",
	// Produced by `oras push` on a directory
	"application/vnd.oci.image.layer.v1.tar+gzip",
	"application/vnd.oci.image.layer.v1.tar+zstd",
	"application/vnd.oci.image.layer.v1.tar",
	"application/vnd.docker.image.rootfs.diff.tar.gzip",
}

// ArtifactDir returns the directory where the artifact identified by
// its manifest digest is extracted.
func ArtifactDir(path, digest string) string {
	return filepath.Join(path, strings.ReplaceAll(digest, ":", "-"))
}

// pull downloads the artifact identified by digest and extracts its
// layer into ArtifactDir. If this directory already exists, the
// artifact is not downloaded.
func pull(dir string, repo name.Repository, digest, layerMediaType string, opts []remote.Option) error {
	artifactDir := ArtifactDir(dir, digest)
	if _, err := os.Stat(artifactDir); err == nil {
		return nil
	}
	// Pulling by digest ensures the content is the one which has
	// been resolved and verified.
	img, err := remote.Image(repo.Digest(digest), opts...)
	if err != nil {
		return err
	}
	layer, err := selectLayer(img, layerMediaType)
	if err != nil {
		return fmt.Errorf("artifact %s: %w", digest, err)
	}
	// Uncompressed detects the gzip and zstd compressions, and
	// verifies the layer digest.
	rc, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer rc.Close() // nolint: errcheck

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	if err := extract(tmpDir, rc); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to extract the artifact %s: %w", digest, err)
	}
	if err := os.Chmod(tmpDir, 0755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := os.Rename(tmpDir, artifactDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	logrus.Infof("oci: the artifact %s has been extracted into %s", digest, artifactDir)
	return nil
}

func selectLayer(img v1.Image, layerMediaType string) (v1.Layer, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return nil, err
	}
	mediaTypes := make([]string, len(manifest.Layers))
	for i, l := range manifest.Layers {
		mediaTypes[i] = string(l.MediaType)
		match := string(l.MediaType) == layerMediaType
		if layerMediaType == "" {
			match = slices.Contains(supportedLayerMediaTypes, string(l.MediaType))
		}
		if match {
			return img.LayerByDigest(l.Digest)
		}
	}
	if layerMediaType != "" {
		return nil, fmt.Errorf("no layer with the media type %s has been found in %v", layerMediaType, mediaTypes)
	}
	return nil, fmt.Errorf("no layer with a supported media type has been found in %v: the layer media type can be configured", mediaTypes)
}

// extract extracts a tar archive into dir. The os.Root API ensures
// files can not be written outside of dir, even through symlinks.
func extract(dir string, r io.Reader) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close() // nolint: errcheck

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(hdr.Name)
		if name == "." {
			continue
		}
		if !filepath.IsLocal(name) {
			return fmt.Errorf("the archive contains the invalid path %s", hdr.Name)
		}
		if err := root.MkdirAll(path.Dir(name), 0755); err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			mode := os.FileMode(0644)
			if hdr.FileInfo().Mode()&0111 != 0 {
				mode = 0755
			}
			f, err := root.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := root.Symlink(hdr.Linkname, name); err != nil {
				return err
			}
		case tar.TypeLink:
			target := path.Clean(hdr.Linkname)
			if !filepath.IsLocal(target) {
				return fmt.Errorf("the archive contains the invalid hard link %s to %s", hdr.Name, hdr.Linkname)
			}
			if err := root.Link(target, name); err != nil {
				return err
			}
		default:
			logrus.Warnf("oci: the archive entry %s of type %c is ignored", hdr.Name, hdr.Typeflag)
		}
	}
}

// gc removes extracted artifacts except the ones listed in keep
func gc(dir string, keep []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	keepDirs := make([]string, 0, len(keep))
	for _, digest := range keep {
		if digest != "" {
			keepDirs = append(keepDirs, filepath.Base(ArtifactDir(dir, digest)))
		}
	}
	for _, e := range entries {
		if slices.Contains(keepDirs, e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		logrus.Debugf("oci: removing %s", p)
		if err := os.RemoveAll(p); err != nil {
			logrus.Errorf("oci: failed to remove %s: %s", p, err)
		}
	}
}

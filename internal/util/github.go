package util

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/go-github/v69/github"

	"github.com/FuturFusion/migration-manager/internal/version"
)

type githubRepo struct {
	owner string
	repo  string
	tag   string

	client        *github.Client
	releaseAssets map[string]int64
}

// GetProjectRepo returns a GitHub repository client for the project at the current tag, and populates release assets.
// If latest is true, release assets will be populated from the latest release instead of the tag.
func GetProjectRepo(ctx context.Context, latest bool) (*githubRepo, error) {
	g := &githubRepo{
		owner: "futurfusion",
		repo:  "migration-manager",
		tag:   version.GoVersion(),

		client: github.NewClient(nil),
	}

	var err error
	var release *github.RepositoryRelease
	if latest {
		release, _, err = g.client.Repositories.GetLatestRelease(ctx, g.owner, g.repo)
		if err != nil {
			return nil, fmt.Errorf("Failed to get latest GitHub release: %w", err)
		}
	} else {
		release, _, err = g.client.Repositories.GetReleaseByTag(ctx, g.owner, g.repo, g.tag)
		if err != nil {
			return nil, fmt.Errorf("Failed to get GitHub release for tag %q: %w", g.tag, err)
		}
	}

	g.releaseAssets = make(map[string]int64, len(release.Assets))
	for _, r := range release.Assets {
		g.releaseAssets[r.GetName()] = r.GetID()
	}

	return g, nil
}

// DownloadAsset downloads the given release asset to the given target location.
func (g *githubRepo) DownloadAsset(ctx context.Context, target string, asset string) error {
	assetID, ok := g.releaseAssets[asset]
	if !ok {
		return fmt.Errorf("No GitHub release asset found with name %q", asset)
	}

	rc, _, err := g.client.Repositories.DownloadReleaseAsset(ctx, g.owner, g.repo, assetID, http.DefaultClient)
	if err != nil {
		return fmt.Errorf("Failed to fetch GitHub release asset with name %q: %w", asset, err)
	}

	defer func() { _ = rc.Close() }()
	f, err := os.Create(target)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("Failed to create file at %q: %w", target, err)
		}

		f, err = os.OpenFile(target, os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("Failed to open file at %q: %w", target, err)
		}
	}

	defer func() { _ = f.Close() }()

	// Setup a gzip reader to decompress during streaming.
	body, err := gzip.NewReader(rc)
	if err != nil {
		return err
	}

	defer func() { _ = body.Close() }()

	// Read from the decompressor in chunks to avoid excessive memory consumption.
	for {
		_, err = io.CopyN(f, body, 4*1024*1024)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("Failed to copy GitHub release asset to %q: %w", target, err)
		}
	}

	return nil
}

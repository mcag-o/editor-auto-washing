package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

type BundleRouter struct{}

func NewBundleRouter() *BundleRouter {
	return &BundleRouter{}
}

func (r *BundleRouter) RouteToProcessed(sourcePath, processedDir string) (string, error) {
	return r.route(sourcePath, processedDir)
}

func (r *BundleRouter) RouteToFailed(sourcePath, failedDir string) (string, error) {
	return r.route(sourcePath, failedDir)
}

func (r *BundleRouter) route(sourcePath, targetDir string) (string, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create bundle route dir: %w", err)
	}
	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return "", fmt.Errorf("route bundle %s: %w", sourcePath, err)
	}
	return targetPath, nil
}

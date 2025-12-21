// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// ContractArtifactURL is the URL to op-node v1.16.3-v20 artifacts hosted on Scaleway S3.
// These match the struct definitions in optimism v1.16.3 (29 fields in DeployImplementationsOutput).
// v20: Further optimizations for contracts that exceeded 24KB limit.
const ContractArtifactURL = "https://op-contracts.s3.nl-ams.scw.cloud/artifacts-op-node-v1.16.3-v20.tzst"

// ContractArtifactDownloader handles downloading and extracting OP Stack contract artifacts.
// It bypasses op-deployer's built-in artifact handling which has issues with
// directory structure expectations.
type ContractArtifactDownloader struct {
	cacheDir string
	mu       sync.Mutex
}

// NewContractArtifactDownloader creates a new contract artifact downloader.
func NewContractArtifactDownloader(cacheDir string) *ContractArtifactDownloader {
	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	return &ContractArtifactDownloader{
		cacheDir: cacheDir,
	}
}

// Download downloads and extracts artifacts, returning the path to the extracted directory.
// The result can be used with artifacts.NewFileLocator() for op-deployer.
// Caching is DISABLED - always downloads fresh to avoid stale artifact issues.
func (d *ContractArtifactDownloader) Download(ctx context.Context, url string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create cache directory if needed
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	// Use URL hash + timestamp as cache key to force fresh download every time
	urlHash := fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
	extractDir := filepath.Join(d.cacheDir, "artifacts-"+timestamp[:16])

	// Always download fresh - no caching
	tzstPath := filepath.Join(d.cacheDir, urlHash+"-"+timestamp[:10]+".tzst")
	if err := d.downloadFile(ctx, url, tzstPath); err != nil {
		return "", fmt.Errorf("download artifacts: %w", err)
	}
	// Log the downloaded file size for debugging
	if info, err := os.Stat(tzstPath); err == nil {
		fmt.Printf("[DEBUG] Downloaded artifact size: %d bytes from %s\n", info.Size(), url)
	}
	// Clean up after extraction
	defer os.Remove(tzstPath)

	// Extract
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("create extract dir: %w", err)
	}

	if err := d.extractTzst(tzstPath, extractDir); err != nil {
		// Clean up on failure
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("extract artifacts: %w", err)
	}

	// Verify forge-artifacts exists
	forgeArtifactsDir := filepath.Join(extractDir, "forge-artifacts")
	if _, err := os.Stat(forgeArtifactsDir); err != nil {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("forge-artifacts directory not found after extraction")
	}

	// Debug: Check the actual bytecode in the extracted file
	validatorPath := filepath.Join(forgeArtifactsDir, "OPContractsManagerStandardValidator.sol", "OPContractsManagerStandardValidator.json")
	if data, err := os.ReadFile(validatorPath); err == nil {
		fmt.Printf("[DEBUG] Extracted OPContractsManagerStandardValidator.json size: %d bytes\n", len(data))
		// Check first 200 chars to verify it's a valid JSON
		if len(data) > 200 {
			fmt.Printf("[DEBUG] First 200 chars: %s\n", string(data[:200]))
		}
	} else {
		fmt.Printf("[DEBUG] Could not read validator file: %v\n", err)
		// List what's actually in forge-artifacts
		if entries, err := os.ReadDir(forgeArtifactsDir); err == nil {
			fmt.Printf("[DEBUG] Contents of forge-artifacts (first 20):\n")
			for i, e := range entries {
				if i >= 20 {
					break
				}
				fmt.Printf("[DEBUG]   - %s\n", e.Name())
			}
		}
	}

	return extractDir, nil
}

// downloadFile downloads a file from URL to the given path.
func (d *ContractArtifactDownloader) downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// extractTzst extracts a .tzst (zstd-compressed tar) file to the given directory.
func (d *ContractArtifactDownloader) extractTzst(tzstPath, destDir string) error {
	f, err := os.Open(tzstPath)
	if err != nil {
		return fmt.Errorf("open tzst file: %w", err)
	}
	defer f.Close()

	// Create zstd decoder
	zr, err := zstd.NewReader(f)
	if err != nil {
		return fmt.Errorf("create zstd reader: %w", err)
	}
	defer zr.Close()

	// Create tar reader
	tr := tar.NewReader(zr)

	// Extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		// Clean the path to prevent path traversal
		cleanName := filepath.Clean(header.Name)
		if cleanName == "." {
			continue
		}
		targetPath := filepath.Join(destDir, cleanName)

		// Ensure the target is within destDir (prevent path traversal attacks)
		absDestDir, _ := filepath.Abs(destDir)
		absTargetPath, _ := filepath.Abs(targetPath)
		if !strings.HasPrefix(absTargetPath, absDestDir+string(os.PathSeparator)) && absTargetPath != absDestDir {
			continue // Skip files outside destDir
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("create parent directory for %s: %w", targetPath, err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			// Handle symlinks
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("create parent directory for symlink %s: %w", targetPath, err)
			}
			os.Remove(targetPath) // Remove existing symlink if any
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("create symlink %s: %w", targetPath, err)
			}
		}
	}

	return nil
}

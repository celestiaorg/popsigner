// Package opstack provides OP Stack chain deployment infrastructure.
package opstack

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// ArtifactVersion is the current version of the contract artifacts.
// Update this when uploading new artifacts to S3.
// v27: Blueprint.sol fix - deployFrom() now skips parsing address(0) for single-blueprint contracts.
const ArtifactVersion = "v27"

// ArtifactBaseURL is the base URL for artifact storage.
const ArtifactBaseURL = "https://op-contracts.s3.nl-ams.scw.cloud"

// ContractArtifactURL is the URL to op-node v1.16.3-v26 artifacts hosted on Scaleway S3.
// These match the struct definitions in optimism v1.16.3 (29 fields in DeployImplementationsOutput).
// v26: Salt propagation fix - passes Create2Salt to DeployImplementations for ALL contracts.
const ContractArtifactURL = ArtifactBaseURL + "/artifacts-op-node-v1.16.3-" + ArtifactVersion + ".tzst"

// ArtifactChecksums contains SHA256 hashes for each artifact version.
// These checksums are verified after downloading to prevent tampering.
// To calculate a checksum: sha256sum artifacts-op-node-v1.16.3-vXX.tzst
// Update this map when uploading new artifacts to S3.
var ArtifactChecksums = map[string]string{
	// v27: Blueprint.sol fix
	// Calculated: curl -skL "https://op-contracts.s3.nl-ams.scw.cloud/artifacts-op-node-v1.16.3-v27.tzst" | sha256sum
	"v27": "363edcd70bc86f19c273b931fe906a68d8085a2649bcbabfe5c9050d282d6001",
}

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
	return d.DownloadWithVersion(ctx, url, ArtifactVersion)
}

// DownloadWithVersion downloads and extracts artifacts for a specific version.
// This allows specifying which version's checksum to use for verification.
func (d *ContractArtifactDownloader) DownloadWithVersion(ctx context.Context, url, version string) (string, error) {
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
		slog.Debug("downloaded artifact", slog.Int64("size_bytes", info.Size()), slog.String("url", url))
	}

	// Verify artifact integrity before extraction
	if err := d.verifyChecksum(tzstPath, version); err != nil {
		os.Remove(tzstPath)
		return "", fmt.Errorf("artifact integrity check failed: %w", err)
	}

	// Clean up after extraction
	defer os.Remove(tzstPath)

	// Extract to a temporary directory first to check structure
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("create extract dir: %w", err)
	}

	// Some archives have forge-artifacts/ at root, others have files directly at root
	// We need to handle both cases by extracting into forge-artifacts/ subdirectory
	forgeArtifactsDir := filepath.Join(extractDir, "forge-artifacts")
	if err := os.MkdirAll(forgeArtifactsDir, 0755); err != nil {
		return "", fmt.Errorf("create forge-artifacts dir: %w", err)
	}

	// Extract directly into forge-artifacts/ directory
	// This handles the case where the archive has files at root level (like v23)
	if err := d.extractTzst(tzstPath, forgeArtifactsDir); err != nil {
		// Clean up on failure
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("extract artifacts: %w", err)
	}

	// Check if we accidentally created a nested forge-artifacts/forge-artifacts structure
	// (happens when the archive already had forge-artifacts/ at root)
	nestedForgeDir := filepath.Join(forgeArtifactsDir, "forge-artifacts")
	if info, err := os.Stat(nestedForgeDir); err == nil && info.IsDir() {
		// Move contents from nested dir to parent
		slog.Debug("detected nested forge-artifacts, restructuring")
		entries, _ := os.ReadDir(nestedForgeDir)
		for _, e := range entries {
			src := filepath.Join(nestedForgeDir, e.Name())
			dst := filepath.Join(forgeArtifactsDir, e.Name())
			os.Rename(src, dst)
		}
		os.RemoveAll(nestedForgeDir)
	}

	// Verify extraction succeeded by checking for a known contract
	testPath := filepath.Join(forgeArtifactsDir, "OPContractsManager.sol")
	if _, err := os.Stat(testPath); err != nil {
		// List what we have for debugging
		entries, _ := os.ReadDir(forgeArtifactsDir)
		slog.Debug("forge-artifacts contents", slog.Int("showing_first", 10))
		for i, e := range entries {
			if i >= 10 {
				break
			}
			slog.Debug("artifact entry", slog.String("name", e.Name()))
		}
	}

	// Debug: Check the actual bytecode in the extracted file
	validatorPath := filepath.Join(forgeArtifactsDir, "OPContractsManagerStandardValidator.sol", "OPContractsManagerStandardValidator.json")
	if data, err := os.ReadFile(validatorPath); err == nil {
		slog.Debug("extracted validator contract", slog.Int("size_bytes", len(data)))
		if len(data) > 200 {
			slog.Debug("validator contract preview", slog.String("first_200", string(data[:min(200, len(data))])))
		}
	} else {
		slog.Debug("could not read validator file", slog.String("error", err.Error()))
	}

	return extractDir, nil
}

// verifyChecksum calculates SHA256 of the file and compares with the expected checksum.
// If the expected checksum is empty (not yet configured), verification is skipped with a warning.
func (d *ContractArtifactDownloader) verifyChecksum(filePath, version string) error {
	expectedHash, ok := ArtifactChecksums[version]
	if !ok {
		return fmt.Errorf("no checksum configured for artifact version %s", version)
	}

	// If checksum is empty, skip verification with a warning
	// This allows rolling out new versions before checksums are calculated
	if expectedHash == "" {
		slog.Warn("no checksum configured for artifact version, skipping integrity verification", slog.String("version", version))
		return nil
	}

	// Open the file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file for checksum: %w", err)
	}
	defer f.Close()

	// Calculate SHA256 hash
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}
	actualHash := fmt.Sprintf("%x", h.Sum(nil))

	// Compare checksums
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	slog.Debug("artifact checksum verified", slog.String("hash", actualHash))
	return nil
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

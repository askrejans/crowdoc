package engine

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// TypstVersion is the Typst release crowdoc is tested with and installs.
const TypstVersion = "0.15.1"

type release struct {
	asset  string
	sha256 string
	size   int64
}

// Official release assets of Typst 0.15.1 with their published digests.
var typstReleases = map[string]release{
	"darwin/arm64":  {"typst-aarch64-apple-darwin.tar.xz", "48f62ed034aa3a7978309579ac6ca00045e2ef0da73114e8af27cfd8e74dc05a", 14438168},
	"darwin/amd64":  {"typst-x86_64-apple-darwin.tar.xz", "7f9fdd9584866245de9a79e0add8f9236fae6f40a8a45e2c4771ccc14db4e0fa", 15635600},
	"linux/amd64":   {"typst-x86_64-unknown-linux-musl.tar.xz", "a6d077d0a95eed5a2eba715b2dae06be954f624ccbf85758a03f389ded33118c", 17462992},
	"linux/arm64":   {"typst-aarch64-unknown-linux-musl.tar.xz", "5aa8d74a3d906e60ea12a66ac2f37f8eef1b14cbad7182a745e393a10c23dcee", 16216812},
	"linux/arm":     {"typst-armv7-unknown-linux-musleabi.tar.xz", "44986312e557b9ac0f2c71d5d5156c0ad93b2da374d54c859d6c0c7c0b73709f", 16243656},
	"linux/riscv64": {"typst-riscv64gc-unknown-linux-gnu.tar.xz", "ec735f732c6a9940c4ef08223b50404396537ad34713eb883ef3bb310b396e5a", 17109888},
	"windows/amd64": {"typst-x86_64-pc-windows-msvc.zip", "19ce3551153c2fe7ee9fa2f95208310c8f4d3209fedb699e0333faf8913f6736", 22463684},
	"windows/arm64": {"typst-aarch64-pc-windows-msvc.zip", "4ab28e1b71ec3184d38d580ab797f499b6770d952b6b19167be5cea5c2662e14", 21246627},
}

// DefaultToolsDir is where crowdoc installs the engine
// (os.UserCacheDir()/crowdoc/typst-<version>).
func DefaultToolsDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "crowdoc", "typst-"+TypstVersion)
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "typst.exe"
	}
	return "typst"
}

// FindTypst locates a typst executable: $CROWDOC_TYPST, the crowdoc tools
// directory, then $PATH.
func FindTypst() (string, error) {
	if p := os.Getenv("CROWDOC_TYPST"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%w: CROWDOC_TYPST=%s does not exist", ErrNotFound, p)
	}
	if p := filepath.Join(DefaultToolsDir(), exeName()); fileOK(p) {
		return p, nil
	}
	if p, err := exec.LookPath("typst"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%w: install it with `crowdoc engine install` (about %d MB) or from https://github.com/typst/typst/releases", ErrNotFound, typstSize()/1_000_000)
}

func typstSize() int64 {
	if r, ok := typstReleases[runtime.GOOS+"/"+runtime.GOARCH]; ok {
		return r.size
	}
	return 20_000_000
}

func fileOK(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Size() > 0
}

// InstallTypst downloads the pinned Typst release for this platform into
// dir (DefaultToolsDir when empty), verifies its SHA-256 digest and returns
// the executable path. An existing installation is reused.
func InstallTypst(ctx context.Context, dir string, client *http.Client) (string, error) {
	if dir == "" {
		dir = DefaultToolsDir()
	}
	exe := filepath.Join(dir, exeName())
	if fileOK(exe) {
		return exe, nil
	}
	rel, ok := typstReleases[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("no Typst %s build for %s/%s; install typst manually", TypstVersion, runtime.GOOS, runtime.GOARCH)
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	url := "https://github.com/typst/typst/releases/download/v" + TypstVersion + "/" + rel.asset
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading Typst: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading Typst: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, rel.size+1))
	if err != nil {
		return "", fmt.Errorf("downloading Typst: %w", err)
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != rel.size || hex.EncodeToString(sum[:]) != rel.sha256 {
		return "", errors.New("downloaded Typst archive failed checksum verification")
	}
	bin, err := extractExe(rel.asset, data)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".typst-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return exe, nil
}

func extractExe(asset string, data []byte) ([]byte, error) {
	want := exeName()
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == want {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(io.LimitReader(rc, 256<<20))
			}
		}
		return nil, errors.New("typst executable not found in archive")
	}
	xr, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(xr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("typst executable not found in archive")
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == want {
			return io.ReadAll(io.LimitReader(tr, 256<<20))
		}
	}
}

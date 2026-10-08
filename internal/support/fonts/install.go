package fonts

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Progress reports bytes downloaded or verified for a family. Calls are
// serialised, so the callback need not be safe for concurrent use.
type Progress func(family string, done, total int64)

const (
	installWorkers = 4
	maxRedirects   = 5
)

// Install makes the families of the given sets plus the named families
// available in dir (DefaultDir when empty). With no sets and no families it
// installs the "core" set; the pseudo-set "all" selects everything.
//
// Files already present with the pinned size and SHA-256 are kept; missing
// or damaged ones are downloaded over HTTPS, verified and moved into place
// atomically, so an interrupted run never leaves a partial font behind and
// re-running is cheap. Font files of an older catalog version in a selected
// family's directory are removed. A failing file does not stop the others;
// all failures are returned together.
func Install(ctx context.Context, dir string, sets []string, families []string, progress Progress) error {
	sel, err := selectFamilies(sets, families)
	if err != nil {
		return err
	}
	if dir == "" {
		dir = DefaultDir()
	}
	return install(ctx, newHTTPClient(), dir, sel, progress)
}

// Installed reports which catalog families are completely present in dir
// (DefaultDir when empty). It checks file sizes only; Install verifies
// hashes.
func Installed(dir string) (present []string, missing []string) {
	if dir == "" {
		dir = DefaultDir()
	}
	for _, f := range catalog {
		if familyPresent(dir, f) {
			present = append(present, f.Name)
		} else {
			missing = append(missing, f.Name)
		}
	}
	return present, missing
}

func familyPresent(dir string, f Family) bool {
	for _, file := range f.Files {
		fi, err := os.Stat(filePath(dir, f, file))
		if err != nil || !fi.Mode().IsRegular() || fi.Size() != file.Size {
			return false
		}
	}
	return true
}

// filePath is where a catalog file lives inside the font directory: one
// subdirectory per family keeps the tree browsable.
func filePath(dir string, f Family, file File) string {
	return filepath.Join(dir, familyDir(f.Name), file.Name)
}

// familyDir turns a family name into a directory name ("EB Garamond" ->
// "eb-garamond").
func familyDir(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return b.String()
}

func selectFamilies(sets, names []string) ([]Family, error) {
	if len(sets) == 0 && len(names) == 0 {
		sets = []string{"core"}
	}
	want := map[string]bool{}
	for _, s := range sets {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "all" && !contains(setNames, s) {
			return nil, fmt.Errorf("fonts: unknown set %q (want one of %s, all)", s, strings.Join(setNames, ", "))
		}
		want[s] = true
	}
	picked := map[string]bool{}
	for _, n := range names {
		f, ok := lookupFamily(strings.TrimSpace(n))
		if !ok {
			return nil, fmt.Errorf("fonts: unknown family %q", n)
		}
		picked[f.Name] = true
	}
	var out []Family
	for _, f := range catalog {
		if want["all"] || want[f.Set] || picked[f.Name] {
			out = append(out, f)
		}
	}
	return out, nil
}

func newHTTPClient() *http.Client {
	t := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t = dt.Clone()
	}
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{
		Transport: t,
		// Generous enough for the largest file on a slow link, but a stalled
		// transfer still ends.
		Timeout:       15 * time.Minute,
		CheckRedirect: checkRedirect,
	}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("too many redirects")
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-https URL %s", req.URL.Redacted())
	}
	return nil
}

type installJob struct {
	family Family
	file   File
}

func install(ctx context.Context, client *http.Client, dir string, fams []Family, progress Progress) error {
	var (
		mu    sync.Mutex
		errs  []error
		done  = map[string]int64{}
		total = map[string]int64{}
		jobs  = make(chan installJob)
		wg    sync.WaitGroup
		zips  = &archives{client: client, dir: dir, m: map[string]*archive{}}
	)
	defer zips.cleanup()
	for _, f := range fams {
		total[f.Name] = f.Size()
	}
	report := func(family string, n int64) {
		mu.Lock()
		defer mu.Unlock()
		done[family] += n
		if progress != nil {
			progress(family, done[family], total[family])
		}
	}

	for range installWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				onBytes := func(n int64) { report(j.family.Name, n) }
				if err := installFile(ctx, client, zips, filePath(dir, j.family, j.file), j.file, onBytes); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("fonts: %s: %s: %w", j.family.Name, j.file.Name, err))
					mu.Unlock()
				}
			}
		}()
	}
feed:
	for _, f := range fams {
		pruneStale(dir, f)
		for _, file := range f.Files {
			select {
			case jobs <- installJob{f, file}:
			case <-ctx.Done():
				break feed
			}
		}
	}
	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.Join(errs...)
}

// pruneStale removes font files that an older catalog version put in the
// family's directory; left in place they would compete with the current
// files for the same family name.
func pruneStale(dir string, f Family) {
	famDir := filepath.Join(dir, familyDir(f.Name))
	entries, err := os.ReadDir(famDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !isFontFile(e.Name()) {
			continue
		}
		stale := true
		for _, file := range f.Files {
			if file.Name == e.Name() {
				stale = false
				break
			}
		}
		if stale {
			os.Remove(filepath.Join(famDir, e.Name()))
		}
	}
}

// installFile keeps a valid existing file or puts a fresh, verified copy in
// place, straight from its URL or out of its release archive.
func installFile(ctx context.Context, client *http.Client, zips *archives, target string, f File, onBytes func(int64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fileValid(target, f) {
		onBytes(f.Size)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if f.Member == "" {
		return writeAtomic(target, func(w io.Writer) error {
			return fetch(ctx, client, f.URL, f.Size, f.SHA256, io.MultiWriter(w, progressWriter(onBytes)))
		})
	}
	zipPath, err := zips.get(ctx, f)
	if err != nil {
		return err
	}
	return writeAtomic(target, func(w io.Writer) error {
		return extract(zipPath, f, io.MultiWriter(w, progressWriter(onBytes)))
	})
}

func fileValid(path string, f File) bool {
	fh, err := os.Open(path)
	if err != nil {
		return false
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() != f.Size {
		return false
	}
	return copyVerified(io.Discard, fh, f.Size, f.SHA256) == nil
}

// fetch streams url to w, enforcing the pinned size and SHA-256.
func fetch(ctx context.Context, client *http.Client, url string, size int64, sum string, w io.Writer) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("refusing non-https URL %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != size {
		return fmt.Errorf("server announced %d bytes, want %d", resp.ContentLength, size)
	}
	return copyVerified(w, resp.Body, size, sum)
}

// copyVerified copies exactly size bytes and checks their SHA-256. Reading
// one byte past size detects oversized input without buffering it.
func copyVerified(w io.Writer, r io.Reader, size int64, sum string) error {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(r, size+1))
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("got %d bytes, want %d", n, size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("SHA-256 mismatch: got %s, want %s", got, sum)
	}
	return nil
}

// writeAtomic fills a temporary file next to target and renames it into
// place only when fill succeeds, so target is either absent, the old file,
// or complete.
func writeAtomic(target string, fill func(io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*.part")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = fill(tmp); err != nil {
		return err
	}
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// extract copies f.Member out of the zip at zipPath. The archive was
// already verified against its pinned hash; the member is checked again.
func extract(zipPath string, f File, w io.Writer) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.Name != f.Member {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return copyVerified(w, rc, f.Size, f.SHA256)
	}
	return fmt.Errorf("archive lacks %s", f.Member)
}

// archives downloads each release archive at most once per Install run and
// removes the copies afterwards.
type archives struct {
	client *http.Client
	dir    string
	mu     sync.Mutex
	m      map[string]*archive
}

type archive struct {
	once sync.Once
	path string
	err  error
}

func (a *archives) get(ctx context.Context, f File) (string, error) {
	a.mu.Lock()
	e, ok := a.m[f.URL]
	if !ok {
		e = &archive{}
		a.m[f.URL] = e
	}
	a.mu.Unlock()
	e.once.Do(func() {
		tmp, err := os.CreateTemp(a.dir, ".archive-*.zip")
		if err != nil {
			e.err = err
			return
		}
		e.path = tmp.Name()
		e.err = fetch(ctx, a.client, f.URL, f.ArchiveSize, f.ArchiveSHA256, tmp)
		if err := tmp.Close(); e.err == nil {
			e.err = err
		}
	})
	return e.path, e.err
}

func (a *archives) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.m {
		if e.path != "" {
			os.Remove(e.path)
		}
	}
}

type progressWriter func(int64)

func (p progressWriter) Write(b []byte) (int, error) {
	p(int64(len(b)))
	return len(b), nil
}

package fonts

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeFonts struct {
	srv  *httptest.Server
	mu   sync.Mutex
	hits map[string]int
	body map[string][]byte
}

// newFakeFonts serves fake font files over TLS, plus misbehaving endpoints.
func newFakeFonts(t *testing.T) *fakeFonts {
	t.Helper()
	f := &fakeFonts{hits: map[string]int{}, body: map[string][]byte{
		"/a.ttf": bytes.Repeat([]byte("A font "), 1000),
		"/b.ttf": bytes.Repeat([]byte("B"), 3000),
	}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits[r.URL.Path]++
		body, ok := f.body[r.URL.Path]
		f.mu.Unlock()
		switch r.URL.Path {
		case "/to-http":
			http.Redirect(w, r, "http://127.0.0.1:1/a.ttf", http.StatusFound)
		case "/oversized":
			// Flushing first forces chunked encoding, so no Content-Length
			// warns the client: the size limit has to catch it.
			w.(http.Flusher).Flush()
			w.Write(f.body["/a.ttf"])
			w.Write([]byte("trailing garbage"))
		case "/short-header":
			w.Header().Set("Content-Length", "3")
			w.Write([]byte("abc"))
		case "/stall":
			w.Write(f.body["/a.ttf"][:100])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(body)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeFonts) client() *http.Client {
	c := f.srv.Client()
	c.CheckRedirect = checkRedirect
	return c
}

// file describes path on the server; content is what the hash and size
// promise (by default the served body).
func (f *fakeFonts) file(name, path string, content []byte) File {
	if content == nil {
		content = f.body[path]
	}
	sum := sha256.Sum256(content)
	return File{Name: name, URL: f.srv.URL + path, Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}
}

func (f *fakeFonts) hitCount(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

// regularFiles lists every file below dir, to prove no partial download
// was left behind.
func regularFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, strings.TrimPrefix(p, dir))
		}
		return nil
	})
	return out
}

func TestInstallDownloadsVerifiesAndIsIdempotent(t *testing.T) {
	srv := newFakeFonts(t)
	fam := Family{Name: "Test Sans", Files: []File{srv.file("TestSans-Regular.ttf", "/a.ttf", nil), srv.file("TestSans-Italic.ttf", "/b.ttf", nil)}}
	dir := t.TempDir()

	var last [2]int64
	progress := func(family string, done, total int64) {
		if family != "Test Sans" {
			t.Errorf("progress for %q", family)
		}
		last = [2]int64{done, total}
	}
	if err := install(context.Background(), srv.client(), dir, []Family{fam}, progress); err != nil {
		t.Fatal(err)
	}
	for _, f := range fam.Files {
		got, err := os.ReadFile(filePath(dir, fam, f))
		if err != nil {
			t.Fatal(err)
		}
		if want := srv.body[strings.TrimPrefix(f.URL, srv.srv.URL)]; !bytes.Equal(got, want) {
			t.Errorf("%s: wrong content", f.Name)
		}
	}
	if want := fam.Size(); last != [2]int64{want, want} {
		t.Errorf("final progress = %v, want done = total = %d", last, want)
	}
	if files := regularFiles(t, dir); len(files) != 2 {
		t.Errorf("unexpected files: %v", files)
	}

	if err := install(context.Background(), srv.client(), dir, []Family{fam}, nil); err != nil {
		t.Fatal(err)
	}
	if srv.hitCount("/a.ttf") != 1 || srv.hitCount("/b.ttf") != 1 {
		t.Errorf("valid files were downloaded again: %v", srv.hits)
	}
	if !familyPresent(dir, fam) {
		t.Error("family not reported as installed")
	}

	// Fonts from an older catalog version are pruned; other files stay.
	famDir := filepath.Dir(filePath(dir, fam, fam.Files[0]))
	os.WriteFile(filepath.Join(famDir, "TestSans-Old.ttf"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(famDir, "notes.txt"), []byte("keep"), 0o644)
	if err := install(context.Background(), srv.client(), dir, []Family{fam}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(famDir, "TestSans-Old.ttf")); err == nil {
		t.Error("stale font was not pruned")
	}
	if _, err := os.Stat(filepath.Join(famDir, "notes.txt")); err != nil {
		t.Error("non-font file was removed")
	}

	// A damaged file of the right size is caught by the hash and replaced.
	target := filePath(dir, fam, fam.Files[0])
	if err := os.WriteFile(target, bytes.Repeat([]byte("x"), int(fam.Files[0].Size)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := install(context.Background(), srv.client(), dir, []Family{fam}, nil); err != nil {
		t.Fatal(err)
	}
	if srv.hitCount("/a.ttf") != 2 || !fileValid(target, fam.Files[0]) {
		t.Error("damaged file was not replaced")
	}
}

func TestInstallRejectsBadDownloads(t *testing.T) {
	srv := newFakeFonts(t)
	a := srv.body["/a.ttf"]
	corrupt := append([]byte{}, a...)
	corrupt[10] ^= 0xff
	tests := []struct {
		name string
		file File
		want string
	}{
		{"hash mismatch", srv.file("x.ttf", "/a.ttf", corrupt), "SHA-256 mismatch"},
		{"body larger than pinned size", srv.file("x.ttf", "/oversized", a), "bytes, want"},
		{"announced length differs", srv.file("x.ttf", "/short-header", a), "announced"},
		{"redirect to plain http", srv.file("x.ttf", "/to-http", a), "non-https"},
		{"not found", srv.file("x.ttf", "/missing", a), "404"},
		{"plain http URL", File{Name: "x.ttf", URL: "http://127.0.0.1:1/a.ttf", Size: 1, SHA256: strings.Repeat("0", 64)}, "non-https"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fam := Family{Name: "Bad", Files: []File{tt.file}}
			err := install(context.Background(), srv.client(), dir, []Family{fam}, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.want)
			}
			if files := regularFiles(t, dir); len(files) != 0 {
				t.Errorf("left files behind: %v", files)
			}
		})
	}
}

func TestInstallExtractsFromArchiveOnce(t *testing.T) {
	srv := newFakeFonts(t)
	members := map[string][]byte{
		"pkg-1.0/OTF/Pkg-Regular.otf": bytes.Repeat([]byte("regular "), 500),
		"pkg-1.0/OTF/Pkg-Italic.otf":  bytes.Repeat([]byte("italic "), 400),
		"pkg-1.0/README":              []byte("not a font"),
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	srv.body["/pkg.zip"] = buf.Bytes()
	archive := srv.file("", "/pkg.zip", nil)
	member := func(path string, content []byte) File {
		f := srv.file(filepath.Base(path), "/pkg.zip", content)
		f.Member, f.ArchiveSHA256, f.ArchiveSize = path, archive.SHA256, archive.Size
		return f
	}
	regular := member("pkg-1.0/OTF/Pkg-Regular.otf", members["pkg-1.0/OTF/Pkg-Regular.otf"])
	italic := member("pkg-1.0/OTF/Pkg-Italic.otf", members["pkg-1.0/OTF/Pkg-Italic.otf"])
	sans := Family{Name: "Pkg Sans", Files: []File{regular, italic}}
	other := Family{Name: "Pkg Other", Files: []File{member("pkg-1.0/OTF/Pkg-Regular.otf", members["pkg-1.0/OTF/Pkg-Regular.otf"])}}

	dir := t.TempDir()
	if err := install(context.Background(), srv.client(), dir, []Family{sans, other}, nil); err != nil {
		t.Fatal(err)
	}
	if !familyPresent(dir, sans) || !familyPresent(dir, other) || !fileValid(filePath(dir, sans, italic), italic) {
		t.Fatal("archive members not installed")
	}
	if n := srv.hitCount("/pkg.zip"); n != 1 {
		t.Errorf("archive downloaded %d times, want once", n)
	}
	if files := regularFiles(t, dir); len(files) != 3 {
		t.Errorf("want exactly the three fonts, got %v", files)
	}

	for _, bad := range []File{
		member("pkg-1.0/OTF/Pkg-Regular.otf", []byte("different content")),
		member("pkg-1.0/OTF/Missing.otf", []byte("x")),
		func() File { f := regular; f.ArchiveSHA256 = strings.Repeat("0", 64); return f }(),
	} {
		dir := t.TempDir()
		err := install(context.Background(), srv.client(), dir, []Family{{Name: "Bad", Files: []File{bad}}}, nil)
		if err == nil {
			t.Errorf("%s: bad archive member accepted", bad.Member)
		}
		if files := regularFiles(t, dir); len(files) != 0 {
			t.Errorf("%s: left files behind: %v", bad.Member, files)
		}
	}
}

func TestInstallFailureDoesNotStopOtherFiles(t *testing.T) {
	srv := newFakeFonts(t)
	good := Family{Name: "Good", Files: []File{srv.file("good.ttf", "/b.ttf", nil)}}
	bad := Family{Name: "Bad", Files: []File{srv.file("bad.ttf", "/missing", []byte("x"))}}
	dir := t.TempDir()
	err := install(context.Background(), srv.client(), dir, []Family{bad, good}, nil)
	if err == nil || !strings.Contains(err.Error(), "Bad: bad.ttf") {
		t.Fatalf("err = %v", err)
	}
	if !familyPresent(dir, good) {
		t.Error("good family was not installed")
	}
}

func TestInstallHonoursCancellation(t *testing.T) {
	srv := newFakeFonts(t)
	fam := Family{Name: "Slow", Files: []File{srv.file("slow.ttf", "/stall", srv.body["/a.ttf"])}}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := install(ctx, srv.client(), dir, []Family{fam}, func(_ string, done, _ int64) {
		if done > 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Errorf("left files behind: %v", files)
	}
}

func TestSelectFamilies(t *testing.T) {
	count := func(set string) int {
		n := 0
		for _, f := range catalog {
			if set == "" || f.Set == set {
				n++
			}
		}
		return n
	}
	tests := []struct {
		sets, names []string
		want        int
		err         string
	}{
		{nil, nil, count("core"), ""},
		{[]string{"all"}, nil, count(""), ""},
		{[]string{" CJK "}, nil, count("cjk"), ""},
		{[]string{"emoji", "cjk"}, nil, count("emoji") + count("cjk"), ""},
		{nil, []string{"inter", "EB GARAMOND", "Noto Sans JP"}, 3, ""},
		{[]string{"core"}, []string{"Inter"}, count("core"), ""},
		{[]string{"bogus"}, nil, 0, "unknown set"},
		{nil, []string{"Comic Sans"}, 0, "unknown family"},
	}
	for _, tt := range tests {
		got, err := selectFamilies(tt.sets, tt.names)
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("selectFamilies(%v, %v) err = %v, want %q", tt.sets, tt.names, err, tt.err)
			}
			continue
		}
		if err != nil || len(got) != tt.want {
			t.Errorf("selectFamilies(%v, %v) = %d families, %v; want %d", tt.sets, tt.names, len(got), err, tt.want)
		}
	}
	if err := Install(context.Background(), t.TempDir(), []string{"bogus"}, nil, nil); err == nil {
		t.Error("Install accepted an unknown set")
	}
}

func TestInstalled(t *testing.T) {
	dir := t.TempDir()
	fam, _ := lookupFamily("Fira Math")
	for _, f := range fam.Files {
		p := filePath(dir, fam, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, f.Size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Wrong size: not installed.
	inter, _ := lookupFamily("Inter")
	p := filePath(dir, inter, inter.Files[0])
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("short"), 0o644)

	present, missing := Installed(dir)
	if len(present) != 1 || present[0] != "Fira Math" {
		t.Errorf("present = %v", present)
	}
	if len(missing) != len(catalog)-1 || !contains(missing, "Inter") {
		t.Errorf("missing has %d entries", len(missing))
	}
}

func TestDefaultAndSearchDirs(t *testing.T) {
	cache := t.TempDir()
	t.Setenv(EnvDir, cache)
	if got := DefaultDir(); got != cache {
		t.Errorf("DefaultDir = %q, want %q", got, cache)
	}
	extra := t.TempDir()
	dirs := SearchDirs(extra, filepath.Join(extra, "missing"), extra+string(filepath.Separator), "")
	if len(dirs) < 2 || dirs[0] != extra || dirs[1] != cache {
		t.Fatalf("SearchDirs = %v, want %s then %s first", dirs, extra, cache)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		if seen[d] {
			t.Errorf("duplicate %s in %v", d, dirs)
		}
		seen[d] = true
	}

	t.Setenv(EnvDir, "")
	if got := DefaultDir(); !strings.HasSuffix(got, filepath.Join("crowdoc", "fonts")) {
		t.Errorf("DefaultDir = %q", got)
	}
}

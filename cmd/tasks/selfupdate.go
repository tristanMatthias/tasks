package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tristanMatthias/tasks/pkg/buildinfo"
)

// updateRepo is the GitHub repo self-update pulls releases from.
const updateRepo = "tristanMatthias/tasks"

// ghRelease is the subset of the GitHub "latest release" payload we use.
type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// updater replaces the running `tasks` binary with the latest release asset for
// this OS/arch, verified against the release checksums. Fields are injectable so
// the whole flow is testable against a local HTTP server + temp file.
type updater struct {
	repo    string
	apiBase string // https://api.github.com (overridable in tests)
	current string // current version (buildinfo.Version)
	goos    string
	goarch  string
	exePath string // the binary to replace (os.Executable in real use)
	client  *http.Client
}

func newUpdater() (*updater, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return &updater{
		repo:    updateRepo,
		apiBase: "https://api.github.com",
		current: buildinfo.Version,
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
		exePath: exe,
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// runSelfUpdate is the CLI entry point for `tasks self-update`.
func runSelfUpdate(args []string) error {
	force, done, err := parseUpdateArgs(args, os.Stdout)
	if err != nil || done {
		return err
	}
	u, err := newUpdater()
	if err != nil {
		return err
	}
	return u.run(force, os.Stdout)
}

// parseUpdateArgs parses self-update flags. done=true means the command already
// handled itself (e.g. --help) and the caller should stop.
func parseUpdateArgs(args []string, out io.Writer) (force, done bool, err error) {
	for _, a := range args {
		switch a {
		case "-f", "--force":
			force = true
		case "-h", "--help":
			fmt.Fprint(out, "Usage: tasks self-update [--force]\n\nReplace this binary with the latest release for your OS/arch.\n")
			return false, true, nil
		default:
			return false, false, fmt.Errorf("unknown flag: %s", a)
		}
	}
	return force, false, nil
}

func (u *updater) assetName() string {
	return fmt.Sprintf("tasks_%s_%s", u.goos, u.goarch)
}

func (u *updater) run(force bool, out io.Writer) error {
	if u.goos == "windows" {
		return fmt.Errorf("self-update isn't supported on Windows yet — download from https://github.com/%s/releases", u.repo)
	}
	rel, err := u.latest()
	if err != nil {
		return err
	}
	if rel.TagName == "" {
		return fmt.Errorf("no published release found for %s", u.repo)
	}
	// "dev" builds (go build / source) always update; released builds only when
	// the remote tag is strictly newer.
	if !force && u.current != "dev" && cmpSemver(rel.TagName, u.current) <= 0 {
		fmt.Fprintf(out, "already on the latest version (%s)\n", u.current)
		return nil
	}

	binURL := findAsset(rel, u.assetName())
	if binURL == "" {
		return fmt.Errorf("release %s has no asset %q for your platform", rel.TagName, u.assetName())
	}
	bin, err := u.download(binURL)
	if err != nil {
		return err
	}
	// Verify against checksums.txt when present (it always is in our releases).
	if sumURL := findAsset(rel, "checksums.txt"); sumURL != "" {
		sums, err := u.download(sumURL)
		if err != nil {
			return err
		}
		want := checksumFor(string(sums), u.assetName())
		if want == "" {
			return fmt.Errorf("no checksum listed for %s", u.assetName())
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(bin)); got != want {
			return fmt.Errorf("checksum mismatch for %s (download corrupted?)", u.assetName())
		}
	}

	if err := replaceExecutable(u.exePath, bin); err != nil {
		return err
	}
	fmt.Fprintf(out, "updated %s → %s\n%s\n", orDev(u.current), rel.TagName, rel.HTMLURL)
	return nil
}

func (u *updater) latest() (*ghRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", u.apiBase, u.repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases returned HTTP %d", resp.StatusCode)
	}
	var r ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (u *updater) download(url string) ([]byte, error) {
	resp, err := u.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func findAsset(r *ghRelease, name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// checksumFor extracts the hex digest for name from a `<sha>  <name>` file.
func checksumFor(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			return fields[0]
		}
	}
	return ""
}

// replaceExecutable atomically swaps the binary at path with data: it writes a
// temp file in the same directory (so the rename is atomic, not cross-device),
// makes it executable, then renames it over the target. On Unix this works even
// while the process is running.
func replaceExecutable(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tasks-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (need write access — try sudo or reinstall): %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("cannot replace %s: %w", path, err)
	}
	return nil
}

// cmpSemver compares two vX.Y.Z strings, ignoring any pre-release/build suffix.
// Returns >0 if a is newer than b, 0 if equal, <0 if older.
func cmpSemver(a, b string) int {
	pa, pb := parseSemver(a), parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

func orDev(s string) string {
	if s == "" {
		return "dev"
	}
	return s
}

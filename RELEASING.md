# Releasing

Releases are cut from **semver git tags** and built by [GoReleaser](https://goreleaser.com)
in CI (`.github/workflows/release.yml`). There is nothing to build by hand.

## Versioning

- Tags are `vMAJOR.MINOR.PATCH` (e.g. `v0.3.1`), following [SemVer](https://semver.org):
  - **MAJOR** — breaking CLI/API changes
  - **MINOR** — new, backwards-compatible features
  - **PATCH** — backwards-compatible fixes
- Pre-releases use a suffix: `v0.4.0-rc.1` → published as a GitHub *pre-release*
  (GoReleaser `prerelease: auto`), so `tasks self-update` and `@latest` skip them.
- The version is baked into the binary via `-ldflags` into `pkg/buildinfo`; check
  it with `tasks version` (or `tasksd version`).

## Cutting a release

```bash
git checkout main && git pull
git tag v0.3.0          # annotate if you like: git tag -a v0.3.0 -m "…"
git push origin v0.3.0
```

That's it. The tag push triggers the `Release` workflow, which:

1. builds static binaries for **linux/darwin × amd64/arm64** (`tasks` + `tasksd`),
2. publishes both `.tar.gz` archives and raw, version-less binaries
   (`tasks_<os>_<arch>` — the stable URLs used by curl-install and self-update),
3. writes `checksums.txt` (SHA-256) and an **SBOM** per archive,
4. **cosign**-signs the checksums (keyless, via GitHub OIDC — no secrets),
5. generates release notes from Conventional Commits and creates the GitHub Release,
6. pushes a multi-arch `tasksd` image to `ghcr.io/tristanmatthias/tasks`.

Preview locally without publishing:

```bash
goreleaser check                       # validate .goreleaser.yaml
goreleaser release --snapshot --clean  # full dry run into ./dist
```

## Installing / updating

```bash
# static binary, no Go required
curl -fsSL https://github.com/tristanMatthias/tasks/releases/latest/download/tasks_linux_amd64 \
  -o /usr/local/bin/tasks && chmod +x /usr/local/bin/tasks

# or with Go
go install github.com/tristanMatthias/tasks/cmd/tasks@latest

# already installed
tasks self-update            # --force to reinstall the same version
```

## Verifying a download

```bash
# checksum
curl -fsSLO https://github.com/tristanMatthias/tasks/releases/download/vX.Y.Z/checksums.txt
sha256sum -c checksums.txt --ignore-missing

# signature (keyless)
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature   checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/tristanMatthias/tasks/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

## Notes

- **Commit messages** drive the changelog — use Conventional Commit prefixes
  (`feat:`, `fix:`, `perf:`, …); `chore/test/ci/docs/style` are excluded.
- macOS binaries are **not** notarized; on first run Gatekeeper may require
  `xattr -d com.apple.quarantine ./tasks` (or install via `go install`).
- Future niceties (not yet wired): a Homebrew tap, Linux packages (deb/rpm),
  and Windows builds — all straightforward GoReleaser additions.

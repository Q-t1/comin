# OCI source: next steps

Follow-up work after the OCI source support (merged in Q-t1/comin#1,
not yet proposed upstream). Items are ordered by priority.

## 1. Propose the work upstream

- [ ] Open a PR from `Q-t1/comin:feature/oci-source` to `nlewo/comin:main`.
  Upstream `main` is still `bbef55c`, so the branch applies cleanly.
  Include the `oci: fix the formatting of repository.go` commit from
  this branch.
- [ ] Discuss with the maintainer whether the sigstore-go dependency
  is acceptable (see item 6) before going further on cosign.

## 2. Fix the build retry / generation GC race (pre-existing upstream bug)

`TestBuilderRetryCancelledOnNewEval` hangs under load. Under the same
stress, upstream `main` failed 5/12 runs and our branch 1/6:

```shell
nix develop --command go test -race -c -o /tmp/builder.test ./internal/builder/
for i in $(seq 1 6); do (/tmp/builder.test -test.count=5 -test.timeout=90s > /tmp/run-$i.log 2>&1; echo "run $i: $?") & done; wait
```

Root causes, in the build retry code from nlewo/comin#203:

- [ ] **Generations are garbage collected before being evaluated.** A
  retry build calls `GenerationBuildStart`, whose `generationsGC` keeps
  only the generations referenced by `lastEvalStarted`,
  `lastEvalFinished`, `lastBuildStarted` and `lastBuildFinished`. A
  generation created by `NewGeneration` but not yet evaluated is
  removed, so the following `Builder.Eval` fails with "no generation
  with uuid". In production, a new commit can be silently skipped if a
  retry starts at that moment. Fix idea: also keep the generations
  created after the last started evaluation (or track a "pending"
  generation in the store).
- [ ] **Data race on `Builder.buildator`.** `stopBuild()` calls
  `b.buildator.Stop()` without `b.mu` (`internal/builder/builder.go`,
  `stopBuild`) while the retry timer goroutine replaces `b.buildator`
  under `b.mu` (`build()`). The build goroutine also reads the
  `b.buildator` field instead of its own `Exec`, so a newer build can
  replace it under its feet. Fix idea: capture the `Exec` in a local
  variable in `build()` and `Eval()`, and read the field under `b.mu`
  in `stopEval`/`stopBuild`.
- [ ] **A retry can start a build while `Stop()` is running.**
  `Stop()` calls `stopBuild()` before `cancelBuildRetry()`, so the
  retry timer can fire in between, call `buildatorWg.Add(1)`
  concurrently with `buildatorWg.Wait()`, and start a build after the
  stop. Fix idea: cancel the retry first, and refuse to start retried
  builds while stopping.

## 3. Restore `self.rev` for flakes evaluated from an artifact

Artifacts are evaluated with a `path:` flake URL, which has no `rev`.
`path:` flake URLs accept `rev` and `lastModified` attributes, which are
then exposed as `self.rev` and `self.lastModified`:

```shell
nix eval "path:/dir?rev=<40 hex commit id>&lastModified=1700000000#info"
# -> self.sourceInfo.rev == "<40 hex commit id>"
```

Verified with Determinate Nix 2.35.

- [ ] Check that this works with the Nix versions supported by comin.
- [ ] In `GitNixFlake.flakeUrl`, append `rev=` (the commit ID parsed
  from the `org.opencontainers.image.revision` annotation, reusing
  `parseRevision` from `internal/utils/postcmd.go`) and `lastModified=`
  (from the `created` annotation) when they are available.
- [ ] Update the howto, which currently says `self.rev` is not
  available.

## 4. Test a full deployment in a NixOS VM

Only the module evaluation, the fetch/verify/extract path (with real
`flux`, `oras` and `cosign` against a `distribution` registry) and the
unit tests have been tested so far.

- [ ] Add a `nixosTest` running a `distribution` registry, pushing an
  artifact containing a NixOS configuration, and checking comin switches
  to it.
- [ ] Extend it with a cosign signed artifact and an unsigned one.
- [ ] Possibly wire it into `flake.nix` checks (it is slow, so maybe
  behind a separate attribute).

## 5. Hardening

- [ ] Limit the extracted artifact size (and the number of entries) to
  protect against decompression bombs. There is currently no limit in
  `internal/oci/extract.go`.
- [ ] Make multi-line errors (built with `errors.Join` in
  `internal/oci/cosign.go`) readable in the TUI and `comin status`,
  for instance by only displaying the first line and logging the rest.

## 6. Cosign

- [ ] Keyless verification (Fulcio certificates + Rekor), the common
  case for artifacts signed from GitHub Actions. sigstore-go supports it
  with a TUF trusted root (to cache in the state directory) and
  `verify.WithCertificateIdentity`. Configuration similar to the Flux
  `matchOIDCIdentity` (issuer and subject regexps).
- [ ] Optional transparency log verification for key-based signatures
  (it currently behaves like `--insecure-ignore-tlog`).
- [ ] Signatures stored in another repository (`COSIGN_REPOSITORY`).
- [ ] If the binary size is a concern (sigstore-go adds about 7.6 MB),
  evaluate a minimal key-based bundle verification without sigstore-go,
  or a build tag.

## 7. Registry authentication

- [ ] Cloud provider authentication like the Flux `provider` field
  (`aws`, `gcp`, `azure`). Docker credential helpers already work with
  `auth.docker_config_path`, but their binaries have to be added to the
  comin service `PATH`: an option such as
  `services.comin.ociCredentialHelpers = [ pkgs.docker-credential-gcr ]`
  would make this easy.

## 8. Housekeeping (pre-existing, not OCI specific)

- [ ] `internal/store/store_test.go` writes `state.json` in the package
  directory (relative path in a test).
- [ ] `.github/workflows/go.yml` uses Go 1.22 while `go.mod` requires
  Go 1.25 (it relies on the toolchain auto-download).
- [ ] Some files are not `gofmt` clean on `main` (`cmd/status.go`,
  `cmd/desktop.go`, `internal/builder/builder.go`, `internal/utils/utils.go`,
  ...): `gofmt -l .` lists them.
- [ ] `nix/go-test.nix` is not part of the flake checks, so the Nix
  sandbox test run is never executed by the CI.

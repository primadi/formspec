# 2026-09-10-006 — Script release satu perintah `git-push-and-tag.sh`

## Apa

Script `scripts/git-push-and-tag.sh` yang menggabungkan langkah 2–4 prosedur
release (`docs/guides/releasing.md`): tag semver → `git push origin main
--tags` → `make release` → `make release-upload`. Output akhir tetap draft
release yang harus di-review lalu Publish manual.

Guard fail-cepat sebelum aksi apa pun: semver, working tree bersih, tag belum
dipakai (lokal & remote via `git ls-remote`), `gh` ter-auth, branch `main`,
plus `go test ./...` (bisa dilewati via `--skip-tests`).

## Kenapa

Release manual multi-langkah rawan terlewat langkahnya (push tag sering
terlupakan karena `make release-upload` hanya butuh tag lokal). Script membuat
urutan yang benar menjadi jalur default; prosedur manual tetap jadi referensi
untuk kondisi khusus.

## File terdampak

- `scripts/git-push-and-tag.sh` — script baru (executable).
- `docs/guides/releasing.md` — section baru "Cara cepat — script satu perintah".
- `scripts/README.md` — dokumentasi script baru.

## Referensi

- `docs/guides/releasing.md` — prosedur manual yang di-automasi.
- `docs_internal/changelog/2026-09-10-005-fix-guard-release-upload.md` — guard `release-upload` yang dipakai script.

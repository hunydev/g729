# g729 GitHub Pages

This directory is the source for <https://g729.huny.dev/>.

GitHub Pages should be configured to publish from the `main` branch and the
`/docs` path. `CNAME` pins the custom domain to `g729.huny.dev`.

## Audio sample slots

The landing page enables four listening samples when these files are present:

- `docs/assets/audio/ffmpeg-encode-g729-decode.wav`
- `docs/assets/audio/ffmpeg-encode-ffmpeg-decode.wav`
- `docs/assets/audio/g729-encode-g729-decode.wav`
- `docs/assets/audio/g729-encode-ffmpeg-decode.wav`

No audio files are tracked in this repository yet. Before publishing samples,
confirm rights to the source speech, document the generation path, and update
`THIRD_PARTY_NOTICES.md` / `IP_PROVENANCE.md` if redistributed media becomes
part of the public release.

Keep generated sample files small enough for GitHub Pages. If the samples become
large, publish them through a reviewed static asset host and update
`docs/index.html` to point at those URLs.

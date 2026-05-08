# External local samples

This directory is for local, non-redistributed audio and raw G.729 payload
samples used during black-box quality checks.

Files such as `asterisk_payload.g729` and `user_quality_input.*` are ignored by
git because they may contain user, customer, or otherwise private speech.

To run the optional Asterisk local decoder gate, place a raw speech-only
`annexb=no` G.729 payload at:

```sh
testdata/external/asterisk_payload.g729
```

Then run:

```sh
G729_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
G729_REQUIRE_DECODER_ASTERISK_FFMPEG_QUALITY=1 \
go test ./internal/decoder -run TestPhase3rAsteriskFFmpegQualityGate -count=1 -v
```

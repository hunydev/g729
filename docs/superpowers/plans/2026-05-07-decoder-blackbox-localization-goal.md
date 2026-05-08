# Decoder Black-Box Quality Localization Goal

Use this as the next long-running `/goal` prompt.

```text
Goal: 외부 독립 decoder-stage numeric verifier가 불가능하므로, G.729 decoder 품질 문제를 SPEECH.PST 등 허용된 최종 출력 벡터와 Asterisk-origin raw G.729 payload를 기준으로 black-box 방식으로 국소화하고, clean-room boundary 안에서 실제 개선 가능한 defect를 찾아 구현한다.

Context:
- 현재 /home/exedev/g729 저장소의 decoder_stage_expected.csv는 신뢰 가능한 oracle이 아니다. decoder_stage_got.csv에서 변환된 self-oracle이므로 decoder conformance evidence로 사용하지 않는다.
- strict handoff 검증에는 G729_REJECT_DECODER_STAGE_SELF_ORACLE=1 guard를 유지한다.
- 외부 ITU reference C, bcg729, FFmpeg, Sipro Lab, 기타 G.729 구현 코드는 절대 보지 않는다.
- 허용 근거는 G.729 spec 문서, 공개 test vector의 bitstream/PCM 입출력 숫자, repo 내부 코드, 우리가 직접 산출한 black-box metrics뿐이다.

Primary objective:
1. SPEECH.BIT -> repo Decoder -> decoded PCM을 SPEECH.PST와 비교해 품질 결함을 stage별로 좁힌다.
2. Asterisk raw G.729 payload -> repo Decoder -> decoded PCM의 audibility/level/continuity metrics도 함께 추적한다.
3. 단순 loudness 보정이 아니라 waveform shape, time alignment, stage contribution, filter/postfilter/high-pass/synthesis/excitation/gain/fixed-codebook/adaptive-codebook 중 어디에서 품질 손실이 가장 커지는지 찾는다.
4. 작은 근거 있는 fix가 발견되면 실제 코드로 구현하고 테스트/문서/verification log까지 갱신한다.

Required implementation work:
- 현재 invalid self-oracle expected를 conformance evidence로 쓰지 않도록 문서와 테스트 guard를 유지/보강한다.
- internal/decoder 아래에 black-box localization diagnostic test를 추가한다. 기본 go test에서는 skip 또는 non-failing diagnostic으로 두고, env flag로 상세 측정을 출력한다.
- diagnostic은 최소한 다음 metrics를 출력해야 한다:
  - final decoded PCM vs SPEECH.PST: RMS, peak, DC offset, global SNR, segmental SNR, normalized correlation
  - small lag sweep alignment: lag -40..+40 samples에서 best correlation/SNR
  - frame/subframe window별 worst segments
  - production final output, pre-scale HP output, postfilter output, synthesis output, excitation-derived proxy output의 상대 metrics
  - postfilter bypass / HP bypass / scale-only / simple alignment-only 변형이 품질을 개선하는지 여부
  - Asterisk payload decode RMS/peak/clipping/silence ratio/frame continuity
- 기존 Phase3c scale/variant probes와 중복되는 경우 통합하거나 더 높은 signal의 새 test로 정리한다.
- 결과는 docs/releases/v0.1.0-rc1-verification-log.md 또는 별도 docs/superpowers/plans/* report에 기록한다.

Decision rules:
- 단순 gain 조정은 RMS만 개선하고 SNR/상관도를 악화시키면 채택하지 않는다.
- best lag alignment가 큰 폭으로 개선되면 frame/sample alignment defect를 1순위로 조사한다.
- synthesis output은 좋고 postfilter/HP 이후 나쁘면 postfilter/HP를 1순위로 조사한다.
- excitation/proxy 단계부터 나쁘면 pitch/gain/fixed-codebook/adaptive-codebook 쪽을 1순위로 조사한다.
- Asterisk payload에서도 같은 결함 양상이 재현되면 decoder-only defect로 우선순위를 올린다.

Validation gates:
- gofmt on touched Go files.
- go test ./internal/decoder -count=1
- go test ./cmd/g729rtpweb -count=1
- go test ./... -count=1 unless runtime becomes excessive; if skipped, state why.
- If a code fix is made, rerun the diagnostic before/after and record numeric delta.

Deliverables:
- code changes for diagnostic and any justified decoder fix
- updated verification/report documentation
- exact commands run and concise result summary
- clear next target if no safe fix is found in this goal
```

One-line command:

```text
/goal 외부 독립 decoder-stage numeric verifier가 불가능하므로, /home/exedev/g729에서 decoder_stage_expected.csv self-oracle은 conformance evidence로 사용하지 않고 G729_REJECT_DECODER_STAGE_SELF_ORACLE=1 guard를 유지한다. SPEECH.BIT->Decoder->PCM을 SPEECH.PST와 black-box 비교해 RMS/peak/DC/global SNR/segmental SNR/correlation, -40..+40 sample lag sweep, worst frame/subframe segments, production/pre-scale HP/postfilter/synthesis/excitation-proxy stage metrics, postfilter bypass/HP bypass/scale-only/alignment-only 변형을 측정하고 Asterisk raw G.729 payload decode metrics도 함께 추적한다. clean-room boundary를 유지하며 외부 G.729 구현 코드는 보지 않는다. 품질 손실 위치를 pitch/gain/fixed/adaptive/excitation/synthesis/postfilter/HP/alignment 중 하나 이상으로 좁히고, 근거 있는 작은 fix가 발견되면 실제 구현한다. touched Go files gofmt, go test ./internal/decoder -count=1, go test ./cmd/g729rtpweb -count=1, 가능하면 go test ./... -count=1까지 실행하고 verification log/report를 갱신해 커밋 가능한 상태로 만든다.
```

# Encoder Profiles

This page documents encoder profile choices for `github.com/hunydev/g729`.
Profiles always emit normal 10-byte G.729 frames. None of the profiles is an
ITU encoder byte-exact or certification claim.

## Product Default

`NewEncoder()` and `NewStreamingEncoder()` use `EncoderProfileCore`. This is
the product default for v0.1.0-rc1.

Core is selected by listening quality, not PESQ alone. A diagnostic PESQ-led
profile can score closer to the local `bcg729` black-box anchor on some private
samples, but recent blind tests preferred Core because the PESQ-led candidate
sounded slightly more muffled.

Core keeps the public frame shape and decoder compatibility:

- 80 int16 samples at 8 kHz in, 10 packed bytes out.
- RTP payload type 18 compatible payload shape.
- `annexb=no`; no SID/CNG/DTX.
- Zero-allocation steady-state frame encode path.

## Core Fast

`EncoderProfileCoreFast` is an explicit opt-in for low-latency or
high-density deployments. It keeps the normal 10-byte G.729 payload shape and
the same public decoder compatibility as Core, but reduces selected encoder
search precision/budget:

- The focused fixed-codebook fourth-loop budget is reduced from the Core
  frame-level 180-entry cap to 60 entries per frame.
- Subframe 0 is capped at 30 entries so it cannot consume the whole fast frame
  budget.
- Gain preselect uses the standard-width center solve instead of Core's wider
  preselect-center precision.
- The wider gain predictor arithmetic is still retained; disabling it was
  tested and caused a large Pages sample quality regression.

Core Fast is not the product default, not a decoder exact path, and not an ITU
encoder byte-exact claim. Treat it as a throughput/latency trade-off profile:
run local listening and regression checks before using it for production
streams that are quality-sensitive.

## Core Algorithm Notes

The Core open-loop path follows Annex A's raw-correlation per-range maxima
before the normalized three-range merge, and range-3 override checks every
lower-range submultiple with the Core `11/10` lift instead of only the current
pairwise winner.

Core closed-loop pitch refinement also evaluates the encodable P1/P2
fractional boundary codepoints rather than silently restricting the search to
only the three fractions around the integer winner.

Its fixed-codebook path uses the K3=0.4 focused threshold search from §3.8.1
with a 180-entry frame cap across the two subframes; subframe 0 is
conservatively capped at 90 so it cannot consume the whole frame budget. Its
LSP VQ path uses the sequential Annex A search rather than the broader
diagnostic second-stage search. Its gain predictor keeps the wider int32
§3.9.1 math path and the Annex A GA/GB preselection search.

## Diagnostic Profiles

Diagnostic profiles are intended for regression, PESQ experiments, and
listening comparisons. They are not conformance claims and should not be used
as stronger standards evidence than Core.

`EncoderProfileQualityPESQ` keeps the broader historical Quality heuristic
surface disabled, and enables native reconstructed-gain residual search, gain
clip repair, and fixed-codebook residual reranking. It has strong PESQ NB
scores on the current private sample set, but blind listening found it slightly
more muffled than Core, so it is not the default.

`EncoderProfileQuality` remains available as the older broad quality-heuristic
profile for diagnostics. Quality uses the sequential Annex A LSP VQ path by
default; the broader second-stage LSP search remains available only as an
internal diagnostic knob.

`EncoderProfileQualityAnnexALSP` is kept as an explicit alias for the
sequential Annex A LSP path in listening diagnostics.

`EncoderProfileQualityClean` is a listening-diagnostic profile for comparing a
smoother, `bcg729`-like candidate against the default Quality profile. It keeps
the standard 10-byte payload shape, disables normalized closed-loop pitch
reranking, and uses stricter, high-residual-aware decoder-in-loop gain MSE
repair.

`EncoderProfileQualityCleanSNR` keeps the same pitch policy but uses the older
high-SNR gain-repair preference for clarity A/B tests.

`EncoderProfileQualityCleanSmooth` lowers the clean repair threshold and biases
more strongly toward high-residual reduction for bitstream-level smoothing
diagnostics.

`EncoderProfileQualityCleanVoiced` keeps the clean pitch policy while allowing
decoder-in-loop gain repair to prefer slightly higher adaptive gain when the
objective-score tradeoff remains bounded.

`EncoderProfileQualityCleanDegrit` keeps the clean pitch policy while allowing
gain repair to prefer lower fixed-codebook gain correction when adaptive gain
is not reduced.

`EncoderProfileQualityCleanHarmonic` keeps the clean pitch policy while
allowing voiced gain repair to trade bounded score loss for higher adaptive
gain with lower fixed-codebook correction.

`EncoderProfileQualityCleanHarmonicStrong` pushes that same gain-balance
tradeoff harder for grit-vs-muffling A/B tests.

`EncoderProfileQualityCleanHarmonicDeep` pushes the same gain-balance tradeoff
beyond the strong candidate to locate the grit-vs-muffling boundary.

`EncoderProfileQualityCleanFCBRerank` keeps the clean pitch policy while
reranking a small fixed-codebook candidate set with decoder-in-loop residual
scoring for grit/noise listening diagnostics.

`EncoderProfileCoreClipRepair` is a listening-diagnostic variant of Core. It
keeps the Core LSP/FCB/gain-preselect policy and only adds decoder-in-loop gain
clip repair with a lower pre-clip threshold. Use it for A/B tests of the Core
sound with a minimal clipping safety valve, not as a conformance claim.

## Selection Guidance

Use `NewEncoder()` or `NewEncoderWithProfile(EncoderProfileCore)` for normal
applications.

Use `NewEncoderWithProfile(EncoderProfileCoreFast)` only when channel density
or latency is more important than preserving the default Core search result.

Use diagnostic profiles only when you are explicitly running the private
quality/PESQ/blind-listening test workflows described in
[validation.md](validation.md).

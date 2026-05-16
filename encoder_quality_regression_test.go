package g729

import "testing"

func TestEncoderCorePagesQualityRegression(t *testing.T) {
	sourcePCM, format := readPCM16WAVFixture(t, "docs/assets/audio/source-8k-16bit.wav")
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("source WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			format.sampleRate, format.channels, format.bitsPerSample)
	}

	samples := s16leToSamples(padPCM16ToFrames(t, sourcePCM))
	payload := encodeSamplesWithProfileForRegression(t, samples, EncoderProfileCore)
	decoded := decodeRawG729WithLocal(t, payload)
	metrics := externalQualityMetricsFor(samples, decoded, 240)

	t.Logf("Pages core quality: shift=%d globalSNR=%.2f segSNR=%.2f corr=%.4f rms/ref=%.4f peak=%d nearClip=%d",
		metrics.shift, metrics.globalSNR, metrics.segSNR, metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)

	if metrics.globalSNR < 2.0 {
		t.Fatalf("Pages core global SNR %.2f dB below regression floor", metrics.globalSNR)
	}
	if metrics.corr < 0.65 {
		t.Fatalf("Pages core correlation %.4f below regression floor", metrics.corr)
	}
	if metrics.rmsRatio < 0.80 || metrics.rmsRatio > 1.10 {
		t.Fatalf("Pages core RMS ratio %.4f outside regression range", metrics.rmsRatio)
	}
	if metrics.nearClip != 0 {
		t.Fatalf("Pages core decoded near-clip count = %d, want 0", metrics.nearClip)
	}
}

func TestEncoderCoreFastPagesQualityRegression(t *testing.T) {
	sourcePCM, format := readPCM16WAVFixture(t, "docs/assets/audio/source-8k-16bit.wav")
	if format.sampleRate != SampleRate || format.channels != 1 || format.bitsPerSample != 16 {
		t.Fatalf("source WAV format = %d Hz, %d channel(s), %d bits; want 8000 Hz mono s16",
			format.sampleRate, format.channels, format.bitsPerSample)
	}

	samples := s16leToSamples(padPCM16ToFrames(t, sourcePCM))
	payload := encodeSamplesWithProfileForRegression(t, samples, EncoderProfileCoreFast)
	decoded := decodeRawG729WithLocal(t, payload)
	metrics := externalQualityMetricsFor(samples, decoded, 240)

	t.Logf("Pages core-fast quality: shift=%d globalSNR=%.2f segSNR=%.2f corr=%.4f rms/ref=%.4f peak=%d nearClip=%d",
		metrics.shift, metrics.globalSNR, metrics.segSNR, metrics.corr, metrics.rmsRatio, metrics.peak, metrics.nearClip)

	if metrics.globalSNR < 1.8 {
		t.Fatalf("Pages core-fast global SNR %.2f dB below regression floor", metrics.globalSNR)
	}
	if metrics.corr < 0.62 {
		t.Fatalf("Pages core-fast correlation %.4f below regression floor", metrics.corr)
	}
	if metrics.rmsRatio < 0.75 || metrics.rmsRatio > 1.15 {
		t.Fatalf("Pages core-fast RMS ratio %.4f outside regression range", metrics.rmsRatio)
	}
	if metrics.nearClip != 0 {
		t.Fatalf("Pages core-fast decoded near-clip count = %d, want 0", metrics.nearClip)
	}
}

func encodeSamplesWithProfileForRegression(t *testing.T, samples []int16, profile EncoderProfile) []byte {
	t.Helper()
	if len(samples)%FrameSamples != 0 {
		t.Fatalf("sample count %d is not frame-aligned", len(samples))
	}
	enc := NewEncoderWithProfile(profile)
	out := make([]byte, 0, len(samples)/FrameSamples*FrameBytes)
	var frameBits [FrameBytes]byte
	for off := 0; off+FrameSamples <= len(samples); off += FrameSamples {
		if err := enc.EncodeFrame(samples[off:off+FrameSamples], frameBits[:]); err != nil {
			t.Fatalf("EncodeFrame frame %d: %v", off/FrameSamples, err)
		}
		out = append(out, frameBits[:]...)
	}
	return out
}

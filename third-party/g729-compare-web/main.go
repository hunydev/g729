package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hunydev/g729"
)

const (
	maxUploadBytes       = 64 << 20
	nearClipThreshold    = 32700
	maxClipMarkers       = 32
	clipClusterGapSample = 80
)

type response struct {
	Input     inputInfo              `json:"input"`
	Payload   payloadInfo            `json:"payload"`
	Audio     map[string]string      `json:"audio"`
	Downloads map[string]string      `json:"downloads"`
	Clips     map[string][]clipEvent `json:"clips"`
	Metrics   []metricRow            `json:"metrics"`
	Noise     []noiseRow             `json:"noise"`
	Notes     []string               `json:"notes"`
	Error     string                 `json:"error,omitempty"`
}

type inputInfo struct {
	Samples       int     `json:"samples"`
	PaddedSamples int     `json:"paddedSamples"`
	Frames        int     `json:"frames"`
	DurationSec   float64 `json:"durationSec"`
}

type payloadInfo struct {
	OurBytes          int     `json:"ourBytes"`
	ExternalBytes     int     `json:"externalBytes"`
	EqualBytes        int     `json:"equalBytes"`
	EqualPercent      float64 `json:"equalPercent"`
	FirstDiffByte     int     `json:"firstDiffByte"`
	ExternalTool      string  `json:"externalTool"`
	ExternalAvailable bool    `json:"externalAvailable"`
}

type metricRow struct {
	Path       string  `json:"path"`
	SNRDB      float64 `json:"snrDb"`
	Corr       float64 `json:"corr"`
	RMSRatio   float64 `json:"rmsRatio"`
	OutputPeak int     `json:"outputPeak"`
	NearClip   int     `json:"nearClip"`
	LagSamples int     `json:"lagSamples"`
}

type noiseRow struct {
	Path          string  `json:"path"`
	Key           string  `json:"key"`
	LagSamples    int     `json:"lagSamples"`
	ErrorRMS      float64 `json:"errorRms"`
	ErrorDB       float64 `json:"errorDb"`
	HighErrorRMS  float64 `json:"highErrorRms"`
	HighErrorDB   float64 `json:"highErrorDb"`
	HighShareDB   float64 `json:"highShareDb"`
	WorstTimeSec  float64 `json:"worstTimeSec"`
	WorstHighDB   float64 `json:"worstHighDb"`
	WorstFrameRMS float64 `json:"worstFrameRms"`
}

type clipEvent struct {
	TimeSec   float64 `json:"timeSec"`
	EndSec    float64 `json:"endSec"`
	Sample    int     `json:"sample"`
	EndSample int     `json:"endSample"`
	Count     int     `json:"count"`
	Peak      int     `json:"peak"`
	Value     int16   `json:"value"`
}

func main() {
	addr := envDefault("ADDR", ":8000")
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/api/compare", compare)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           allowHost(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("g729 compare web listening on %s\n", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

func allowHost(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"127.0.0.1":      true,
		"localhost":      true,
		"0.0.0.0":        true,
		"g729ab.exe.xyz": true,
		"g729.huny.dev":  true,
		"[::1]":          true,
		"":               true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if !allowed[strings.ToLower(host)] {
			http.Error(w, "host not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, pageHTML)
}

func compare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(response{Error: "POST required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, fmt.Errorf("file upload required: %w", err))
		return
	}
	defer file.Close()

	tmp, err := os.MkdirTemp("", "g729-compare-*")
	if err != nil {
		writeError(w, err)
		return
	}
	defer os.RemoveAll(tmp)

	uploadPath := filepath.Join(tmp, filepath.Base(header.Filename))
	uploadData, err := io.ReadAll(file)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(uploadData) == 0 {
		writeError(w, errors.New("uploaded file is empty"))
		return
	}
	if err := os.WriteFile(uploadPath, uploadData, 0o600); err != nil {
		writeError(w, err)
		return
	}

	var pcm []byte
	if r.FormValue("mode") == "raw" {
		pcm = uploadData
	} else {
		pcm, err = ffmpegToPCM(tmp, uploadPath)
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if len(pcm)%2 != 0 {
		writeError(w, fmt.Errorf("PCM byte length %d is not even", len(pcm)))
		return
	}

	originalSamples := len(pcm) / 2
	paddedPCM := padPCMToFrame(pcm)
	frames := len(paddedPCM) / (g729.FrameSamples * 2)

	ourPayload, err := encodeWithLocal(paddedPCM)
	if err != nil {
		writeError(w, err)
		return
	}
	cleanPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityClean)
	if err != nil {
		writeError(w, err)
		return
	}
	externalPayload, err := encodeWithBCG729(paddedPCM)
	if err != nil {
		writeError(w, err)
		return
	}

	localOurPCM, err := decodeWithLocal(ourPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of our payload: %w", err))
		return
	}
	localCleanPCM, err := decodeWithLocal(cleanPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of clean payload: %w", err))
		return
	}
	localExternalPCM, err := decodeWithLocal(externalPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of external payload: %w", err))
		return
	}
	ffmpegOurPCM, err := decodeWithFFmpeg(tmp, "our", ourPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of our payload: %w", err))
		return
	}
	ffmpegCleanPCM, err := decodeWithFFmpeg(tmp, "clean", cleanPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of clean payload: %w", err))
		return
	}
	ffmpegExternalPCM, err := decodeWithFFmpeg(tmp, "external", externalPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of external payload: %w", err))
		return
	}

	equal, firstDiff := byteEquality(ourPayload, externalPayload)
	den := maxInt(len(ourPayload), len(externalPayload))
	equalPercent := 0.0
	if den > 0 {
		equalPercent = float64(equal) * 100 / float64(den)
	}

	ourLocalMetric := qualityMetric("our encode -> local decode", paddedPCM, localOurPCM)
	ourFFmpegMetric := qualityMetric("our encode -> ffmpeg decode", paddedPCM, ffmpegOurPCM)
	cleanLocalMetric := qualityMetric("our clean encode -> local decode", paddedPCM, localCleanPCM)
	cleanFFmpegMetric := qualityMetric("our clean encode -> ffmpeg decode", paddedPCM, ffmpegCleanPCM)
	externalLocalMetric := qualityMetric("bcg729 encode -> local decode", paddedPCM, localExternalPCM)
	externalFFmpegMetric := qualityMetric("bcg729 encode -> ffmpeg decode", paddedPCM, ffmpegExternalPCM)
	localPayloadMetric := qualityMetric("local decoder: our payload vs bcg729 payload", localOurPCM, localExternalPCM)
	ffmpegPayloadMetric := qualityMetric("ffmpeg decoder: our payload vs bcg729 payload", ffmpegOurPCM, ffmpegExternalPCM)
	localCleanPayloadMetric := qualityMetric("local decoder: clean payload vs bcg729 payload", localCleanPCM, localExternalPCM)
	ffmpegCleanPayloadMetric := qualityMetric("ffmpeg decoder: clean payload vs bcg729 payload", ffmpegCleanPCM, ffmpegExternalPCM)

	resp := response{
		Input: inputInfo{
			Samples:       originalSamples,
			PaddedSamples: len(paddedPCM)/2 - originalSamples,
			Frames:        frames,
			DurationSec:   float64(originalSamples) / g729.SampleRate,
		},
		Payload: payloadInfo{
			OurBytes:          len(ourPayload),
			ExternalBytes:     len(externalPayload),
			EqualBytes:        equal,
			EqualPercent:      equalPercent,
			FirstDiffByte:     firstDiff,
			ExternalTool:      "libbcg729 via local black-box CLI",
			ExternalAvailable: true,
		},
		Audio: map[string]string{
			"source":          wavDataURL(paddedPCM),
			"our_local":       wavDataURL(localOurPCM),
			"our_ffmpeg":      wavDataURL(ffmpegOurPCM),
			"clean_local":     wavDataURL(localCleanPCM),
			"clean_ffmpeg":    wavDataURL(ffmpegCleanPCM),
			"external_local":  wavDataURL(localExternalPCM),
			"external_ffmpeg": wavDataURL(ffmpegExternalPCM),
		},
		Downloads: map[string]string{
			"our_g729":      payloadDataURL(ourPayload),
			"clean_g729":    payloadDataURL(cleanPayload),
			"external_g729": payloadDataURL(externalPayload),
		},
		Clips: map[string][]clipEvent{
			"source":          clipEvents(paddedPCM, maxClipMarkers),
			"our_local":       clipEvents(localOurPCM, maxClipMarkers),
			"our_ffmpeg":      clipEvents(ffmpegOurPCM, maxClipMarkers),
			"clean_local":     clipEvents(localCleanPCM, maxClipMarkers),
			"clean_ffmpeg":    clipEvents(ffmpegCleanPCM, maxClipMarkers),
			"external_local":  clipEvents(localExternalPCM, maxClipMarkers),
			"external_ffmpeg": clipEvents(ffmpegExternalPCM, maxClipMarkers),
		},
		Metrics: []metricRow{
			ourLocalMetric,
			ourFFmpegMetric,
			cleanLocalMetric,
			cleanFFmpegMetric,
			externalLocalMetric,
			externalFFmpegMetric,
			localPayloadMetric,
			ffmpegPayloadMetric,
			localCleanPayloadMetric,
			ffmpegCleanPayloadMetric,
		},
		Noise: []noiseRow{
			residualNoiseMetric("our encode -> local residual vs source", "our_local", paddedPCM, localOurPCM, ourLocalMetric.LagSamples),
			residualNoiseMetric("our encode -> ffmpeg residual vs source", "our_ffmpeg", paddedPCM, ffmpegOurPCM, ourFFmpegMetric.LagSamples),
			residualNoiseMetric("our clean encode -> local residual vs source", "clean_local", paddedPCM, localCleanPCM, cleanLocalMetric.LagSamples),
			residualNoiseMetric("our clean encode -> ffmpeg residual vs source", "clean_ffmpeg", paddedPCM, ffmpegCleanPCM, cleanFFmpegMetric.LagSamples),
			residualNoiseMetric("bcg729 encode -> local residual vs source", "external_local", paddedPCM, localExternalPCM, externalLocalMetric.LagSamples),
			residualNoiseMetric("bcg729 encode -> ffmpeg residual vs source", "external_ffmpeg", paddedPCM, ffmpegExternalPCM, externalFFmpegMetric.LagSamples),
			residualNoiseMetric("local decoder delta on our payload", "our_local", ffmpegOurPCM, localOurPCM, 0),
			residualNoiseMetric("local decoder delta on clean payload", "clean_local", ffmpegCleanPCM, localCleanPCM, 0),
			residualNoiseMetric("local decoder delta on bcg729 payload", "external_local", ffmpegExternalPCM, localExternalPCM, 0),
			residualNoiseMetric("encoder delta under local decode", "our_local", localExternalPCM, localOurPCM, localPayloadMetric.LagSamples),
			residualNoiseMetric("encoder delta under ffmpeg decode", "our_ffmpeg", ffmpegExternalPCM, ffmpegOurPCM, ffmpegPayloadMetric.LagSamples),
			residualNoiseMetric("clean encoder delta under local decode", "clean_local", localExternalPCM, localCleanPCM, localCleanPayloadMetric.LagSamples),
			residualNoiseMetric("clean encoder delta under ffmpeg decode", "clean_ffmpeg", ffmpegExternalPCM, ffmpegCleanPCM, ffmpegCleanPayloadMetric.LagSamples),
		},
		Notes: []string{
			"External encoder is isolated under gitignored third-party/ and used only as a black-box executable.",
			"Clean candidate uses EncoderProfileQualityClean: no normalized closed-loop pitch reranking, stricter gain MSE repair.",
			"FFmpeg is used only as a black-box G.729 decoder.",
			"Scores are delay-compensated listening diagnostics, not ITU conformance certification.",
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(response{Error: err.Error()})
}

func ffmpegToPCM(tmp, input string) ([]byte, error) {
	out := filepath.Join(tmp, "input.pcm")
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", input, "-ar", "8000", "-ac", "1", "-f", "s16le", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg input conversion failed: %v: %s", err, string(b))
	}
	return os.ReadFile(out)
}

func encodeWithLocal(pcm []byte) ([]byte, error) {
	return encodeWithLocalProfile(pcm, g729.EncoderProfileQuality)
}

func encodeWithLocalProfile(pcm []byte, profile g729.EncoderProfile) ([]byte, error) {
	enc := g729.NewEncoderWithProfile(profile)
	out := make([]byte, 0, len(pcm)/(g729.FrameSamples*2)*g729.FrameBytes)
	frame := make([]int16, g729.FrameSamples)
	bits := make([]byte, g729.FrameBytes)
	for off := 0; off < len(pcm); off += g729.FrameSamples * 2 {
		for i := range frame {
			frame[i] = int16(binary.LittleEndian.Uint16(pcm[off+i*2:]))
		}
		if err := enc.EncodeFrame(frame, bits); err != nil {
			return nil, err
		}
		out = append(out, bits...)
	}
	return out, nil
}

func encodeWithBCG729(pcm []byte) ([]byte, error) {
	path := bcg729EncoderPath()
	cmd := exec.Command(path)
	cmd.Stdin = bytes.NewReader(pcm)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %v: %s", path, err, stderr.String())
	}
	if len(out)%g729.FrameBytes != 0 {
		return nil, fmt.Errorf("external encoder returned %d bytes, not divisible by %d", len(out), g729.FrameBytes)
	}
	return out, nil
}

func bcg729EncoderPath() string {
	if v := os.Getenv("BCG729_ENCODER"); v != "" {
		return v
	}
	for _, path := range []string{
		"../bcg729-blackbox/bcg729_encode",
		"third-party/bcg729-blackbox/bcg729_encode",
	} {
		if resolved, err := exec.LookPath(path); err == nil {
			return resolved
		}
	}
	return "../bcg729-blackbox/bcg729_encode"
}

func decodeWithLocal(payload []byte) ([]byte, error) {
	dec := g729.NewDecoder()
	out := make([]byte, 0, len(payload)/g729.FrameBytes*g729.FrameSamples*2)
	frame := make([]int16, g729.FrameSamples)
	var pair [2]byte
	for off := 0; off < len(payload); off += g729.FrameBytes {
		if err := dec.DecodeFrame(payload[off:off+g729.FrameBytes], frame); err != nil {
			return nil, err
		}
		for _, sample := range frame {
			binary.LittleEndian.PutUint16(pair[:], uint16(sample))
			out = append(out, pair[:]...)
		}
	}
	return out, nil
}

func decodeWithFFmpeg(tmp, name string, payload []byte) ([]byte, error) {
	in := filepath.Join(tmp, name+".g729")
	out := filepath.Join(tmp, name+".pcm")
	if err := os.WriteFile(in, payload, 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-f", "g729", "-i", in, "-ar", "8000", "-ac", "1", "-f", "s16le", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %v: %s", err, string(b))
	}
	return os.ReadFile(out)
}

func padPCMToFrame(pcm []byte) []byte {
	frameBytes := g729.FrameSamples * 2
	pad := (frameBytes - len(pcm)%frameBytes) % frameBytes
	out := append([]byte(nil), pcm...)
	if pad > 0 {
		out = append(out, make([]byte, pad)...)
	}
	return out
}

func wavDataURL(pcm []byte) string {
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(wavBytes(pcm))
}

func payloadDataURL(payload []byte) string {
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(payload)
}

func clipEvents(pcm []byte, maxMarkers int) []clipEvent {
	if maxMarkers <= 0 {
		maxMarkers = maxClipMarkers
	}
	out := make([]clipEvent, 0)
	var current clipEvent
	inCluster := false
	lastClipSample := -clipClusterGapSample - 1
	flush := func() bool {
		if !inCluster {
			return false
		}
		out = append(out, current)
		inCluster = false
		return len(out) >= maxMarkers
	}
	for sample := 0; sample < len(pcm)/2; sample++ {
		value := samplePCM16(pcm, sample)
		abs := int(value)
		if abs < 0 {
			abs = -abs
		}
		if abs < nearClipThreshold {
			continue
		}
		timeSec := float64(sample) / g729.SampleRate
		if !inCluster || sample-lastClipSample > clipClusterGapSample {
			if flush() {
				return out
			}
			current = clipEvent{
				TimeSec:   timeSec,
				EndSec:    timeSec,
				Sample:    sample,
				EndSample: sample,
				Count:     1,
				Peak:      abs,
				Value:     value,
			}
			inCluster = true
		} else {
			current.EndSec = timeSec
			current.EndSample = sample
			current.Count++
			if abs > current.Peak {
				current.Peak = abs
				current.Value = value
			}
		}
		lastClipSample = sample
	}
	flush()
	return out
}

func wavBytes(pcm []byte) []byte {
	var b bytes.Buffer
	dataLen := uint32(len(pcm))
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36)+dataLen)
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(g729.SampleRate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(g729.SampleRate*2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, dataLen)
	b.Write(pcm)
	return b.Bytes()
}

func qualityMetric(path string, refPCM, outPCM []byte) metricRow {
	refSamples := len(refPCM) / 2
	outSamples := len(outPCM) / 2
	if refSamples == 0 || outSamples == 0 {
		return metricRow{Path: path, SNRDB: math.Inf(-1)}
	}
	bestLag := 0
	bestCorr := math.Inf(-1)
	for lag := -240; lag <= 240; lag++ {
		corr := metricCorrAtLag(refPCM, outPCM, lag)
		if corr > bestCorr {
			bestCorr = corr
			bestLag = lag
		}
	}
	return metricAtLag(path, refPCM, outPCM, bestLag)
}

func metricCorrAtLag(refPCM, outPCM []byte, lag int) float64 {
	startRef, startOut, n := alignedWindow(len(refPCM)/2, len(outPCM)/2, lag)
	if n <= 0 {
		return math.Inf(-1)
	}
	var refEnergy, outEnergy, cross float64
	for i := 0; i < n; i++ {
		ref := float64(samplePCM16(refPCM, startRef+i))
		out := float64(samplePCM16(outPCM, startOut+i))
		refEnergy += ref * ref
		outEnergy += out * out
		cross += ref * out
	}
	if refEnergy == 0 || outEnergy == 0 {
		return math.Inf(-1)
	}
	return cross / math.Sqrt(refEnergy*outEnergy)
}

func metricAtLag(path string, refPCM, outPCM []byte, lag int) metricRow {
	startRef, startOut, n := alignedWindow(len(refPCM)/2, len(outPCM)/2, lag)
	if n <= 0 {
		return metricRow{Path: path, SNRDB: math.Inf(-1), LagSamples: lag}
	}
	var signal, noise, refEnergy, outEnergy, cross float64
	var peak int
	var nearClip int
	for i := 0; i < n; i++ {
		ref := float64(samplePCM16(refPCM, startRef+i))
		outSample := samplePCM16(outPCM, startOut+i)
		out := float64(outSample)
		diff := ref - out
		signal += ref * ref
		noise += diff * diff
		refEnergy += ref * ref
		outEnergy += out * out
		cross += ref * out
		abs := int(outSample)
		if abs < 0 {
			abs = -abs
		}
		if abs > peak {
			peak = abs
		}
		if abs >= nearClipThreshold {
			nearClip++
		}
	}
	snr := 99.0
	if noise > 0 {
		snr = 10 * math.Log10((signal+1)/noise)
	}
	corr := 0.0
	if refEnergy > 0 && outEnergy > 0 {
		corr = cross / math.Sqrt(refEnergy*outEnergy)
	}
	rmsRatio := 0.0
	if refEnergy > 0 {
		rmsRatio = math.Sqrt(outEnergy / refEnergy)
	}
	return metricRow{
		Path:       path,
		SNRDB:      snr,
		Corr:       corr,
		RMSRatio:   rmsRatio,
		OutputPeak: peak,
		NearClip:   nearClip,
		LagSamples: lag,
	}
}

func residualNoiseMetric(path, key string, refPCM, outPCM []byte, lag int) noiseRow {
	startRef, startOut, n := alignedWindow(len(refPCM)/2, len(outPCM)/2, lag)
	row := noiseRow{Path: path, Key: key, LagSamples: lag}
	if n <= 1 {
		row.ErrorDB = math.Inf(-1)
		row.HighErrorDB = math.Inf(-1)
		row.HighShareDB = math.Inf(-1)
		row.WorstHighDB = math.Inf(-1)
		return row
	}

	var signal, errEnergy, highEnergy float64
	var prevErr float64

	for i := 0; i < n; i++ {
		ref := float64(samplePCM16(refPCM, startRef+i))
		out := float64(samplePCM16(outPCM, startOut+i))
		err := out - ref
		signal += ref * ref
		errEnergy += err * err
		if i > 0 {
			high := err - prevErr
			highEnergy += high * high
		}
		prevErr = err
	}

	refRMS := math.Sqrt(signal / float64(n))
	errRMS := math.Sqrt(errEnergy / float64(n))
	highRMS := math.Sqrt(highEnergy / float64(n-1))

	const frameSamples = g729.FrameSamples
	minFrameRefRMS := math.Max(200, refRMS*0.15)
	worstHighDB := math.Inf(-1)
	worstTime := 0.0
	worstFrameRMS := 0.0

	for frameStart := 0; frameStart+1 < n; frameStart += frameSamples {
		frameEnd := minInt(n, frameStart+frameSamples)
		var frameSignal, frameHigh float64
		prev := float64(samplePCM16(outPCM, startOut+frameStart)) - float64(samplePCM16(refPCM, startRef+frameStart))
		for i := frameStart; i < frameEnd; i++ {
			ref := float64(samplePCM16(refPCM, startRef+i))
			out := float64(samplePCM16(outPCM, startOut+i))
			err := out - ref
			frameSignal += ref * ref
			if i > frameStart {
				high := err - prev
				frameHigh += high * high
			}
			prev = err
		}
		if frameEnd-frameStart <= 1 || frameSignal <= 0 {
			continue
		}
		highRMS := math.Sqrt(frameHigh / float64(frameEnd-frameStart-1))
		refRMS := math.Sqrt(frameSignal / float64(frameEnd-frameStart))
		if refRMS < minFrameRefRMS {
			continue
		}
		highDB := 20 * math.Log10((highRMS+1)/(refRMS+1))
		if highDB > worstHighDB {
			worstHighDB = highDB
			worstTime = float64(startOut+frameStart) / g729.SampleRate
			worstFrameRMS = highRMS
		}
	}
	if math.IsInf(worstHighDB, -1) {
		worstHighDB = 20 * math.Log10((highRMS+1)/(refRMS+1))
	}

	row.ErrorRMS = errRMS
	row.ErrorDB = 20 * math.Log10((errRMS+1)/(refRMS+1))
	row.HighErrorRMS = highRMS
	row.HighErrorDB = 20 * math.Log10((highRMS+1)/(refRMS+1))
	row.HighShareDB = 10 * math.Log10((highEnergy+1)/(2*errEnergy+1))
	row.WorstTimeSec = worstTime
	row.WorstHighDB = worstHighDB
	row.WorstFrameRMS = worstFrameRMS
	return row
}

func alignedWindow(refSamples, outSamples, lag int) (startRef, startOut, n int) {
	if lag >= 0 {
		startRef = 0
		startOut = lag
		n = minInt(refSamples, outSamples-lag)
		return
	}
	startRef = -lag
	startOut = 0
	n = minInt(refSamples+lag, outSamples)
	return
}

func samplePCM16(pcm []byte, index int) int16 {
	return int16(binary.LittleEndian.Uint16(pcm[index*2:]))
}

func byteEquality(a, b []byte) (equal, firstDiff int) {
	firstDiff = -1
	n := minInt(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			equal++
		} else if firstDiff < 0 {
			firstDiff = i
		}
	}
	if len(a) != len(b) && firstDiff < 0 {
		firstDiff = n
	}
	return equal, firstDiff
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

const pageHTML = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>G.729 encoder comparison</title>
  <style>
    :root { color-scheme: light; font-family: Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; --ink:#111318; --muted:#5d6472; --line:#dde1e7; --paper:#faf9f6; --card:#fff; --green:#0f766e; --orange:#c2410c; }
    * { box-sizing: border-box; }
    body { margin:0; background:var(--paper); color:var(--ink); }
    header { padding:32px; border-bottom:1px solid var(--line); background:#111318; color:#fff; }
    h1 { margin:0; font-size:clamp(36px, 7vw, 72px); line-height:.96; letter-spacing:0; }
    header p { max-width:880px; margin:18px 0 0; color:rgba(255,255,255,.78); font-size:18px; }
    main { width:min(1180px, 100%); margin:0 auto; padding:28px; }
    .panel, .card { border:1px solid var(--line); border-radius:8px; background:var(--card); }
    .panel { padding:22px; display:grid; gap:18px; }
    .controls { display:grid; grid-template-columns: 1fr auto auto; gap:12px; align-items:end; }
    .battle-controls { grid-template-columns: 1.35fr .8fr 1.05fr auto; }
    label { display:grid; gap:8px; color:var(--muted); font-size:13px; font-weight:800; text-transform:uppercase; }
    input, select { width:100%; min-height:44px; padding:9px 11px; border:1px solid var(--line); border-radius:8px; background:#fff; color:var(--ink); }
    button, .download { min-height:44px; padding:0 16px; border:0; border-radius:8px; background:var(--green); color:#fff; font-weight:850; cursor:pointer; text-decoration:none; display:inline-flex; align-items:center; justify-content:center; }
    button:disabled { opacity:.45; cursor:not-allowed; }
    .status { margin:0; color:var(--muted); }
    .tabs { display:flex; gap:10px; margin-bottom:18px; }
    .tab { background:#fff; color:var(--ink); border:1px solid var(--line); }
    .tab.active { background:#111318; color:#fff; }
    .tab-panel { display:none; }
    .tab-panel.active { display:block; }
    .grid { display:grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap:16px; margin-top:18px; }
    .battle-grid { display:grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap:16px; }
    .battle-arena { display:grid; gap:16px; margin-top:18px; }
    .battle-head { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; }
    .battle-choice { min-height:50px; width:100%; }
    .battle-choice.left { background:#0f766e; }
    .battle-choice.right { background:#334155; }
    .battle-choice.tie { background:#8a5a16; }
    .battle-result { display:grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap:12px; }
    .score { padding:16px; border:1px solid var(--line); border-radius:8px; background:#f8fafc; }
    .score strong { display:block; font-size:30px; margin-top:8px; }
    .card { padding:18px; display:grid; gap:12px; }
    .card h2 { margin:0; font-size:18px; }
    .card p { margin:0; color:var(--muted); }
    audio { width:100%; }
    .metric-table { width:100%; border-collapse:collapse; margin-top:18px; background:#fff; border:1px solid var(--line); border-radius:8px; overflow:hidden; }
    th, td { padding:10px 12px; border-bottom:1px solid var(--line); text-align:left; font-size:14px; }
    th { color:#333843; background:#f1f4f2; }
    .pill { display:inline-flex; padding:5px 8px; border-radius:999px; background:#eef7f5; color:var(--green); font-size:12px; font-weight:850; }
    .warn { color:var(--orange); }
    .downloads { display:flex; flex-wrap:wrap; gap:10px; margin-top:14px; }
    .clip-card { margin-top:18px; }
    .clip-list { display:grid; gap:8px; }
    .clip-button { min-height:38px; justify-content:flex-start; background:#fff7ed; color:var(--orange); border:1px solid #fed7aa; text-align:left; }
    .clip-empty { margin:0; color:var(--muted); }
    .noise-note { margin:10px 0 0; color:var(--muted); font-size:13px; line-height:1.45; }
    .seek-cell { display:flex; gap:8px; align-items:center; }
    .seek { min-height:28px; padding:0 10px; background:#111318; font-size:12px; }
    @media (max-width: 820px) { .controls, .battle-controls, .grid, .battle-grid, .battle-result { grid-template-columns:1fr; } main, header { padding:20px; } }
  </style>
</head>
<body>
  <header>
    <span class="pill">black-box comparison</span>
    <h1>G.729 encoder comparison</h1>
    <p>업로드한 음원을 8 kHz mono signed 16-bit PCM으로 변환한 뒤, 이 저장소의 pure-Go encoder와 외부 libbcg729 encoder를 나란히 비교합니다. 외부 코덱은 gitignore된 third-party 실행 파일로만 호출합니다.</p>
  </header>
  <main>
    <nav class="tabs" aria-label="comparison modes">
      <button class="tab active" type="button" data-tab="compareTab">Full compare</button>
      <button class="tab" type="button" data-tab="battleTab">Blind 1:1</button>
    </nav>

    <section class="tab-panel active" id="compareTab">
      <section class="panel">
        <div class="controls">
          <label>Audio file<input id="file" type="file" accept="audio/*,.wav,.mp3,.pcm,.raw,.sln,.s16le"></label>
          <label>Input mode<select id="mode"><option value="audio">WAV/MP3/browser audio</option><option value="raw">Raw 8 kHz mono s16le PCM</option></select></label>
          <button id="run">Compare</button>
        </div>
        <p class="status" id="status">Ready. For raw PCM, use 8 kHz mono signed 16-bit little-endian samples.</p>
        <div class="downloads" id="downloads"></div>
      </section>
      <section class="grid" id="audioGrid"></section>
      <section id="metrics"></section>
    </section>

    <section class="tab-panel" id="battleTab">
      <section class="panel">
        <div class="controls battle-controls">
          <label>Audio files<input id="battleFiles" type="file" multiple accept="audio/*,.wav,.mp3,.pcm,.raw,.sln,.s16le"></label>
          <label>Input mode<select id="battleMode"><option value="audio">WAV/MP3/browser audio</option><option value="raw">Raw 8 kHz mono s16le PCM</option></select></label>
          <label>Battle pair<select id="battlePair">
            <option value="our_ffmpeg|clean_ffmpeg">Current quality vs clean candidate</option>
            <option value="clean_ffmpeg|external_ffmpeg">Clean candidate vs bcg729</option>
            <option value="our_ffmpeg|external_ffmpeg">Current quality vs bcg729</option>
            <option value="our_local|clean_local">Current quality local vs clean local</option>
          </select></label>
          <button id="battleStart">Start blind test</button>
        </div>
        <p class="status" id="battleStatus">여러 음원을 올리면 각 음원을 한 번씩 A/B trial로 만들고 좌/우 후보를 무작위로 섞습니다.</p>
      </section>
      <section class="battle-arena" id="battleArena"></section>
    </section>
  </main>
  <script>
    const $ = (id) => document.getElementById(id);
    const labels = {
      source: "Converted source PCM",
      our_local: "our encode -> our decode",
      our_ffmpeg: "our encode -> FFmpeg decode",
      clean_local: "our clean candidate -> our decode",
      clean_ffmpeg: "our clean candidate -> FFmpeg decode",
      external_local: "bcg729 encode -> our decode",
      external_ffmpeg: "bcg729 encode -> FFmpeg decode"
    };
    const battleCandidates = {
      our_ffmpeg: { label: "Current quality -> FFmpeg decode" },
      clean_ffmpeg: { label: "Clean candidate -> FFmpeg decode" },
      external_ffmpeg: { label: "bcg729 -> FFmpeg decode" },
      our_local: { label: "Current quality -> local decode" },
      clean_local: { label: "Clean candidate -> local decode" },
      external_local: { label: "bcg729 -> local decode" }
    };
    const battleState = { trials: [], index: 0, pair: [] };

    document.querySelectorAll(".tab").forEach((tab) => {
      tab.addEventListener("click", () => setActiveTab(tab.dataset.tab));
    });
    function setActiveTab(id) {
      document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("active", tab.dataset.tab === id));
      document.querySelectorAll(".tab-panel").forEach((panel) => panel.classList.toggle("active", panel.id === id));
    }

    $("run").addEventListener("click", async () => {
      const file = $("file").files[0];
      if (!file) { $("status").textContent = "파일을 선택하세요."; return; }
      $("run").disabled = true;
      $("status").textContent = "Uploading and running comparison.";
      $("audioGrid").replaceChildren();
      $("metrics").replaceChildren();
      $("downloads").replaceChildren();
      const form = new FormData();
      form.append("file", file);
      form.append("mode", $("mode").value);
      try {
        const res = await fetch("/api/compare", { method:"POST", body: form });
        const data = await res.json();
        if (!res.ok || data.error) throw new Error(data.error || "comparison failed");
        render(data);
        $("status").textContent = data.input.frames + " frames, " + data.input.paddedSamples + " padded samples. Payload byte equality " + data.payload.equalPercent.toFixed(2) + "%.";
      } catch (err) {
        $("status").textContent = err.message;
      } finally {
        $("run").disabled = false;
      }
    });

    $("battleStart").addEventListener("click", startBattle);
    async function startBattle() {
      const files = Array.from($("battleFiles").files || []);
      if (!files.length) { $("battleStatus").textContent = "블라인드 테스트할 파일을 하나 이상 선택하세요."; return; }
      const pair = $("battlePair").value.split("|");
      if (pair.length !== 2 || !battleCandidates[pair[0]] || !battleCandidates[pair[1]]) {
        $("battleStatus").textContent = "대결 후보 설정이 잘못되었습니다.";
        return;
      }
      $("battleStart").disabled = true;
      $("battleArena").replaceChildren();
      battleState.trials = [];
      battleState.index = 0;
      battleState.pair = pair;
      try {
        for (let i = 0; i < files.length; i++) {
          $("battleStatus").textContent = "Preparing blind test " + (i + 1) + " / " + files.length + ": " + files[i].name;
          const data = await compareFile(files[i], $("battleMode").value);
          battleState.trials.push(makeBattleTrial(files[i].name, data, pair));
        }
        shuffleInPlace(battleState.trials);
        $("battleStatus").textContent = "Blind test ready. 후보 이름은 최종 결과에서 공개됩니다.";
        renderBattleTrial();
      } catch (err) {
        $("battleStatus").textContent = err.message;
      } finally {
        $("battleStart").disabled = false;
      }
    }

    async function compareFile(file, mode) {
      const form = new FormData();
      form.append("file", file);
      form.append("mode", mode);
      const res = await fetch("/api/compare", { method:"POST", body: form });
      const data = await res.json();
      if (!res.ok || data.error) throw new Error(file.name + ": " + (data.error || "comparison failed"));
      return data;
    }

    function makeBattleTrial(fileName, data, pair) {
      if (!data.audio || !data.audio[pair[0]] || !data.audio[pair[1]]) {
        throw new Error(fileName + ": selected battle audio is missing from API response");
      }
      const flip = Math.random() < 0.5;
      const leftKey = flip ? pair[1] : pair[0];
      const rightKey = flip ? pair[0] : pair[1];
      return {
        fileName: fileName,
        data: data,
        leftKey: leftKey,
        rightKey: rightKey,
        choice: "",
        winnerKey: ""
      };
    }

    function renderBattleTrial() {
      const arena = $("battleArena");
      arena.replaceChildren();
      if (!battleState.trials.length) return;
      if (battleState.index >= battleState.trials.length) {
        renderBattleResults();
        return;
      }
      const trial = battleState.trials[battleState.index];
      const wrap = document.createElement("section");
      wrap.className = "card";
      wrap.innerHTML =
        "<div class=\"battle-head\"><div><h2>Blind trial " + (battleState.index + 1) + " / " + battleState.trials.length + "</h2>" +
        "<p>" + escapeHTML(trial.fileName) + "</p></div><span class=\"pill\">left/right randomized</span></div>" +
        "<article class=\"card\"><h2>Reference source</h2><audio controls preload=\"metadata\"></audio></article>" +
        "<div class=\"battle-grid\">" +
        "<article class=\"card\"><h2>Left</h2><audio controls preload=\"metadata\"></audio><button class=\"battle-choice left\" type=\"button\" data-choice=\"left\">Left가 더 낫다</button></article>" +
        "<article class=\"card\"><h2>Right</h2><audio controls preload=\"metadata\"></audio><button class=\"battle-choice right\" type=\"button\" data-choice=\"right\">Right가 더 낫다</button></article>" +
        "</div><button class=\"battle-choice tie\" type=\"button\" data-choice=\"tie\">비슷함 / 판단 보류</button>";
      const audios = wrap.querySelectorAll("audio");
      audios[0].src = trial.data.audio.source;
      audios[1].src = trial.data.audio[trial.leftKey];
      audios[2].src = trial.data.audio[trial.rightKey];
      wrap.querySelectorAll("[data-choice]").forEach((button) => {
        button.addEventListener("click", () => recordBattleChoice(button.dataset.choice));
      });
      arena.append(wrap);
    }

    function recordBattleChoice(choice) {
      const trial = battleState.trials[battleState.index];
      trial.choice = choice;
      if (choice === "left") trial.winnerKey = trial.leftKey;
      if (choice === "right") trial.winnerKey = trial.rightKey;
      if (choice === "tie") trial.winnerKey = "tie";
      battleState.index++;
      renderBattleTrial();
    }

    function renderBattleResults() {
      const arena = $("battleArena");
      const pair = battleState.pair;
      const counts = {};
      counts[pair[0]] = 0;
      counts[pair[1]] = 0;
      counts.tie = 0;
      battleState.trials.forEach((trial) => {
        if (trial.winnerKey && Object.prototype.hasOwnProperty.call(counts, trial.winnerKey)) counts[trial.winnerKey]++;
      });
      const rows = battleState.trials.map((trial, i) => {
        const picked = trial.winnerKey === "tie" ? "Tie / unsure" : battleCandidates[trial.winnerKey].label;
        return "<tr><td>" + (i + 1) + "</td><td>" + escapeHTML(trial.fileName) + "</td><td>" +
          escapeHTML(battleCandidates[trial.leftKey].label) + "</td><td>" +
          escapeHTML(battleCandidates[trial.rightKey].label) + "</td><td>" + escapeHTML(picked) + "</td></tr>";
      }).join("");
      const section = document.createElement("section");
      section.className = "card";
      section.innerHTML =
        "<h2>Blind test result</h2>" +
        "<div class=\"battle-result\">" +
        scoreHTML(battleCandidates[pair[0]].label, counts[pair[0]]) +
        scoreHTML(battleCandidates[pair[1]].label, counts[pair[1]]) +
        scoreHTML("Tie / unsure", counts.tie) +
        "</div>" +
        "<table class=\"metric-table\"><thead><tr><th>#</th><th>File</th><th>Left was</th><th>Right was</th><th>Picked</th></tr></thead><tbody>" + rows + "</tbody></table>" +
        "<button id=\"battleAgain\" type=\"button\">Run another blind test</button>";
      arena.replaceChildren(section);
      $("battleStatus").textContent = "완료: " + battleState.trials.length + " trials.";
      $("battleAgain").addEventListener("click", startBattle);
    }

    function scoreHTML(label, count) {
      return "<div class=\"score\"><span>" + escapeHTML(label) + "</span><strong>" + count + "</strong></div>";
    }

    function render(data) {
      const grid = $("audioGrid");
      Object.entries(labels).forEach(([key, label]) => {
        const card = document.createElement("article");
        card.className = "card";
        card.innerHTML = "<h2>" + label + "</h2><audio controls preload=\"metadata\"></audio>";
        const audio = card.querySelector("audio");
        audio.src = data.audio[key];
        audio.dataset.key = key;
        grid.append(card);
      });
      const downloads = $("downloads");
      downloads.append(downloadLink(data.downloads.our_g729, "our-encoder.g729", "Download our .g729"));
      downloads.append(downloadLink(data.downloads.clean_g729, "our-clean-candidate.g729", "Download clean candidate .g729"));
      downloads.append(downloadLink(data.downloads.external_g729, "bcg729-encoder.g729", "Download bcg729 .g729"));
      const table = document.createElement("table");
      table.className = "metric-table";
      table.innerHTML = "<thead><tr><th>Path</th><th>SNR dB</th><th>Corr</th><th>RMS ratio</th><th>Lag</th><th>Peak</th><th>Near clip</th></tr></thead><tbody></tbody>";
      const tbody = table.querySelector("tbody");
      data.metrics.forEach((m) => {
        const tr = document.createElement("tr");
        tr.innerHTML = "<td>" + m.path + "</td><td>" + fmt(m.snrDb) + "</td><td>" + fmt(m.corr, 4) + "</td><td>" + fmt(m.rmsRatio, 4) + "</td><td>" + m.lagSamples + "</td><td>" + m.outputPeak + "</td><td>" + m.nearClip + "</td>";
        tbody.append(tr);
      });
      $("metrics").append(table);
      renderNoise(data);
      renderClips(data);
    }
    function renderNoise(data) {
      if (!data.noise || !data.noise.length) return;
      const wrap = document.createElement("section");
      wrap.className = "card clip-card";
      wrap.innerHTML = "<h2>Residual / 자글자글 진단</h2><p class=\"noise-note\">High residual은 residual의 sample-to-sample 변화량입니다. 값이 덜 음수일수록 hiss/자글자글 후보가 큽니다. Worst time은 10 ms 프레임 단위 후보 위치입니다.</p>";
      const table = document.createElement("table");
      table.className = "metric-table";
      table.innerHTML = "<thead><tr><th>Path</th><th>Error dB</th><th>High dB</th><th>High share</th><th>Worst time</th><th>Worst high dB</th><th>High RMS</th></tr></thead><tbody></tbody>";
      const tbody = table.querySelector("tbody");
      data.noise.forEach((n) => {
        const tr = document.createElement("tr");
        const time = Number.isFinite(n.worstTimeSec) ? fmt(n.worstTimeSec, 3) + "s" : "-";
        tr.innerHTML = "<td>" + n.path + "</td><td>" + fmt(n.errorDb) + "</td><td>" + fmt(n.highErrorDb) + "</td><td>" + fmt(n.highShareDb) + "</td><td><span class=\"seek-cell\"><button class=\"seek\" type=\"button\">seek</button>" + time + "</span></td><td>" + fmt(n.worstHighDb) + "</td><td>" + fmt(n.highErrorRms) + "</td>";
        const btn = tr.querySelector("button");
        btn.addEventListener("click", () => {
          const audio = document.querySelector("audio[data-key='" + n.key + "']");
          if (!audio) return;
          audio.currentTime = Math.max(0, n.worstTimeSec - 0.15);
          audio.play().catch(() => {});
        });
        tbody.append(tr);
      });
      wrap.append(table);
      $("metrics").append(wrap);
    }
    function renderClips(data) {
      const events = [];
      Object.entries(labels).forEach(([key, label]) => {
        const clips = (data.clips && data.clips[key]) || [];
        clips.forEach((clip) => {
          events.push({
            key: key,
            label: label,
            timeSec: clip.timeSec,
            endSec: clip.endSec,
            sample: clip.sample,
            endSample: clip.endSample,
            count: clip.count,
            peak: clip.peak,
            value: clip.value
          });
        });
      });
      const wrap = document.createElement("section");
      wrap.className = "card clip-card";
      if (!events.length) {
        wrap.innerHTML = "<h2>Near-clip markers</h2><p class=\"clip-empty\">No near-clip samples at threshold >= 32700.</p>";
        $("metrics").append(wrap);
        return;
      }
      wrap.innerHTML = "<h2>Near-clip markers</h2><p class=\"clip-empty\">Click a marker to seek that decoded audio to 150 ms before the clipped region.</p><div class=\"clip-list\"></div>";
      const list = wrap.querySelector(".clip-list");
      events.forEach((event) => {
        const button = document.createElement("button");
        const range = event.endSec > event.timeSec ? " - " + fmt(event.endSec, 3) + "s" : "";
        const sampleRange = event.endSample > event.sample ? event.sample + "-" + event.endSample : event.sample;
        button.type = "button";
        button.className = "clip-button";
        button.textContent = event.label + " @ " + fmt(event.timeSec, 3) + "s" + range + " | samples " + sampleRange + " | count " + event.count + " | peak " + event.peak + " | value " + event.value;
        button.addEventListener("click", () => {
          const audio = document.querySelector("audio[data-key='" + event.key + "']");
          if (!audio) return;
          audio.currentTime = Math.max(0, event.timeSec - 0.15);
          audio.play().catch(() => {});
        });
        list.append(button);
      });
      $("metrics").append(wrap);
    }
    function downloadLink(href, name, text) {
      const a = document.createElement("a");
      a.className = "download";
      a.href = href;
      a.download = name;
      a.textContent = text;
      return a;
    }
    function fmt(v, digits = 2) {
      if (!Number.isFinite(v)) return String(v);
      return Number(v).toFixed(digits);
    }
    function escapeHTML(s) {
      return String(s).replace(/[&<>"']/g, (ch) => ({ "&":"&amp;", "<":"&lt;", ">":"&gt;", "\"":"&quot;", "'":"&#39;" }[ch]));
    }
    function shuffleInPlace(items) {
      for (let i = items.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        const tmp = items[i];
        items[i] = items[j];
        items[j] = tmp;
      }
      return items;
    }
  </script>
</body>
</html>`

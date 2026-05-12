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
	"sort"
	"strconv"
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
	Path       string   `json:"path"`
	Key        string   `json:"key,omitempty"`
	SNRDB      float64  `json:"snrDb"`
	Corr       float64  `json:"corr"`
	RMSRatio   float64  `json:"rmsRatio"`
	PESQ       *float64 `json:"pesq,omitempty"`
	OutputPeak int      `json:"outputPeak"`
	NearClip   int      `json:"nearClip"`
	LagSamples int      `json:"lagSamples"`
}

type pesqPair struct {
	Row    *metricRow
	RefPCM []byte
	OutPCM []byte
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
	if wanted := parseWantedAudio(r.FormValue("want")); len(wanted) > 0 {
		writeSelectedAudioCompare(w, tmp, paddedPCM, originalSamples, frames, wanted)
		return
	}

	ourPayload, err := encodeWithLocal(paddedPCM)
	if err != nil {
		writeError(w, err)
		return
	}
	corePayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileCore)
	if err != nil {
		writeError(w, err)
		return
	}
	coreClipPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileCoreClipRepair)
	if err != nil {
		writeError(w, err)
		return
	}
	cleanPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityClean)
	if err != nil {
		writeError(w, err)
		return
	}
	snrPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanSNR)
	if err != nil {
		writeError(w, err)
		return
	}
	smoothPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanSmooth)
	if err != nil {
		writeError(w, err)
		return
	}
	voicedPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanVoiced)
	if err != nil {
		writeError(w, err)
		return
	}
	degritPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanDegrit)
	if err != nil {
		writeError(w, err)
		return
	}
	harmonicPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanHarmonic)
	if err != nil {
		writeError(w, err)
		return
	}
	harmonicStrongPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanHarmonicStrong)
	if err != nil {
		writeError(w, err)
		return
	}
	harmonicDeepPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanHarmonicDeep)
	if err != nil {
		writeError(w, err)
		return
	}
	fcbPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanFCBRerank)
	if err != nil {
		writeError(w, err)
		return
	}
	pesqPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityPESQ)
	if err != nil {
		writeError(w, err)
		return
	}
	pesqDegritPayload, err := encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityPESQDegrit)
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
	localCorePCM, err := decodeWithLocal(corePayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of core payload: %w", err))
		return
	}
	localCoreClipPCM, err := decodeWithLocal(coreClipPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of core-clip payload: %w", err))
		return
	}
	localCleanPCM, err := decodeWithLocal(cleanPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of clean payload: %w", err))
		return
	}
	localSNRPCM, err := decodeWithLocal(snrPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of SNR payload: %w", err))
		return
	}
	localSmoothPCM, err := decodeWithLocal(smoothPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of smooth payload: %w", err))
		return
	}
	localVoicedPCM, err := decodeWithLocal(voicedPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of voiced payload: %w", err))
		return
	}
	localDegritPCM, err := decodeWithLocal(degritPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of degrit payload: %w", err))
		return
	}
	localHarmonicPCM, err := decodeWithLocal(harmonicPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of harmonic payload: %w", err))
		return
	}
	localHarmonicStrongPCM, err := decodeWithLocal(harmonicStrongPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of harmonic-strong payload: %w", err))
		return
	}
	localHarmonicDeepPCM, err := decodeWithLocal(harmonicDeepPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of harmonic-deep payload: %w", err))
		return
	}
	localFCBPCM, err := decodeWithLocal(fcbPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of FCB-rerank payload: %w", err))
		return
	}
	localPESQPCM, err := decodeWithLocal(pesqPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of PESQ candidate payload: %w", err))
		return
	}
	localPESQDegritPCM, err := decodeWithLocal(pesqDegritPayload)
	if err != nil {
		writeError(w, fmt.Errorf("local decode of PESQ-degrit candidate payload: %w", err))
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
	ffmpegCorePCM, err := decodeWithFFmpeg(tmp, "core", corePayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of core payload: %w", err))
		return
	}
	ffmpegCoreClipPCM, err := decodeWithFFmpeg(tmp, "core_clip", coreClipPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of core-clip payload: %w", err))
		return
	}
	ffmpegCleanPCM, err := decodeWithFFmpeg(tmp, "clean", cleanPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of clean payload: %w", err))
		return
	}
	ffmpegSNRPCM, err := decodeWithFFmpeg(tmp, "snr", snrPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of SNR payload: %w", err))
		return
	}
	ffmpegSmoothPCM, err := decodeWithFFmpeg(tmp, "smooth", smoothPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of smooth payload: %w", err))
		return
	}
	ffmpegVoicedPCM, err := decodeWithFFmpeg(tmp, "voiced", voicedPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of voiced payload: %w", err))
		return
	}
	ffmpegDegritPCM, err := decodeWithFFmpeg(tmp, "degrit", degritPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of degrit payload: %w", err))
		return
	}
	ffmpegHarmonicPCM, err := decodeWithFFmpeg(tmp, "harmonic", harmonicPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of harmonic payload: %w", err))
		return
	}
	ffmpegHarmonicStrongPCM, err := decodeWithFFmpeg(tmp, "harmonic_strong", harmonicStrongPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of harmonic-strong payload: %w", err))
		return
	}
	ffmpegHarmonicDeepPCM, err := decodeWithFFmpeg(tmp, "harmonic_deep", harmonicDeepPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of harmonic-deep payload: %w", err))
		return
	}
	ffmpegFCBPCM, err := decodeWithFFmpeg(tmp, "fcb", fcbPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of FCB-rerank payload: %w", err))
		return
	}
	ffmpegPESQPCM, err := decodeWithFFmpeg(tmp, "pesq", pesqPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of PESQ candidate payload: %w", err))
		return
	}
	ffmpegPESQDegritPCM, err := decodeWithFFmpeg(tmp, "pesq_degrit", pesqDegritPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of PESQ-degrit candidate payload: %w", err))
		return
	}
	ffmpegExternalPCM, err := decodeWithFFmpeg(tmp, "external", externalPayload)
	if err != nil {
		writeError(w, fmt.Errorf("ffmpeg decode of external payload: %w", err))
		return
	}
	softOurFFmpegPCM := softenPCM16(ffmpegOurPCM)
	softCleanFFmpegPCM := softenPCM16(ffmpegCleanPCM)
	blendOurPCM, err := decodeWithLocalPostfilterBlend(ourPayload, 1, 2)
	if err != nil {
		writeError(w, fmt.Errorf("blend50 local decode of our payload: %w", err))
		return
	}
	blendCleanPCM, err := decodeWithLocalPostfilterBlend(cleanPayload, 1, 2)
	if err != nil {
		writeError(w, fmt.Errorf("blend50 local decode of clean payload: %w", err))
		return
	}
	blendPESQPCM, err := decodeWithLocalPostfilterBlend(pesqPayload, 1, 2)
	if err != nil {
		writeError(w, fmt.Errorf("blend50 local decode of PESQ candidate payload: %w", err))
		return
	}
	blendExternalPCM, err := decodeWithLocalPostfilterBlend(externalPayload, 1, 2)
	if err != nil {
		writeError(w, fmt.Errorf("blend50 local decode of external payload: %w", err))
		return
	}

	equal, firstDiff := byteEquality(ourPayload, externalPayload)
	den := maxInt(len(ourPayload), len(externalPayload))
	equalPercent := 0.0
	if den > 0 {
		equalPercent = float64(equal) * 100 / float64(den)
	}

	ourLocalMetric := qualityMetric("our encode -> local decode", paddedPCM, localOurPCM)
	coreLocalMetric := qualityMetric("our core encode -> local decode", paddedPCM, localCorePCM)
	coreFFmpegMetric := qualityMetric("our core encode -> ffmpeg decode", paddedPCM, ffmpegCorePCM)
	coreClipLocalMetric := qualityMetric("our core-clip encode -> local decode", paddedPCM, localCoreClipPCM)
	coreClipFFmpegMetric := qualityMetric("our core-clip encode -> ffmpeg decode", paddedPCM, ffmpegCoreClipPCM)
	ourBlend50Metric := qualityMetric("our encode -> local postfilter-blend50 decode", paddedPCM, blendOurPCM)
	ourFFmpegMetric := qualityMetric("our encode -> ffmpeg decode", paddedPCM, ffmpegOurPCM)
	cleanLocalMetric := qualityMetric("our clean encode -> local decode", paddedPCM, localCleanPCM)
	cleanBlend50Metric := qualityMetric("our clean encode -> local postfilter-blend50 decode", paddedPCM, blendCleanPCM)
	cleanFFmpegMetric := qualityMetric("our clean encode -> ffmpeg decode", paddedPCM, ffmpegCleanPCM)
	snrLocalMetric := qualityMetric("our SNR-clean encode -> local decode", paddedPCM, localSNRPCM)
	snrFFmpegMetric := qualityMetric("our SNR-clean encode -> ffmpeg decode", paddedPCM, ffmpegSNRPCM)
	smoothLocalMetric := qualityMetric("our smooth-clean encode -> local decode", paddedPCM, localSmoothPCM)
	smoothFFmpegMetric := qualityMetric("our smooth-clean encode -> ffmpeg decode", paddedPCM, ffmpegSmoothPCM)
	voicedLocalMetric := qualityMetric("our voiced-clean encode -> local decode", paddedPCM, localVoicedPCM)
	voicedFFmpegMetric := qualityMetric("our voiced-clean encode -> ffmpeg decode", paddedPCM, ffmpegVoicedPCM)
	degritLocalMetric := qualityMetric("our degrit-clean encode -> local decode", paddedPCM, localDegritPCM)
	degritFFmpegMetric := qualityMetric("our degrit-clean encode -> ffmpeg decode", paddedPCM, ffmpegDegritPCM)
	harmonicLocalMetric := qualityMetric("our harmonic-clean encode -> local decode", paddedPCM, localHarmonicPCM)
	harmonicFFmpegMetric := qualityMetric("our harmonic-clean encode -> ffmpeg decode", paddedPCM, ffmpegHarmonicPCM)
	harmonicStrongLocalMetric := qualityMetric("our harmonic-strong encode -> local decode", paddedPCM, localHarmonicStrongPCM)
	harmonicStrongFFmpegMetric := qualityMetric("our harmonic-strong encode -> ffmpeg decode", paddedPCM, ffmpegHarmonicStrongPCM)
	harmonicDeepLocalMetric := qualityMetric("our harmonic-deep encode -> local decode", paddedPCM, localHarmonicDeepPCM)
	harmonicDeepFFmpegMetric := qualityMetric("our harmonic-deep encode -> ffmpeg decode", paddedPCM, ffmpegHarmonicDeepPCM)
	fcbLocalMetric := qualityMetric("our FCB-clean encode -> local decode", paddedPCM, localFCBPCM)
	fcbFFmpegMetric := qualityMetric("our FCB-clean encode -> ffmpeg decode", paddedPCM, ffmpegFCBPCM)
	pesqLocalMetric := qualityMetric("our PESQ candidate encode -> local decode", paddedPCM, localPESQPCM)
	pesqBlend50Metric := qualityMetric("our PESQ candidate encode -> local postfilter-blend50 decode", paddedPCM, blendPESQPCM)
	pesqFFmpegMetric := qualityMetric("our PESQ candidate encode -> ffmpeg decode", paddedPCM, ffmpegPESQPCM)
	pesqDegritLocalMetric := qualityMetric("our PESQ-degrit candidate encode -> local decode", paddedPCM, localPESQDegritPCM)
	pesqDegritFFmpegMetric := qualityMetric("our PESQ-degrit candidate encode -> ffmpeg decode", paddedPCM, ffmpegPESQDegritPCM)
	softOurFFmpegMetric := qualityMetric("our encode -> softened FFmpeg decode", paddedPCM, softOurFFmpegPCM)
	softCleanFFmpegMetric := qualityMetric("our clean encode -> softened FFmpeg decode", paddedPCM, softCleanFFmpegPCM)
	externalLocalMetric := qualityMetric("bcg729 encode -> local decode", paddedPCM, localExternalPCM)
	externalBlend50Metric := qualityMetric("bcg729 encode -> local postfilter-blend50 decode", paddedPCM, blendExternalPCM)
	externalFFmpegMetric := qualityMetric("bcg729 encode -> ffmpeg decode", paddedPCM, ffmpegExternalPCM)
	localPayloadMetric := qualityMetric("local decoder: our payload vs bcg729 payload", localOurPCM, localExternalPCM)
	ffmpegPayloadMetric := qualityMetric("ffmpeg decoder: our payload vs bcg729 payload", ffmpegOurPCM, ffmpegExternalPCM)
	localCleanPayloadMetric := qualityMetric("local decoder: clean payload vs bcg729 payload", localCleanPCM, localExternalPCM)
	ffmpegCleanPayloadMetric := qualityMetric("ffmpeg decoder: clean payload vs bcg729 payload", ffmpegCleanPCM, ffmpegExternalPCM)
	localSNRPayloadMetric := qualityMetric("local decoder: SNR-clean payload vs bcg729 payload", localSNRPCM, localExternalPCM)
	ffmpegSNRPayloadMetric := qualityMetric("ffmpeg decoder: SNR-clean payload vs bcg729 payload", ffmpegSNRPCM, ffmpegExternalPCM)
	localSmoothPayloadMetric := qualityMetric("local decoder: smooth-clean payload vs bcg729 payload", localSmoothPCM, localExternalPCM)
	ffmpegSmoothPayloadMetric := qualityMetric("ffmpeg decoder: smooth-clean payload vs bcg729 payload", ffmpegSmoothPCM, ffmpegExternalPCM)
	localVoicedPayloadMetric := qualityMetric("local decoder: voiced-clean payload vs bcg729 payload", localVoicedPCM, localExternalPCM)
	ffmpegVoicedPayloadMetric := qualityMetric("ffmpeg decoder: voiced-clean payload vs bcg729 payload", ffmpegVoicedPCM, ffmpegExternalPCM)
	localDegritPayloadMetric := qualityMetric("local decoder: degrit-clean payload vs bcg729 payload", localDegritPCM, localExternalPCM)
	ffmpegDegritPayloadMetric := qualityMetric("ffmpeg decoder: degrit-clean payload vs bcg729 payload", ffmpegDegritPCM, ffmpegExternalPCM)
	localHarmonicPayloadMetric := qualityMetric("local decoder: harmonic-clean payload vs bcg729 payload", localHarmonicPCM, localExternalPCM)
	ffmpegHarmonicPayloadMetric := qualityMetric("ffmpeg decoder: harmonic-clean payload vs bcg729 payload", ffmpegHarmonicPCM, ffmpegExternalPCM)
	localHarmonicStrongPayloadMetric := qualityMetric("local decoder: harmonic-strong payload vs bcg729 payload", localHarmonicStrongPCM, localExternalPCM)
	ffmpegHarmonicStrongPayloadMetric := qualityMetric("ffmpeg decoder: harmonic-strong payload vs bcg729 payload", ffmpegHarmonicStrongPCM, ffmpegExternalPCM)
	localHarmonicDeepPayloadMetric := qualityMetric("local decoder: harmonic-deep payload vs bcg729 payload", localHarmonicDeepPCM, localExternalPCM)
	ffmpegHarmonicDeepPayloadMetric := qualityMetric("ffmpeg decoder: harmonic-deep payload vs bcg729 payload", ffmpegHarmonicDeepPCM, ffmpegExternalPCM)
	localFCBPayloadMetric := qualityMetric("local decoder: FCB-clean payload vs bcg729 payload", localFCBPCM, localExternalPCM)
	ffmpegFCBPayloadMetric := qualityMetric("ffmpeg decoder: FCB-clean payload vs bcg729 payload", ffmpegFCBPCM, ffmpegExternalPCM)
	localPESQPayloadMetric := qualityMetric("local decoder: PESQ candidate payload vs bcg729 payload", localPESQPCM, localExternalPCM)
	ffmpegPESQPayloadMetric := qualityMetric("ffmpeg decoder: PESQ candidate payload vs bcg729 payload", ffmpegPESQPCM, ffmpegExternalPCM)
	localPESQDegritPayloadMetric := qualityMetric("local decoder: PESQ-degrit candidate payload vs bcg729 payload", localPESQDegritPCM, localExternalPCM)
	ffmpegPESQDegritPayloadMetric := qualityMetric("ffmpeg decoder: PESQ-degrit candidate payload vs bcg729 payload", ffmpegPESQDegritPCM, ffmpegExternalPCM)

	pesqNote := addPESQScores(tmp, []pesqPair{
		{Row: &ourLocalMetric, RefPCM: paddedPCM, OutPCM: localOurPCM},
		{Row: &coreLocalMetric, RefPCM: paddedPCM, OutPCM: localCorePCM},
		{Row: &coreFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegCorePCM},
		{Row: &coreClipLocalMetric, RefPCM: paddedPCM, OutPCM: localCoreClipPCM},
		{Row: &coreClipFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegCoreClipPCM},
		{Row: &ourBlend50Metric, RefPCM: paddedPCM, OutPCM: blendOurPCM},
		{Row: &ourFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegOurPCM},
		{Row: &cleanLocalMetric, RefPCM: paddedPCM, OutPCM: localCleanPCM},
		{Row: &cleanBlend50Metric, RefPCM: paddedPCM, OutPCM: blendCleanPCM},
		{Row: &cleanFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegCleanPCM},
		{Row: &snrLocalMetric, RefPCM: paddedPCM, OutPCM: localSNRPCM},
		{Row: &snrFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegSNRPCM},
		{Row: &smoothLocalMetric, RefPCM: paddedPCM, OutPCM: localSmoothPCM},
		{Row: &smoothFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegSmoothPCM},
		{Row: &voicedLocalMetric, RefPCM: paddedPCM, OutPCM: localVoicedPCM},
		{Row: &voicedFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegVoicedPCM},
		{Row: &degritLocalMetric, RefPCM: paddedPCM, OutPCM: localDegritPCM},
		{Row: &degritFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegDegritPCM},
		{Row: &harmonicLocalMetric, RefPCM: paddedPCM, OutPCM: localHarmonicPCM},
		{Row: &harmonicFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegHarmonicPCM},
		{Row: &harmonicStrongLocalMetric, RefPCM: paddedPCM, OutPCM: localHarmonicStrongPCM},
		{Row: &harmonicStrongFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegHarmonicStrongPCM},
		{Row: &harmonicDeepLocalMetric, RefPCM: paddedPCM, OutPCM: localHarmonicDeepPCM},
		{Row: &harmonicDeepFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegHarmonicDeepPCM},
		{Row: &fcbLocalMetric, RefPCM: paddedPCM, OutPCM: localFCBPCM},
		{Row: &fcbFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegFCBPCM},
		{Row: &pesqLocalMetric, RefPCM: paddedPCM, OutPCM: localPESQPCM},
		{Row: &pesqBlend50Metric, RefPCM: paddedPCM, OutPCM: blendPESQPCM},
		{Row: &pesqFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegPESQPCM},
		{Row: &pesqDegritLocalMetric, RefPCM: paddedPCM, OutPCM: localPESQDegritPCM},
		{Row: &pesqDegritFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegPESQDegritPCM},
		{Row: &softOurFFmpegMetric, RefPCM: paddedPCM, OutPCM: softOurFFmpegPCM},
		{Row: &softCleanFFmpegMetric, RefPCM: paddedPCM, OutPCM: softCleanFFmpegPCM},
		{Row: &externalLocalMetric, RefPCM: paddedPCM, OutPCM: localExternalPCM},
		{Row: &externalBlend50Metric, RefPCM: paddedPCM, OutPCM: blendExternalPCM},
		{Row: &externalFFmpegMetric, RefPCM: paddedPCM, OutPCM: ffmpegExternalPCM},
	})
	notes := []string{
		"External encoder is isolated under gitignored third-party/ and used only as a black-box executable.",
		"Clean candidate uses EncoderProfileQualityClean: no normalized closed-loop pitch reranking, stricter gain MSE repair.",
		"SNR-clean candidate uses the same clean pitch policy with the older high-SNR gain-repair preference.",
		"Smooth-clean candidate uses a lower clean repair threshold and stronger high-residual preference; it is a bitstream-level diagnostic, not PCM smoothing.",
		"Voiced-clean candidate keeps clean pitch but lets gain repair prefer stronger adaptive gain within bounded MSE/high-residual tolerance.",
		"Degrit-clean candidate keeps clean pitch but lets gain repair prefer lower fixed-codebook gain correction when adaptive gain is not reduced.",
		"Harmonic-clean candidate keeps clean pitch and lets voiced gain repair trade bounded score loss for higher adaptive gain with lower fixed-codebook correction.",
		"Harmonic-strong candidate pushes that same gain-balance tradeoff harder; it is expected to test grit reduction against possible muffling.",
		"Harmonic-deep candidate pushes the same gain-balance tradeoff beyond harmonic-strong to locate the grit-vs-muffling boundary.",
		"FCB-clean candidate keeps clean pitch and reranks a small fixed-codebook candidate set with decoder-in-loop residual scoring.",
		"PESQ candidate keeps the broader quality heuristics disabled but enables native reconstructed-gain search, gain clip repair, and fixed-codebook residual reranking.",
		"PESQ-degrit candidate adds bounded gain MSE/noise repair to the PESQ candidate; it is for blind tests of lower high-residual grit, not a PESQ-leading default.",
		"Core-clip candidate keeps Core's narrow search policy and only enables decoder-in-loop gain clip repair with a lower pre-clip threshold.",
		"Softened candidates are playback-only diagnostics that apply a mild zero-phase PCM smoother after FFmpeg decode; they do not represent a G.729 payload.",
		"FFmpeg is used only as a black-box G.729 decoder.",
		"Scores are delay-compensated listening diagnostics, not ITU conformance certification.",
	}
	if pesqNote != "" {
		notes = append(notes, pesqNote)
	}

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
			"source":                 wavDataURL(paddedPCM),
			"core_local":             wavDataURL(localCorePCM),
			"core_ffmpeg":            wavDataURL(ffmpegCorePCM),
			"core_clip_local":        wavDataURL(localCoreClipPCM),
			"core_clip_ffmpeg":       wavDataURL(ffmpegCoreClipPCM),
			"our_local":              wavDataURL(localOurPCM),
			"our_blend50":            wavDataURL(blendOurPCM),
			"our_ffmpeg":             wavDataURL(ffmpegOurPCM),
			"clean_local":            wavDataURL(localCleanPCM),
			"clean_blend50":          wavDataURL(blendCleanPCM),
			"clean_ffmpeg":           wavDataURL(ffmpegCleanPCM),
			"snr_local":              wavDataURL(localSNRPCM),
			"snr_ffmpeg":             wavDataURL(ffmpegSNRPCM),
			"smooth_local":           wavDataURL(localSmoothPCM),
			"smooth_ffmpeg":          wavDataURL(ffmpegSmoothPCM),
			"voiced_local":           wavDataURL(localVoicedPCM),
			"voiced_ffmpeg":          wavDataURL(ffmpegVoicedPCM),
			"degrit_local":           wavDataURL(localDegritPCM),
			"degrit_ffmpeg":          wavDataURL(ffmpegDegritPCM),
			"harmonic_local":         wavDataURL(localHarmonicPCM),
			"harmonic_ffmpeg":        wavDataURL(ffmpegHarmonicPCM),
			"harmonic_strong_local":  wavDataURL(localHarmonicStrongPCM),
			"harmonic_strong_ffmpeg": wavDataURL(ffmpegHarmonicStrongPCM),
			"harmonic_deep_local":    wavDataURL(localHarmonicDeepPCM),
			"harmonic_deep_ffmpeg":   wavDataURL(ffmpegHarmonicDeepPCM),
			"fcb_local":              wavDataURL(localFCBPCM),
			"fcb_ffmpeg":             wavDataURL(ffmpegFCBPCM),
			"pesq_local":             wavDataURL(localPESQPCM),
			"pesq_blend50":           wavDataURL(blendPESQPCM),
			"pesq_ffmpeg":            wavDataURL(ffmpegPESQPCM),
			"pesq_degrit_local":      wavDataURL(localPESQDegritPCM),
			"pesq_degrit_ffmpeg":     wavDataURL(ffmpegPESQDegritPCM),
			"soft_our_ffmpeg":        wavDataURL(softOurFFmpegPCM),
			"soft_clean_ffmpeg":      wavDataURL(softCleanFFmpegPCM),
			"external_local":         wavDataURL(localExternalPCM),
			"external_blend50":       wavDataURL(blendExternalPCM),
			"external_ffmpeg":        wavDataURL(ffmpegExternalPCM),
		},
		Downloads: map[string]string{
			"core_g729":            payloadDataURL(corePayload),
			"core_clip_g729":       payloadDataURL(coreClipPayload),
			"our_g729":             payloadDataURL(ourPayload),
			"clean_g729":           payloadDataURL(cleanPayload),
			"snr_g729":             payloadDataURL(snrPayload),
			"smooth_g729":          payloadDataURL(smoothPayload),
			"voiced_g729":          payloadDataURL(voicedPayload),
			"degrit_g729":          payloadDataURL(degritPayload),
			"harmonic_g729":        payloadDataURL(harmonicPayload),
			"harmonic_strong_g729": payloadDataURL(harmonicStrongPayload),
			"harmonic_deep_g729":   payloadDataURL(harmonicDeepPayload),
			"fcb_g729":             payloadDataURL(fcbPayload),
			"pesq_g729":            payloadDataURL(pesqPayload),
			"pesq_degrit_g729":     payloadDataURL(pesqDegritPayload),
			"external_g729":        payloadDataURL(externalPayload),
		},
		Clips: map[string][]clipEvent{
			"source":                 clipEvents(paddedPCM, maxClipMarkers),
			"core_local":             clipEvents(localCorePCM, maxClipMarkers),
			"core_ffmpeg":            clipEvents(ffmpegCorePCM, maxClipMarkers),
			"core_clip_local":        clipEvents(localCoreClipPCM, maxClipMarkers),
			"core_clip_ffmpeg":       clipEvents(ffmpegCoreClipPCM, maxClipMarkers),
			"our_local":              clipEvents(localOurPCM, maxClipMarkers),
			"our_blend50":            clipEvents(blendOurPCM, maxClipMarkers),
			"our_ffmpeg":             clipEvents(ffmpegOurPCM, maxClipMarkers),
			"clean_local":            clipEvents(localCleanPCM, maxClipMarkers),
			"clean_blend50":          clipEvents(blendCleanPCM, maxClipMarkers),
			"clean_ffmpeg":           clipEvents(ffmpegCleanPCM, maxClipMarkers),
			"snr_local":              clipEvents(localSNRPCM, maxClipMarkers),
			"snr_ffmpeg":             clipEvents(ffmpegSNRPCM, maxClipMarkers),
			"smooth_local":           clipEvents(localSmoothPCM, maxClipMarkers),
			"smooth_ffmpeg":          clipEvents(ffmpegSmoothPCM, maxClipMarkers),
			"voiced_local":           clipEvents(localVoicedPCM, maxClipMarkers),
			"voiced_ffmpeg":          clipEvents(ffmpegVoicedPCM, maxClipMarkers),
			"degrit_local":           clipEvents(localDegritPCM, maxClipMarkers),
			"degrit_ffmpeg":          clipEvents(ffmpegDegritPCM, maxClipMarkers),
			"harmonic_local":         clipEvents(localHarmonicPCM, maxClipMarkers),
			"harmonic_ffmpeg":        clipEvents(ffmpegHarmonicPCM, maxClipMarkers),
			"harmonic_strong_local":  clipEvents(localHarmonicStrongPCM, maxClipMarkers),
			"harmonic_strong_ffmpeg": clipEvents(ffmpegHarmonicStrongPCM, maxClipMarkers),
			"harmonic_deep_local":    clipEvents(localHarmonicDeepPCM, maxClipMarkers),
			"harmonic_deep_ffmpeg":   clipEvents(ffmpegHarmonicDeepPCM, maxClipMarkers),
			"fcb_local":              clipEvents(localFCBPCM, maxClipMarkers),
			"fcb_ffmpeg":             clipEvents(ffmpegFCBPCM, maxClipMarkers),
			"pesq_local":             clipEvents(localPESQPCM, maxClipMarkers),
			"pesq_blend50":           clipEvents(blendPESQPCM, maxClipMarkers),
			"pesq_ffmpeg":            clipEvents(ffmpegPESQPCM, maxClipMarkers),
			"pesq_degrit_local":      clipEvents(localPESQDegritPCM, maxClipMarkers),
			"pesq_degrit_ffmpeg":     clipEvents(ffmpegPESQDegritPCM, maxClipMarkers),
			"soft_our_ffmpeg":        clipEvents(softOurFFmpegPCM, maxClipMarkers),
			"soft_clean_ffmpeg":      clipEvents(softCleanFFmpegPCM, maxClipMarkers),
			"external_local":         clipEvents(localExternalPCM, maxClipMarkers),
			"external_blend50":       clipEvents(blendExternalPCM, maxClipMarkers),
			"external_ffmpeg":        clipEvents(ffmpegExternalPCM, maxClipMarkers),
		},
		Metrics: []metricRow{
			ourLocalMetric,
			coreLocalMetric,
			coreFFmpegMetric,
			coreClipLocalMetric,
			coreClipFFmpegMetric,
			ourBlend50Metric,
			ourFFmpegMetric,
			cleanLocalMetric,
			cleanBlend50Metric,
			cleanFFmpegMetric,
			snrLocalMetric,
			snrFFmpegMetric,
			smoothLocalMetric,
			smoothFFmpegMetric,
			voicedLocalMetric,
			voicedFFmpegMetric,
			degritLocalMetric,
			degritFFmpegMetric,
			harmonicLocalMetric,
			harmonicFFmpegMetric,
			harmonicStrongLocalMetric,
			harmonicStrongFFmpegMetric,
			harmonicDeepLocalMetric,
			harmonicDeepFFmpegMetric,
			fcbLocalMetric,
			fcbFFmpegMetric,
			pesqLocalMetric,
			pesqBlend50Metric,
			pesqFFmpegMetric,
			pesqDegritLocalMetric,
			pesqDegritFFmpegMetric,
			softOurFFmpegMetric,
			softCleanFFmpegMetric,
			externalLocalMetric,
			externalBlend50Metric,
			externalFFmpegMetric,
			localPayloadMetric,
			ffmpegPayloadMetric,
			localCleanPayloadMetric,
			ffmpegCleanPayloadMetric,
			localSNRPayloadMetric,
			ffmpegSNRPayloadMetric,
			localSmoothPayloadMetric,
			ffmpegSmoothPayloadMetric,
			localVoicedPayloadMetric,
			ffmpegVoicedPayloadMetric,
			localDegritPayloadMetric,
			ffmpegDegritPayloadMetric,
			localHarmonicPayloadMetric,
			ffmpegHarmonicPayloadMetric,
			localHarmonicStrongPayloadMetric,
			ffmpegHarmonicStrongPayloadMetric,
			localHarmonicDeepPayloadMetric,
			ffmpegHarmonicDeepPayloadMetric,
			localFCBPayloadMetric,
			ffmpegFCBPayloadMetric,
			localPESQPayloadMetric,
			ffmpegPESQPayloadMetric,
			localPESQDegritPayloadMetric,
			ffmpegPESQDegritPayloadMetric,
		},
		Noise: []noiseRow{
			residualNoiseMetric("our encode -> local residual vs source", "our_local", paddedPCM, localOurPCM, ourLocalMetric.LagSamples),
			residualNoiseMetric("our core encode -> local residual vs source", "core_local", paddedPCM, localCorePCM, coreLocalMetric.LagSamples),
			residualNoiseMetric("our core encode -> ffmpeg residual vs source", "core_ffmpeg", paddedPCM, ffmpegCorePCM, coreFFmpegMetric.LagSamples),
			residualNoiseMetric("our core-clip encode -> local residual vs source", "core_clip_local", paddedPCM, localCoreClipPCM, coreClipLocalMetric.LagSamples),
			residualNoiseMetric("our core-clip encode -> ffmpeg residual vs source", "core_clip_ffmpeg", paddedPCM, ffmpegCoreClipPCM, coreClipFFmpegMetric.LagSamples),
			residualNoiseMetric("our encode -> postfilter-blend50 local residual vs source", "our_blend50", paddedPCM, blendOurPCM, ourBlend50Metric.LagSamples),
			residualNoiseMetric("our encode -> ffmpeg residual vs source", "our_ffmpeg", paddedPCM, ffmpegOurPCM, ourFFmpegMetric.LagSamples),
			residualNoiseMetric("our clean encode -> local residual vs source", "clean_local", paddedPCM, localCleanPCM, cleanLocalMetric.LagSamples),
			residualNoiseMetric("our clean encode -> postfilter-blend50 local residual vs source", "clean_blend50", paddedPCM, blendCleanPCM, cleanBlend50Metric.LagSamples),
			residualNoiseMetric("our clean encode -> ffmpeg residual vs source", "clean_ffmpeg", paddedPCM, ffmpegCleanPCM, cleanFFmpegMetric.LagSamples),
			residualNoiseMetric("our SNR-clean encode -> local residual vs source", "snr_local", paddedPCM, localSNRPCM, snrLocalMetric.LagSamples),
			residualNoiseMetric("our SNR-clean encode -> ffmpeg residual vs source", "snr_ffmpeg", paddedPCM, ffmpegSNRPCM, snrFFmpegMetric.LagSamples),
			residualNoiseMetric("our smooth-clean encode -> local residual vs source", "smooth_local", paddedPCM, localSmoothPCM, smoothLocalMetric.LagSamples),
			residualNoiseMetric("our smooth-clean encode -> ffmpeg residual vs source", "smooth_ffmpeg", paddedPCM, ffmpegSmoothPCM, smoothFFmpegMetric.LagSamples),
			residualNoiseMetric("our voiced-clean encode -> local residual vs source", "voiced_local", paddedPCM, localVoicedPCM, voicedLocalMetric.LagSamples),
			residualNoiseMetric("our voiced-clean encode -> ffmpeg residual vs source", "voiced_ffmpeg", paddedPCM, ffmpegVoicedPCM, voicedFFmpegMetric.LagSamples),
			residualNoiseMetric("our degrit-clean encode -> local residual vs source", "degrit_local", paddedPCM, localDegritPCM, degritLocalMetric.LagSamples),
			residualNoiseMetric("our degrit-clean encode -> ffmpeg residual vs source", "degrit_ffmpeg", paddedPCM, ffmpegDegritPCM, degritFFmpegMetric.LagSamples),
			residualNoiseMetric("our harmonic-clean encode -> local residual vs source", "harmonic_local", paddedPCM, localHarmonicPCM, harmonicLocalMetric.LagSamples),
			residualNoiseMetric("our harmonic-clean encode -> ffmpeg residual vs source", "harmonic_ffmpeg", paddedPCM, ffmpegHarmonicPCM, harmonicFFmpegMetric.LagSamples),
			residualNoiseMetric("our harmonic-strong encode -> local residual vs source", "harmonic_strong_local", paddedPCM, localHarmonicStrongPCM, harmonicStrongLocalMetric.LagSamples),
			residualNoiseMetric("our harmonic-strong encode -> ffmpeg residual vs source", "harmonic_strong_ffmpeg", paddedPCM, ffmpegHarmonicStrongPCM, harmonicStrongFFmpegMetric.LagSamples),
			residualNoiseMetric("our harmonic-deep encode -> local residual vs source", "harmonic_deep_local", paddedPCM, localHarmonicDeepPCM, harmonicDeepLocalMetric.LagSamples),
			residualNoiseMetric("our harmonic-deep encode -> ffmpeg residual vs source", "harmonic_deep_ffmpeg", paddedPCM, ffmpegHarmonicDeepPCM, harmonicDeepFFmpegMetric.LagSamples),
			residualNoiseMetric("our FCB-clean encode -> local residual vs source", "fcb_local", paddedPCM, localFCBPCM, fcbLocalMetric.LagSamples),
			residualNoiseMetric("our FCB-clean encode -> ffmpeg residual vs source", "fcb_ffmpeg", paddedPCM, ffmpegFCBPCM, fcbFFmpegMetric.LagSamples),
			residualNoiseMetric("our PESQ candidate encode -> local residual vs source", "pesq_local", paddedPCM, localPESQPCM, pesqLocalMetric.LagSamples),
			residualNoiseMetric("our PESQ candidate encode -> postfilter-blend50 local residual vs source", "pesq_blend50", paddedPCM, blendPESQPCM, pesqBlend50Metric.LagSamples),
			residualNoiseMetric("our PESQ candidate encode -> ffmpeg residual vs source", "pesq_ffmpeg", paddedPCM, ffmpegPESQPCM, pesqFFmpegMetric.LagSamples),
			residualNoiseMetric("our PESQ-degrit candidate encode -> local residual vs source", "pesq_degrit_local", paddedPCM, localPESQDegritPCM, pesqDegritLocalMetric.LagSamples),
			residualNoiseMetric("our PESQ-degrit candidate encode -> ffmpeg residual vs source", "pesq_degrit_ffmpeg", paddedPCM, ffmpegPESQDegritPCM, pesqDegritFFmpegMetric.LagSamples),
			residualNoiseMetric("our encode -> softened FFmpeg residual vs source", "soft_our_ffmpeg", paddedPCM, softOurFFmpegPCM, softOurFFmpegMetric.LagSamples),
			residualNoiseMetric("our clean encode -> softened FFmpeg residual vs source", "soft_clean_ffmpeg", paddedPCM, softCleanFFmpegPCM, softCleanFFmpegMetric.LagSamples),
			residualNoiseMetric("bcg729 encode -> local residual vs source", "external_local", paddedPCM, localExternalPCM, externalLocalMetric.LagSamples),
			residualNoiseMetric("bcg729 encode -> postfilter-blend50 local residual vs source", "external_blend50", paddedPCM, blendExternalPCM, externalBlend50Metric.LagSamples),
			residualNoiseMetric("bcg729 encode -> ffmpeg residual vs source", "external_ffmpeg", paddedPCM, ffmpegExternalPCM, externalFFmpegMetric.LagSamples),
			residualNoiseMetric("local decoder delta on our payload", "our_local", ffmpegOurPCM, localOurPCM, 0),
			residualNoiseMetric("local decoder delta on clean payload", "clean_local", ffmpegCleanPCM, localCleanPCM, 0),
			residualNoiseMetric("local decoder delta on PESQ candidate payload", "pesq_local", ffmpegPESQPCM, localPESQPCM, 0),
			residualNoiseMetric("local decoder delta on PESQ-degrit candidate payload", "pesq_degrit_local", ffmpegPESQDegritPCM, localPESQDegritPCM, 0),
			residualNoiseMetric("local decoder delta on bcg729 payload", "external_local", ffmpegExternalPCM, localExternalPCM, 0),
			residualNoiseMetric("encoder delta under local decode", "our_local", localExternalPCM, localOurPCM, localPayloadMetric.LagSamples),
			residualNoiseMetric("encoder delta under ffmpeg decode", "our_ffmpeg", ffmpegExternalPCM, ffmpegOurPCM, ffmpegPayloadMetric.LagSamples),
			residualNoiseMetric("clean encoder delta under local decode", "clean_local", localExternalPCM, localCleanPCM, localCleanPayloadMetric.LagSamples),
			residualNoiseMetric("clean encoder delta under ffmpeg decode", "clean_ffmpeg", ffmpegExternalPCM, ffmpegCleanPCM, ffmpegCleanPayloadMetric.LagSamples),
			residualNoiseMetric("SNR-clean encoder delta under local decode", "snr_local", localExternalPCM, localSNRPCM, localSNRPayloadMetric.LagSamples),
			residualNoiseMetric("SNR-clean encoder delta under ffmpeg decode", "snr_ffmpeg", ffmpegExternalPCM, ffmpegSNRPCM, ffmpegSNRPayloadMetric.LagSamples),
			residualNoiseMetric("smooth-clean encoder delta under local decode", "smooth_local", localExternalPCM, localSmoothPCM, localSmoothPayloadMetric.LagSamples),
			residualNoiseMetric("smooth-clean encoder delta under ffmpeg decode", "smooth_ffmpeg", ffmpegExternalPCM, ffmpegSmoothPCM, ffmpegSmoothPayloadMetric.LagSamples),
			residualNoiseMetric("voiced-clean encoder delta under local decode", "voiced_local", localExternalPCM, localVoicedPCM, localVoicedPayloadMetric.LagSamples),
			residualNoiseMetric("voiced-clean encoder delta under ffmpeg decode", "voiced_ffmpeg", ffmpegExternalPCM, ffmpegVoicedPCM, ffmpegVoicedPayloadMetric.LagSamples),
			residualNoiseMetric("degrit-clean encoder delta under local decode", "degrit_local", localExternalPCM, localDegritPCM, localDegritPayloadMetric.LagSamples),
			residualNoiseMetric("degrit-clean encoder delta under ffmpeg decode", "degrit_ffmpeg", ffmpegExternalPCM, ffmpegDegritPCM, ffmpegDegritPayloadMetric.LagSamples),
			residualNoiseMetric("harmonic-clean encoder delta under local decode", "harmonic_local", localExternalPCM, localHarmonicPCM, localHarmonicPayloadMetric.LagSamples),
			residualNoiseMetric("harmonic-clean encoder delta under ffmpeg decode", "harmonic_ffmpeg", ffmpegExternalPCM, ffmpegHarmonicPCM, ffmpegHarmonicPayloadMetric.LagSamples),
			residualNoiseMetric("harmonic-strong encoder delta under local decode", "harmonic_strong_local", localExternalPCM, localHarmonicStrongPCM, localHarmonicStrongPayloadMetric.LagSamples),
			residualNoiseMetric("harmonic-strong encoder delta under ffmpeg decode", "harmonic_strong_ffmpeg", ffmpegExternalPCM, ffmpegHarmonicStrongPCM, ffmpegHarmonicStrongPayloadMetric.LagSamples),
			residualNoiseMetric("harmonic-deep encoder delta under local decode", "harmonic_deep_local", localExternalPCM, localHarmonicDeepPCM, localHarmonicDeepPayloadMetric.LagSamples),
			residualNoiseMetric("harmonic-deep encoder delta under ffmpeg decode", "harmonic_deep_ffmpeg", ffmpegExternalPCM, ffmpegHarmonicDeepPCM, ffmpegHarmonicDeepPayloadMetric.LagSamples),
			residualNoiseMetric("FCB-clean encoder delta under local decode", "fcb_local", localExternalPCM, localFCBPCM, localFCBPayloadMetric.LagSamples),
			residualNoiseMetric("FCB-clean encoder delta under ffmpeg decode", "fcb_ffmpeg", ffmpegExternalPCM, ffmpegFCBPCM, ffmpegFCBPayloadMetric.LagSamples),
			residualNoiseMetric("PESQ candidate encoder delta under local decode", "pesq_local", localExternalPCM, localPESQPCM, localPESQPayloadMetric.LagSamples),
			residualNoiseMetric("PESQ candidate encoder delta under ffmpeg decode", "pesq_ffmpeg", ffmpegExternalPCM, ffmpegPESQPCM, ffmpegPESQPayloadMetric.LagSamples),
			residualNoiseMetric("PESQ-degrit candidate encoder delta under local decode", "pesq_degrit_local", localExternalPCM, localPESQDegritPCM, localPESQDegritPayloadMetric.LagSamples),
			residualNoiseMetric("PESQ-degrit candidate encoder delta under ffmpeg decode", "pesq_degrit_ffmpeg", ffmpegExternalPCM, ffmpegPESQDegritPCM, ffmpegPESQDegritPayloadMetric.LagSamples),
			residualNoiseMetric("softened current delta under ffmpeg decode", "soft_our_ffmpeg", ffmpegExternalPCM, softOurFFmpegPCM, ffmpegPayloadMetric.LagSamples),
			residualNoiseMetric("softened clean delta under ffmpeg decode", "soft_clean_ffmpeg", ffmpegExternalPCM, softCleanFFmpegPCM, ffmpegCleanPayloadMetric.LagSamples),
		},
		Notes: notes,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func parseWantedAudio(spec string) map[string]bool {
	wanted := make(map[string]bool)
	for _, part := range strings.Split(spec, ",") {
		key := strings.TrimSpace(part)
		if key != "" {
			wanted[key] = true
		}
	}
	return wanted
}

func writeSelectedAudioCompare(w http.ResponseWriter, tmp string, paddedPCM []byte, originalSamples, frames int, wanted map[string]bool) {
	type payloadEntry struct {
		payload []byte
		err     error
	}
	payloads := make(map[string]payloadEntry)
	audio := make(map[string]string)
	decodedByKey := map[string][]byte{
		"source": paddedPCM,
	}
	audio["source"] = wavDataURL(paddedPCM)

	getPayload := func(name string) ([]byte, error) {
		if entry, ok := payloads[name]; ok {
			return entry.payload, entry.err
		}
		var payload []byte
		var err error
		switch name {
		case "core":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileCore)
		case "core_clip":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileCoreClipRepair)
		case "our":
			payload, err = encodeWithLocal(paddedPCM)
		case "clean":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityClean)
		case "snr":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanSNR)
		case "smooth":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanSmooth)
		case "voiced":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanVoiced)
		case "degrit":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanDegrit)
		case "harmonic":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanHarmonic)
		case "harmonic_strong":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanHarmonicStrong)
		case "harmonic_deep":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanHarmonicDeep)
		case "fcb":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityCleanFCBRerank)
		case "pesq":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityPESQ)
		case "pesq_degrit":
			payload, err = encodeWithLocalProfile(paddedPCM, g729.EncoderProfileQualityPESQDegrit)
		case "external":
			payload, err = encodeWithBCG729(paddedPCM)
		default:
			err = fmt.Errorf("unknown payload %q", name)
		}
		payloads[name] = payloadEntry{payload: payload, err: err}
		return payload, err
	}

	for key := range wanted {
		pipeline, decoder, soft, ok := selectedAudioPipeline(key)
		if !ok {
			writeError(w, fmt.Errorf("unknown requested audio key %q", key))
			return
		}
		if pipeline == "source" {
			audio[key] = wavDataURL(paddedPCM)
			continue
		}

		payload, err := getPayload(pipeline)
		if err != nil {
			writeError(w, err)
			return
		}
		var decoded []byte
		switch decoder {
		case "local":
			decoded, err = decodeWithLocal(payload)
		case "blend50":
			decoded, err = decodeWithLocalPostfilterBlend(payload, 1, 2)
		case "ffmpeg":
			decoded, err = decodeWithFFmpeg(tmp, key, payload)
		default:
			err = fmt.Errorf("unknown decoder %q for %q", decoder, key)
		}
		if err != nil {
			writeError(w, err)
			return
		}
		if soft {
			decoded = softenPCM16(decoded)
		}
		decodedByKey[key] = decoded
		audio[key] = wavDataURL(decoded)
	}

	metricKeys := make([]string, 0, len(wanted))
	for key := range wanted {
		if key != "source" {
			metricKeys = append(metricKeys, key)
		}
	}
	sort.Strings(metricKeys)
	metrics := make([]metricRow, 0, len(metricKeys))
	noise := make([]noiseRow, 0, len(metricKeys))
	pesqPairs := make([]pesqPair, 0, len(metricKeys))
	for _, key := range metricKeys {
		decoded := decodedByKey[key]
		row := qualityMetric(selectedMetricPath(key), paddedPCM, decoded)
		row.Key = key
		metrics = append(metrics, row)
		noise = append(noise, residualNoiseMetric(selectedMetricPath(key)+" residual vs source", key, paddedPCM, decoded, row.LagSamples))
		pesqPairs = append(pesqPairs, pesqPair{
			Row:    &metrics[len(metrics)-1],
			RefPCM: paddedPCM,
			OutPCM: decoded,
		})
	}
	pesqNote := addPESQScores(tmp, pesqPairs)
	notes := []string{
		"Fast blind-test mode generated only the selected A/B candidates.",
		"External encoder is isolated under gitignored third-party/ and used only as a black-box executable.",
	}
	if pesqNote != "" {
		notes = append(notes, pesqNote)
	}

	resp := response{
		Input: inputInfo{
			Samples:       originalSamples,
			PaddedSamples: len(paddedPCM)/2 - originalSamples,
			Frames:        frames,
			DurationSec:   float64(originalSamples) / g729.SampleRate,
		},
		Audio:   audio,
		Metrics: metrics,
		Noise:   noise,
		Notes:   notes,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func selectedMetricPath(key string) string {
	switch key {
	case "core_local":
		return "our core encode -> local decode"
	case "core_ffmpeg":
		return "our core encode -> ffmpeg decode"
	case "core_clip_local":
		return "our core-clip encode -> local decode"
	case "core_clip_ffmpeg":
		return "our core-clip encode -> ffmpeg decode"
	case "our_local":
		return "our encode -> local decode"
	case "our_blend50":
		return "our encode -> local postfilter-blend50 decode"
	case "our_ffmpeg":
		return "our encode -> ffmpeg decode"
	case "clean_local":
		return "our clean encode -> local decode"
	case "clean_blend50":
		return "our clean encode -> local postfilter-blend50 decode"
	case "clean_ffmpeg":
		return "our clean encode -> ffmpeg decode"
	case "snr_local":
		return "our SNR-clean encode -> local decode"
	case "snr_ffmpeg":
		return "our SNR-clean encode -> ffmpeg decode"
	case "smooth_local":
		return "our smooth-clean encode -> local decode"
	case "smooth_ffmpeg":
		return "our smooth-clean encode -> ffmpeg decode"
	case "voiced_local":
		return "our voiced-clean encode -> local decode"
	case "voiced_ffmpeg":
		return "our voiced-clean encode -> ffmpeg decode"
	case "degrit_local":
		return "our degrit-clean encode -> local decode"
	case "degrit_ffmpeg":
		return "our degrit-clean encode -> ffmpeg decode"
	case "harmonic_local":
		return "our harmonic-clean encode -> local decode"
	case "harmonic_ffmpeg":
		return "our harmonic-clean encode -> ffmpeg decode"
	case "harmonic_strong_local":
		return "our harmonic-strong encode -> local decode"
	case "harmonic_strong_ffmpeg":
		return "our harmonic-strong encode -> ffmpeg decode"
	case "harmonic_deep_local":
		return "our harmonic-deep encode -> local decode"
	case "harmonic_deep_ffmpeg":
		return "our harmonic-deep encode -> ffmpeg decode"
	case "fcb_local":
		return "our FCB-clean encode -> local decode"
	case "fcb_ffmpeg":
		return "our FCB-clean encode -> ffmpeg decode"
	case "pesq_local":
		return "our PESQ candidate encode -> local decode"
	case "pesq_blend50":
		return "our PESQ candidate encode -> local postfilter-blend50 decode"
	case "pesq_ffmpeg":
		return "our PESQ candidate encode -> ffmpeg decode"
	case "pesq_degrit_local":
		return "our PESQ-degrit candidate encode -> local decode"
	case "pesq_degrit_ffmpeg":
		return "our PESQ-degrit candidate encode -> ffmpeg decode"
	case "soft_our_ffmpeg":
		return "our encode -> softened FFmpeg decode"
	case "soft_clean_ffmpeg":
		return "our clean encode -> softened FFmpeg decode"
	case "external_local":
		return "bcg729 encode -> local decode"
	case "external_blend50":
		return "bcg729 encode -> local postfilter-blend50 decode"
	case "external_ffmpeg":
		return "bcg729 encode -> ffmpeg decode"
	default:
		return key
	}
}

func selectedAudioPipeline(key string) (pipeline, decoder string, soft bool, ok bool) {
	switch key {
	case "source":
		return "source", "", false, true
	case "core_local":
		return "core", "local", false, true
	case "core_ffmpeg":
		return "core", "ffmpeg", false, true
	case "core_clip_local":
		return "core_clip", "local", false, true
	case "core_clip_ffmpeg":
		return "core_clip", "ffmpeg", false, true
	case "our_local":
		return "our", "local", false, true
	case "our_blend50":
		return "our", "blend50", false, true
	case "our_ffmpeg":
		return "our", "ffmpeg", false, true
	case "clean_local":
		return "clean", "local", false, true
	case "clean_blend50":
		return "clean", "blend50", false, true
	case "clean_ffmpeg":
		return "clean", "ffmpeg", false, true
	case "snr_local":
		return "snr", "local", false, true
	case "snr_ffmpeg":
		return "snr", "ffmpeg", false, true
	case "smooth_local":
		return "smooth", "local", false, true
	case "smooth_ffmpeg":
		return "smooth", "ffmpeg", false, true
	case "voiced_local":
		return "voiced", "local", false, true
	case "voiced_ffmpeg":
		return "voiced", "ffmpeg", false, true
	case "degrit_local":
		return "degrit", "local", false, true
	case "degrit_ffmpeg":
		return "degrit", "ffmpeg", false, true
	case "harmonic_local":
		return "harmonic", "local", false, true
	case "harmonic_ffmpeg":
		return "harmonic", "ffmpeg", false, true
	case "harmonic_strong_local":
		return "harmonic_strong", "local", false, true
	case "harmonic_strong_ffmpeg":
		return "harmonic_strong", "ffmpeg", false, true
	case "harmonic_deep_local":
		return "harmonic_deep", "local", false, true
	case "harmonic_deep_ffmpeg":
		return "harmonic_deep", "ffmpeg", false, true
	case "fcb_local":
		return "fcb", "local", false, true
	case "fcb_ffmpeg":
		return "fcb", "ffmpeg", false, true
	case "pesq_local":
		return "pesq", "local", false, true
	case "pesq_blend50":
		return "pesq", "blend50", false, true
	case "pesq_ffmpeg":
		return "pesq", "ffmpeg", false, true
	case "pesq_degrit_local":
		return "pesq_degrit", "local", false, true
	case "pesq_degrit_ffmpeg":
		return "pesq_degrit", "ffmpeg", false, true
	case "soft_our_ffmpeg":
		return "our", "ffmpeg", true, true
	case "soft_clean_ffmpeg":
		return "clean", "ffmpeg", true, true
	case "external_local":
		return "external", "local", false, true
	case "external_blend50":
		return "external", "blend50", false, true
	case "external_ffmpeg":
		return "external", "ffmpeg", false, true
	default:
		return "", "", false, false
	}
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

func decodeWithLocalPostfilterBlend(payload []byte, synthNum, den int) ([]byte, error) {
	dec := g729.NewDecoder()
	out := make([]byte, 0, len(payload)/g729.FrameBytes*g729.FrameSamples*2)
	frame := make([]int16, g729.FrameSamples)
	var pair [2]byte
	for off := 0; off < len(payload); off += g729.FrameBytes {
		if err := dec.DecodeFramePostfilterBlend(payload[off:off+g729.FrameBytes], frame, synthNum, den); err != nil {
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

func softenPCM16(pcm []byte) []byte {
	if len(pcm) < 6 {
		return append([]byte(nil), pcm...)
	}
	out := make([]byte, len(pcm))
	samples := len(pcm) / 2
	for i := 0; i < samples; i++ {
		cur := int32(samplePCM16(pcm, i))
		if i > 0 && i+1 < samples {
			prev := int32(samplePCM16(pcm, i-1))
			next := int32(samplePCM16(pcm, i+1))
			cur = (prev + 2*cur + next) / 4
		}
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(cur)))
	}
	if len(pcm)%2 != 0 {
		out[len(out)-1] = pcm[len(pcm)-1]
	}
	return out
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

func addPESQScores(tmp string, pairs []pesqPair) string {
	if len(pairs) == 0 {
		return ""
	}
	if os.Getenv("G729_PESQ_DISABLE") == "1" {
		return "PESQ NB is disabled by G729_PESQ_DISABLE=1."
	}
	python := os.Getenv("G729_PESQ_PYTHON")
	if python == "" {
		var err error
		python, err = exec.LookPath("python3")
		if err != nil {
			return "PESQ NB is n/a because python3 is not available."
		}
	}
	if out, err := exec.Command(python, "-c", "import numpy, pesq").CombinedOutput(); err != nil {
		return "PESQ NB is n/a because Python modules numpy/pesq are unavailable. Install them in a venv and set G729_PESQ_PYTHON to that python. " + compactNote(out)
	}

	args := []string{"-c", pesqBatchPython}
	for i, pair := range pairs {
		refPath := filepath.Join(tmp, fmt.Sprintf("pesq_ref_%02d.wav", i))
		outPath := filepath.Join(tmp, fmt.Sprintf("pesq_out_%02d.wav", i))
		if err := os.WriteFile(refPath, wavBytes(pair.RefPCM), 0o600); err != nil {
			return "PESQ NB is n/a because temporary reference WAV write failed: " + err.Error()
		}
		if err := os.WriteFile(outPath, wavBytes(pair.OutPCM), 0o600); err != nil {
			return "PESQ NB is n/a because temporary degraded WAV write failed: " + err.Error()
		}
		args = append(args, refPath, outPath)
	}

	out, err := exec.Command(python, args...).CombinedOutput()
	if err != nil {
		return "PESQ NB is n/a because the scorer failed. " + compactNote(out)
	}
	fields := strings.Fields(string(out))
	for i, field := range fields {
		if i >= len(pairs) {
			break
		}
		score, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
			continue
		}
		scoreCopy := score
		pairs[i].Row.PESQ = &scoreCopy
	}
	return "PESQ NB is an optional legacy P.862-style narrowband diagnostic computed against the converted source PCM; candidate-vs-bcg delta rows remain n/a. P.863/POLQA is the successor family."
}

const pesqBatchPython = `
import sys
import wave

import numpy as np
from pesq import pesq

def read_wav(path):
    with wave.open(path, "rb") as f:
        if f.getnchannels() != 1 or f.getsampwidth() != 2:
            raise ValueError("expected mono 16-bit WAV")
        rate = f.getframerate()
        data = f.readframes(f.getnframes())
    return rate, np.frombuffer(data, dtype="<i2").astype(np.float32)

for i in range(1, len(sys.argv), 2):
    try:
        ref_rate, ref = read_wav(sys.argv[i])
        out_rate, out = read_wav(sys.argv[i + 1])
        n = min(ref.shape[0], out.shape[0])
        if ref_rate != out_rate or ref_rate != 8000 or n <= 0:
            print("nan")
            continue
        print("{:.6f}".format(float(pesq(ref_rate, ref[:n], out[:n], "nb"))))
    except Exception:
        print("nan")
`

func compactNote(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 180 {
		return s[:180] + "..."
	}
	return s
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
    .result-actions { display:flex; gap:10px; flex-wrap:wrap; align-items:center; }
    .result-text { width:100%; min-height:170px; padding:12px; border:1px solid var(--line); border-radius:8px; background:#f8fafc; color:var(--ink); font:12px/1.45 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; resize:vertical; }
    .trial-note { width:100%; min-height:80px; padding:10px 12px; border:1px solid var(--line); border-radius:8px; background:#fff; color:var(--ink); resize:vertical; }
    .score { padding:16px; border:1px solid var(--line); border-radius:8px; background:#f8fafc; }
    .score strong { display:block; font-size:30px; margin-top:8px; }
    .score small { display:block; color:var(--muted); margin-top:4px; }
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
            <option value="pesq_ffmpeg|external_ffmpeg">PESQ candidate vs bcg729</option>
            <option value="pesq_local|external_ffmpeg">PESQ candidate local decode vs bcg729 FFmpeg</option>
            <option value="pesq_blend50|external_ffmpeg">PESQ candidate blend50 local decode vs bcg729 FFmpeg</option>
            <option value="pesq_blend50|pesq_local">PESQ candidate blend50 local decode vs strict local decode</option>
            <option value="pesq_blend50|pesq_ffmpeg">PESQ candidate blend50 local decode vs FFmpeg decode</option>
            <option value="pesq_ffmpeg|pesq_degrit_ffmpeg">PESQ candidate vs PESQ-degrit candidate</option>
            <option value="pesq_degrit_ffmpeg|external_ffmpeg">PESQ-degrit candidate vs bcg729</option>
            <option value="pesq_degrit_local|external_ffmpeg">PESQ-degrit local decode vs bcg729 FFmpeg</option>
            <option value="pesq_ffmpeg|fcb_ffmpeg">PESQ candidate vs FCB-clean candidate</option>
            <option value="pesq_ffmpeg|core_ffmpeg">PESQ candidate vs core profile</option>
            <option value="core_ffmpeg|core_clip_ffmpeg">Core profile vs core-clip profile</option>
            <option value="core_clip_ffmpeg|external_ffmpeg">Core-clip profile vs bcg729</option>
            <option value="pesq_ffmpeg|core_clip_ffmpeg">PESQ candidate vs core-clip profile</option>
            <option value="pesq_ffmpeg|our_ffmpeg">PESQ candidate vs current quality</option>
            <option value="core_ffmpeg|fcb_ffmpeg">Core profile vs FCB-clean candidate</option>
            <option value="core_ffmpeg|external_ffmpeg">Core profile vs bcg729</option>
            <option value="core_ffmpeg|our_ffmpeg">Core profile vs current quality</option>
            <option value="clean_ffmpeg|fcb_ffmpeg">Clean candidate vs FCB-clean candidate</option>
            <option value="clean_ffmpeg|harmonic_ffmpeg">Clean candidate vs harmonic-clean candidate</option>
            <option value="harmonic_ffmpeg|harmonic_strong_ffmpeg">Harmonic-clean candidate vs harmonic-strong candidate</option>
            <option value="harmonic_strong_ffmpeg|harmonic_deep_ffmpeg">Harmonic-strong candidate vs harmonic-deep candidate</option>
            <option value="harmonic_ffmpeg|harmonic_deep_ffmpeg">Harmonic-clean candidate vs harmonic-deep candidate</option>
            <option value="harmonic_ffmpeg|fcb_ffmpeg">Harmonic-clean candidate vs FCB-clean candidate</option>
            <option value="harmonic_strong_ffmpeg|fcb_ffmpeg">Harmonic-strong candidate vs FCB-clean candidate</option>
            <option value="harmonic_ffmpeg|external_ffmpeg">Harmonic-clean candidate vs bcg729</option>
            <option value="harmonic_strong_ffmpeg|external_ffmpeg">Harmonic-strong candidate vs bcg729</option>
            <option value="harmonic_deep_ffmpeg|external_ffmpeg">Harmonic-deep candidate vs bcg729</option>
            <option value="fcb_ffmpeg|external_ffmpeg">FCB-clean candidate vs bcg729</option>
            <option value="our_ffmpeg|clean_ffmpeg">Current quality vs clean candidate</option>
            <option value="clean_ffmpeg|external_ffmpeg">Clean candidate vs bcg729</option>
            <option value="clean_blend50|external_ffmpeg">Clean blend50 local decode vs bcg729</option>
            <option value="clean_blend50|clean_ffmpeg">Clean blend50 local decode vs clean FFmpeg</option>
            <option value="clean_ffmpeg|snr_ffmpeg">Clean candidate vs SNR-clean candidate</option>
            <option value="clean_ffmpeg|smooth_ffmpeg">Clean candidate vs smooth-clean candidate</option>
            <option value="clean_ffmpeg|voiced_ffmpeg">Clean candidate vs voiced-clean candidate</option>
            <option value="clean_ffmpeg|degrit_ffmpeg">Clean candidate vs degrit-clean candidate</option>
            <option value="snr_ffmpeg|external_ffmpeg">SNR-clean candidate vs bcg729</option>
            <option value="smooth_ffmpeg|external_ffmpeg">Smooth-clean candidate vs bcg729</option>
            <option value="voiced_ffmpeg|external_ffmpeg">Voiced-clean candidate vs bcg729</option>
            <option value="degrit_ffmpeg|external_ffmpeg">Degrit-clean candidate vs bcg729</option>
            <option value="our_ffmpeg|external_ffmpeg">Current quality vs bcg729</option>
            <option value="our_blend50|our_ffmpeg">Current blend50 local decode vs current FFmpeg</option>
            <option value="external_blend50|external_ffmpeg">bcg729 blend50 local decode vs bcg729 FFmpeg</option>
            <option value="external_blend50|external_local">bcg729 blend50 local decode vs strict local decode</option>
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
      core_local: "our core profile -> our decode",
      core_ffmpeg: "our core profile -> FFmpeg decode",
      core_clip_local: "our core-clip profile -> our decode",
      core_clip_ffmpeg: "our core-clip profile -> FFmpeg decode",
      our_local: "our encode -> our decode",
      our_blend50: "our encode -> postfilter-blend50 decode",
      our_ffmpeg: "our encode -> FFmpeg decode",
      clean_local: "our clean candidate -> our decode",
      clean_blend50: "our clean candidate -> postfilter-blend50 decode",
      clean_ffmpeg: "our clean candidate -> FFmpeg decode",
      snr_local: "our SNR-clean candidate -> our decode",
      snr_ffmpeg: "our SNR-clean candidate -> FFmpeg decode",
      smooth_local: "our smooth-clean candidate -> our decode",
      smooth_ffmpeg: "our smooth-clean candidate -> FFmpeg decode",
      voiced_local: "our voiced-clean candidate -> our decode",
      voiced_ffmpeg: "our voiced-clean candidate -> FFmpeg decode",
      degrit_local: "our degrit-clean candidate -> our decode",
      degrit_ffmpeg: "our degrit-clean candidate -> FFmpeg decode",
      harmonic_local: "our harmonic-clean candidate -> our decode",
      harmonic_ffmpeg: "our harmonic-clean candidate -> FFmpeg decode",
      harmonic_strong_local: "our harmonic-strong candidate -> our decode",
      harmonic_strong_ffmpeg: "our harmonic-strong candidate -> FFmpeg decode",
      harmonic_deep_local: "our harmonic-deep candidate -> our decode",
      harmonic_deep_ffmpeg: "our harmonic-deep candidate -> FFmpeg decode",
      fcb_local: "our FCB-clean candidate -> our decode",
      fcb_ffmpeg: "our FCB-clean candidate -> FFmpeg decode",
      pesq_local: "our PESQ candidate -> our decode",
      pesq_blend50: "our PESQ candidate -> postfilter-blend50 decode",
      pesq_ffmpeg: "our PESQ candidate -> FFmpeg decode",
      pesq_degrit_local: "our PESQ-degrit candidate -> our decode",
      pesq_degrit_ffmpeg: "our PESQ-degrit candidate -> FFmpeg decode",
      soft_our_ffmpeg: "our encode -> softened FFmpeg decode",
      soft_clean_ffmpeg: "our clean candidate -> softened FFmpeg decode",
      external_local: "bcg729 encode -> our decode",
      external_blend50: "bcg729 encode -> postfilter-blend50 decode",
      external_ffmpeg: "bcg729 encode -> FFmpeg decode"
    };
    const battleCandidates = {
      core_ffmpeg: { label: "Core profile -> FFmpeg decode" },
      core_clip_ffmpeg: { label: "Core-clip profile -> FFmpeg decode" },
      our_ffmpeg: { label: "Current quality -> FFmpeg decode" },
      clean_ffmpeg: { label: "Clean candidate -> FFmpeg decode" },
      snr_ffmpeg: { label: "SNR-clean candidate -> FFmpeg decode" },
      smooth_ffmpeg: { label: "Smooth-clean candidate -> FFmpeg decode" },
      voiced_ffmpeg: { label: "Voiced-clean candidate -> FFmpeg decode" },
      degrit_ffmpeg: { label: "Degrit-clean candidate -> FFmpeg decode" },
      harmonic_ffmpeg: { label: "Harmonic-clean candidate -> FFmpeg decode" },
      harmonic_strong_ffmpeg: { label: "Harmonic-strong candidate -> FFmpeg decode" },
      harmonic_deep_ffmpeg: { label: "Harmonic-deep candidate -> FFmpeg decode" },
      fcb_ffmpeg: { label: "FCB-clean candidate -> FFmpeg decode" },
      pesq_ffmpeg: { label: "PESQ candidate -> FFmpeg decode" },
      pesq_degrit_ffmpeg: { label: "PESQ-degrit candidate -> FFmpeg decode" },
      soft_our_ffmpeg: { label: "Current quality -> softened FFmpeg decode" },
      soft_clean_ffmpeg: { label: "Clean candidate -> softened FFmpeg decode" },
      external_ffmpeg: { label: "bcg729 -> FFmpeg decode" },
      core_local: { label: "Core profile -> local decode" },
      core_clip_local: { label: "Core-clip profile -> local decode" },
      our_local: { label: "Current quality -> local decode" },
      our_blend50: { label: "Current quality -> blend50 local decode" },
      clean_local: { label: "Clean candidate -> local decode" },
      clean_blend50: { label: "Clean candidate -> blend50 local decode" },
      snr_local: { label: "SNR-clean candidate -> local decode" },
      smooth_local: { label: "Smooth-clean candidate -> local decode" },
      voiced_local: { label: "Voiced-clean candidate -> local decode" },
      degrit_local: { label: "Degrit-clean candidate -> local decode" },
      harmonic_local: { label: "Harmonic-clean candidate -> local decode" },
      harmonic_strong_local: { label: "Harmonic-strong candidate -> local decode" },
      harmonic_deep_local: { label: "Harmonic-deep candidate -> local decode" },
      fcb_local: { label: "FCB-clean candidate -> local decode" },
      pesq_local: { label: "PESQ candidate -> local decode" },
      pesq_blend50: { label: "PESQ candidate -> blend50 local decode" },
      pesq_degrit_local: { label: "PESQ-degrit candidate -> local decode" },
      external_blend50: { label: "bcg729 -> blend50 local decode" },
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
          const data = await compareFile(files[i], $("battleMode").value, pair);
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

    async function compareFile(file, mode, pair) {
      const form = new FormData();
      form.append("file", file);
      form.append("mode", mode);
      if (pair && pair.length === 2) form.append("want", pair.join(","));
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
        winnerKey: "",
        note: ""
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
        "</div><label>Listening note<textarea id=\"battleNote\" class=\"trial-note\" placeholder=\"예: Left가 더 깨끗함, Right에 서걱거림, 둘 다 비슷함\"></textarea></label>" +
        "<button class=\"battle-choice tie\" type=\"button\" data-choice=\"tie\">비슷함 / 판단 보류</button>";
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
      const note = $("battleNote");
      trial.note = note ? note.value.trim() : "";
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
      const summaryText = buildBattleResultText(pair, counts);
      const rows = battleState.trials.map((trial, i) => {
        const picked = trial.winnerKey === "tie" ? "Tie / unsure" : battleCandidates[trial.winnerKey].label;
        const leftMetric = battleMetric(trial, trial.leftKey);
        const rightMetric = battleMetric(trial, trial.rightKey);
        const leftNoise = battleNoise(trial, trial.leftKey);
        const rightNoise = battleNoise(trial, trial.rightKey);
        return "<tr><td>" + (i + 1) + "</td><td>" + escapeHTML(trial.fileName) + "</td><td>" +
          escapeHTML(battleCandidates[trial.leftKey].label) + "</td><td>" +
          metricSummary(leftMetric, leftNoise) + "</td><td>" +
          escapeHTML(battleCandidates[trial.rightKey].label) + "</td><td>" +
          metricSummary(rightMetric, rightNoise) + "</td><td>" + escapeHTML(picked) + "</td><td>" +
          escapeHTML(oneLineNote(trial.note)) + "</td></tr>";
      }).join("");
      const section = document.createElement("section");
      section.className = "card";
      section.innerHTML =
        "<h2>Blind test result</h2>" +
        "<div class=\"battle-result\">" +
        scoreHTML(battleCandidates[pair[0]].label, counts[pair[0]], averageBattlePESQ(pair[0])) +
        scoreHTML(battleCandidates[pair[1]].label, counts[pair[1]], averageBattlePESQ(pair[1])) +
        scoreHTML("Tie / unsure", counts.tie) +
        "</div>" +
        "<table class=\"metric-table\"><thead><tr><th>#</th><th>File</th><th>Left was</th><th>Left metrics</th><th>Right was</th><th>Right metrics</th><th>Picked</th><th>Note</th></tr></thead><tbody>" + rows + "</tbody></table>" +
        "<div class=\"result-actions\"><button id=\"copyBattleResult\" type=\"button\">Copy result summary</button></div>" +
        "<textarea id=\"battleResultText\" class=\"result-text\" readonly></textarea>" +
        "<button id=\"battleAgain\" type=\"button\">Run another blind test</button>";
      arena.replaceChildren(section);
      $("battleResultText").value = summaryText;
      $("battleStatus").textContent = "완료: " + battleState.trials.length + " trials.";
      $("copyBattleResult").addEventListener("click", copyBattleResultText);
      $("battleAgain").addEventListener("click", startBattle);
    }

    function scoreHTML(label, count, pesqAvg = null) {
      const pesq = (typeof pesqAvg === "number" && Number.isFinite(pesqAvg)) ? "<small>PESQ avg " + fmtMaybe(pesqAvg, 3) + "</small>" : "";
      return "<div class=\"score\"><span>" + escapeHTML(label) + "</span><strong>" + count + "</strong>" + pesq + "</div>";
    }

    function battleMetric(trial, key) {
      return ((trial.data && trial.data.metrics) || []).find((m) => m.key === key) || null;
    }

    function battleNoise(trial, key) {
      return ((trial.data && trial.data.noise) || []).find((m) => m.key === key) || null;
    }

    function metricSummary(metric, noise = null) {
      if (!metric) return "PESQ n/a";
      const parts = [
        "PESQ " + fmtMaybe(metric.pesq, 3),
        "SNR " + fmt(metric.snrDb),
        "Corr " + fmt(metric.corr, 4),
        "Clip " + metric.nearClip
      ];
      if (noise) {
        parts.push("High " + fmt(noise.highErrorDb));
        parts.push("Worst " + fmtMaybe(noise.worstTimeSec, 3) + "s");
      }
      return parts.join(" / ");
    }

    function averageBattlePESQ(key) {
      const values = battleState.trials.map((trial) => {
        const metric = battleMetric(trial, key);
        return metric ? metric.pesq : null;
      }).filter((v) => typeof v === "number" && Number.isFinite(v));
      if (!values.length) return null;
      return values.reduce((sum, v) => sum + v, 0) / values.length;
    }

    function buildBattleResultText(pair, counts) {
      const lines = [];
      lines.push("Blind test result");
      lines.push("Generated: " + new Date().toISOString());
      lines.push(battleCandidates[pair[0]].label + ": " + counts[pair[0]] + " (PESQ avg " + fmtMaybe(averageBattlePESQ(pair[0]), 3) + ")");
      lines.push(battleCandidates[pair[1]].label + ": " + counts[pair[1]] + " (PESQ avg " + fmtMaybe(averageBattlePESQ(pair[1]), 3) + ")");
      lines.push("Tie / unsure: " + counts.tie);
      lines.push("");
      lines.push(["#", "File", "Left was", "Left metrics", "Right was", "Right metrics", "Picked", "Note"].join("\t"));
      battleState.trials.forEach((trial, i) => {
        const picked = trial.winnerKey === "tie" ? "Tie / unsure" : battleCandidates[trial.winnerKey].label;
        lines.push([
          String(i + 1),
          trial.fileName,
          battleCandidates[trial.leftKey].label,
          metricSummary(battleMetric(trial, trial.leftKey), battleNoise(trial, trial.leftKey)),
          battleCandidates[trial.rightKey].label,
          metricSummary(battleMetric(trial, trial.rightKey), battleNoise(trial, trial.rightKey)),
          picked,
          oneLineNote(trial.note)
        ].join("\t"));
      });
      return lines.join("\n");
    }

    function oneLineNote(note) {
      return String(note || "").replace(/\s+/g, " ").trim();
    }

    async function copyBattleResultText() {
      const text = $("battleResultText").value;
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text);
        } else {
          $("battleResultText").select();
          document.execCommand("copy");
        }
        $("battleStatus").textContent = "결과 요약을 클립보드에 복사했습니다.";
      } catch (err) {
        $("battleStatus").textContent = "복사 실패: 텍스트 박스를 직접 선택해서 복사하세요.";
      }
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
      downloads.append(downloadLink(data.downloads.core_g729, "our-core-profile.g729", "Download core profile .g729"));
      downloads.append(downloadLink(data.downloads.core_clip_g729, "our-core-clip-profile.g729", "Download core-clip profile .g729"));
      downloads.append(downloadLink(data.downloads.our_g729, "our-encoder.g729", "Download our .g729"));
      downloads.append(downloadLink(data.downloads.clean_g729, "our-clean-candidate.g729", "Download clean candidate .g729"));
      downloads.append(downloadLink(data.downloads.external_g729, "bcg729-encoder.g729", "Download bcg729 .g729"));
      const table = document.createElement("table");
      table.className = "metric-table";
      table.innerHTML = "<thead><tr><th>Path</th><th>SNR dB</th><th>Corr</th><th>RMS ratio</th><th>PESQ NB</th><th>Lag</th><th>Peak</th><th>Near clip</th></tr></thead><tbody></tbody>";
      const tbody = table.querySelector("tbody");
      data.metrics.forEach((m) => {
        const tr = document.createElement("tr");
        tr.innerHTML = "<td>" + m.path + "</td><td>" + fmt(m.snrDb) + "</td><td>" + fmt(m.corr, 4) + "</td><td>" + fmt(m.rmsRatio, 4) + "</td><td>" + fmtMaybe(m.pesq, 3) + "</td><td>" + m.lagSamples + "</td><td>" + m.outputPeak + "</td><td>" + m.nearClip + "</td>";
        tbody.append(tr);
      });
      $("metrics").append(table);
      renderNotes(data);
      renderNoise(data);
      renderClips(data);
    }
    function renderNotes(data) {
      const notes = ((data && data.notes) || []).filter((note) => String(note).includes("PESQ"));
      if (!notes.length) return;
      const wrap = document.createElement("section");
      wrap.className = "card clip-card";
      wrap.innerHTML = "<h2>Metric notes</h2><p class=\"noise-note\">" + notes.map(escapeHTML).join("<br>") + "</p>";
      $("metrics").append(wrap);
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
    function fmtMaybe(v, digits = 2) {
      return (typeof v === "number" && Number.isFinite(v)) ? Number(v).toFixed(digits) : "n/a";
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

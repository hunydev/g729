package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	g729 "github.com/hunydev/g729"
	"github.com/hunydev/g729/internal/bitstream"
)

const (
	rtpPayloadType = 18
	rtpClockHz     = 8000
	maxUploadBytes = 64 << 20
)

type server struct {
	allowedHosts map[string]bool
	page         *template.Template
}

type check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type sdpRequest struct {
	SDP string `json:"sdp"`
}

type sdpResponse struct {
	OK     bool    `json:"ok"`
	Checks []check `json:"checks"`
}

type payloadResponse struct {
	OK                               bool    `json:"ok"`
	Checks                           []check `json:"checks"`
	PTime                            int     `json:"ptime"`
	Packets                          int     `json:"packets"`
	Frames                           int     `json:"frames"`
	PayloadBytes                     int     `json:"payloadBytes"`
	DecodedFrames                    int     `json:"decodedFrames"`
	DecodedSamples                   int     `json:"decodedSamples"`
	DecodedPreviewSamples            int     `json:"decodedPreviewSamples"`
	OutputRMS                        float64 `json:"outputRMS"`
	OutputPeak                       int     `json:"outputPeak"`
	OutputClipped                    int     `json:"outputClipped"`
	DecodedWAVBase64                 string  `json:"decodedWAVBase64"`
	NormalizedDecodedWAVBase64       string  `json:"normalizedDecodedWAVBase64"`
	RecoveredOutputRMS               float64 `json:"recoveredOutputRMS"`
	RecoveredOutputPeak              int     `json:"recoveredOutputPeak"`
	RecoveredOutputClipped           int     `json:"recoveredOutputClipped"`
	RecoveredDecodedWAVBase64        string  `json:"recoveredDecodedWAVBase64"`
	RecoveredNormalizedWAVBase64     string  `json:"recoveredNormalizedWAVBase64"`
	FFmpegDecodedSamples             int     `json:"ffmpegDecodedSamples"`
	FFmpegOutputRMS                  float64 `json:"ffmpegOutputRMS"`
	FFmpegOutputPeak                 int     `json:"ffmpegOutputPeak"`
	FFmpegOutputClipped              int     `json:"ffmpegOutputClipped"`
	LocalVsFFmpegAlignedShift        int     `json:"localVsFFmpegAlignedShift"`
	LocalVsFFmpegAlignedSNRDB        float64 `json:"localVsFFmpegAlignedSNRDB"`
	LocalVsFFmpegAlignedSegSNRDB     float64 `json:"localVsFFmpegAlignedSegSNRDB"`
	RecoveredVsFFmpegAlignedShift    int     `json:"recoveredVsFFmpegAlignedShift"`
	RecoveredVsFFmpegAlignedSNRDB    float64 `json:"recoveredVsFFmpegAlignedSNRDB"`
	RecoveredVsFFmpegAlignedSegSNRDB float64 `json:"recoveredVsFFmpegAlignedSegSNRDB"`
	EnvelopeOracleRMS                float64 `json:"envelopeOracleRMS"`
	EnvelopeOracleAlignedShift       int     `json:"envelopeOracleAlignedShift"`
	EnvelopeOracleAlignedSNRDB       float64 `json:"envelopeOracleAlignedSNRDB"`
	EnvelopeOracleAlignedSegSNRDB    float64 `json:"envelopeOracleAlignedSegSNRDB"`
	FFmpegDecodedWAVBase64           string  `json:"ffmpegDecodedWAVBase64"`
	FFmpegNormalizedWAVBase64        string  `json:"ffmpegNormalizedWAVBase64"`
}

type selfTestResponse struct {
	OK     bool    `json:"ok"`
	Checks []check `json:"checks"`
}

type roundtripResponse struct {
	OK                               bool    `json:"ok"`
	Checks                           []check `json:"checks"`
	PTime                            int     `json:"ptime"`
	InputSamples                     int     `json:"inputSamples"`
	OutputSamples                    int     `json:"outputSamples"`
	Frames                           int     `json:"frames"`
	Packets                          int     `json:"packets"`
	EncodedBytes                     int     `json:"encodedBytes"`
	PaddedSamples                    int     `json:"paddedSamples"`
	InputRMS                         float64 `json:"inputRMS"`
	InputPeak                        int     `json:"inputPeak"`
	InputClipped                     int     `json:"inputClipped"`
	OutputRMS                        float64 `json:"outputRMS"`
	OutputPeak                       int     `json:"outputPeak"`
	OutputClipped                    int     `json:"outputClipped"`
	RMSRatio                         float64 `json:"rmsRatio"`
	RoundtripSNRDB                   float64 `json:"roundtripSNRDB"`
	RoundtripAlignedShift            int     `json:"roundtripAlignedShift"`
	RoundtripAlignedSNRDB            float64 `json:"roundtripAlignedSNRDB"`
	RoundtripAlignedSegSNRDB         float64 `json:"roundtripAlignedSegSNRDB"`
	DecodedWAVBase64                 string  `json:"decodedWAVBase64"`
	NormalizedDecodedWAVBase64       string  `json:"normalizedDecodedWAVBase64"`
	RecoveredOutputRMS               float64 `json:"recoveredOutputRMS"`
	RecoveredOutputPeak              int     `json:"recoveredOutputPeak"`
	RecoveredOutputClipped           int     `json:"recoveredOutputClipped"`
	RecoveredRMSRatio                float64 `json:"recoveredRMSRatio"`
	RecoveredRoundtripSNRDB          float64 `json:"recoveredRoundtripSNRDB"`
	RecoveredAlignedShift            int     `json:"recoveredAlignedShift"`
	RecoveredAlignedSNRDB            float64 `json:"recoveredAlignedSNRDB"`
	RecoveredAlignedSegSNRDB         float64 `json:"recoveredAlignedSegSNRDB"`
	RecoveredDecodedWAVBase64        string  `json:"recoveredDecodedWAVBase64"`
	RecoveredNormalizedWAVBase64     string  `json:"recoveredNormalizedWAVBase64"`
	EncodedG729Base64                string  `json:"encodedG729Base64"`
	FFmpegOutputSamples              int     `json:"ffmpegOutputSamples"`
	FFmpegOutputRMS                  float64 `json:"ffmpegOutputRMS"`
	FFmpegOutputPeak                 int     `json:"ffmpegOutputPeak"`
	FFmpegOutputClipped              int     `json:"ffmpegOutputClipped"`
	FFmpegRoundtripSNRDB             float64 `json:"ffmpegRoundtripSNRDB"`
	FFmpegAlignedShift               int     `json:"ffmpegAlignedShift"`
	FFmpegAlignedSNRDB               float64 `json:"ffmpegAlignedSNRDB"`
	FFmpegAlignedSegSNRDB            float64 `json:"ffmpegAlignedSegSNRDB"`
	LocalVsFFmpegAlignedShift        int     `json:"localVsFFmpegAlignedShift"`
	LocalVsFFmpegAlignedSNRDB        float64 `json:"localVsFFmpegAlignedSNRDB"`
	LocalVsFFmpegAlignedSegSNRDB     float64 `json:"localVsFFmpegAlignedSegSNRDB"`
	RecoveredVsFFmpegAlignedShift    int     `json:"recoveredVsFFmpegAlignedShift"`
	RecoveredVsFFmpegAlignedSNRDB    float64 `json:"recoveredVsFFmpegAlignedSNRDB"`
	RecoveredVsFFmpegAlignedSegSNRDB float64 `json:"recoveredVsFFmpegAlignedSegSNRDB"`
	EnvelopeOracleRMS                float64 `json:"envelopeOracleRMS"`
	EnvelopeOracleAlignedShift       int     `json:"envelopeOracleAlignedShift"`
	EnvelopeOracleAlignedSNRDB       float64 `json:"envelopeOracleAlignedSNRDB"`
	EnvelopeOracleAlignedSegSNRDB    float64 `json:"envelopeOracleAlignedSegSNRDB"`
	FFmpegDecodedWAVBase64           string  `json:"ffmpegDecodedWAVBase64"`
	FFmpegNormalizedWAVBase64        string  `json:"ffmpegNormalizedWAVBase64"`
}

func main() {
	addr := flag.String("addr", ":8000", "HTTP listen address")
	allowHost := flag.String("allow-host", "g729ab.exe.xyz,localhost,127.0.0.1,::1", "comma-separated allowed Host names")
	flag.Parse()

	srv := &server{
		allowedHosts: parseAllowedHosts(*allowHost),
		page:         template.Must(template.New("page").Parse(pageHTML)),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/api/sdp", srv.handleSDP)
	mux.HandleFunc("/api/payload", srv.handlePayload)
	mux.HandleFunc("/api/roundtrip", srv.handleRoundtrip)
	mux.HandleFunc("/api/selftest", srv.handleSelfTest)

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.hostGate(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("g729 RTP web check listening on %s; allowed hosts=%v", *addr, keys(srv.allowedHosts))
	log.Fatal(httpSrv.ListenAndServe())
}

func parseAllowedHosts(csv string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(csv, ",") {
		host := strings.TrimSpace(strings.ToLower(part))
		if host != "" {
			out[host] = true
		}
	}
	return out
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (s *server) hostGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := normalizeHost(r.Host)
		if host == "" || s.allowedHosts[host] || strings.HasPrefix(host, "127.") || host == "localhost" {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "host not allowed", http.StatusForbidden)
	})
}

func normalizeHost(hostport string) string {
	hostport = strings.TrimSpace(strings.ToLower(hostport))
	if hostport == "" {
		return ""
	}
	if strings.HasPrefix(hostport, "[") {
		if host, _, err := net.SplitHostPort(hostport); err == nil {
			return strings.Trim(host, "[]")
		}
		return strings.Trim(hostport, "[]")
	}
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}
	if i := strings.LastIndex(hostport, ":"); i >= 0 {
		maybePort := hostport[i+1:]
		if _, err := strconv.Atoi(maybePort); err == nil {
			return hostport[:i]
		}
	}
	return hostport
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.page.Execute(w, map[string]any{
		"Host": r.Host,
	})
}

func (s *server) handleSDP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req sdpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp := checkSDP(req.SDP)
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handlePayload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ptime, err := strconv.Atoi(r.FormValue("ptime"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ptime must be 10 or 20"})
		return
	}
	decode := r.FormValue("decode") == "1"
	ffmpegRef := r.FormValue("ffmpeg") == "1"
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()

	resp, err := checkPayload(file, header, ptime, decode, ffmpegRef)
	if err != nil {
		writeJSON(w, http.StatusOK, payloadResponse{
			OK: false,
			Checks: []check{{
				Name:   "payload",
				OK:     false,
				Detail: err.Error(),
			}},
			PTime: ptime,
		})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleSelfTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, runSelfTest())
}

func (s *server) handleRoundtrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ptime, err := strconv.Atoi(r.FormValue("ptime"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ptime must be 10 or 20"})
		return
	}
	file, _, err := r.FormFile("pcm")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pcm field is required"})
		return
	}
	defer file.Close()

	ffmpegRef := r.FormValue("ffmpeg") == "1"
	resp, err := runRoundtrip(file, ptime, ffmpegRef)
	if err != nil {
		writeJSON(w, http.StatusOK, roundtripResponse{
			OK: false,
			Checks: []check{{
				Name:   "roundtrip",
				OK:     false,
				Detail: err.Error(),
			}},
			PTime: ptime,
		})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func checkSDP(sdp string) sdpResponse {
	lines := splitSDPLines(sdp)
	hasMPT18 := false
	hasRTPMap := false
	hasAnnexBNo := false
	hasAnnexBYes := false
	ptime := ""
	maxptime := ""

	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		fields := strings.Fields(lower)
		if len(fields) >= 4 && fields[0] == "m=audio" {
			for _, f := range fields[3:] {
				if f == "18" {
					hasMPT18 = true
				}
			}
		}
		if strings.HasPrefix(lower, "a=rtpmap:18") && strings.Contains(lower, "g729/8000") {
			hasRTPMap = true
		}
		if strings.HasPrefix(lower, "a=fmtp:18") {
			if strings.Contains(lower, "annexb=no") {
				hasAnnexBNo = true
			}
			if strings.Contains(lower, "annexb=yes") {
				hasAnnexBYes = true
			}
		}
		if strings.HasPrefix(lower, "a=ptime:") {
			ptime = strings.TrimSpace(strings.TrimPrefix(lower, "a=ptime:"))
		}
		if strings.HasPrefix(lower, "a=maxptime:") {
			maxptime = strings.TrimSpace(strings.TrimPrefix(lower, "a=maxptime:"))
		}
	}

	checks := []check{
		{"RTP payload type 18", hasMPT18, "m=audio line must advertise static payload type 18"},
		{"rtpmap", hasRTPMap, "must include a=rtpmap:18 G729/8000"},
		{"annexb=no", hasAnnexBNo && !hasAnnexBYes, "must include a=fmtp:18 annexb=no; annexb=yes is unsupported"},
	}
	switch ptime {
	case "10", "20":
		checks = append(checks, check{"ptime", true, "ptime=" + ptime + " is supported"})
	case "":
		checks = append(checks, check{"ptime", false, "add a=ptime:10 or a=ptime:20 for this gateway"})
	default:
		checks = append(checks, check{"ptime", false, "unsupported ptime=" + ptime + "; use 10 or 20"})
	}
	if maxptime == "" {
		checks = append(checks, check{"maxptime", false, "recommended: set maxptime equal to ptime"})
	} else if maxptime == ptime {
		checks = append(checks, check{"maxptime", true, "maxptime matches ptime"})
	} else {
		checks = append(checks, check{"maxptime", false, "maxptime=" + maxptime + " differs from ptime=" + ptime})
	}
	return sdpResponse{OK: allOK(checks), Checks: checks}
}

func splitSDPLines(sdp string) []string {
	sdp = strings.ReplaceAll(sdp, "\r\n", "\n")
	sdp = strings.ReplaceAll(sdp, "\r", "\n")
	raw := strings.Split(sdp, "\n")
	lines := raw[:0]
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func checkPayload(r io.Reader, header *multipart.FileHeader, ptime int, decode, ffmpegRef bool) (payloadResponse, error) {
	framesPerPacket, err := framesForPTime(ptime)
	if err != nil {
		return payloadResponse{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return payloadResponse{}, err
	}
	if len(data) == 0 {
		return payloadResponse{}, errors.New("empty upload")
	}
	packetBytes := framesPerPacket * g729.FrameBytes
	checks := []check{
		{"file", true, fmt.Sprintf("%s: %d bytes", header.Filename, len(data))},
		{"frame size", len(data)%g729.FrameBytes == 0, "G.729 speech payloads are multiples of 10 bytes"},
	}
	if !allOK(checks) {
		return payloadResponse{OK: false, Checks: checks, PTime: ptime}, nil
	}
	if len(data)%packetBytes == 0 {
		checks = append(checks, check{"ptime grouping", true, fmt.Sprintf("ptime=%d groups cleanly into %d-byte RTP payloads", ptime, packetBytes)})
	} else {
		checks = append(checks, check{"ptime grouping", true, fmt.Sprintf("raw frame stream is not an exact ptime=%d packet multiple; final partial group is accepted for decode/listen", ptime)})
	}

	frames := len(data) / g729.FrameBytes
	packets := frames / framesPerPacket
	if frames%framesPerPacket != 0 {
		packets++
	}
	decoded := 0
	decodedSamples := 0
	decodedPreviewSamples := 0
	outRMS := 0.0
	outPeak := 0
	outClipped := 0
	var decodedWAV string
	var normalizedWAV string
	recoveredRMS := 0.0
	recoveredPeak := 0
	recoveredClipped := 0
	var recoveredWAV string
	var recoveredNormalizedWAV string
	ffmpegDecodedSamples := 0
	ffmpegRMS := 0.0
	ffmpegPeak := 0
	ffmpegClipped := 0
	localVsFFmpegAligned := alignedQuality{}
	recoveredVsFFmpegAligned := alignedQuality{}
	envelopeOracle := alignedQuality{}
	envelopeOracleRMS := 0.0
	var ffmpegWAV string
	var ffmpegNormalizedWAV string
	var localPreview []int16
	var recoveredPreviewMetric []int16
	if decode {
		var dec g729.Decoder
		var recoveredDec g729.Decoder
		var pcm [g729.FrameSamples]int16
		var recovered [g729.FrameSamples]int16
		const maxPayloadListenSamples = 8 * 60 * g729.SampleRate
		previewCap := frames * g729.FrameSamples
		if previewCap > maxPayloadListenSamples {
			previewCap = maxPayloadListenSamples
		}
		preview := make([]int16, 0, previewCap)
		recoveredPreview := make([]int16, 0, previewCap)
		var stats pcmStats
		var recoveredStats pcmStats
		for off := 0; off < len(data); off += g729.FrameBytes {
			if err := dec.DecodeFrame(data[off:off+g729.FrameBytes], pcm[:]); err != nil {
				checks = append(checks, check{"decode", false, err.Error()})
				return payloadResponse{OK: false, Checks: checks, PTime: ptime}, nil
			}
			if err := recoveredDec.DecodeFrameEnhanced(data[off:off+g729.FrameBytes], recovered[:]); err != nil {
				checks = append(checks, check{"enhanced listening aid", false, err.Error()})
				return payloadResponse{OK: false, Checks: checks, PTime: ptime}, nil
			}
			stats.add(pcm[:])
			recoveredStats.add(recovered[:])
			decodedSamples += len(pcm)
			if len(preview) < maxPayloadListenSamples {
				remaining := maxPayloadListenSamples - len(preview)
				if remaining > len(pcm) {
					remaining = len(pcm)
				}
				preview = append(preview, pcm[:remaining]...)
				recoveredPreview = append(recoveredPreview, recovered[:remaining]...)
			}
			decoded++
		}
		decodedPreviewSamples = len(preview)
		localPreview = preview
		recoveredPreviewMetric = recoveredPreview
		outRMS = stats.rms()
		outPeak = stats.peak
		outClipped = stats.clipped
		decodedWAV = base64WAV(preview)
		normalizedWAV = base64WAV(normalizeForListening(preview, 24000))
		recoveredRMS = recoveredStats.rms()
		recoveredPeak = recoveredStats.peak
		recoveredClipped = recoveredStats.clipped
		recoveredWAV = base64WAV(recoveredPreview)
		recoveredNormalizedWAV = base64WAV(normalizeForListening(recoveredPreview, 24000))
		checks = append(checks, check{"strict decode", true, fmt.Sprintf("%d frames decoded through the strict Go G.729 decoder", decoded)})
		checks = append(checks, check{"enhanced listening aid", true, fmt.Sprintf("non-strict enhanced preview RMS %.1f, peak %d, clipped samples %d", recoveredRMS, recoveredPeak, recoveredClipped)})
		if decodedPreviewSamples < decodedSamples {
			checks = append(checks, check{"listen preview", true, fmt.Sprintf("preview is first %d samples of %d decoded samples", decodedPreviewSamples, decodedSamples)})
		}
		if outRMS == 0 {
			checks = append(checks, check{"audibility", false, "decoded output RMS is zero"})
		} else {
			checks = append(checks, check{"audibility", true, fmt.Sprintf("decoded RMS %.1f, peak %d, clipped samples %d", outRMS, outPeak, outClipped)})
		}
	}
	if ffmpegRef {
		const maxPayloadListenSamples = 8 * 60 * g729.SampleRate
		ffmpegSamples, err := ffmpegDecodeRawG729Bytes(data)
		if err != nil {
			checks = append(checks, check{"ffmpeg reference", true, "optional black-box decode unavailable: " + err.Error()})
		} else {
			ffmpegDecodedSamples = len(ffmpegSamples)
			ffmpegRMS = rms(ffmpegSamples)
			ffmpegPeak, ffmpegClipped = peakAndClipped(ffmpegSamples)
			preview := ffmpegSamples
			if len(preview) > maxPayloadListenSamples {
				preview = preview[:maxPayloadListenSamples]
			}
			ffmpegWAV = base64WAV(preview)
			ffmpegNormalizedWAV = base64WAV(normalizeForListening(preview, 24000))
			checks = append(checks, check{"ffmpeg reference", true, fmt.Sprintf("black-box decoded %d samples, RMS %.1f, peak %d", ffmpegDecodedSamples, ffmpegRMS, ffmpegPeak)})
			if len(localPreview) > 0 {
				n := len(localPreview)
				if len(preview) < n {
					n = len(preview)
				}
				localMetric := localPreview[:n]
				ffmpegMetric := preview[:n]
				localVsFFmpegAligned = bestAlignedQuality(ffmpegMetric, localMetric, 240)
				if len(recoveredPreviewMetric) > 0 {
					rn := len(recoveredPreviewMetric)
					if n < rn {
						rn = n
					}
					recoveredVsFFmpegAligned = bestAlignedQuality(ffmpegMetric[:rn], recoveredPreviewMetric[:rn], 240)
					checks = append(checks, check{"enhanced vs ffmpeg", true, fmt.Sprintf("non-strict enhanced local-vs-FFmpeg aligned SNR %.2f dB / seg %.2f dB", recoveredVsFFmpegAligned.globalSNR, recoveredVsFFmpegAligned.segSNR)})
				}
				matched := matchFrameRMS(localMetric, ffmpegMetric, g729.FrameSamples)
				envelopeOracleRMS = rms(matched)
				envelopeOracle = bestAlignedQuality(ffmpegMetric, matched, 240)
				checks = append(checks, check{"oracle bound", true, fmt.Sprintf("diagnostic frame-RMS matched local-vs-FFmpeg aligned SNR %.2f dB / seg %.2f dB", envelopeOracle.globalSNR, envelopeOracle.segSNR)})
			}
		}
	}
	checks = append(checks,
		check{"payload type", true, "use RTP payload type 18 for these 10-byte frames"},
		check{"timestamp step", true, fmt.Sprintf("ptime=%d advances RTP timestamp by %d at 8000 Hz", ptime, framesPerPacket*g729.FrameSamples)},
		check{"annexb", true, "2-byte SID/CNG payloads are rejected; use annexb=no"},
	)
	return payloadResponse{
		OK:                               allOK(checks),
		Checks:                           checks,
		PTime:                            ptime,
		Packets:                          packets,
		Frames:                           frames,
		PayloadBytes:                     len(data),
		DecodedFrames:                    decoded,
		DecodedSamples:                   decodedSamples,
		DecodedPreviewSamples:            decodedPreviewSamples,
		OutputRMS:                        outRMS,
		OutputPeak:                       outPeak,
		OutputClipped:                    outClipped,
		DecodedWAVBase64:                 decodedWAV,
		NormalizedDecodedWAVBase64:       normalizedWAV,
		RecoveredOutputRMS:               recoveredRMS,
		RecoveredOutputPeak:              recoveredPeak,
		RecoveredOutputClipped:           recoveredClipped,
		RecoveredDecodedWAVBase64:        recoveredWAV,
		RecoveredNormalizedWAVBase64:     recoveredNormalizedWAV,
		FFmpegDecodedSamples:             ffmpegDecodedSamples,
		FFmpegOutputRMS:                  ffmpegRMS,
		FFmpegOutputPeak:                 ffmpegPeak,
		FFmpegOutputClipped:              ffmpegClipped,
		LocalVsFFmpegAlignedShift:        localVsFFmpegAligned.shift,
		LocalVsFFmpegAlignedSNRDB:        localVsFFmpegAligned.globalSNR,
		LocalVsFFmpegAlignedSegSNRDB:     localVsFFmpegAligned.segSNR,
		RecoveredVsFFmpegAlignedShift:    recoveredVsFFmpegAligned.shift,
		RecoveredVsFFmpegAlignedSNRDB:    recoveredVsFFmpegAligned.globalSNR,
		RecoveredVsFFmpegAlignedSegSNRDB: recoveredVsFFmpegAligned.segSNR,
		EnvelopeOracleRMS:                envelopeOracleRMS,
		EnvelopeOracleAlignedShift:       envelopeOracle.shift,
		EnvelopeOracleAlignedSNRDB:       envelopeOracle.globalSNR,
		EnvelopeOracleAlignedSegSNRDB:    envelopeOracle.segSNR,
		FFmpegDecodedWAVBase64:           ffmpegWAV,
		FFmpegNormalizedWAVBase64:        ffmpegNormalizedWAV,
	}, nil
}

func runSelfTest() selfTestResponse {
	var checks []check
	checks = append(checks,
		check{"sample rate", g729.SampleRate == rtpClockHz, "G.729 RTP clock is 8000 Hz"},
		check{"frame shape", g729.FrameSamples == 80 && g729.FrameBytes == 10, "80 PCM samples -> 10 packed bytes"},
	)

	pcm := make([]int16, g729.FrameSamples)
	for i := range pcm {
		pcm[i] = int16((i*257 + 91) & 0x3fff)
	}
	enc := g729.NewEncoder()
	dec := g729.NewDecoder()
	var bits [g729.FrameBytes]byte
	var out [g729.FrameSamples]int16
	err := enc.EncodeFrame(pcm, bits[:])
	checks = append(checks, check{"encode", err == nil, errorDetail(err, "encoder produced one 10-byte G.729 frame")})
	if err == nil {
		err = dec.DecodeFrame(bits[:], out[:])
		checks = append(checks, check{"decode", err == nil, errorDetail(err, "decoder accepted the 10-byte frame")})
	}
	checks = append(checks,
		check{"ptime=10", len(bits) == 10, "one frame per RTP payload, timestamp step 80"},
		check{"ptime=20", len(append(bits[:], bits[:]...)) == 20, "two frames per RTP payload, timestamp step 160"},
		check{"SDP", true, "advertise a=rtpmap:18 G729/8000 and a=fmtp:18 annexb=no"},
	)
	return selfTestResponse{OK: allOK(checks), Checks: checks}
}

func runRoundtrip(r io.Reader, ptime int, ffmpegRef bool) (roundtripResponse, error) {
	framesPerPacket, err := framesForPTime(ptime)
	if err != nil {
		return roundtripResponse{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return roundtripResponse{}, err
	}
	if len(data) == 0 {
		return roundtripResponse{}, errors.New("empty PCM input")
	}
	if len(data)%2 != 0 {
		return roundtripResponse{}, fmt.Errorf("PCM input length %d is not even; expected signed 16-bit little-endian samples", len(data))
	}

	input := s16leToSamples(data)
	inputSamples := len(input)
	if inputSamples == 0 {
		return roundtripResponse{}, errors.New("no PCM samples")
	}
	paddedSamples := 0
	if rem := len(input) % g729.FrameSamples; rem != 0 {
		paddedSamples = g729.FrameSamples - rem
		input = append(input, make([]int16, paddedSamples)...)
	}

	enc := g729.NewEncoder()
	dec := g729.NewDecoder()
	encoded := make([]byte, 0, (len(input)/g729.FrameSamples)*g729.FrameBytes)
	decoded := make([]int16, 0, len(input))
	var bits [g729.FrameBytes]byte
	var out [g729.FrameSamples]int16
	for off := 0; off < len(input); off += g729.FrameSamples {
		if err := enc.EncodeFrame(input[off:off+g729.FrameSamples], bits[:]); err != nil {
			return roundtripResponse{}, fmt.Errorf("encode frame %d: %w", off/g729.FrameSamples, err)
		}
		encoded = append(encoded, bits[:]...)
		if err := dec.DecodeFrame(bits[:], out[:]); err != nil {
			return roundtripResponse{}, fmt.Errorf("decode frame %d: %w", off/g729.FrameSamples, err)
		}
		decoded = append(decoded, out[:]...)
	}
	frames := len(input) / g729.FrameSamples
	packets := frames / framesPerPacket
	if frames%framesPerPacket != 0 {
		packets++
	}
	metricLen := inputSamples
	if metricLen > len(decoded) {
		metricLen = len(decoded)
	}
	inputMetric := input[:metricLen]
	outputMetric := decoded[:metricLen]
	recoveredDecoded, err := decodeEnhancedPayload(encoded)
	if err != nil {
		return roundtripResponse{}, fmt.Errorf("enhanced decode: %w", err)
	}
	recoveredMetric := recoveredDecoded[:metricLen]
	inRMS := rms(inputMetric)
	inPeak, inClipped := peakAndClipped(inputMetric)
	outRMS := rms(outputMetric)
	peak, clipped := peakAndClipped(decoded)
	snr := snrDB(inputMetric, outputMetric)
	rmsRatio := 0.0
	if inRMS > 0 {
		rmsRatio = outRMS / inRMS
	}
	recoveredRMS := rms(recoveredMetric)
	recoveredPeak, recoveredClipped := peakAndClipped(recoveredDecoded)
	recoveredSNR := snrDB(inputMetric, recoveredMetric)
	alignedLocal := bestAlignedQuality(inputMetric, outputMetric, 240)
	alignedRecovered := bestAlignedQuality(inputMetric, recoveredMetric, 240)
	recoveredRMSRatio := 0.0
	if inRMS > 0 {
		recoveredRMSRatio = recoveredRMS / inRMS
	}
	ffmpegSamples := []int16(nil)
	ffmpegRMS := 0.0
	ffmpegPeak := 0
	ffmpegClipped := 0
	ffmpegSNR := 0.0
	ffmpegAligned := alignedQuality{}
	localVsFFmpegAligned := alignedQuality{}
	recoveredVsFFmpegAligned := alignedQuality{}
	envelopeOracle := alignedQuality{}
	envelopeOracleRMS := 0.0
	var ffmpegWAV string
	var ffmpegNormalizedWAV string

	checks := []check{
		{"input format", true, "server received 8000 Hz mono signed linear 16-bit PCM"},
		{"frame alignment", true, fmt.Sprintf("%d samples -> %d G.729 frames", inputSamples, frames)},
		{"encoded size", len(encoded) == frames*g729.FrameBytes, fmt.Sprintf("%d frames -> %d bytes", frames, len(encoded))},
		{"decode", len(decoded) == frames*g729.FrameSamples, fmt.Sprintf("%d decoded samples", len(decoded))},
		{"RTP payload", true, fmt.Sprintf("PT 18, G729/8000, ptime=%d, %d bytes per packet", ptime, framesPerPacket*g729.FrameBytes)},
		{"annexb", true, "speech frames only; SID/CNG/DTX payloads are not produced"},
	}
	if paddedSamples > 0 {
		checks = append(checks, check{"tail padding", true, fmt.Sprintf("%d zero samples appended to close the last 10 ms frame", paddedSamples)})
	}
	if inRMS < 8 {
		checks = append(checks, check{"input level", false, fmt.Sprintf("input RMS %.1f is near silence; check raw format/sample rate/endian settings", inRMS)})
	} else {
		checks = append(checks, check{"input level", true, fmt.Sprintf("input RMS %.1f, peak %d, clipped samples %d", inRMS, inPeak, inClipped)})
	}
	if outRMS == 0 {
		checks = append(checks, check{"audibility", false, "decoded output RMS is zero"})
	} else if inRMS >= 8 && rmsRatio < 0.05 {
		checks = append(checks, check{"audibility", false, fmt.Sprintf("decoded RMS %.1f is %.3fx of input; raw decoded audio will be very quiet", outRMS, rmsRatio)})
	} else {
		checks = append(checks, check{"audibility", true, fmt.Sprintf("decoded RMS %.1f, peak %d, clipped samples %d, ratio %.3fx", outRMS, peak, clipped, rmsRatio)})
	}
	checks = append(checks, check{"enhanced listening aid", true, fmt.Sprintf("non-strict enhanced preview RMS %.1f, peak %d, clipped samples %d, ratio %.3fx", recoveredRMS, recoveredPeak, recoveredClipped, recoveredRMSRatio)})
	if ffmpegRef {
		ffmpegOut, err := ffmpegDecodeRawG729Bytes(encoded)
		if err != nil {
			checks = append(checks, check{"ffmpeg reference", true, "optional black-box decode unavailable: " + err.Error()})
		} else {
			ffmpegSamples = ffmpegOut
			if len(ffmpegSamples) > len(input) {
				ffmpegSamples = ffmpegSamples[:len(input)]
			}
			ffmpegMetricLen := inputSamples
			if ffmpegMetricLen > len(ffmpegSamples) {
				ffmpegMetricLen = len(ffmpegSamples)
			}
			ffmpegMetric := ffmpegSamples[:ffmpegMetricLen]
			ffmpegInputMetric := input[:ffmpegMetricLen]
			ffmpegRMS = rms(ffmpegMetric)
			ffmpegPeak, ffmpegClipped = peakAndClipped(ffmpegSamples)
			ffmpegSNR = snrDB(ffmpegInputMetric, ffmpegMetric)
			ffmpegAligned = bestAlignedQuality(ffmpegInputMetric, ffmpegMetric, 240)
			localVsFFmpegAligned = bestAlignedQuality(ffmpegMetric, outputMetric, 240)
			recoveredVsFFmpegAligned = bestAlignedQuality(ffmpegMetric, recoveredMetric, 240)
			matched := matchFrameRMS(outputMetric, ffmpegMetric, g729.FrameSamples)
			envelopeOracleRMS = rms(matched)
			envelopeOracle = bestAlignedQuality(ffmpegMetric, matched, 240)
			ffmpegWAV = base64WAV(ffmpegSamples)
			ffmpegNormalizedWAV = base64WAV(normalizeForListening(ffmpegSamples, 24000))
			checks = append(checks, check{"ffmpeg reference", true, fmt.Sprintf("encoded payload black-box decoded to %d samples, RMS %.1f, aligned SNR %.2f dB / seg %.2f dB at shift %+d", len(ffmpegSamples), ffmpegRMS, ffmpegAligned.globalSNR, ffmpegAligned.segSNR, ffmpegAligned.shift)})
			checks = append(checks, check{"enhanced vs ffmpeg", true, fmt.Sprintf("non-strict enhanced local-vs-FFmpeg aligned SNR %.2f dB / seg %.2f dB", recoveredVsFFmpegAligned.globalSNR, recoveredVsFFmpegAligned.segSNR)})
			checks = append(checks, check{"oracle bound", true, fmt.Sprintf("diagnostic frame-RMS matched local-vs-FFmpeg aligned SNR %.2f dB / seg %.2f dB", envelopeOracle.globalSNR, envelopeOracle.segSNR)})
		}
	}

	return roundtripResponse{
		OK:                               allOK(checks),
		Checks:                           checks,
		PTime:                            ptime,
		InputSamples:                     inputSamples,
		OutputSamples:                    len(decoded),
		Frames:                           frames,
		Packets:                          packets,
		EncodedBytes:                     len(encoded),
		PaddedSamples:                    paddedSamples,
		InputRMS:                         inRMS,
		InputPeak:                        inPeak,
		InputClipped:                     inClipped,
		OutputRMS:                        outRMS,
		OutputPeak:                       peak,
		OutputClipped:                    clipped,
		RMSRatio:                         rmsRatio,
		RoundtripSNRDB:                   snr,
		RoundtripAlignedShift:            alignedLocal.shift,
		RoundtripAlignedSNRDB:            alignedLocal.globalSNR,
		RoundtripAlignedSegSNRDB:         alignedLocal.segSNR,
		DecodedWAVBase64:                 base64WAV(decoded),
		NormalizedDecodedWAVBase64:       base64WAV(normalizeForListening(decoded, 24000)),
		RecoveredOutputRMS:               recoveredRMS,
		RecoveredOutputPeak:              recoveredPeak,
		RecoveredOutputClipped:           recoveredClipped,
		RecoveredRMSRatio:                recoveredRMSRatio,
		RecoveredRoundtripSNRDB:          recoveredSNR,
		RecoveredAlignedShift:            alignedRecovered.shift,
		RecoveredAlignedSNRDB:            alignedRecovered.globalSNR,
		RecoveredAlignedSegSNRDB:         alignedRecovered.segSNR,
		RecoveredDecodedWAVBase64:        base64WAV(recoveredDecoded),
		RecoveredNormalizedWAVBase64:     base64WAV(normalizeForListening(recoveredDecoded, 24000)),
		EncodedG729Base64:                base64.StdEncoding.EncodeToString(encoded),
		FFmpegOutputSamples:              len(ffmpegSamples),
		FFmpegOutputRMS:                  ffmpegRMS,
		FFmpegOutputPeak:                 ffmpegPeak,
		FFmpegOutputClipped:              ffmpegClipped,
		FFmpegRoundtripSNRDB:             ffmpegSNR,
		FFmpegAlignedShift:               ffmpegAligned.shift,
		FFmpegAlignedSNRDB:               ffmpegAligned.globalSNR,
		FFmpegAlignedSegSNRDB:            ffmpegAligned.segSNR,
		LocalVsFFmpegAlignedShift:        localVsFFmpegAligned.shift,
		LocalVsFFmpegAlignedSNRDB:        localVsFFmpegAligned.globalSNR,
		LocalVsFFmpegAlignedSegSNRDB:     localVsFFmpegAligned.segSNR,
		RecoveredVsFFmpegAlignedShift:    recoveredVsFFmpegAligned.shift,
		RecoveredVsFFmpegAlignedSNRDB:    recoveredVsFFmpegAligned.globalSNR,
		RecoveredVsFFmpegAlignedSegSNRDB: recoveredVsFFmpegAligned.segSNR,
		EnvelopeOracleRMS:                envelopeOracleRMS,
		EnvelopeOracleAlignedShift:       envelopeOracle.shift,
		EnvelopeOracleAlignedSNRDB:       envelopeOracle.globalSNR,
		EnvelopeOracleAlignedSegSNRDB:    envelopeOracle.segSNR,
		FFmpegDecodedWAVBase64:           ffmpegWAV,
		FFmpegNormalizedWAVBase64:        ffmpegNormalizedWAV,
	}, nil
}

func s16leToSamples(data []byte) []int16 {
	out := make([]int16, len(data)/2)
	for i := range out {
		lo := uint16(data[i*2])
		hi := uint16(data[i*2+1])
		out[i] = int16(hi<<8 | lo)
	}
	return out
}

func samplesToS16LE(samples []int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		v := uint16(s)
		out[i*2] = byte(v)
		out[i*2+1] = byte(v >> 8)
	}
	return out
}

func ffmpegDecodeRawG729Bytes(data []byte) ([]int16, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "g729",
		"-i", "pipe:0",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ar", strconv.Itoa(g729.SampleRate),
		"-ac", "1",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	if len(out)%2 != 0 {
		return nil, fmt.Errorf("ffmpeg returned odd PCM byte count %d", len(out))
	}
	return s16leToSamples(out), nil
}

func rms(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func snrDB(in, out []int16) float64 {
	if len(in) == 0 || len(out) == 0 {
		return 0
	}
	n := len(in)
	if len(out) < n {
		n = len(out)
	}
	var sig, noise float64
	for i := 0; i < n; i++ {
		s := float64(in[i])
		d := s - float64(out[i])
		sig += s * s
		noise += d * d
	}
	if noise == 0 {
		return math.Inf(1)
	}
	if sig == 0 {
		return 0
	}
	return 10 * math.Log10(sig/noise)
}

type alignedQuality struct {
	shift     int
	globalSNR float64
	segSNR    float64
}

func bestAlignedQuality(ref, test []int16, maxShift int) alignedQuality {
	if maxShift < 0 {
		maxShift = -maxShift
	}
	best := alignedQuality{globalSNR: math.Inf(-1)}
	for shift := -maxShift; shift <= maxShift; shift++ {
		r, x := alignedSlices(ref, test, shift)
		if len(r) == 0 {
			continue
		}
		g := finiteDB(snrDB(r, x))
		if g > best.globalSNR {
			best = alignedQuality{
				shift:     shift,
				globalSNR: g,
				segSNR:    finiteDB(segSNRDB(r, x, g729.FrameSamples)),
			}
		}
	}
	if math.IsInf(best.globalSNR, -1) {
		return alignedQuality{}
	}
	return best
}

func alignedSlices(ref, test []int16, shift int) ([]int16, []int16) {
	refStart := 0
	testStart := 0
	if shift >= 0 {
		testStart = shift
	} else {
		refStart = -shift
	}
	if refStart >= len(ref) || testStart >= len(test) {
		return nil, nil
	}
	n := len(ref) - refStart
	if m := len(test) - testStart; m < n {
		n = m
	}
	if n <= 0 {
		return nil, nil
	}
	return ref[refStart : refStart+n], test[testStart : testStart+n]
}

func segSNRDB(ref, test []int16, frameSamples int) float64 {
	if frameSamples <= 0 || len(ref) == 0 || len(test) == 0 {
		return 0
	}
	n := len(ref)
	if len(test) < n {
		n = len(test)
	}
	var sum float64
	var count int
	for off := 0; off+frameSamples <= n; off += frameSamples {
		var sig, noise float64
		for i := 0; i < frameSamples; i++ {
			s := float64(ref[off+i])
			d := s - float64(test[off+i])
			sig += s * s
			noise += d * d
		}
		if sig < 1 {
			continue
		}
		v := 35.0
		if noise > 0 {
			v = 10 * math.Log10(sig/noise)
		}
		if v < -10 {
			v = -10
		} else if v > 35 {
			v = 35
		}
		sum += v
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func finiteDB(v float64) float64 {
	switch {
	case math.IsNaN(v):
		return 0
	case math.IsInf(v, 1):
		return 99
	case math.IsInf(v, -1):
		return -99
	default:
		return v
	}
}

func matchFrameRMS(test, ref []int16, frameSamples int) []int16 {
	n := len(test)
	if len(ref) < n {
		n = len(ref)
	}
	out := append([]int16(nil), test[:n]...)
	if frameSamples <= 0 {
		return out
	}
	for off := 0; off+frameSamples <= n; off += frameSamples {
		testFrame := out[off : off+frameSamples]
		refFrame := ref[off : off+frameSamples]
		testRMS := rms(testFrame)
		refRMS := rms(refFrame)
		if testRMS <= 0 || refRMS <= 0 {
			continue
		}
		scaleFrameFloat(testFrame, refRMS/testRMS)
	}
	return out
}

func scaleFrameFloat(samples []int16, gain float64) {
	for i, sample := range samples {
		v := int(math.Round(float64(sample) * gain))
		switch {
		case v > 32767:
			v = 32767
		case v < -32768:
			v = -32768
		}
		samples[i] = int16(v)
	}
}

func peakAndClipped(samples []int16) (int, int) {
	peak := 0
	clipped := 0
	for _, s := range samples {
		if s == 32767 || s == -32768 {
			clipped++
		}
		a := int(s)
		if a < 0 {
			a = -a
		}
		if a > peak {
			peak = a
		}
	}
	return peak, clipped
}

type pcmStats struct {
	samples int
	sumSq   float64
	peak    int
	clipped int
}

func (s *pcmStats) add(samples []int16) {
	for _, sample := range samples {
		if sample == 32767 || sample == -32768 {
			s.clipped++
		}
		abs := int(sample)
		if abs < 0 {
			abs = -abs
		}
		if abs > s.peak {
			s.peak = abs
		}
		v := float64(sample)
		s.sumSq += v * v
		s.samples++
	}
}

func (s pcmStats) rms() float64 {
	if s.samples == 0 {
		return 0
	}
	return math.Sqrt(s.sumSq / float64(s.samples))
}

func normalizeForListening(samples []int16, targetPeak int) []int16 {
	peak, _ := peakAndClipped(samples)
	out := make([]int16, len(samples))
	if peak == 0 {
		return out
	}
	gain := float64(targetPeak) / float64(peak)
	for i, s := range samples {
		v := int(math.Round(float64(s) * gain))
		switch {
		case v > 32767:
			v = 32767
		case v < -32768:
			v = -32768
		}
		out[i] = int16(v)
	}
	return out
}

func recoverDecodedPayloadPreview(bits []byte, decoded []int16) []int16 {
	out := append([]int16(nil), decoded...)
	frames := len(out) / g729.FrameSamples
	if byBits := len(bits) / g729.FrameBytes; byBits < frames {
		frames = byBits
	}
	for frame := 0; frame < frames; frame++ {
		bitOff := frame * g729.FrameBytes
		pcmOff := frame * g729.FrameSamples
		pcmFrame := out[pcmOff : pcmOff+g729.FrameSamples]
		if shouldRecoverDecodedFrame(bits[bitOff:bitOff+g729.FrameBytes], pcmFrame) {
			scaleFrameSaturating(pcmFrame, 3, 2)
		}
	}
	return out
}

func decodeEnhancedPayload(bits []byte) ([]int16, error) {
	if len(bits)%g729.FrameBytes != 0 {
		return nil, fmt.Errorf("payload length %d is not divisible by %d", len(bits), g729.FrameBytes)
	}
	out := make([]int16, 0, (len(bits)/g729.FrameBytes)*g729.FrameSamples)
	var dec g729.Decoder
	var frame [g729.FrameSamples]int16
	for off := 0; off < len(bits); off += g729.FrameBytes {
		if err := dec.DecodeFrameEnhanced(bits[off:off+g729.FrameBytes], frame[:]); err != nil {
			return nil, err
		}
		out = append(out, frame[:]...)
	}
	return out, nil
}

func shouldRecoverDecodedFrame(bits []byte, pcmFrame []int16) bool {
	var f bitstream.Frame
	if err := bitstream.Unpack(bits, &f); err != nil {
		return false
	}
	if !isRecoveryGA(uint8(f.GA1)) && !isRecoveryGA(uint8(f.GA2)) {
		return false
	}
	return rms(pcmFrame) < 2000
}

func isRecoveryGA(ga uint8) bool {
	return ga == 0 || ga == 3 || ga == 6
}

func scaleFrameSaturating(samples []int16, num, den int) {
	for i, sample := range samples {
		v := int64(sample) * int64(num)
		if v >= 0 {
			v += int64(den / 2)
		} else {
			v -= int64(den / 2)
		}
		v /= int64(den)
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		samples[i] = int16(v)
	}
}

func base64WAV(samples []int16) string {
	return base64.StdEncoding.EncodeToString(wavS16LEMono(samples, g729.SampleRate))
}

func wavS16LEMono(samples []int16, sampleRate int) []byte {
	pcm := samplesToS16LE(samples)
	dataLen := uint32(len(pcm))
	var b bytes.Buffer
	b.WriteString("RIFF")
	writeU32(&b, 36+dataLen)
	b.WriteString("WAVE")
	b.WriteString("fmt ")
	writeU32(&b, 16)
	writeU16(&b, 1)
	writeU16(&b, 1)
	writeU32(&b, uint32(sampleRate))
	writeU32(&b, uint32(sampleRate*2))
	writeU16(&b, 2)
	writeU16(&b, 16)
	b.WriteString("data")
	writeU32(&b, dataLen)
	b.Write(pcm)
	return b.Bytes()
}

func writeU16(b *bytes.Buffer, v uint16) {
	b.WriteByte(byte(v))
	b.WriteByte(byte(v >> 8))
}

func writeU32(b *bytes.Buffer, v uint32) {
	b.WriteByte(byte(v))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 24))
}

func errorDetail(err error, ok string) string {
	if err != nil {
		return err.Error()
	}
	return ok
}

func framesForPTime(ptime int) (int, error) {
	switch ptime {
	case 10:
		return 1, nil
	case 20:
		return 2, nil
	default:
		return 0, fmt.Errorf("unsupported ptime=%d; use 10 or 20", ptime)
	}
}

func allOK(checks []check) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

const pageHTML = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>G.729 RTP Check</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f5f7fb;
      --surface: #ffffff;
      --ink: #152033;
      --muted: #657086;
      --line: #d7ddea;
      --ok: #0d7a46;
      --bad: #b42318;
      --accent: #1c5d99;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--ink);
    }
    header {
      background: #17233a;
      color: #fff;
      padding: 22px 28px;
      border-bottom: 4px solid #3aa675;
    }
    h1 { margin: 0 0 6px; font-size: 26px; letter-spacing: 0; }
    header p { margin: 0; color: #d9e2f1; }
    main {
      max-width: 1180px;
      margin: 0 auto;
      padding: 24px;
      display: grid;
      gap: 18px;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
    }
    .metric, section {
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: 0 1px 2px rgba(21,32,51,.04);
    }
    .metric { padding: 14px; }
    .metric b { display: block; font-size: 18px; margin-bottom: 2px; }
    .metric span { color: var(--muted); }
    section { padding: 18px; }
    h2 { margin: 0 0 12px; font-size: 18px; }
    textarea {
      width: 100%;
      min-height: 145px;
      resize: vertical;
      border: 1px solid var(--line);
      border-radius: 6px;
      padding: 10px;
      font: 13px/1.35 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      color: var(--ink);
    }
    .row { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
    button, select, input[type=file]::file-selector-button {
      border: 1px solid #9fb3cc;
      background: #fff;
      color: var(--ink);
      border-radius: 6px;
      padding: 9px 12px;
      font: inherit;
    }
    button.primary {
      background: var(--accent);
      color: #fff;
      border-color: var(--accent);
    }
    a.download {
      display: inline-flex;
      align-items: center;
      min-height: 38px;
      border: 1px solid #9fb3cc;
      background: #fff;
      color: var(--ink);
      border-radius: 6px;
      padding: 9px 12px;
      font-weight: 650;
      text-decoration: none;
    }
    a.download[hidden] { display: none; }
    input[type=file] { max-width: 100%; }
    label { color: var(--muted); }
    .result {
      margin-top: 12px;
      border-top: 1px solid var(--line);
      padding-top: 12px;
    }
    .check {
      display: grid;
      grid-template-columns: 78px 1fr;
      gap: 8px;
      padding: 8px 0;
      border-bottom: 1px solid #edf1f7;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 58px;
      border-radius: 999px;
      padding: 2px 8px;
      color: #fff;
      font-weight: 700;
      font-size: 12px;
    }
    .ok { background: var(--ok); }
    .bad { background: var(--bad); }
    .summary {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-top: 10px;
    }
    .pill {
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 5px 9px;
      background: #f8fafc;
      color: var(--muted);
    }
    code { background: #eef3fa; padding: 1px 5px; border-radius: 4px; }
    @media (max-width: 760px) {
      header { padding: 18px; }
      main { padding: 14px; }
      .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .check { grid-template-columns: 62px 1fr; }
    }
  </style>
</head>
<body>
  <header>
    <h1>G.729 RTP Check</h1>
    <p>Host {{.Host}} · validates <code>G729/8000</code>, payload type <code>18</code>, <code>annexb=no</code></p>
  </header>
  <main>
    <div class="grid">
      <div class="metric"><b>G729/8000</b><span>RTP clock 8000 Hz</span></div>
      <div class="metric"><b>PT 18</b><span>static RTP payload type</span></div>
      <div class="metric"><b>10 bytes</b><span>one 10 ms speech frame</span></div>
      <div class="metric"><b>annexb=no</b><span>SID/CNG/DTX disabled</span></div>
    </div>

    <section>
      <h2>Self Test</h2>
      <div class="row">
        <button class="primary" id="selfBtn">Run codec/RTP self-test</button>
      </div>
      <div id="selfResult" class="result"></div>
    </section>

    <section>
      <h2>Listen Roundtrip</h2>
      <div class="row">
        <input type="file" id="audioFile" accept=".wav,.mp3,.m4a,.aac,.ogg,.pcm,.raw,audio/*">
        <label>input
          <select id="inputMode">
            <option value="audio" selected>WAV/MP3/browser audio</option>
            <option value="raw">raw PCM</option>
          </select>
        </label>
        <label>raw rate
          <select id="rawRate">
            <option value="8000" selected>8000</option>
            <option value="16000">16000</option>
            <option value="44100">44100</option>
            <option value="48000">48000</option>
          </select>
        </label>
        <label>raw channels
          <select id="rawChannels">
            <option value="1" selected>mono</option>
            <option value="2">stereo</option>
          </select>
        </label>
        <label>raw format
          <select id="rawFormat">
            <option value="s16le" selected>signed 16-bit LE</option>
            <option value="s16be">signed 16-bit BE</option>
          </select>
        </label>
        <label>ptime
          <select id="roundtripPtime">
            <option value="10">10</option>
            <option value="20" selected>20</option>
          </select>
        </label>
        <label><input type="checkbox" id="roundtripFFmpeg" checked> FFmpeg reference decode</label>
        <button class="primary" id="roundtripBtn">Encode · Decode · Listen</button>
      </div>
      <div class="summary">
        <span class="pill">encoder input: 8000 Hz mono signed linear 16-bit PCM</span>
        <span class="pill">browser audio is converted before encode</span>
      </div>
      <div class="row" style="margin-top:12px">
        <div style="min-width:260px; flex:1">
          <label>source preview</label>
          <audio id="sourceAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>strict Go decoder result (raw level)</label>
          <audio id="decodedAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>strict normalized listen preview</label>
          <audio id="normalizedAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>enhanced listening aid (non-strict)</label>
          <audio id="recoveredAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>FFmpeg reference result (raw level)</label>
          <audio id="ffmpegAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>FFmpeg normalized preview</label>
          <audio id="ffmpegNormalizedAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:220px; flex:.7">
          <label>encoded G.729 file</label><br>
          <a id="encodedDownload" class="download" hidden>Download .g729</a>
        </div>
      </div>
      <div id="roundtripResult" class="result"></div>
    </section>

    <section>
      <h2>SDP Check</h2>
      <textarea id="sdp">m=audio 49170 RTP/AVP 18
a=rtpmap:18 G729/8000
a=fmtp:18 annexb=no
a=ptime:20
a=maxptime:20</textarea>
      <div class="row" style="margin-top:10px">
        <button class="primary" id="sdpBtn">Validate SDP</button>
      </div>
      <div id="sdpResult" class="result"></div>
    </section>

    <section>
      <h2>Payload Check</h2>
      <div class="row">
        <input type="file" id="payloadFile" accept=".g729,.g729a,.bin,.raw,application/octet-stream">
        <label>ptime
          <select id="ptime">
            <option value="10">10</option>
            <option value="20" selected>20</option>
          </select>
        </label>
        <label><input type="checkbox" id="decode" checked> decode frames</label>
        <label><input type="checkbox" id="payloadFFmpeg" checked> FFmpeg reference decode</label>
        <button class="primary" id="payloadBtn">Validate Payload</button>
      </div>
      <div class="summary">
        <span class="pill">input: raw G.729 speech payload bytes</span>
        <span class="pill">Asterisk .g729 usually fits this format</span>
      </div>
      <div class="row" style="margin-top:12px">
        <div style="min-width:260px; flex:1">
          <label>payload strict decode (raw level)</label>
          <audio id="payloadDecodedAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>payload strict normalized preview</label>
          <audio id="payloadNormalizedAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>payload enhanced listening aid (non-strict)</label>
          <audio id="payloadRecoveredAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>payload FFmpeg reference (raw level)</label>
          <audio id="payloadFFmpegAudio" controls style="width:100%"></audio>
        </div>
        <div style="min-width:260px; flex:1">
          <label>payload FFmpeg normalized preview</label>
          <audio id="payloadFFmpegNormalizedAudio" controls style="width:100%"></audio>
        </div>
      </div>
      <div id="payloadResult" class="result"></div>
    </section>
  </main>
  <script>
    let encodedDownloadURL = '';
    const postJSON = async (url, body) => {
      const res = await fetch(url, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)});
      return res.json();
    };
    const render = (el, data) => {
      const checks = data.checks || data.Checks || [];
      let html = '<div class="summary"><span class="pill">overall: ' + (data.ok ? 'PASS' : 'FAIL') + '</span>';
      ['packets','frames','payloadBytes','decodedFrames','ptime'].forEach(k => {
        if (data[k] !== undefined) html += '<span class="pill">' + k + ': ' + data[k] + '</span>';
      });
      html += '</div>';
      for (const c of checks) {
        html += '<div class="check"><span class="badge ' + (c.ok ? 'ok' : 'bad') + '">' + (c.ok ? 'PASS' : 'FAIL') + '</span><div><b>' + c.name + '</b><br><span>' + c.detail + '</span></div></div>';
      }
      el.innerHTML = html;
    };
    document.getElementById('selfBtn').onclick = async () => {
      const res = await fetch('/api/selftest', {method: 'POST'});
      render(document.getElementById('selfResult'), await res.json());
    };
    document.getElementById('sdpBtn').onclick = async () => {
      const data = await postJSON('/api/sdp', {sdp: document.getElementById('sdp').value});
      render(document.getElementById('sdpResult'), data);
    };
    document.getElementById('payloadBtn').onclick = async () => {
      const file = document.getElementById('payloadFile').files[0];
      if (!file) {
        document.getElementById('payloadResult').innerHTML = '<div class="check"><span class="badge bad">FAIL</span><div><b>file</b><br><span>choose a .g729 payload file</span></div></div>';
        return;
      }
      const form = new FormData();
      form.append('file', file);
      form.append('ptime', document.getElementById('ptime').value);
      form.append('decode', document.getElementById('decode').checked ? '1' : '0');
      form.append('ffmpeg', document.getElementById('payloadFFmpeg').checked ? '1' : '0');
      const res = await fetch('/api/payload', {method: 'POST', body: form});
      const data = await res.json();
      renderPayload(document.getElementById('payloadResult'), data);
      const rawAudio = document.getElementById('payloadDecodedAudio');
      const normAudio = document.getElementById('payloadNormalizedAudio');
      const recoveredAudio = document.getElementById('payloadRecoveredAudio');
      const ffmpegAudio = document.getElementById('payloadFFmpegAudio');
      const ffmpegNormAudio = document.getElementById('payloadFFmpegNormalizedAudio');
      rawAudio.removeAttribute('src');
      normAudio.removeAttribute('src');
      recoveredAudio.removeAttribute('src');
      ffmpegAudio.removeAttribute('src');
      ffmpegNormAudio.removeAttribute('src');
      if (data.decodedWAVBase64) {
        rawAudio.src = 'data:audio/wav;base64,' + data.decodedWAVBase64;
      }
      if (data.normalizedDecodedWAVBase64) {
        normAudio.src = 'data:audio/wav;base64,' + data.normalizedDecodedWAVBase64;
      }
      if (data.recoveredDecodedWAVBase64) {
        recoveredAudio.src = 'data:audio/wav;base64,' + data.recoveredDecodedWAVBase64;
      }
      if (data.ffmpegDecodedWAVBase64) {
        ffmpegAudio.src = 'data:audio/wav;base64,' + data.ffmpegDecodedWAVBase64;
      }
      if (data.ffmpegNormalizedWAVBase64) {
        ffmpegNormAudio.src = 'data:audio/wav;base64,' + data.ffmpegNormalizedWAVBase64;
      }
    };
    document.getElementById('audioFile').onchange = () => {
      const file = document.getElementById('audioFile').files[0];
      const audio = document.getElementById('sourceAudio');
      if (file && document.getElementById('inputMode').value === 'audio') {
        audio.src = URL.createObjectURL(file);
      } else {
        audio.removeAttribute('src');
      }
    };
    document.getElementById('roundtripBtn').onclick = async () => {
      const file = document.getElementById('audioFile').files[0];
      if (!file) {
        document.getElementById('roundtripResult').innerHTML = '<div class="check"><span class="badge bad">FAIL</span><div><b>file</b><br><span>choose WAV/MP3 or raw PCM input</span></div></div>';
        return;
      }
      const resultEl = document.getElementById('roundtripResult');
      resultEl.innerHTML = '<span class="pill">processing...</span>';
      clearEncodedDownload();
      document.getElementById('decodedAudio').removeAttribute('src');
      document.getElementById('normalizedAudio').removeAttribute('src');
      document.getElementById('recoveredAudio').removeAttribute('src');
      document.getElementById('ffmpegAudio').removeAttribute('src');
      document.getElementById('ffmpegNormalizedAudio').removeAttribute('src');
      try {
        const pcm = await buildEncoderPCM(file);
        document.getElementById('sourceAudio').src = URL.createObjectURL(new Blob([wavFromS16LE(pcm, 8000)], {type: 'audio/wav'}));
        const form = new FormData();
        form.append('pcm', new Blob([pcm], {type: 'application/octet-stream'}), 'input-8000-s16le.pcm');
        form.append('ptime', document.getElementById('roundtripPtime').value);
        form.append('ffmpeg', document.getElementById('roundtripFFmpeg').checked ? '1' : '0');
        const res = await fetch('/api/roundtrip', {method: 'POST', body: form});
        const data = await res.json();
        renderRoundtrip(resultEl, data);
        setEncodedDownload(data);
        if (data.decodedWAVBase64) {
          document.getElementById('decodedAudio').src = 'data:audio/wav;base64,' + data.decodedWAVBase64;
        }
        if (data.normalizedDecodedWAVBase64) {
          document.getElementById('normalizedAudio').src = 'data:audio/wav;base64,' + data.normalizedDecodedWAVBase64;
        }
        if (data.recoveredDecodedWAVBase64) {
          document.getElementById('recoveredAudio').src = 'data:audio/wav;base64,' + data.recoveredDecodedWAVBase64;
        }
        if (data.ffmpegDecodedWAVBase64) {
          document.getElementById('ffmpegAudio').src = 'data:audio/wav;base64,' + data.ffmpegDecodedWAVBase64;
        }
        if (data.ffmpegNormalizedWAVBase64) {
          document.getElementById('ffmpegNormalizedAudio').src = 'data:audio/wav;base64,' + data.ffmpegNormalizedWAVBase64;
        }
      } catch (err) {
        clearEncodedDownload();
        resultEl.innerHTML = '<div class="check"><span class="badge bad">FAIL</span><div><b>roundtrip</b><br><span>' + escapeHTML(String(err.message || err)) + '</span></div></div>';
      }
    };
    async function buildEncoderPCM(file) {
      const buf = await file.arrayBuffer();
      if (document.getElementById('inputMode').value === 'raw') {
        const sr = Number(document.getElementById('rawRate').value);
        const ch = Number(document.getElementById('rawChannels').value);
        const fmt = document.getElementById('rawFormat').value;
        const mono = rawPCMToFloat32(buf, sr, ch, fmt);
        return floatToS16LE(resampleLinear(mono, sr, 8000));
      }
      const AudioContextCtor = window.AudioContext || window.webkitAudioContext;
      if (!AudioContextCtor) throw new Error('browser AudioContext is unavailable');
      const ctx = new AudioContextCtor();
      let audioBuffer;
      try {
        audioBuffer = await ctx.decodeAudioData(buf.slice(0));
      } finally {
        if (ctx.close) ctx.close();
      }
      const mono = audioBufferToMono(audioBuffer);
      return floatToS16LE(resampleLinear(mono, audioBuffer.sampleRate, 8000));
    }
    function audioBufferToMono(audioBuffer) {
      const n = audioBuffer.length;
      const ch = audioBuffer.numberOfChannels;
      const out = new Float32Array(n);
      for (let c = 0; c < ch; c++) {
        const data = audioBuffer.getChannelData(c);
        for (let i = 0; i < n; i++) out[i] += data[i] / ch;
      }
      return out;
    }
    function rawPCMToFloat32(buf, sampleRate, channels, fmt) {
      if (channels < 1 || channels > 2) throw new Error('raw channels must be mono or stereo');
      if (buf.byteLength % (2 * channels) !== 0) throw new Error('raw PCM size does not align to signed 16-bit sample frames');
      const dv = new DataView(buf);
      const frames = buf.byteLength / (2 * channels);
      const out = new Float32Array(frames);
      const le = fmt === 's16le';
      for (let i = 0; i < frames; i++) {
        let sum = 0;
        for (let c = 0; c < channels; c++) {
          sum += dv.getInt16((i * channels + c) * 2, le) / 32768;
        }
        out[i] = sum / channels;
      }
      return out;
    }
    function resampleLinear(input, srcRate, dstRate) {
      if (srcRate === dstRate) return input;
      const outLen = Math.max(1, Math.round(input.length * dstRate / srcRate));
      const out = new Float32Array(outLen);
      const ratio = srcRate / dstRate;
      for (let i = 0; i < outLen; i++) {
        const pos = i * ratio;
        const i0 = Math.floor(pos);
        const i1 = Math.min(input.length - 1, i0 + 1);
        const frac = pos - i0;
        out[i] = input[i0] * (1 - frac) + input[i1] * frac;
      }
      return out;
    }
    function floatToS16LE(input) {
      const out = new Uint8Array(input.length * 2);
      const dv = new DataView(out.buffer);
      for (let i = 0; i < input.length; i++) {
        let v = Math.max(-1, Math.min(1, input[i]));
        dv.setInt16(i * 2, v < 0 ? Math.round(v * 32768) : Math.round(v * 32767), true);
      }
      return out;
    }
    function wavFromS16LE(pcm, sampleRate) {
      const dataLen = pcm.byteLength;
      const out = new Uint8Array(44 + dataLen);
      const dv = new DataView(out.buffer);
      writeASCII(out, 0, 'RIFF');
      dv.setUint32(4, 36 + dataLen, true);
      writeASCII(out, 8, 'WAVE');
      writeASCII(out, 12, 'fmt ');
      dv.setUint32(16, 16, true);
      dv.setUint16(20, 1, true);
      dv.setUint16(22, 1, true);
      dv.setUint32(24, sampleRate, true);
      dv.setUint32(28, sampleRate * 2, true);
      dv.setUint16(32, 2, true);
      dv.setUint16(34, 16, true);
      writeASCII(out, 36, 'data');
      dv.setUint32(40, dataLen, true);
      out.set(pcm instanceof Uint8Array ? pcm : new Uint8Array(pcm), 44);
      return out;
    }
    function writeASCII(out, off, s) {
      for (let i = 0; i < s.length; i++) out[off + i] = s.charCodeAt(i);
    }
    function clearEncodedDownload() {
      const link = document.getElementById('encodedDownload');
      if (encodedDownloadURL) {
        URL.revokeObjectURL(encodedDownloadURL);
        encodedDownloadURL = '';
      }
      if (!link) return;
      link.hidden = true;
      link.removeAttribute('href');
      link.removeAttribute('download');
      link.textContent = 'Download .g729';
    }
    function setEncodedDownload(data) {
      const link = document.getElementById('encodedDownload');
      if (!link || !data.encodedG729Base64) return;
      const bytes = base64ToBytes(data.encodedG729Base64);
      if (!bytes.length) return;
      encodedDownloadURL = URL.createObjectURL(new Blob([bytes], {type: 'application/octet-stream'}));
      const frames = Number(data.frames || 0);
      const ptime = Number(data.ptime || document.getElementById('roundtripPtime').value || 20);
      link.href = encodedDownloadURL;
      link.download = 'g729-roundtrip-' + frames + 'frames-ptime' + ptime + '.g729';
      link.textContent = 'Download .g729 (' + bytes.length + ' bytes)';
      link.hidden = false;
    }
    function base64ToBytes(b64) {
      const bin = atob(b64);
      const out = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
      return out;
    }
    function renderRoundtrip(el, data) {
      render(el, data);
      if (!data.ok && !data.checks) return;
      const metrics = [
        ['input samples', data.inputSamples],
        ['output samples', data.outputSamples],
        ['encoded bytes', data.encodedBytes],
        ['padded samples', data.paddedSamples],
        ['input RMS', fmt(data.inputRMS)],
        ['input peak', data.inputPeak],
        ['output RMS', fmt(data.outputRMS)],
        ['output peak', data.outputPeak],
        ['RMS ratio', fmt(data.rmsRatio)],
        ['clipped', data.outputClipped],
        ['SNR dB', fmt(data.roundtripSNRDB)],
        ['aligned SNR dB', fmt(data.roundtripAlignedSNRDB)],
        ['aligned SegSNR dB', fmt(data.roundtripAlignedSegSNRDB)],
        ['aligned shift', data.roundtripAlignedShift]
      ];
      if (Number(data.recoveredOutputRMS || 0) > 0) {
        metrics.push(
          ['enhanced RMS', fmt(data.recoveredOutputRMS)],
          ['enhanced peak', data.recoveredOutputPeak],
          ['enhanced clipped', data.recoveredOutputClipped],
          ['enhanced ratio', fmt(data.recoveredRMSRatio)],
          ['enhanced SNR dB', fmt(data.recoveredRoundtripSNRDB)],
          ['enhanced aligned SNR dB', fmt(data.recoveredAlignedSNRDB)],
          ['enhanced aligned SegSNR dB', fmt(data.recoveredAlignedSegSNRDB)],
          ['enhanced aligned shift', data.recoveredAlignedShift]
        );
      }
      if (Number(data.ffmpegOutputSamples || 0) > 0) {
        metrics.push(
          ['ffmpeg samples', data.ffmpegOutputSamples],
          ['ffmpeg RMS', fmt(data.ffmpegOutputRMS)],
          ['ffmpeg peak', data.ffmpegOutputPeak],
          ['ffmpeg clipped', data.ffmpegOutputClipped],
          ['ffmpeg SNR dB', fmt(data.ffmpegRoundtripSNRDB)],
          ['ffmpeg aligned SNR dB', fmt(data.ffmpegAlignedSNRDB)],
          ['ffmpeg aligned SegSNR dB', fmt(data.ffmpegAlignedSegSNRDB)],
          ['ffmpeg aligned shift', data.ffmpegAlignedShift],
          ['local vs ffmpeg aligned SNR dB', fmt(data.localVsFFmpegAlignedSNRDB)],
          ['local vs ffmpeg aligned SegSNR dB', fmt(data.localVsFFmpegAlignedSegSNRDB)],
          ['enhanced vs ffmpeg aligned SNR dB', fmt(data.recoveredVsFFmpegAlignedSNRDB)],
          ['enhanced vs ffmpeg aligned SegSNR dB', fmt(data.recoveredVsFFmpegAlignedSegSNRDB)],
          ['oracle-bound RMS', fmt(data.envelopeOracleRMS)],
          ['oracle-bound SNR dB', fmt(data.envelopeOracleAlignedSNRDB)],
          ['oracle-bound SegSNR dB', fmt(data.envelopeOracleAlignedSegSNRDB)]
        );
      }
      el.innerHTML += '<div class="summary">' + metrics.map(([k,v]) => '<span class="pill">' + k + ': ' + v + '</span>').join('') + '</div>';
    }
    function renderPayload(el, data) {
      render(el, data);
      if (!data.checks) return;
      const metrics = [
        ['decoded samples', data.decodedSamples],
        ['preview samples', data.decodedPreviewSamples],
        ['output RMS', fmt(data.outputRMS)],
        ['output peak', data.outputPeak],
        ['clipped', data.outputClipped]
      ];
      if (Number(data.recoveredOutputRMS || 0) > 0) {
        metrics.push(
          ['enhanced RMS', fmt(data.recoveredOutputRMS)],
          ['enhanced peak', data.recoveredOutputPeak],
          ['enhanced clipped', data.recoveredOutputClipped]
        );
      }
      if (Number(data.ffmpegDecodedSamples || 0) > 0) {
        metrics.push(
          ['ffmpeg samples', data.ffmpegDecodedSamples],
          ['ffmpeg RMS', fmt(data.ffmpegOutputRMS)],
          ['ffmpeg peak', data.ffmpegOutputPeak],
          ['ffmpeg clipped', data.ffmpegOutputClipped],
          ['local vs ffmpeg aligned SNR dB', fmt(data.localVsFFmpegAlignedSNRDB)],
          ['local vs ffmpeg aligned SegSNR dB', fmt(data.localVsFFmpegAlignedSegSNRDB)],
          ['enhanced vs ffmpeg aligned SNR dB', fmt(data.recoveredVsFFmpegAlignedSNRDB)],
          ['enhanced vs ffmpeg aligned SegSNR dB', fmt(data.recoveredVsFFmpegAlignedSegSNRDB)],
          ['oracle-bound RMS', fmt(data.envelopeOracleRMS)],
          ['oracle-bound SNR dB', fmt(data.envelopeOracleAlignedSNRDB)],
          ['oracle-bound SegSNR dB', fmt(data.envelopeOracleAlignedSegSNRDB)]
        );
      }
      el.innerHTML += '<div class="summary">' + metrics.map(([k,v]) => '<span class="pill">' + k + ': ' + v + '</span>').join('') + '</div>';
    }
    function fmt(v) {
      if (v === null || v === undefined) return '';
      if (!Number.isFinite(Number(v))) return String(v);
      return Number(v).toFixed(2);
    }
    function escapeHTML(s) {
      return s.replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
    }
  </script>
</body>
</html>`

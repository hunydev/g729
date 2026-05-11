package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/hunydev/g729"
)

type pathPCM struct {
	name string
	pcm  []byte
}

type frameStat struct {
	frame       int
	startMS     float64
	sourcePeak  int
	ourPeak     int
	bcgPeak     int
	ourClips    int
	bcgClips    int
	ourRMS      float64
	bcgRMS      float64
	sourceRMS   float64
	ourMinusBCG float64
	ourSNR      float64
	bcgSNR      float64
	ourCorr     float64
	bcgCorr     float64
	ourErrRMS   float64
	bcgErrRMS   float64
	ourHighRMS  float64
	bcgHighRMS  float64
	ourHighDB   float64
	bcgHighDB   float64
}

func main() {
	input := flag.String("input", "", "input audio file")
	bcgPath := flag.String("bcg", defaultBCG729EncoderPath(), "black-box bcg729 encoder executable")
	gateRMS := flag.Float64("gate-rms", 0, "zero local-encoder input frames whose source RMS is below this threshold")
	startFrame := flag.Int("start", -1, "first frame to print detailed diagnostics for")
	endFrame := flag.Int("end", -1, "last frame to print detailed diagnostics for")
	topLimit := flag.Int("top", 24, "number of top-ranked frames to print")
	flag.Parse()
	if *input == "" {
		fmt.Fprintln(os.Stderr, "set -input")
		os.Exit(2)
	}

	tmp, err := os.MkdirTemp("", "g729-frame-diag-*")
	must(err)
	defer os.RemoveAll(tmp)

	source := mustPCMFromInput(tmp, *input)
	padded := pad(source)
	ourPayload := mustLocalEncode(padded, *gateRMS)
	bcgPayload := mustExternalEncode(*bcgPath, padded)
	paths := []pathPCM{
		{"our encode -> local decode", mustLocalDecode(ourPayload)},
		{"our encode -> ffmpeg decode", mustFFmpegDecode(tmp, "our", ourPayload)},
		{"bcg729 encode -> local decode", mustLocalDecode(bcgPayload)},
		{"bcg729 encode -> ffmpeg decode", mustFFmpegDecode(tmp, "bcg", bcgPayload)},
	}

	fmt.Printf("input=%s samples=%d frames=%d padded=%d\n", *input, len(source)/2, len(padded)/(g729.FrameSamples*2), len(padded)/2-len(source)/2)
	for _, p := range paths {
		printPathSummary(p.name, p.pcm)
	}
	fmt.Println()

	ourFF := paths[1].pcm
	bcgFF := paths[3].pcm
	ourSNR, ourCorr, ourRatio, ourLag := globalMetric(padded, ourFF)
	bcgSNR, bcgCorr, bcgRatio, bcgLag := globalMetric(padded, bcgFF)
	stats := collectFrameStats(padded, ourFF, bcgFF, ourLag, bcgLag)
	printOverallMetrics(ourSNR, ourCorr, ourRatio, ourLag, bcgSNR, bcgCorr, bcgRatio, bcgLag)
	fmt.Println()
	printBitstreamSummary(ourPayload, bcgPayload)
	if *startFrame >= 0 && *endFrame >= *startFrame {
		fmt.Println()
		printFrameStatsRange(stats, *startFrame, *endFrame)
		fmt.Println()
		printFrameFields(ourPayload, bcgPayload, *startFrame, *endFrame)
	}
	fmt.Println()
	printEnergyBins(stats)
	fmt.Println()
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].ourClips != stats[j].ourClips {
			return stats[i].ourClips > stats[j].ourClips
		}
		if stats[i].ourPeak != stats[j].ourPeak {
			return stats[i].ourPeak > stats[j].ourPeak
		}
		return stats[i].ourMinusBCG > stats[j].ourMinusBCG
	})

	fmt.Println("top frames by our ffmpeg decoded clipping/peak:")
	fmt.Println("frame,start_ms,source_peak,our_peak,bcg_peak,our_clip_samples,bcg_clip_samples,source_rms,our_rms,bcg_rms,our_minus_bcg_rms")
	limit := *topLimit
	if limit < 0 {
		limit = 0
	}
	if len(stats) < limit {
		limit = len(stats)
	}
	for _, s := range stats[:limit] {
		fmt.Printf("%d,%.1f,%d,%d,%d,%d,%d,%.1f,%.1f,%.1f,%.1f\n",
			s.frame, s.startMS, s.sourcePeak, s.ourPeak, s.bcgPeak, s.ourClips, s.bcgClips,
			s.sourceRMS, s.ourRMS, s.bcgRMS, s.ourMinusBCG)
	}
	fmt.Println()

	sort.Slice(stats, func(i, j int) bool {
		gapI := stats[i].bcgSNR - stats[i].ourSNR
		gapJ := stats[j].bcgSNR - stats[j].ourSNR
		if gapI != gapJ {
			return gapI > gapJ
		}
		return stats[i].bcgCorr-stats[i].ourCorr > stats[j].bcgCorr-stats[j].ourCorr
	})
	fmt.Println("top frames by bcg729 ffmpeg decode SNR advantage:")
	fmt.Println("frame,start_ms,source_rms,our_snr,bcg_snr,snr_gap,our_corr,bcg_corr,corr_gap,our_rms,bcg_rms,our_peak,bcg_peak")
	for _, s := range stats[:limit] {
		fmt.Printf("%d,%.1f,%.1f,%.2f,%.2f,%.2f,%.4f,%.4f,%.4f,%.1f,%.1f,%d,%d\n",
			s.frame, s.startMS, s.sourceRMS, s.ourSNR, s.bcgSNR, s.bcgSNR-s.ourSNR,
			s.ourCorr, s.bcgCorr, s.bcgCorr-s.ourCorr, s.ourRMS, s.bcgRMS, s.ourPeak, s.bcgPeak)
	}
	fmt.Println()

	sort.Slice(stats, func(i, j int) bool {
		gapI := stats[i].ourHighDB - stats[i].bcgHighDB
		gapJ := stats[j].ourHighDB - stats[j].bcgHighDB
		if gapI != gapJ {
			return gapI > gapJ
		}
		return stats[i].sourceRMS > stats[j].sourceRMS
	})
	fmt.Println("top voiced frames by our high residual excess over bcg729:")
	fmt.Println("frame,start_ms,source_rms,our_high_db,bcg_high_db,high_db_gap,our_high_rms,bcg_high_rms,our_err_rms,bcg_err_rms,our_snr,bcg_snr,our_corr,bcg_corr")
	printed := 0
	for _, s := range stats {
		if printed >= limit {
			break
		}
		if s.sourceRMS < 200 {
			continue
		}
		fmt.Printf("%d,%.1f,%.1f,%.2f,%.2f,%.2f,%.1f,%.1f,%.1f,%.1f,%.2f,%.2f,%.4f,%.4f\n",
			s.frame, s.startMS, s.sourceRMS, s.ourHighDB, s.bcgHighDB, s.ourHighDB-s.bcgHighDB,
			s.ourHighRMS, s.bcgHighRMS, s.ourErrRMS, s.bcgErrRMS, s.ourSNR, s.bcgSNR, s.ourCorr, s.bcgCorr)
		printed++
	}
}

func printOverallMetrics(ourSNR, ourCorr, ourRatio float64, ourLag int, bcgSNR, bcgCorr, bcgRatio float64, bcgLag int) {
	fmt.Println("overall aligned ffmpeg-decode metrics:")
	fmt.Println("path,snr,corr,rms_ratio,lag")
	fmt.Printf("our,%.2f,%.4f,%.4f,%d\n", ourSNR, ourCorr, ourRatio, ourLag)
	fmt.Printf("bcg729,%.2f,%.4f,%.4f,%d\n", bcgSNR, bcgCorr, bcgRatio, bcgLag)
}

func globalMetric(source, out []byte) (snr, corr, rmsRatio float64, lag int) {
	bestLag := 0
	bestCorr := math.Inf(-1)
	for candidate := -240; candidate <= 240; candidate++ {
		_, corr, _, ok := globalMetricAtLag(source, out, candidate)
		if ok && corr > bestCorr {
			bestCorr = corr
			bestLag = candidate
		}
	}
	snr, corr, rmsRatio, _ = globalMetricAtLag(source, out, bestLag)
	return snr, corr, rmsRatio, bestLag
}

func globalMetricAtLag(source, out []byte, lag int) (snr, corr, rmsRatio float64, ok bool) {
	startSource, startOut, n := alignedWindow(len(source)/2, len(out)/2, lag)
	if n <= 0 {
		return math.Inf(-1), 0, 0, false
	}
	var signal, noise, sourceEnergy, outEnergy, cross float64
	for i := 0; i < n; i++ {
		src := float64(sample(source, startSource+i))
		decoded := float64(sample(out, startOut+i))
		diff := src - decoded
		signal += src * src
		noise += diff * diff
		sourceEnergy += src * src
		outEnergy += decoded * decoded
		cross += src * decoded
	}
	if noise > 0 {
		snr = 10 * math.Log10((signal+1)/noise)
	} else {
		snr = 99
	}
	if sourceEnergy > 0 && outEnergy > 0 {
		corr = cross / math.Sqrt(sourceEnergy*outEnergy)
		rmsRatio = math.Sqrt(outEnergy / sourceEnergy)
	}
	return snr, corr, rmsRatio, true
}

func printEnergyBins(stats []frameStat) {
	type bin struct {
		name     string
		min, max float64
	}
	bins := []bin{
		{"rms<100", 0, 100},
		{"100<=rms<500", 100, 500},
		{"500<=rms<2000", 500, 2000},
		{"rms>=2000", 2000, math.Inf(1)},
	}
	fmt.Println("aligned frame metrics by source RMS bin:")
	fmt.Println("bin,frames,our_snr,bcg_snr,snr_gap,our_corr,bcg_corr,our_rms,bcg_rms,our_peak,bcg_peak")
	for _, b := range bins {
		var n int
		var ourSNR, bcgSNR, ourCorr, bcgCorr, ourRMS, bcgRMS float64
		var ourPeak, bcgPeak int
		for _, s := range stats {
			if s.sourceRMS < b.min || s.sourceRMS >= b.max || math.IsInf(s.ourSNR, 0) || math.IsInf(s.bcgSNR, 0) {
				continue
			}
			n++
			ourSNR += s.ourSNR
			bcgSNR += s.bcgSNR
			ourCorr += s.ourCorr
			bcgCorr += s.bcgCorr
			ourRMS += s.ourRMS
			bcgRMS += s.bcgRMS
			ourPeak = max(ourPeak, s.ourPeak)
			bcgPeak = max(bcgPeak, s.bcgPeak)
		}
		if n == 0 {
			continue
		}
		inv := 1 / float64(n)
		fmt.Printf("%s,%d,%.2f,%.2f,%.2f,%.4f,%.4f,%.1f,%.1f,%d,%d\n",
			b.name, n, ourSNR*inv, bcgSNR*inv, (bcgSNR-ourSNR)*inv,
			ourCorr*inv, bcgCorr*inv, ourRMS*inv, bcgRMS*inv, ourPeak, bcgPeak)
	}
}

func printFrameStatsRange(stats []frameStat, startFrame, endFrame int) {
	fmt.Printf("aligned ffmpeg-decode frame diagnostics for frames %d..%d:\n", startFrame, endFrame)
	fmt.Println("frame,start_ms,source_rms,our_snr,bcg_snr,snr_gap,our_corr,bcg_corr,corr_gap,our_err_rms,bcg_err_rms,our_high_db,bcg_high_db,high_db_gap,our_rms,bcg_rms,our_peak,bcg_peak")
	for _, s := range stats {
		if s.frame < startFrame || s.frame > endFrame {
			continue
		}
		fmt.Printf("%d,%.1f,%.1f,%.2f,%.2f,%.2f,%.4f,%.4f,%.4f,%.1f,%.1f,%.2f,%.2f,%.2f,%.1f,%.1f,%d,%d\n",
			s.frame, s.startMS, s.sourceRMS, s.ourSNR, s.bcgSNR, s.bcgSNR-s.ourSNR,
			s.ourCorr, s.bcgCorr, s.bcgCorr-s.ourCorr, s.ourErrRMS, s.bcgErrRMS,
			s.ourHighDB, s.bcgHighDB, s.ourHighDB-s.bcgHighDB,
			s.ourRMS, s.bcgRMS, s.ourPeak, s.bcgPeak)
	}
}

type frameBits struct {
	L0, L1, L2, L3 uint16
	P1, P0, C1, S1 uint16
	GA1, GB1       uint16
	P2, C2, S2     uint16
	GA2, GB2       uint16
}

func printBitstreamSummary(ourPayload, bcgPayload []byte) {
	fields := []string{"L0", "L1", "L2", "L3", "P1", "P0", "C1", "S1", "GA1", "GB1", "P2", "C2", "S2", "GA2", "GB2"}
	matches := make([]int, len(fields))
	frames := min(len(ourPayload), len(bcgPayload)) / g729.FrameBytes
	for f := 0; f < frames; f++ {
		our := unpackFrame(ourPayload[f*g729.FrameBytes : (f+1)*g729.FrameBytes])
		bcg := unpackFrame(bcgPayload[f*g729.FrameBytes : (f+1)*g729.FrameBytes])
		ov := frameValues(our)
		bv := frameValues(bcg)
		for i := range fields {
			if ov[i] == bv[i] {
				matches[i]++
			}
		}
	}
	fmt.Println("field equality against bcg729 black-box payload:")
	for i, name := range fields {
		rate := 0.0
		if frames > 0 {
			rate = float64(matches[i]) * 100 / float64(frames)
		}
		fmt.Printf("  %s: %d/%d %.2f%%\n", name, matches[i], frames, rate)
	}
}

func printFrameFields(ourPayload, bcgPayload []byte, startFrame, endFrame int) {
	fmt.Printf("bitstream fields for frames %d..%d:\n", startFrame, endFrame)
	fmt.Println("frame,path,L0,L1,L2,L3,P1,P0,C1,S1,GA1,GB1,P2,C2,S2,GA2,GB2,hex")
	frames := min(len(ourPayload), len(bcgPayload)) / g729.FrameBytes
	for f := startFrame; f <= endFrame && f < frames; f++ {
		ourRaw := ourPayload[f*g729.FrameBytes : (f+1)*g729.FrameBytes]
		bcgRaw := bcgPayload[f*g729.FrameBytes : (f+1)*g729.FrameBytes]
		printOneFrame(f, "our", unpackFrame(ourRaw), ourRaw)
		printOneFrame(f, "bcg729", unpackFrame(bcgRaw), bcgRaw)
	}
}

func printOneFrame(frame int, name string, f frameBits, raw []byte) {
	fmt.Printf("%d,%s,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%x\n",
		frame, name,
		f.L0, f.L1, f.L2, f.L3,
		f.P1, f.P0, f.C1, f.S1, f.GA1, f.GB1,
		f.P2, f.C2, f.S2, f.GA2, f.GB2,
		raw)
}

func frameValues(f frameBits) []uint16 {
	return []uint16{
		f.L0, f.L1, f.L2, f.L3,
		f.P1, f.P0, f.C1, f.S1, f.GA1, f.GB1,
		f.P2, f.C2, f.S2, f.GA2, f.GB2,
	}
}

func unpackFrame(raw []byte) frameBits {
	r := bitReader{data: raw}
	return frameBits{
		L0: r.read(1), L1: r.read(7), L2: r.read(5), L3: r.read(5),
		P1: r.read(8), P0: r.read(1), C1: r.read(13), S1: r.read(4), GA1: r.read(3), GB1: r.read(4),
		P2: r.read(5), C2: r.read(13), S2: r.read(4), GA2: r.read(3), GB2: r.read(4),
	}
}

type bitReader struct {
	data []byte
	pos  int
}

func (r *bitReader) read(width int) uint16 {
	var v uint16
	for i := 0; i < width; i++ {
		byteIndex := r.pos / 8
		bitIndex := 7 - (r.pos % 8)
		v = (v << 1) | uint16((r.data[byteIndex]>>bitIndex)&1)
		r.pos++
	}
	return v
}

func mustPCMFromInput(tmp, input string) []byte {
	out := filepath.Join(tmp, "source.pcm")
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", input, "-ar", "8000", "-ac", "1", "-f", "s16le", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("ffmpeg convert: %v: %s", err, string(b)))
	}
	data, err := os.ReadFile(out)
	must(err)
	return data
}

func pad(pcm []byte) []byte {
	frameBytes := g729.FrameSamples * 2
	out := append([]byte(nil), pcm...)
	if rem := len(out) % frameBytes; rem != 0 {
		out = append(out, make([]byte, frameBytes-rem)...)
	}
	return out
}

func mustLocalEncode(pcm []byte, gateRMS float64) []byte {
	enc := g729.NewEncoder()
	out := make([]byte, 0, len(pcm)/(g729.FrameSamples*2)*g729.FrameBytes)
	frame := make([]int16, g729.FrameSamples)
	bits := make([]byte, g729.FrameBytes)
	for off := 0; off < len(pcm); off += g729.FrameSamples * 2 {
		for i := range frame {
			frame[i] = int16(binary.LittleEndian.Uint16(pcm[off+i*2:]))
		}
		if gateRMS > 0 && frameRMS(frame) < gateRMS {
			for i := range frame {
				frame[i] = 0
			}
		}
		must(enc.EncodeFrame(frame, bits))
		out = append(out, bits...)
	}
	return out
}

func frameRMS(frame []int16) float64 {
	var sum float64
	for _, s := range frame {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(frame)))
}

func mustExternalEncode(path string, pcm []byte) []byte {
	cmd := exec.Command(path)
	cmd.Stdin = bytes.NewReader(pcm)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("external encode: %v: %s", err, stderr.String()))
	}
	return out
}

func defaultBCG729EncoderPath() string {
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

func mustLocalDecode(payload []byte) []byte {
	dec := g729.NewDecoder()
	out := make([]byte, 0, len(payload)/g729.FrameBytes*g729.FrameSamples*2)
	frame := make([]int16, g729.FrameSamples)
	var pair [2]byte
	for off := 0; off < len(payload); off += g729.FrameBytes {
		must(dec.DecodeFrame(payload[off:off+g729.FrameBytes], frame))
		for _, sample := range frame {
			binary.LittleEndian.PutUint16(pair[:], uint16(sample))
			out = append(out, pair[:]...)
		}
	}
	return out
}

func mustFFmpegDecode(tmp, name string, payload []byte) []byte {
	in := filepath.Join(tmp, name+".g729")
	out := filepath.Join(tmp, name+".pcm")
	must(os.WriteFile(in, payload, 0o600))
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-f", "g729", "-i", in, "-ar", "8000", "-ac", "1", "-f", "s16le", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("ffmpeg decode: %v: %s", err, string(b)))
	}
	data, err := os.ReadFile(out)
	must(err)
	return data
}

func printPathSummary(name string, pcm []byte) {
	samples := len(pcm) / 2
	clips := 0
	frames := map[int]bool{}
	peak := 0
	for i := 0; i < samples; i++ {
		s := sample(pcm, i)
		a := absInt(int(s))
		if a > peak {
			peak = a
		}
		if s == -32768 || s == 32767 || a >= 32700 {
			clips++
			frames[i/g729.FrameSamples] = true
		}
	}
	fmt.Printf("%s: peak=%d clipped_or_near=%d frames=%d\n", name, peak, clips, len(frames))
}

func collectFrameStats(source, our, bcg []byte, ourLag, bcgLag int) []frameStat {
	frames := min(len(source), min(len(our), len(bcg))) / (g729.FrameSamples * 2)
	out := make([]frameStat, 0, frames)
	for f := 0; f < frames; f++ {
		s := frameStat{frame: f, startMS: float64(f) * 10}
		var srcSum, ourSum, bcgSum float64
		for i := 0; i < g729.FrameSamples; i++ {
			idx := f*g729.FrameSamples + i
			src := int(sample(source, idx))
			o := int(sample(our, idx))
			b := int(sample(bcg, idx))
			s.sourcePeak = max(s.sourcePeak, absInt(src))
			s.ourPeak = max(s.ourPeak, absInt(o))
			s.bcgPeak = max(s.bcgPeak, absInt(b))
			if absInt(o) >= 32700 {
				s.ourClips++
			}
			if absInt(b) >= 32700 {
				s.bcgClips++
			}
			srcSum += float64(src * src)
			ourSum += float64(o * o)
			bcgSum += float64(b * b)
		}
		s.sourceRMS = math.Sqrt(srcSum / g729.FrameSamples)
		s.ourRMS = math.Sqrt(ourSum / g729.FrameSamples)
		s.bcgRMS = math.Sqrt(bcgSum / g729.FrameSamples)
		s.ourMinusBCG = s.ourRMS - s.bcgRMS
		s.ourSNR, s.ourCorr, s.ourErrRMS, s.ourHighRMS, s.ourHighDB = alignedFrameMetric(source, our, f, ourLag)
		s.bcgSNR, s.bcgCorr, s.bcgErrRMS, s.bcgHighRMS, s.bcgHighDB = alignedFrameMetric(source, bcg, f, bcgLag)
		out = append(out, s)
	}
	return out
}

func alignedFrameMetric(source, out []byte, frame, lag int) (snr, corr, errRMS, highErrRMS, highErrDB float64) {
	startSrc := frame * g729.FrameSamples
	startOut := startSrc + lag
	if startOut < 0 || startOut+g729.FrameSamples > len(out)/2 || startSrc+g729.FrameSamples > len(source)/2 {
		return math.Inf(-1), 0, 0, 0, math.Inf(-1)
	}
	var signal, noise, srcEnergy, outEnergy, cross, highNoise float64
	var prevErr float64
	for i := 0; i < g729.FrameSamples; i++ {
		src := float64(sample(source, startSrc+i))
		decoded := float64(sample(out, startOut+i))
		diff := src - decoded
		signal += src * src
		noise += diff * diff
		srcEnergy += src * src
		outEnergy += decoded * decoded
		cross += src * decoded
		if i > 0 {
			d := diff - prevErr
			highNoise += d * d
		}
		prevErr = diff
	}
	errRMS = math.Sqrt(noise / g729.FrameSamples)
	highErrRMS = math.Sqrt(highNoise / (g729.FrameSamples - 1))
	if noise > 0 {
		snr = 10 * math.Log10((signal+1)/noise)
	} else {
		snr = 99
	}
	if srcEnergy > 0 && outEnergy > 0 {
		corr = cross / math.Sqrt(srcEnergy*outEnergy)
	}
	sourceRMS := math.Sqrt(srcEnergy / g729.FrameSamples)
	highErrDB = 20 * math.Log10((highErrRMS+1)/(sourceRMS+1))
	return snr, corr, errRMS, highErrRMS, highErrDB
}

func alignedWindow(refSamples, outSamples, lag int) (startRef, startOut, n int) {
	if lag >= 0 {
		startRef = 0
		startOut = lag
		n = min(refSamples, outSamples-lag)
		return
	}
	startRef = -lag
	startOut = 0
	n = min(refSamples+lag, outSamples)
	return
}

func sample(pcm []byte, i int) int16 {
	return int16(binary.LittleEndian.Uint16(pcm[i*2:]))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hunydev/g729"
)

const frameDeadline = 10 * time.Millisecond

var stdout io.Writer = os.Stdout

func main() {
	var (
		mode              = flag.String("mode", "encode", "operation: encode, decode, loopback, or streaming")
		profileName       = flag.String("profile", "core", "encoder profile: core or corefast")
		streams           = flag.Int("streams", 1, "number of independent codec streams")
		duration          = flag.Duration("duration", 10*time.Second, "test duration")
		gomaxprocs        = flag.Int("gomaxprocs", 1, "runtime GOMAXPROCS value")
		realtime          = flag.Bool("realtime", true, "pace each stream at one 10 ms frame per tick")
		maxCodecMissRatio = flag.Float64("max-codec-deadline-miss-ratio", 1.0, "fail if processing-time deadline miss ratio exceeds this value")
		maxP99Ratio       = flag.Float64("max-p99-deadline-ratio", 1.0, "fail if p99 processing time divided by the 10 ms frame deadline exceeds this value")
		maxWakeLateRatio  = flag.Float64("max-wake-late-ratio", 1.0, "fail if realtime wake-late ratio exceeds this value")
		wakeLateThreshold = flag.Duration("wake-late-threshold", time.Millisecond, "scheduler wake lateness threshold")
		jsonOutput        = flag.Bool("json", false, "print a machine-readable JSON report")
	)
	flag.Parse()

	if *streams < 1 {
		failf("streams must be >= 1")
	}
	if *duration <= 0 {
		failf("duration must be > 0")
	}
	if *gomaxprocs < 1 {
		failf("gomaxprocs must be >= 1")
	}

	profile, err := parseProfile(*profileName)
	if err != nil {
		failf("%v", err)
	}
	operation, err := parseMode(*mode)
	if err != nil {
		failf("%v", err)
	}

	runtime.GOMAXPROCS(*gomaxprocs)

	start := time.Now()
	deadline := start.Add(*duration)
	results := make([]streamResult, *streams)
	var wg sync.WaitGroup
	wg.Add(*streams)
	for id := 0; id < *streams; id++ {
		go func(id int) {
			defer wg.Done()
			results[id] = runStream(id, *streams, operation, profile, start, deadline, *realtime, *wakeLateThreshold)
		}(id)
	}
	wg.Wait()
	elapsed := time.Since(start)

	report := mergeResults(results, elapsed, *streams, *gomaxprocs, operation, profileNameString(profile), *realtime, *wakeLateThreshold)
	printReport(report, *jsonOutput)

	exitCode := 0
	if report.Errors > 0 {
		fmt.Fprintf(os.Stderr, "loadtest failed: errors=%d\n", report.Errors)
		exitCode = 1
	}
	if report.CodecDeadlineMissRatio > *maxCodecMissRatio {
		fmt.Fprintf(os.Stderr, "loadtest failed: codec deadline miss ratio %.6f > %.6f\n", report.CodecDeadlineMissRatio, *maxCodecMissRatio)
		exitCode = 1
	}
	if report.P99DeadlineRatio > *maxP99Ratio {
		fmt.Fprintf(os.Stderr, "loadtest failed: p99 deadline ratio %.6f > %.6f\n", report.P99DeadlineRatio, *maxP99Ratio)
		exitCode = 1
	}
	if *realtime && report.WakeLateRatio > *maxWakeLateRatio {
		fmt.Fprintf(os.Stderr, "loadtest failed: wake-late ratio %.6f > %.6f\n", report.WakeLateRatio, *maxWakeLateRatio)
		exitCode = 1
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

type operation int

const (
	opEncode operation = iota
	opDecode
	opLoopback
	opStreaming
)

func parseMode(value string) (operation, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "encode":
		return opEncode, nil
	case "decode":
		return opDecode, nil
	case "loopback", "encode-decode", "encodedecode":
		return opLoopback, nil
	case "streaming", "stream":
		return opStreaming, nil
	default:
		return 0, fmt.Errorf("unknown mode %q", value)
	}
}

func (op operation) String() string {
	switch op {
	case opEncode:
		return "encode"
	case opDecode:
		return "decode"
	case opLoopback:
		return "loopback"
	case opStreaming:
		return "streaming"
	default:
		return "unknown"
	}
}

func parseProfile(value string) (g729.EncoderProfile, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "core":
		return g729.EncoderProfileCore, nil
	case "corefast", "fast":
		return g729.EncoderProfileCoreFast, nil
	default:
		return 0, fmt.Errorf("unknown profile %q", value)
	}
}

func profileNameString(profile g729.EncoderProfile) string {
	switch profile {
	case g729.EncoderProfileCoreFast:
		return "corefast"
	default:
		return "core"
	}
}

type streamResult struct {
	Frames                int64
	Errors                int64
	CodecDeadlineMisses   int64
	WakeLate              int64
	ProcessingTotal       time.Duration
	ProcessingDurations   []int64
	WakeLateDurations     []int64
	MaxProcessingDuration time.Duration
	MaxWakeLateDuration   time.Duration
}

func runStream(id, streams int, op operation, profile g729.EncoderProfile, start, deadline time.Time, realtime bool, wakeLateThreshold time.Duration) streamResult {
	state := newStreamState(id, profile)
	result := streamResult{}
	next := start
	if realtime && streams > 1 {
		next = next.Add(time.Duration(id) * frameDeadline / time.Duration(streams))
	}
	for {
		if realtime {
			now := time.Now()
			if now.Before(next) {
				time.Sleep(next.Sub(now))
			}
			woke := time.Now()
			if woke.After(next.Add(wakeLateThreshold)) {
				late := woke.Sub(next)
				result.WakeLate++
				result.WakeLateDurations = append(result.WakeLateDurations, late.Nanoseconds())
				if late > result.MaxWakeLateDuration {
					result.MaxWakeLateDuration = late
				}
			}
			if woke.After(deadline) {
				break
			}
		} else if time.Now().After(deadline) {
			break
		}

		opStart := time.Now()
		if err := state.run(op); err != nil {
			result.Errors++
		}
		processing := time.Since(opStart)
		result.Frames++
		result.ProcessingTotal += processing
		result.ProcessingDurations = append(result.ProcessingDurations, processing.Nanoseconds())
		if processing > result.MaxProcessingDuration {
			result.MaxProcessingDuration = processing
		}
		if processing > frameDeadline {
			result.CodecDeadlineMisses++
		}
		next = next.Add(frameDeadline)
	}
	return result
}

type streamState struct {
	encoder   *g729.Encoder
	decoder   *g729.Decoder
	streaming *g729.Encoder
	pcm       [g729.FrameSamples]int16
	bits      [g729.FrameBytes]byte
	out       [g729.FrameSamples]int16
}

func newStreamState(id int, profile g729.EncoderProfile) *streamState {
	s := &streamState{
		encoder:   g729.NewEncoderWithProfile(profile),
		decoder:   g729.NewDecoder(),
		streaming: g729.NewStreamingEncoderWithProfile(io.Discard, profile),
	}
	fillDeterministicPCM(&s.pcm, id)
	if err := s.encoder.EncodeFrame(s.pcm[:], s.bits[:]); err != nil {
		panic(err)
	}
	return s
}

func (s *streamState) run(op operation) error {
	switch op {
	case opEncode:
		return s.encoder.EncodeFrame(s.pcm[:], s.bits[:])
	case opDecode:
		return s.decoder.DecodeFrame(s.bits[:], s.out[:])
	case opLoopback:
		if err := s.encoder.EncodeFrame(s.pcm[:], s.bits[:]); err != nil {
			return err
		}
		return s.decoder.DecodeFrame(s.bits[:], s.out[:])
	case opStreaming:
		_, err := s.streaming.Write(s.pcm[:])
		return err
	default:
		return fmt.Errorf("unknown operation")
	}
}

func fillDeterministicPCM(dst *[g729.FrameSamples]int16, streamID int) {
	base := 0.013 * float64(streamID+1)
	for i := range dst {
		x := float64(i)
		v := 9000*math.Sin(2*math.Pi*(x/37+base)) + 2600*math.Sin(2*math.Pi*(x/17+base*0.5))
		dst[i] = int16(math.Round(v))
	}
}

type loadReport struct {
	Streams                int
	GOMAXPROCS             int
	Mode                   string
	Profile                string
	Realtime               bool
	Elapsed                time.Duration
	Frames                 int64
	Errors                 int64
	CodecDeadlineMisses    int64
	WakeLate               int64
	WakeLateThreshold      time.Duration
	CodecDeadlineMissRatio float64
	WakeLateRatio          float64
	MediaDuration          time.Duration
	ProcessingTotal        time.Duration
	WallRTF                float64
	WallXRealtime          float64
	CodecRTF               float64
	CodecStreamsPerCore    float64
	P99DeadlineRatio       float64
	ProcessingStats        durationStats
	WakeLateStats          durationStats
}

func mergeResults(results []streamResult, elapsed time.Duration, streams, gomaxprocs int, op operation, profile string, realtime bool, wakeLateThreshold time.Duration) loadReport {
	report := loadReport{
		Streams:           streams,
		GOMAXPROCS:        gomaxprocs,
		Mode:              op.String(),
		Profile:           profile,
		Realtime:          realtime,
		Elapsed:           elapsed,
		WakeLateThreshold: wakeLateThreshold,
	}
	var processingDurations []int64
	var wakeLateDurations []int64
	for _, result := range results {
		report.Frames += result.Frames
		report.Errors += result.Errors
		report.CodecDeadlineMisses += result.CodecDeadlineMisses
		report.WakeLate += result.WakeLate
		report.ProcessingTotal += result.ProcessingTotal
		processingDurations = append(processingDurations, result.ProcessingDurations...)
		wakeLateDurations = append(wakeLateDurations, result.WakeLateDurations...)
	}

	report.MediaDuration = time.Duration(report.Frames) * frameDeadline
	if report.Frames > 0 {
		report.CodecDeadlineMissRatio = float64(report.CodecDeadlineMisses) / float64(report.Frames)
		report.WakeLateRatio = float64(report.WakeLate) / float64(report.Frames)
	}
	if report.MediaDuration > 0 && elapsed > 0 {
		report.WallRTF = elapsed.Seconds() / report.MediaDuration.Seconds()
		report.WallXRealtime = report.MediaDuration.Seconds() / elapsed.Seconds()
	}
	if report.MediaDuration > 0 {
		report.CodecRTF = report.ProcessingTotal.Seconds() / report.MediaDuration.Seconds()
		if report.CodecRTF > 0 {
			report.CodecStreamsPerCore = 1 / report.CodecRTF
		}
	}
	report.ProcessingStats = summarizeDurations(processingDurations)
	report.WakeLateStats = summarizeDurations(wakeLateDurations)
	report.P99DeadlineRatio = float64(report.ProcessingStats.P99) / float64(frameDeadline)
	return report
}

type durationStats struct {
	Mean time.Duration
	P50  time.Duration
	P95  time.Duration
	P99  time.Duration
	Max  time.Duration
}

func summarizeDurations(values []int64) durationStats {
	if len(values) == 0 {
		return durationStats{}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	var total int64
	for _, value := range values {
		total += value
	}
	return durationStats{
		Mean: time.Duration(total / int64(len(values))),
		P50:  time.Duration(percentile(values, 0.50)),
		P95:  time.Duration(percentile(values, 0.95)),
		P99:  time.Duration(percentile(values, 0.99)),
		Max:  time.Duration(values[len(values)-1]),
	}
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func printReport(report loadReport, jsonOutput bool) {
	if jsonOutput {
		data, err := json.MarshalIndent(report.toJSON(), "", "  ")
		if err != nil {
			failf("%v", err)
		}
		fmt.Fprintln(stdout, string(data))
		return
	}

	fmt.Fprintf(stdout, "g729 load test\n")
	fmt.Fprintf(stdout, "mode=%s profile=%s realtime=%t streams=%d gomaxprocs=%d elapsed=%s\n",
		report.Mode, report.Profile, report.Realtime, report.Streams, report.GOMAXPROCS, report.Elapsed)
	fmt.Fprintf(stdout, "frames=%d errors=%d media=%s processing=%s\n",
		report.Frames, report.Errors, report.MediaDuration, report.ProcessingTotal)
	fmt.Fprintf(stdout, "wall_rtf=%.6f wall_x_realtime=%.2fx codec_rtf=%.6f codec_streams_per_core=%.2f\n",
		report.WallRTF, report.WallXRealtime, report.CodecRTF, report.CodecStreamsPerCore)
	fmt.Fprintf(stdout, "codec_deadline_misses=%d ratio=%.6f deadline=%s\n",
		report.CodecDeadlineMisses, report.CodecDeadlineMissRatio, frameDeadline)
	fmt.Fprintf(stdout, "processing_us mean=%.2f p50=%.2f p95=%.2f p99=%.2f max=%.2f p99_deadline=%.6f\n",
		us(report.ProcessingStats.Mean), us(report.ProcessingStats.P50), us(report.ProcessingStats.P95), us(report.ProcessingStats.P99), us(report.ProcessingStats.Max), report.P99DeadlineRatio)
	if report.Realtime {
		fmt.Fprintf(stdout, "wake_late_over_%s=%d ratio=%.6f\n", report.WakeLateThreshold, report.WakeLate, report.WakeLateRatio)
		fmt.Fprintf(stdout, "wake_late_us mean=%.2f p50=%.2f p95=%.2f p99=%.2f max=%.2f\n",
			us(report.WakeLateStats.Mean), us(report.WakeLateStats.P50), us(report.WakeLateStats.P95), us(report.WakeLateStats.P99), us(report.WakeLateStats.Max))
	}
}

type loadReportJSON struct {
	Streams                int               `json:"streams"`
	GOMAXPROCS             int               `json:"gomaxprocs"`
	Mode                   string            `json:"mode"`
	Profile                string            `json:"profile"`
	Realtime               bool              `json:"realtime"`
	ElapsedNanos           int64             `json:"elapsedNanos"`
	Frames                 int64             `json:"frames"`
	Errors                 int64             `json:"errors"`
	CodecDeadlineMisses    int64             `json:"codecDeadlineMisses"`
	WakeLate               int64             `json:"wakeLate"`
	WakeLateThresholdNanos int64             `json:"wakeLateThresholdNanos"`
	CodecDeadlineMissRatio float64           `json:"codecDeadlineMissRatio"`
	WakeLateRatio          float64           `json:"wakeLateRatio"`
	MediaDurationNanos     int64             `json:"mediaDurationNanos"`
	ProcessingTotalNanos   int64             `json:"processingTotalNanos"`
	WallRTF                float64           `json:"wallRTF"`
	WallXRealtime          float64           `json:"wallXRealtime"`
	CodecRTF               float64           `json:"codecRTF"`
	CodecStreamsPerCore    float64           `json:"codecStreamsPerCore"`
	P99DeadlineRatio       float64           `json:"p99DeadlineRatio"`
	ProcessingStats        durationStatsJSON `json:"processingStats"`
	WakeLateStats          durationStatsJSON `json:"wakeLateStats"`
}

type durationStatsJSON struct {
	MeanNanos int64   `json:"meanNanos"`
	P50Nanos  int64   `json:"p50Nanos"`
	P95Nanos  int64   `json:"p95Nanos"`
	P99Nanos  int64   `json:"p99Nanos"`
	MaxNanos  int64   `json:"maxNanos"`
	MeanUS    float64 `json:"meanUS"`
	P50US     float64 `json:"p50US"`
	P95US     float64 `json:"p95US"`
	P99US     float64 `json:"p99US"`
	MaxUS     float64 `json:"maxUS"`
}

func (report loadReport) toJSON() loadReportJSON {
	return loadReportJSON{
		Streams:                report.Streams,
		GOMAXPROCS:             report.GOMAXPROCS,
		Mode:                   report.Mode,
		Profile:                report.Profile,
		Realtime:               report.Realtime,
		ElapsedNanos:           report.Elapsed.Nanoseconds(),
		Frames:                 report.Frames,
		Errors:                 report.Errors,
		CodecDeadlineMisses:    report.CodecDeadlineMisses,
		WakeLate:               report.WakeLate,
		WakeLateThresholdNanos: report.WakeLateThreshold.Nanoseconds(),
		CodecDeadlineMissRatio: report.CodecDeadlineMissRatio,
		WakeLateRatio:          report.WakeLateRatio,
		MediaDurationNanos:     report.MediaDuration.Nanoseconds(),
		ProcessingTotalNanos:   report.ProcessingTotal.Nanoseconds(),
		WallRTF:                report.WallRTF,
		WallXRealtime:          report.WallXRealtime,
		CodecRTF:               report.CodecRTF,
		CodecStreamsPerCore:    report.CodecStreamsPerCore,
		P99DeadlineRatio:       report.P99DeadlineRatio,
		ProcessingStats:        report.ProcessingStats.toJSON(),
		WakeLateStats:          report.WakeLateStats.toJSON(),
	}
}

func (stats durationStats) toJSON() durationStatsJSON {
	return durationStatsJSON{
		MeanNanos: stats.Mean.Nanoseconds(),
		P50Nanos:  stats.P50.Nanoseconds(),
		P95Nanos:  stats.P95.Nanoseconds(),
		P99Nanos:  stats.P99.Nanoseconds(),
		MaxNanos:  stats.Max.Nanoseconds(),
		MeanUS:    us(stats.Mean),
		P50US:     us(stats.P50),
		P95US:     us(stats.P95),
		P99US:     us(stats.P99),
		MaxUS:     us(stats.Max),
	}
}

func us(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1000
}

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

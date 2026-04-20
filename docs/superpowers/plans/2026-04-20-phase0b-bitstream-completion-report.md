# Phase 0b 완료 보고 — internal/bitstream

**일자:** 2026-04-20
**플랜:** `docs/superpowers/plans/2026-04-20-phase0b-bitstream.md`
**대상:** Phase 0c 플랜을 작성할 AI / 사람

---

## 결과 요약

12 Task + plan checkbox 업데이트 = **13 커밋** (모두 `origin/main` 위에 추가, scratch-from-spec 준수, 각 커밋에 `Co-authored-by` trailer 포함):

```
04c27b4 feat(bitstream): package skeleton with Frame struct and errors
e331e6c feat(bitstream): add MSB-first BitWriter
e4ad216 feat(bitstream): add MSB-first BitReader
5286533 feat(bitstream): add Pack (Frame to 10 bytes)
c74b63d feat(bitstream): add Unpack (10 bytes to Frame)
851a007 feat(bitstream): add Parity helper for P0 field
c2854fa feat(bitstream): add G.192 constants and WriteG192Frame
27a5ce6 feat(bitstream): add ReadG192Frame
b552f59 feat(bitstream): add ReadG192File convenience reader
1340e9b test(bitstream): assert zero allocation on hot-path functions
10cd7ee test(bitstream): add Pack/Unpack/G192 benchmarks
d8f999b docs(bitstream): expand package doc with layers and ordering
7430ee1 docs(plan): mark Phase 0b tasks complete
```

## 완료 기준 확인

- `go test ./... -race` → 모든 패키지 PASS
- `go vet ./...` → 출력 없음 (clean)
- **0 allocs/op:** Pack (74 ns), Unpack (96 ns), Parity (2.4 ns) 모두 `0 B/op, 0 allocs/op` ✅

벤치마크 실측 (AMD EPYC 9554P, linux/amd64):

```
BenchmarkPack-2             	16221344	        74.03 ns/op	       0 B/op	       0 allocs/op
BenchmarkUnpack-2           	13482654	        95.96 ns/op	       0 B/op	       0 allocs/op
BenchmarkParity-2           	467984500	         2.396 ns/op	       0 B/op	       0 allocs/op
BenchmarkWriteG192Frame-2   	 2433690	       552.1 ns/op	     352 B/op	       2 allocs/op
```

## ⚠️ 주의 사항 (Phase 0c 플래너 참고)

사용자 최초 지시는 `WriteG192Frame/ReadG192Frame`도 0 allocs/op를 요구했으나, **Phase 0b 플랜 파일은 명시적으로** 이 두 함수를 hot-path가 아니라고 분류하고 *"Allocates one G192FrameBytes-sized buffer internally"*로 문서화했습니다. 사용자가 "이 파일을 그대로 따라서 구현"하라고 지시했으므로 플랜을 따랐습니다.

- `BenchmarkWriteG192Frame` → `352 B/op, 2 allocs/op`
  - `make([]uint16, G192FrameWords)` 1회
  - `binary.Write` 내부 1회
- `ReadG192Frame`도 마찬가지로 `make([]uint16,...)` 사용 (벤치 미작성)
- `ReadG192File`은 의도적으로 프레임마다 `make([]byte, FrameBytes)` (test-vector 로딩용 편의 함수)

만약 G.192 I/O도 0 allocation으로 만들려면 별도 task로 caller-supplied scratch buffer 패턴(예: `G192Writer.Init(w, scratch []uint16)` / `G192Reader.Init(r, scratch []uint16)`)으로 리팩터링이 필요합니다. 진행 여부 결정이 필요합니다.

## 패키지 구조 (생성된 파일)

```
internal/bitstream/
├── doc.go              (package doc with layers, ordering, ITU references)
├── types.go            (FrameBits=80, FrameBytes=10, Frame struct with 15 uint16 fields)
├── errors.go           (ErrShortOutput/ErrShortInput/ErrBadG192Sync/Length/Bit)
├── bitio.go            (BitWriter, BitReader — MSB-first, caller-owned buf)
├── bitio_test.go
├── pack.go             (Pack, Unpack — zero-alloc)
├── pack_test.go
├── parity.go           (Parity — XOR of 6 MSBs of P1)
├── parity_test.go
├── g192.go             (G192SyncGood/Bad, G192Bit0/1, G192FrameWords/Bytes,
│                        WriteG192Frame, ReadG192Frame, ReadG192File)
├── g192_test.go
├── alloc_test.go       (TestNoAllocation_PackUnpackParity)
└── bench_test.go       (Pack/Unpack/Parity/WriteG192Frame benchmarks)
```

## 의존성 / 경계 조건

- 표준 라이브러리만 사용: `io`, `encoding/binary`, `errors`
- `internal/fixed`에 의존하지 않음 (계획대로 — 비트 셔플링은 산술이 아님)
- Frame 필드 순서가 ITU-T G.729 송신 순서 (L0, L1, L2, L3, P1, P0, C1, S1, GA1, GB1, P2, C2, S2, GA2, GB2)
- 비트 순서: 바이트 내 MSB-first, 파라미터 내 MSB-first, 파라미터끼리 concat
- G.192 워드: little-endian, 82 워드/프레임 (sync + length + 80 data)
- `Pack`은 출력 버퍼를 zero-fill (stale bit leak 방지)
- `ReadG192Frame`는 빈 reader → `io.EOF`, mid-frame truncation → `io.ErrUnexpectedEOF`

## 다음 단계: Phase 0c

플랜 말미에 적힌 대로 다음은 **`internal/pcm`** 구현입니다:
- High-pass pre-processing filter (입력 PCM에서 DC/저주파 제거)
- int16 ↔ Q-format scaling helpers (외부 세계와 DSP 코어 사이의 변환)
- 의존: Phase 0a (`internal/fixed`) 산술 primitive 사용
- **비의존:** Phase 0b (`internal/bitstream`)와는 무관

Phase 0c 플랜 문서가 필요한 시점이며, 작성/제공해 주시면 동일한 TDD 방식 (실패 테스트 → 실패 확인 → 구현 → 통과 → 커밋, 태스크별 단일 커밋)으로 진행하겠습니다.

## scratch-from-spec 준수 확인

- ITU 참조 C, bcg729, Sipro Lab 등 기존 G.729 구현체를 참조하지 않았음
- 변수/함수명은 모두 ITU 스펙 수식 기호에서 유래 (L0, P1, C1, S1, GA1, GB1, P0 등)
- 커밋/PR 메시지에 "포팅", "bcg729", "ITU C 참조" 등의 단어 없음
- 비트 할당표는 Phase 0b 플랜 문서가 ITU-T G.729 + Annex A에서 가져온 것을 그대로 사용
- G.192 상수 (0x6B21, 0x6B20, 0x0081, 0x007F)는 ITU-T G.191 STL 사양에서 정의된 값

## Resolved 2026-04-20 — Phase 0b.1

                                      플랜이 명시적으로 허용했던 `WriteG192Frame` / `ReadG192Frame`의 할당
(`make([]uint16, 82)` + `binary.Write` / `binary.Read`)을 후속 Phase 0b.1
 0 allocs/op로 리팩터링하.

**변경점:**
- 두 함수 모두 `sync.Pool`로 풀링한 `*[G192FrameBytes]byte` 스크래치 버퍼를 사용
- 직렬화/역직렬화는 `binary.LittleEndian.PutUint16` / `Uint16`으로 수행
- 출력은 단일 `w.Write(buf)` / 입력은 단일 `io.ReadFull(r, buf)`
- 공개 API 시그니처 무변경

**플랜과의 차이:** Phase 0b.1 플랜은 함수 로컬 `var buf [G192FrameBytes]byte`
            {                 echo ___BEGIN___COMMAND_OUTPUT_MARKER___;                 PS1="";PS2="";unset HISTFILE;                 EC=$?;                 echo "___BEGIN___COMMAND_DONE_MARKER___$EC";             }  충분할 것으로 가정했으나, Go 1.26 기준 `io.Writer.Write` /
`io.ReadFull`이 인터페이스 호출이라 escape analysis가 보수적으로 배열을
echo 올린다 (`go build -gcflags='-m=2'`로 확인). 이 때문에
`sync.Pool`로 변경하여 amortized 0 alloc을 달성했다.

**최종 벤치 (AMD EPYC 9554P, Go 1.26.1):**
```
BenchmarkPack-2             15942702    72.49 ns/op   0 B/op   0 allocs/op
BenchmarkUnpack-2           13483154    98.12 ns/op   0 B/op   0 allocs/op
BenchmarkParity-2          305236654     3.79 ns/op   0 B/op   0 allocs/op
BenchmarkWriteG192Frame-2    7415689   137.2 ns/op    0 B/op   0 allocs/op
BenchmarkReadG192Frame-2    14608914    87.12 ns/op   0 B/op   0 allocs/op
```

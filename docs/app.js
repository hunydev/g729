const heroCanvas = document.querySelector("#wave-scene");

function drawHeroWave(time = 0) {
  if (!heroCanvas) return;

  const rect = heroCanvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  const width = Math.max(1, Math.floor(rect.width * dpr));
  const height = Math.max(1, Math.floor(rect.height * dpr));

  if (heroCanvas.width !== width || heroCanvas.height !== height) {
    heroCanvas.width = width;
    heroCanvas.height = height;
  }

  const ctx = heroCanvas.getContext("2d");
  ctx.clearRect(0, 0, width, height);
  ctx.fillStyle = "#111318";
  ctx.fillRect(0, 0, width, height);

  const bands = [
    { color: "rgba(121, 216, 202, 0.72)", amp: 0.19, freq: 1.7, speed: 0.00035, y: 0.38 },
    { color: "rgba(255, 166, 92, 0.58)", amp: 0.14, freq: 2.2, speed: 0.00028, y: 0.54 },
    { color: "rgba(154, 142, 235, 0.48)", amp: 0.11, freq: 3.1, speed: 0.00042, y: 0.68 }
  ];

  bands.forEach((band, bandIndex) => {
    ctx.beginPath();
    ctx.lineWidth = Math.max(2, width * 0.0028);
    ctx.strokeStyle = band.color;

    for (let x = -20 * dpr; x <= width + 20 * dpr; x += 8 * dpr) {
      const unit = x / width;
      const wave =
        Math.sin(unit * Math.PI * 2 * band.freq + time * band.speed + bandIndex) *
        Math.sin(unit * Math.PI * 1.35);
      const carrier = Math.sin(unit * Math.PI * 19 + time * band.speed * 2) * 0.025;
      const y = height * (band.y + wave * band.amp + carrier);
      if (x === -20 * dpr) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }

    ctx.stroke();
  });

  const columns = 28;
  const baseY = height * 0.78;
  for (let i = 0; i < columns; i += 1) {
    const x = (width / columns) * i + width * 0.03;
    const tone = Math.sin(i * 0.8 + time * 0.001) * 0.5 + 0.5;
    const barHeight = height * (0.04 + tone * 0.12);
    ctx.fillStyle = i % 3 === 0 ? "rgba(121,216,202,0.34)" : "rgba(255,255,255,0.12)";
    ctx.fillRect(x, baseY - barHeight, Math.max(3, width * 0.004), barHeight);
  }

  window.requestAnimationFrame(drawHeroWave);
}

async function activateAudioSamples() {
  const cards = Array.from(document.querySelectorAll(".sample-card[data-audio]"));

  await Promise.all(cards.map(async (card, index) => {
    const audioPath = card.dataset.audio;
    const audio = card.querySelector("audio");
    const canvas = card.querySelector(".mini-wave");
    const playButton = card.querySelector(".play-toggle");
    const seek = card.querySelector(".seek-bar");
    const time = card.querySelector(".time-readout");
    const state = card.querySelector(".sample-state");
    const unavailable = card.dataset.unavailable;

    if (unavailable) {
      audio.removeAttribute("src");
      if (playButton) playButton.disabled = true;
      if (seek) seek.disabled = true;
      state.textContent = unavailable;
      return;
    }

    try {
      const response = await fetch(audioPath, { cache: "no-store" });
      if (!response.ok) throw new Error(`missing ${response.status}`);
      const wav = parsePCM16WAV(await response.arrayBuffer());
      audio.src = audioPath;
      audio.preload = "metadata";
      setupSamplePlayer({ card, audio, canvas, playButton, seek, time, state, wav, index });
    } catch {
      audio.removeAttribute("src");
      if (canvas) drawEmptyWave(canvas);
      if (playButton) playButton.disabled = true;
      if (seek) seek.disabled = true;
      state.textContent = `Add a reviewed sample at ${audioPath} to publish this slot.`;
    }
  }));
}

let activeSampleAudio = null;
const samplePlayers = [];

function setupSamplePlayer({ audio, canvas, playButton, seek, time, state, wav, index }) {
  const accent = index === 0 ? "#3558c7" : (index % 2 === 0 ? "#0f766e" : "#c2410c");
  const duration = wav.samples.length / wav.sampleRate;
  const player = { audio, canvas, samples: wav.samples, accent };
  samplePlayers.push(player);

  drawWaveform(canvas, wav.samples, 0, accent);
  state.textContent = `${formatDuration(duration)} · ${wav.sampleRate / 1000} kHz mono PCM`;
  playButton.disabled = false;
  seek.disabled = false;
  seek.value = "0";
  time.textContent = `0:00 / ${formatDuration(duration)}`;

  playButton.addEventListener("click", async () => {
    if (activeSampleAudio && activeSampleAudio !== audio) activeSampleAudio.pause();
    if (audio.paused) {
      activeSampleAudio = audio;
      await audio.play();
    } else {
      audio.pause();
    }
  });

  seek.addEventListener("input", () => {
    if (!Number.isFinite(audio.duration) || audio.duration <= 0) return;
    audio.currentTime = (Number(seek.value) / 1000) * audio.duration;
  });

  canvas.addEventListener("click", (event) => {
    if (!Number.isFinite(audio.duration) || audio.duration <= 0) return;
    const rect = canvas.getBoundingClientRect();
    const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
    audio.currentTime = ratio * audio.duration;
  });

  audio.addEventListener("play", () => {
    playButton.textContent = "Pause";
  });

  audio.addEventListener("pause", () => {
    playButton.textContent = "Play";
  });

  audio.addEventListener("ended", () => {
    playButton.textContent = "Play";
    seek.value = "0";
    drawWaveform(canvas, wav.samples, 0, accent);
    time.textContent = `0:00 / ${formatDuration(duration)}`;
  });

  audio.addEventListener("timeupdate", () => {
    const audioDuration = Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : duration;
    const progress = audioDuration > 0 ? audio.currentTime / audioDuration : 0;
    seek.value = String(Math.round(progress * 1000));
    drawWaveform(canvas, wav.samples, progress, accent);
    time.textContent = `${formatDuration(audio.currentTime)} / ${formatDuration(audioDuration)}`;
  });
}

function redrawSamplePlayers() {
  samplePlayers.forEach((player) => {
    const duration = Number.isFinite(player.audio.duration) && player.audio.duration > 0 ? player.audio.duration : 0;
    const progress = duration > 0 ? player.audio.currentTime / duration : 0;
    drawWaveform(player.canvas, player.samples, progress, player.accent);
  });
}

function parsePCM16WAV(buffer) {
  const view = new DataView(buffer);
  if (readASCII(view, 0, 4) !== "RIFF" || readASCII(view, 8, 4) !== "WAVE") {
    throw new Error("expected RIFF/WAVE");
  }

  let channels = 0;
  let sampleRate = 0;
  let bitsPerSample = 0;
  let audioFormat = 0;
  let dataOffset = -1;
  let dataSize = 0;

  for (let offset = 12; offset + 8 <= view.byteLength;) {
    const id = readASCII(view, offset, 4);
    const size = view.getUint32(offset + 4, true);
    const start = offset + 8;
    const end = start + size;
    if (end > view.byteLength) break;

    if (id === "fmt ") {
      audioFormat = view.getUint16(start, true);
      channels = view.getUint16(start + 2, true);
      sampleRate = view.getUint32(start + 4, true);
      bitsPerSample = view.getUint16(start + 14, true);
    } else if (id === "data") {
      dataOffset = start;
      dataSize = size;
    }

    offset = end + (size % 2);
  }

  if (audioFormat !== 1 || channels < 1 || bitsPerSample !== 16 || dataOffset < 0) {
    throw new Error("expected PCM16 WAV");
  }

  const frameBytes = channels * 2;
  const totalFrames = Math.floor(dataSize / frameBytes);
  const samples = new Float32Array(totalFrames);
  for (let i = 0; i < totalFrames; i += 1) {
    let sum = 0;
    const frameOffset = dataOffset + i * frameBytes;
    for (let ch = 0; ch < channels; ch += 1) {
      sum += view.getInt16(frameOffset + ch * 2, true);
    }
    samples[i] = (sum / channels) / 32768;
  }

  return { samples, sampleRate };
}

function readASCII(view, offset, length) {
  let out = "";
  for (let i = 0; i < length; i += 1) {
    out += String.fromCharCode(view.getUint8(offset + i));
  }
  return out;
}

function drawEmptyWave(canvas) {
  drawWaveform(canvas, new Float32Array(0), 0, "#8b919e");
}

function drawWaveform(canvas, samples, progress, accent) {
  if (!canvas) return;

  const rect = canvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  const cssWidth = Math.max(1, Math.floor(rect.width || canvas.width));
  const cssHeight = Math.max(1, Math.floor(rect.height || canvas.height));
  const width = Math.floor(cssWidth * dpr);
  const height = Math.floor(cssHeight * dpr);
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
  }

  const ctx = canvas.getContext("2d");
  ctx.clearRect(0, 0, width, height);
  ctx.fillStyle = "#f6f8f5";
  ctx.fillRect(0, 0, width, height);

  const pad = Math.max(14 * dpr, height * 0.14);
  const center = height / 2;
  const usable = height - pad * 2;
  const columns = Math.max(80, Math.floor(cssWidth / 2));
  const barGap = Math.max(1, Math.floor(1.5 * dpr));
  const barWidth = Math.max(1, Math.floor(width / columns) - barGap);
  const peaks = waveformPeaks(samples, columns);

  ctx.strokeStyle = "rgba(17, 19, 24, 0.07)";
  ctx.lineWidth = 1;
  for (let i = 1; i <= 3; i += 1) {
    const y = pad + (usable / 4) * i;
    ctx.beginPath();
    ctx.moveTo(12 * dpr, y);
    ctx.lineTo(width - 12 * dpr, y);
    ctx.stroke();
  }

  drawWaveBars(ctx, peaks, 0, columns, center, usable, barWidth, barGap, dpr, "rgba(17, 19, 24, 0.16)");
  drawWaveBars(ctx, peaks, 0, Math.floor(columns * Math.max(0, Math.min(1, progress))), center, usable, barWidth, barGap, dpr, accent);

  ctx.fillStyle = "rgba(17, 19, 24, 0.46)";
  ctx.fillRect(12 * dpr, center, width - 24 * dpr, Math.max(1, dpr));
}

function waveformPeaks(samples, columns) {
  const peaks = new Float32Array(columns);
  if (!samples.length) return peaks;
  const step = samples.length / columns;
  for (let i = 0; i < columns; i += 1) {
    const start = Math.floor(i * step);
    const end = Math.max(start + 1, Math.floor((i + 1) * step));
    let peak = 0;
    for (let j = start; j < end && j < samples.length; j += 1) {
      const value = Math.abs(samples[j]);
      if (value > peak) peak = value;
    }
    peaks[i] = Math.pow(peak, 0.72);
  }
  return peaks;
}

function drawWaveBars(ctx, peaks, start, end, center, usable, barWidth, barGap, dpr, color) {
  ctx.fillStyle = color;
  const minHeight = Math.max(2 * dpr, usable * 0.025);
  for (let i = start; i < end; i += 1) {
    const peak = peaks[i] || 0;
    const h = Math.max(minHeight, peak * usable);
    const x = 12 * dpr + i * (barWidth + barGap);
    const y = center - h / 2;
    ctx.fillRect(x, y, barWidth, h);
  }
}

function formatDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const total = Math.floor(seconds);
  const minutes = Math.floor(total / 60);
  const rest = String(total % 60).padStart(2, "0");
  return `${minutes}:${rest}`;
}

function waitForWasmReady() {
  if (window.g729Wasm) return Promise.resolve(window.g729Wasm);
  return new Promise((resolve) => {
    window.addEventListener("g729wasmready", () => resolve(window.g729Wasm), { once: true });
  });
}

async function loadG729Wasm() {
  if (window.g729Wasm) return window.g729Wasm;
  if (typeof Go === "undefined") throw new Error("Go WASM runtime is unavailable");

  const go = new Go();
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(fetch("assets/wasm/g729.wasm"), go.importObject);
  } catch {
    const response = await fetch("assets/wasm/g729.wasm");
    const bytes = await response.arrayBuffer();
    result = await WebAssembly.instantiate(bytes, go.importObject);
  }
  go.run(result.instance).catch((err) => console.error("g729 wasm stopped", err));
  return waitForWasmReady();
}

function pcmBytesToWavBlob(bytes, sampleRate = 8000) {
  const dataLen = bytes.byteLength;
  const buffer = new ArrayBuffer(44 + dataLen);
  const view = new DataView(buffer);
  writeASCII(view, 0, "RIFF");
  view.setUint32(4, 36 + dataLen, true);
  writeASCII(view, 8, "WAVE");
  writeASCII(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeASCII(view, 36, "data");
  view.setUint32(40, dataLen, true);
  new Uint8Array(buffer, 44).set(bytes);
  return new Blob([buffer], { type: "audio/wav" });
}

function writeASCII(view, offset, text) {
  for (let i = 0; i < text.length; i += 1) {
    view.setUint8(offset + i, text.charCodeAt(i));
  }
}

async function decodeToPCM16(file) {
  const sourceURL = URL.createObjectURL(file);
  const inputBytes = await file.arrayBuffer();
  const AudioCtor = window.AudioContext || window.webkitAudioContext;
  if (!AudioCtor || !window.OfflineAudioContext) {
    throw new Error("Web Audio API is unavailable in this browser");
  }

  const audioContext = new AudioCtor();
  const decoded = await audioContext.decodeAudioData(inputBytes.slice(0));
  const sampleRate = 8000;
  const length = Math.max(1, Math.ceil(decoded.duration * sampleRate));
  const offline = new OfflineAudioContext(1, length, sampleRate);
  const source = offline.createBufferSource();
  source.buffer = decoded;
  source.connect(offline.destination);
  source.start(0);
  const rendered = await offline.startRendering();
  await audioContext.close();

  const channel = rendered.getChannelData(0);
  const pcm = new Uint8Array(channel.length * 2);
  const view = new DataView(pcm.buffer);
  for (let i = 0; i < channel.length; i += 1) {
    const clamped = Math.max(-1, Math.min(1, channel[i]));
    const sample = clamped < 0 ? Math.round(clamped * 32768) : Math.round(clamped * 32767);
    view.setInt16(i * 2, sample, true);
  }

  return { pcm, sourceURL, samples: channel.length, sampleRate };
}

function renderMetrics(container, rows) {
  container.replaceChildren(...rows.map(([label, value]) => {
    const item = document.createElement("div");
    item.className = "metric";
    const labelEl = document.createElement("span");
    labelEl.textContent = label;
    const valueEl = document.createElement("strong");
    valueEl.textContent = value;
    item.append(labelEl, valueEl);
    return item;
  }));
}

function streamPCM16(bytes, sampleRate = 8000) {
  const AudioCtor = window.AudioContext || window.webkitAudioContext;
  if (!AudioCtor) throw new Error("Web Audio API is unavailable in this browser");

  const ctx = new AudioCtor();
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const totalSamples = bytes.byteLength / 2;
  const buffer = ctx.createBuffer(1, totalSamples, sampleRate);
  const channel = buffer.getChannelData(0);

  for (let i = 0; i < totalSamples; i += 1) {
    channel[i] = sampleAtPCM16(view, i, totalSamples) / 32768;
  }

  const fadeSamples = Math.min(Math.floor(sampleRate * 0.005), Math.floor(totalSamples / 2));
  for (let i = 0; i < fadeSamples; i += 1) {
    const gain = i / fadeSamples;
    channel[i] *= gain;
    channel[totalSamples - 1 - i] *= gain;
  }

  const source = ctx.createBufferSource();
  source.buffer = buffer;
  source.connect(ctx.destination);
  source.onended = () => ctx.close();

  const prebufferSeconds = 0.12;
  source.start(ctx.currentTime + prebufferSeconds);
  return {
    contextRate: ctx.sampleRate,
    bufferRate: sampleRate,
    outputSamples: totalSamples,
    prebufferMS: Math.round(prebufferSeconds * 1000)
  };
}

function sampleAtPCM16(view, index, totalSamples) {
  if (index < 0) return 0;
  if (index >= totalSamples) return totalSamples ? view.getInt16((totalSamples - 1) * 2, true) : 0;
  return view.getInt16(index * 2, true);
}

function isG729Payload(file) {
  const name = file.name.toLowerCase();
  return name.endsWith(".g729") || name.endsWith(".payload");
}

async function activateWasmDemo() {
  const demo = document.querySelector("#wasm-demo");
  if (!demo) return;

  const fileInput = demo.querySelector("#wasm-file");
  const runButton = demo.querySelector("#wasm-run");
  const streamButton = demo.querySelector("#wasm-stream");
  const downloadG729 = demo.querySelector("#wasm-download-g729");
  const downloadWAV = demo.querySelector("#wasm-download-wav");
  const status = demo.querySelector("#wasm-status");
  const metrics = demo.querySelector("#wasm-metrics");
  const sourceAudio = demo.querySelector("#wasm-source-audio");
  const decodedAudio = demo.querySelector("#wasm-decoded-audio");
  let wasm;
  let selected;
  let decodedPCM;
  let objectURLs = [];

  function rememberURL(url) {
    objectURLs.push(url);
    return url;
  }

  function clearObjectURLs() {
    objectURLs.forEach((url) => URL.revokeObjectURL(url));
    objectURLs = [];
  }

  function disableDownload(link) {
    link.href = "#";
    link.classList.add("disabled");
    link.setAttribute("aria-disabled", "true");
  }

  function enableDownload(link, blob, filename) {
    link.href = rememberURL(URL.createObjectURL(blob));
    link.download = filename;
    link.classList.remove("disabled");
    link.setAttribute("aria-disabled", "false");
  }

  function resetResult() {
    clearObjectURLs();
    selected = null;
    decodedPCM = null;
    metrics.replaceChildren();
    decodedAudio.removeAttribute("src");
    sourceAudio.removeAttribute("src");
    streamButton.disabled = true;
    disableDownload(downloadG729);
    disableDownload(downloadWAV);
  }

  try {
    wasm = await loadG729Wasm();
    status.textContent = "Go WASM codec ready.";
    runButton.disabled = !fileInput.files.length;
  } catch (err) {
    status.textContent = `WASM load failed: ${err.message}`;
    return;
  }

  fileInput.addEventListener("change", () => {
    resetResult();
    runButton.disabled = !fileInput.files.length || !wasm;
    if (fileInput.files.length) {
      const file = fileInput.files[0];
      runButton.textContent = isG729Payload(file) ? "Decode Payload" : "Encode / Decode";
      status.textContent = isG729Payload(file) ? "G.729 payload selected." : "Audio selected.";
    }
  });

  runButton.addEventListener("click", async () => {
    if (!fileInput.files.length || !wasm) return;
    const file = fileInput.files[0];
    runButton.disabled = true;
    streamButton.disabled = true;
    disableDownload(downloadG729);
    disableDownload(downloadWAV);
    status.textContent = isG729Payload(file)
      ? "Decoding G.729 payload through WASM."
      : "Resampling and running G.729 WASM.";

    try {
      if (isG729Payload(file)) {
        const payload = new Uint8Array(await file.arrayBuffer());
        const result = wasm.decodePayload(payload);
        if (!result.ok) throw new Error(result.error || "WASM payload decode failed");

        decodedPCM = result.decodedPCM16;
        const decodedBlob = pcmBytesToWavBlob(decodedPCM, result.sampleRate);
        decodedAudio.src = rememberURL(URL.createObjectURL(decodedBlob));
        enableDownload(downloadG729, new Blob([payload], { type: "application/octet-stream" }), file.name);
        enableDownload(downloadWAV, decodedBlob, "g729-wasm-decoded.wav");
        renderMetrics(metrics, [
          ["path", "payload decode"],
          ["decoder", "Go WASM local decoder"],
          ["frames", String(result.frames)],
          ["payload bytes", String(payload.byteLength)],
          ["decoded samples", String(result.decodedSamples)]
        ]);
        status.textContent = "WASM payload decode complete.";
      } else {
        selected = await decodeToPCM16(file);
        sourceAudio.src = rememberURL(selected.sourceURL);
        const result = wasm.roundTripPCM16(selected.pcm);
        if (!result.ok) throw new Error(result.error || "WASM roundtrip failed");

        decodedPCM = result.decodedPCM16;
        const encodedBlob = new Blob([result.encoded], { type: "application/octet-stream" });
        const decodedBlob = pcmBytesToWavBlob(decodedPCM, result.sampleRate);
        decodedAudio.src = rememberURL(URL.createObjectURL(decodedBlob));
        enableDownload(downloadG729, encodedBlob, "g729-wasm-encoded.g729");
        enableDownload(downloadWAV, decodedBlob, "g729-wasm-roundtrip.wav");
        renderMetrics(metrics, [
          ["profile", "EncoderProfileCore"],
          ["path", "Core encode -> local decode"],
          ["input samples", String(result.inputSamples)],
          ["frames", String(result.frames)],
          ["encoded bytes", String(result.encoded.byteLength)],
          ["tail padding", `${result.paddedSamples} samples`]
        ]);
        status.textContent = "WASM encode/decode complete.";
      }
      streamButton.disabled = false;
    } catch (err) {
      status.textContent = `WASM demo failed: ${err.message}`;
    } finally {
      runButton.disabled = !fileInput.files.length;
    }
  });

  streamButton.addEventListener("click", () => {
    if (!decodedPCM) return;
    const playback = streamPCM16(decodedPCM, 8000);
    status.textContent = `Current WASM decoded PCM scheduled as one ${playback.prebufferMS} ms buffered 8 kHz AudioBuffer; browser output context is ${playback.contextRate} Hz.`;
  });
}

window.addEventListener("resize", () => {
  redrawSamplePlayers();
}, { passive: true });
window.requestAnimationFrame(drawHeroWave);
activateAudioSamples();
activateWasmDemo();

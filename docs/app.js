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
    const player = createAudioPlayer(card, {
      accent: sampleAccent(index),
      emptyText: "Sample pending."
    });
    const unavailable = card.dataset.unavailable;

    if (unavailable) {
      clearAudioPlayer(player, unavailable);
      return;
    }

    await loadAudioPlayerURL(player, card.dataset.audio, {
      missingText: `Add a reviewed sample at ${card.dataset.audio} to publish this slot.`
    });
  }));
}

let activeAudioPlayer = null;
let audioAnimationFrame = 0;
const audioPlayers = [];

function sampleAccent(index) {
  if (index === 0) return "#3558c7";
  return index % 2 === 0 ? "#0f766e" : "#c2410c";
}

function createAudioPlayer(root, options = {}) {
  const audio = root.querySelector("audio");
  const canvas = root.querySelector(".mini-wave");
  const playButton = root.querySelector(".play-toggle");
  const seek = root.querySelector(".seek-bar");
  const time = root.querySelector(".time-readout");
  const state = root.querySelector(".sample-state");
  const player = {
    root,
    audio,
    canvas,
    playButton,
    seek,
    time,
    state,
    accent: options.accent || "#0f766e",
    emptyText: options.emptyText || "Audio pending.",
    samples: new Float32Array(0),
    sampleRate: 8000,
    duration: 0,
    peaks: new Float32Array(0),
    waveKey: "",
    ready: false
  };

  audio.preload = "metadata";
  audioPlayers.push(player);
  installAudioPlayerEvents(player);
  clearAudioPlayer(player, player.emptyText);
  return player;
}

function installAudioPlayerEvents(player) {
  player.playButton.addEventListener("click", async () => {
    if (!player.ready) return;
    if (activeAudioPlayer && activeAudioPlayer !== player) {
      activeAudioPlayer.audio.pause();
    }
    if (player.audio.paused) {
      activeAudioPlayer = player;
      try {
        await player.audio.play();
      } catch (err) {
        player.state.textContent = `Playback failed: ${err.message}`;
      }
    } else {
      player.audio.pause();
    }
  });

  player.seek.addEventListener("input", () => {
    if (!player.ready || player.duration <= 0) return;
    player.audio.currentTime = (Number(player.seek.value) / Number(player.seek.max || 10000)) * player.duration;
    updateAudioPlayerProgress(player);
  });

  player.canvas.addEventListener("click", (event) => {
    if (!player.ready || player.duration <= 0) return;
    const rect = player.canvas.getBoundingClientRect();
    const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
    player.audio.currentTime = ratio * player.duration;
    updateAudioPlayerProgress(player);
  });

  player.audio.addEventListener("play", () => {
    player.playButton.textContent = "Pause";
    scheduleAudioAnimation();
  });

  player.audio.addEventListener("pause", () => {
    player.playButton.textContent = "Play";
    updateAudioPlayerProgress(player);
  });

  player.audio.addEventListener("ended", () => {
    player.playButton.textContent = "Play";
    player.audio.currentTime = 0;
    updateAudioPlayerProgress(player);
  });

  player.audio.addEventListener("loadedmetadata", () => {
    if (Number.isFinite(player.audio.duration) && player.audio.duration > 0) {
      player.duration = player.audio.duration;
      updateAudioPlayerProgress(player);
    }
  });

  player.audio.addEventListener("timeupdate", () => {
    if (player.audio.paused) updateAudioPlayerProgress(player);
  });
}

async function loadAudioPlayerURL(player, audioPath, options = {}) {
  clearAudioPlayer(player, "Loading sample.");
  try {
    const response = await fetch(audioPath, { cache: "no-store" });
    if (!response.ok) throw new Error(`missing ${response.status}`);
    const wav = parsePCM16WAV(await response.arrayBuffer());
    setAudioPlayerSource(player, {
      src: audioPath,
      samples: wav.samples,
      sampleRate: wav.sampleRate,
      stateText: options.stateText || `${formatDuration(wav.samples.length / wav.sampleRate)} · ${wav.sampleRate / 1000} kHz mono PCM`
    });
  } catch {
    clearAudioPlayer(player, options.missingText || `Add a reviewed sample at ${audioPath} to publish this slot.`);
  }
}

function setAudioPlayerSource(player, { src, samples, sampleRate = 8000, stateText }) {
  player.samples = samples || new Float32Array(0);
  player.sampleRate = sampleRate;
  player.duration = player.samples.length && sampleRate ? player.samples.length / sampleRate : 0;
  player.peaks = new Float32Array(0);
  player.waveKey = "";
  player.ready = Boolean(src && player.samples.length);
  player.audio.src = src;
  player.audio.preload = "metadata";
  player.playButton.disabled = !player.ready;
  player.seek.disabled = !player.ready;
  player.seek.value = "0";
  player.playButton.textContent = "Play";
  player.time.textContent = `0:00 / ${formatDuration(player.duration)}`;
  player.state.textContent = stateText || (player.ready ? `${formatDuration(player.duration)} · ${sampleRate / 1000} kHz mono PCM` : player.emptyText);
  drawAudioPlayerWaveform(player, 0);
}

function clearAudioPlayer(player, text) {
  player.audio.pause();
  player.audio.removeAttribute("src");
  player.samples = new Float32Array(0);
  player.duration = 0;
  player.ready = false;
  player.peaks = new Float32Array(0);
  player.waveKey = "";
  player.playButton.disabled = true;
  player.seek.disabled = true;
  player.seek.value = "0";
  player.playButton.textContent = "Play";
  player.time.textContent = "0:00 / 0:00";
  player.state.textContent = text;
  drawAudioPlayerWaveform(player, 0);
}

function updateAudioPlayerProgress(player) {
  if (!player.ready) return;
  const duration = player.duration > 0 ? player.duration : 0;
  const progress = duration > 0 ? Math.max(0, Math.min(1, player.audio.currentTime / duration)) : 0;
  player.seek.value = String(Math.round(progress * Number(player.seek.max || 10000)));
  player.time.textContent = `${formatDuration(player.audio.currentTime)} / ${formatDuration(duration)}`;
  drawAudioPlayerWaveform(player, progress);
}

function scheduleAudioAnimation() {
  if (!audioAnimationFrame) {
    audioAnimationFrame = window.requestAnimationFrame(updateAudioAnimation);
  }
}

function updateAudioAnimation() {
  audioAnimationFrame = 0;
  let keepGoing = false;
  for (const player of audioPlayers) {
    if (player.ready && !player.audio.paused && !player.audio.ended) {
      updateAudioPlayerProgress(player);
      keepGoing = true;
    }
  }
  if (keepGoing) scheduleAudioAnimation();
}

function redrawSamplePlayers() {
  audioPlayers.forEach((player) => updateAudioPlayerProgress(player));
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

function drawAudioPlayerWaveform(player, progress) {
  const canvas = player.canvas;
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

  const columns = Math.max(96, Math.floor(cssWidth / 2));
  const key = `${width}:${height}:${columns}:${player.samples.length}`;
  if (player.waveKey !== key) {
    player.peaks = waveformPeaks(player.samples, columns);
    player.waveKey = key;
  }

  drawWaveformPeaks(canvas, player.peaks, progress, player.accent);
}

function drawWaveformPeaks(canvas, peaks, progress, accent) {
  const width = canvas.width;
  const height = canvas.height;
  const dpr = window.devicePixelRatio || 1;
  const ctx = canvas.getContext("2d");
  ctx.clearRect(0, 0, width, height);
  ctx.fillStyle = "#f6f8f5";
  ctx.fillRect(0, 0, width, height);

  const pad = Math.max(14 * dpr, height * 0.14);
  const center = height / 2;
  const usable = height - pad * 2;
  const columns = peaks.length;
  const barGap = Math.max(1, Math.floor(1.5 * dpr));
  const barWidth = Math.max(1, Math.floor(width / columns) - barGap);

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

function samplesFromPCM16Bytes(bytes) {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const count = Math.floor(bytes.byteLength / 2);
  const samples = new Float32Array(count);
  for (let i = 0; i < count; i += 1) {
    samples[i] = view.getInt16(i * 2, true) / 32768;
  }
  return samples;
}

function activateBlindArena() {
  const arena = document.querySelector("#blind-arena");
  if (!arena) return;

  const total = Number(arena.dataset.trials || 10);
  const progress = arena.querySelector("#arena-progress");
  const result = arena.querySelector("#arena-result");
  const reset = arena.querySelector("#arena-reset");
  const buttons = Array.from(arena.querySelectorAll(".arena-pick"));
  const leftPlayer = createAudioPlayer(arena.querySelector("#arena-left-player"), {
    accent: "#3558c7",
    emptyText: "Left sample pending."
  });
  const rightPlayer = createAudioPlayer(arena.querySelector("#arena-right-player"), {
    accent: "#c2410c",
    emptyText: "Right sample pending."
  });
  const arenaAssetVersion = "speech-active-v3";

  const trials = Array.from({ length: total }, (_, index) => {
    const id = String(index + 1).padStart(2, "0");
    return {
      label: `Trial ${index + 1}`,
      bcg: `assets/audio/arena/trial-${id}-bcg729-ffmpeg.wav?v=${arenaAssetVersion}`,
      our: `assets/audio/arena/trial-${id}-our-loopback.wav?v=${arenaAssetVersion}`
    };
  });

  let order = [];
  let index = 0;
  let picks = [];

  function resetArena() {
    order = trials.map((trial) => ({
      ...trial,
      left: Math.random() < 0.5 ? "bcg" : "our"
    }));
    index = 0;
    picks = [];
    result.hidden = true;
    result.replaceChildren();
    buttons.forEach((button) => {
      button.disabled = false;
    });
    loadArenaTrial();
  }

  async function loadArenaTrial() {
    const trial = order[index];
    if (!trial) {
      showArenaResult();
      return;
    }

    const leftKind = trial.left;
    const rightKind = leftKind === "bcg" ? "our" : "bcg";
    progress.textContent = `${trial.label} / ${total}`;
    buttons.forEach((button) => {
      button.disabled = true;
    });
    await Promise.all([
      loadAudioPlayerURL(leftPlayer, trial[leftKind], { stateText: "Blind sample A · 2.8 s" }),
      loadAudioPlayerURL(rightPlayer, trial[rightKind], { stateText: "Blind sample B · 2.8 s" })
    ]);
    buttons.forEach((button) => {
      button.disabled = false;
    });
  }

  function recordPick(choice) {
    const trial = order[index];
    if (!trial) return;
    const leftKind = trial.left;
    const rightKind = leftKind === "bcg" ? "our" : "bcg";
    const picked = choice === "tie" ? "tie" : (choice === "left" ? leftKind : rightKind);
    picks.push({ trial: trial.label, left: leftKind, right: rightKind, picked });
    leftPlayer.audio.pause();
    rightPlayer.audio.pause();
    index += 1;
    loadArenaTrial();
  }

  function showArenaResult() {
    const counts = picks.reduce((acc, pick) => {
      acc[pick.picked] += 1;
      return acc;
    }, { our: 0, bcg: 0, tie: 0 });
    progress.textContent = "Arena complete";
    buttons.forEach((button) => {
      button.disabled = true;
    });
    clearAudioPlayer(leftPlayer, "Arena complete.");
    clearAudioPlayer(rightPlayer, "Arena complete.");
    result.hidden = false;
    result.innerHTML = `
      <h3>Blind result</h3>
      <div class="arena-score-grid">
        <div><span>our Core encode -> our decode</span><strong>${counts.our}</strong></div>
        <div><span>bcg729 encode -> FFmpeg decode</span><strong>${counts.bcg}</strong></div>
        <div><span>Tie / unsure</span><strong>${counts.tie}</strong></div>
      </div>
      <p>Restart to reshuffle left/right placement. Trial order stays fixed.</p>
    `;
  }

  buttons.forEach((button) => {
    button.addEventListener("click", () => recordPick(button.dataset.pick));
  });
  reset.addEventListener("click", resetArena);
  resetArena();
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
  const wasmURL = "assets/wasm/g729.wasm?v=1799f324b282";
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(fetch(wasmURL, { cache: "no-store" }), go.importObject);
  } catch {
    const response = await fetch(wasmURL, { cache: "no-store" });
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

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function schedulePCM16Bytes(audioContext, bytes, startTime, sampleRate = 8000) {
  if (!bytes || bytes.byteLength === 0) return startTime;
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const samples = bytes.byteLength / 2;
  const buffer = audioContext.createBuffer(1, samples, sampleRate);
  const channel = buffer.getChannelData(0);
  for (let i = 0; i < samples; i += 1) {
    const value = view.getInt16(i * 2, true);
    channel[i] = value < 0 ? value / 32768 : value / 32767;
  }
  const source = audioContext.createBufferSource();
  source.buffer = buffer;
  source.connect(audioContext.destination);
  source.start(startTime);
  return startTime + samples / sampleRate;
}

async function decodeToPCM16(file) {
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

  return { pcm, samples: channel.length, sampleRate };
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
  const streamPtime = demo.querySelector("#wasm-stream-ptime");
  const downloadG729 = demo.querySelector("#wasm-download-g729");
  const downloadWAV = demo.querySelector("#wasm-download-wav");
  const status = demo.querySelector("#wasm-status");
  const metrics = demo.querySelector("#wasm-metrics");
  const sourcePlayer = createAudioPlayer(demo.querySelector("#wasm-source-player"), {
    accent: "#3558c7",
    emptyText: "Input preview pending."
  });
  const decodedPlayer = createAudioPlayer(demo.querySelector("#wasm-decoded-player"), {
    accent: "#0f766e",
    emptyText: "Roundtrip pending."
  });
  let wasm;
  let selected;
  let decodedPCM;
  let objectURLs = [];
  let liveContext;
  let liveRunID = 0;
  let livePlaying = false;

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

  async function stopLiveLoopback(message = "Live loopback stopped.") {
    liveRunID += 1;
    livePlaying = false;
    if (liveContext) {
      const ctx = liveContext;
      liveContext = null;
      await ctx.close().catch(() => {});
    }
    streamButton.textContent = "Play live loopback";
    streamButton.disabled = !selected;
    runButton.disabled = !fileInput.files.length;
    status.textContent = message;
  }

  function resetResult() {
    stopLiveLoopback("Live loopback reset.").catch(() => {});
    clearAudioPlayer(sourcePlayer, "Input preview pending.");
    clearAudioPlayer(decodedPlayer, "Roundtrip pending.");
    clearObjectURLs();
    selected = null;
    decodedPCM = null;
    metrics.replaceChildren();
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
        clearAudioPlayer(sourcePlayer, "Payload input has no browser audio preview.");
        setAudioPlayerSource(decodedPlayer, {
          src: rememberURL(URL.createObjectURL(decodedBlob)),
          samples: samplesFromPCM16Bytes(decodedPCM),
          sampleRate: result.sampleRate,
          stateText: `${formatDuration(result.decodedSamples / result.sampleRate)} · WASM local decode`
        });
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
        const sourceBlob = pcmBytesToWavBlob(selected.pcm, selected.sampleRate);
        setAudioPlayerSource(sourcePlayer, {
          src: rememberURL(URL.createObjectURL(sourceBlob)),
          samples: samplesFromPCM16Bytes(selected.pcm),
          sampleRate: selected.sampleRate,
          stateText: `${formatDuration(selected.samples / selected.sampleRate)} · 8 kHz input fed to WASM`
        });
        const result = wasm.roundTripPCM16(selected.pcm);
        if (!result.ok) throw new Error(result.error || "WASM roundtrip failed");

        decodedPCM = result.decodedPCM16;
        const encodedBlob = new Blob([result.encoded], { type: "application/octet-stream" });
        const decodedBlob = pcmBytesToWavBlob(decodedPCM, result.sampleRate);
        setAudioPlayerSource(decodedPlayer, {
          src: rememberURL(URL.createObjectURL(decodedBlob)),
          samples: samplesFromPCM16Bytes(decodedPCM),
          sampleRate: result.sampleRate,
          stateText: `${formatDuration(result.decodedSamples / result.sampleRate)} · WASM Core loopback`
        });
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
      streamButton.disabled = !selected || isG729Payload(file);
    } catch (err) {
      status.textContent = `WASM demo failed: ${err.message}`;
    } finally {
      runButton.disabled = !fileInput.files.length;
    }
  });

  streamButton.addEventListener("click", async () => {
    if (livePlaying) {
      await stopLiveLoopback();
      return;
    }
    if (!selected || !wasm || !wasm.newLoopbackStream) return;
    if (activeAudioPlayer) {
      activeAudioPlayer.audio.pause();
    }

    const AudioCtor = window.AudioContext || window.webkitAudioContext;
    if (!AudioCtor) {
      status.textContent = "Live loopback requires Web Audio playback support.";
      return;
    }

    liveRunID += 1;
    const runID = liveRunID;
    if (liveContext) {
      await liveContext.close().catch(() => {});
    }
    try {
      liveContext = new AudioCtor({ sampleRate: selected.sampleRate });
    } catch {
      liveContext = new AudioCtor();
    }
    if (liveContext.state === "suspended") {
      await liveContext.resume();
    }

    const chunkMS = streamPtime && streamPtime.value === "10" ? 10 : 20;
    const chunkSamples = chunkMS === 10 ? 80 : 160;
    const chunkBytes = chunkSamples * 2;
    const stream = wasm.newLoopbackStream();
    let scheduledTime = liveContext.currentTime + 0.06;
    const startedAt = performance.now();
    let frames = 0;

    livePlaying = true;
    streamButton.textContent = "Stop live loopback";
    streamButton.disabled = false;
    runButton.disabled = true;
    status.textContent = `Live loopback started: feeding ${chunkMS} ms PCM chunks through WASM Write/Flush.`;

    try {
      for (let off = 0; off < selected.pcm.byteLength; off += chunkBytes) {
        if (runID !== liveRunID) return;
        const end = Math.min(off + chunkBytes, selected.pcm.byteLength);
        const chunk = selected.pcm.slice(off, end);
        const result = stream.write(chunk);
        if (!result.ok) throw new Error(result.error || "WASM stream write failed");
        frames += result.frames;
        scheduledTime = schedulePCM16Bytes(liveContext, result.decodedPCM16, scheduledTime, result.sampleRate);
        const fedSamples = end / 2;
        const targetElapsed = (fedSamples / selected.sampleRate) * 1000;
        status.textContent = `Live loopback: ${formatDuration(fedSamples / selected.sampleRate)} fed, ${frames} frames emitted, ${result.bufferedSamples} samples buffered.`;
        const waitMS = startedAt + targetElapsed - performance.now();
        if (waitMS > 1) await sleep(waitMS);
      }

      const flushed = stream.flush();
      if (!flushed.ok) throw new Error(flushed.error || "WASM stream flush failed");
      frames += flushed.frames;
      scheduledTime = schedulePCM16Bytes(liveContext, flushed.decodedPCM16, scheduledTime, flushed.sampleRate);
      const tailSamples = selected.samples % 80;
      const paddedSamples = tailSamples === 0 ? 0 : 80 - tailSamples;

      renderMetrics(metrics, [
        ["profile", "EncoderProfileCore"],
        ["path", "streaming Core loopback"],
        ["chunk", `${chunkMS} ms`],
        ["input samples", String(selected.samples)],
        ["frames", String(frames)],
        ["tail padding", `${paddedSamples} samples`]
      ]);

      const remainingMS = Math.max(0, (scheduledTime - liveContext.currentTime) * 1000);
      status.textContent = `Live loopback scheduled ${frames} frames through Web Audio.`;
      await sleep(remainingMS + 40);
      if (runID === liveRunID) {
        status.textContent = `Live loopback complete: ${frames} frames at ${chunkMS} ms input cadence.`;
      }
    } catch (err) {
      status.textContent = `Live loopback failed: ${err.message}`;
    } finally {
      if (runID === liveRunID) {
        livePlaying = false;
        streamButton.textContent = "Play live loopback";
        streamButton.disabled = !selected;
        runButton.disabled = !fileInput.files.length;
        if (liveContext) {
          const ctx = liveContext;
          liveContext = null;
          ctx.close().catch(() => {});
        }
      }
    }
  });
}

window.addEventListener("resize", () => {
  redrawSamplePlayers();
}, { passive: true });
window.requestAnimationFrame(drawHeroWave);
activateAudioSamples();
activateBlindArena();
activateWasmDemo();

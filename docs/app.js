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

function drawMiniWave(canvas, seed) {
  const ctx = canvas.getContext("2d");
  const width = canvas.width;
  const height = canvas.height;

  ctx.clearRect(0, 0, width, height);
  ctx.fillStyle = "#f3f5f2";
  ctx.fillRect(0, 0, width, height);

  ctx.strokeStyle = seed % 2 === 0 ? "#0f766e" : "#c2410c";
  ctx.lineWidth = 3;
  ctx.beginPath();

  for (let x = 0; x <= width; x += 4) {
    const unit = x / width;
    const envelope = Math.sin(Math.PI * unit);
    const detail = Math.sin(unit * Math.PI * (14 + seed)) * 0.45;
    const voice = Math.sin(unit * Math.PI * (35 + seed * 2)) * 0.16;
    const y = height * (0.5 + (detail + voice) * envelope * 0.42);
    if (x === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  }

  ctx.stroke();
}

async function activateAudioSamples() {
  const cards = Array.from(document.querySelectorAll(".sample-card[data-audio]"));

  cards.forEach((card, index) => {
    const canvas = card.querySelector(".mini-wave");
    if (canvas) drawMiniWave(canvas, index + 1);
  });

  await Promise.all(cards.map(async (card) => {
    const audioPath = card.dataset.audio;
    const audio = card.querySelector("audio");
    const state = card.querySelector(".sample-state");
    const unavailable = card.dataset.unavailable;

    if (unavailable) {
      audio.removeAttribute("src");
      state.textContent = unavailable;
      return;
    }

    try {
      const response = await fetch(audioPath, { method: "HEAD", cache: "no-store" });
      if (!response.ok) throw new Error(`missing ${response.status}`);
      audio.src = audioPath;
      state.textContent = "Sample available.";
    } catch {
      audio.removeAttribute("src");
      state.textContent = `Add a reviewed sample at ${audioPath} to publish this slot.`;
    }
  }));
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
  const outputRate = ctx.sampleRate;
  const outputSamples = Math.max(1, Math.ceil((totalSamples / sampleRate) * outputRate));
  const buffer = ctx.createBuffer(1, outputSamples, outputRate);
  const channel = buffer.getChannelData(0);

  for (let i = 0; i < outputSamples; i += 1) {
    const src = (i * sampleRate) / outputRate;
    const base = Math.floor(src);
    const frac = src - base;
    const s0 = sampleAtPCM16(view, base, totalSamples);
    const s1 = sampleAtPCM16(view, base + 1, totalSamples);
    channel[i] = (s0 + (s1 - s0) * frac) / 32768;
  }

  const fadeSamples = Math.min(Math.floor(outputRate * 0.005), Math.floor(outputSamples / 2));
  for (let i = 0; i < fadeSamples; i += 1) {
    const gain = i / fadeSamples;
    channel[i] *= gain;
    channel[outputSamples - 1 - i] *= gain;
  }

  const source = ctx.createBufferSource();
  source.buffer = buffer;
  source.connect(ctx.destination);
  source.onended = () => ctx.close();

  const prebufferSeconds = 0.12;
  source.start(ctx.currentTime + prebufferSeconds);
  return {
    outputRate,
    outputSamples,
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
    status.textContent = `Decoded PCM scheduled as one ${playback.prebufferMS} ms buffered AudioContext stream at ${playback.outputRate} Hz.`;
  });
}

window.addEventListener("resize", () => drawHeroWave(performance.now()), { passive: true });
window.requestAnimationFrame(drawHeroWave);
activateAudioSamples();
activateWasmDemo();

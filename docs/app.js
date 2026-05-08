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

window.addEventListener("resize", () => drawHeroWave(performance.now()), { passive: true });
window.requestAnimationFrame(drawHeroWave);
activateAudioSamples();

const canvas = document.getElementById('audio-viz');
const ctx = canvas.getContext('2d');
const W = 320, H = 320, CX = W/2, CY = H/2, R = 130;

function noise(t, seed) {
  return Math.sin(t * 1.3 + seed) * 0.4
       + Math.sin(t * 2.7 + seed * 1.7) * 0.3
       + Math.sin(t * 5.1 + seed * 0.9) * 0.2
       + Math.sin(t * 9.3 + seed * 2.3) * 0.1;
}

const sparks = Array.from({length: 28}, newSpark);
function newSpark() {
  const angle = Math.random() * Math.PI * 2;
  const dist = R * (0.55 + Math.random() * 0.35);
  return {
    x: CX + Math.cos(angle) * dist,
    y: CY + Math.sin(angle) * dist,
    vx: (Math.random() - 0.5) * 0.35,
    vy: (Math.random() - 0.5) * 0.35,
    life: Math.random(),
    maxLife: 0.6 + Math.random() * 2.2,
    age: Math.random() * 3,
    size: 0.8 + Math.random() * 1.6,
    hue: 260 + Math.random() * 40,
  };
}

function updateSpark(s, dt) {
  s.age += dt;
  s.x += s.vx;
  s.y += s.vy;
  const t = s.age / s.maxLife;
  s.life = t < 0.2 ? t/0.2 : t > 0.7 ? 1-(t-0.7)/0.3 : 1;
  if (s.age > s.maxLife) Object.assign(s, newSpark(), {age: 0, life: 0});
}

const BARS = 120;
let lastTime = 0;

function draw(ts) {
  const dt = Math.min((ts - lastTime) / 1000, 0.05);
  lastTime = ts;
  const t = ts / 1000;

  ctx.clearRect(0, 0, W, H);

  const grd = ctx.createRadialGradient(CX, CY, 10, CX, CY, R);
  grd.addColorStop(0, 'rgba(109,40,217,0.08)');
  grd.addColorStop(0.6, 'rgba(109,40,217,0.04)');
  grd.addColorStop(1, 'rgba(0,0,0,0)');
  ctx.fillStyle = grd;
  ctx.beginPath();
  ctx.arc(CX, CY, R, 0, Math.PI*2);
  ctx.fill();

  ctx.beginPath();
  ctx.arc(CX, CY, R, 0, Math.PI*2);
  ctx.strokeStyle = 'rgba(139,92,246,0.18)';
  ctx.lineWidth = 1;
  ctx.stroke();

  const pts = [];
  for (let i = 0; i < BARS; i++) {
    const angle = (i / BARS) * Math.PI * 2 - Math.PI / 2;
    const n = noise(i / BARS * 4 + t * 0.7, 1.2)
            + noise(i / BARS * 2 - t * 0.4, 3.7) * 0.5;
    const amp = 14 + n * 18;
    const r = R - 4 + amp * 0.5;
    pts.push([CX + Math.cos(angle) * r, CY + Math.sin(angle) * r]);
  }

  ctx.beginPath();
  ctx.moveTo(pts[0][0], pts[0][1]);
  for (let i = 1; i < pts.length; i++) {
    const [x0, y0] = pts[(i-1) % pts.length];
    const [x1, y1] = pts[i % pts.length];
    const [x2, y2] = pts[(i+1) % pts.length];
    const cpx = x1 + (x2-x0)*0.15;
    const cpy = y1 + (y2-y0)*0.15;
    ctx.quadraticCurveTo(x1, y1, cpx, cpy);
  }
  ctx.closePath();

  const waveGrd = ctx.createLinearGradient(CX-R, CY, CX+R, CY);
  waveGrd.addColorStop(0, 'rgba(167,139,250,0.55)');
  waveGrd.addColorStop(0.5, 'rgba(139,92,246,0.85)');
  waveGrd.addColorStop(1, 'rgba(167,139,250,0.55)');
  ctx.strokeStyle = waveGrd;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  const fillGrd = ctx.createRadialGradient(CX, CY, 0, CX, CY, R);
  fillGrd.addColorStop(0, 'rgba(109,40,217,0.06)');
  fillGrd.addColorStop(1, 'rgba(0,0,0,0)');
  ctx.fillStyle = fillGrd;
  ctx.fill();

  sparks.forEach(s => {
    updateSpark(s, dt);
    const alpha = s.life * 0.9;
    const glow = ctx.createRadialGradient(s.x, s.y, 0, s.x, s.y, s.size * 3);
    glow.addColorStop(0, `hsla(${s.hue},80%,80%,${alpha})`);
    glow.addColorStop(1, `hsla(${s.hue},80%,70%,0)`);
    ctx.beginPath();
    ctx.arc(s.x, s.y, s.size * 3, 0, Math.PI*2);
    ctx.fillStyle = glow;
    ctx.fill();
    ctx.beginPath();
    ctx.arc(s.x, s.y, s.size * 0.7, 0, Math.PI*2);
    ctx.fillStyle = `hsla(${s.hue},90%,90%,${alpha})`;
    ctx.fill();
  });

  requestAnimationFrame(draw);
}
requestAnimationFrame(draw);
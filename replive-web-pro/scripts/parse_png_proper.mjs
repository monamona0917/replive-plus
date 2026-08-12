import fs from "node:fs";
import zlib from "node:zlib";

const filePath = "C:/Users/Fu/.gemini/antigravity/brain/1cb2791b-b636-419f-b48a-24a9b8d472dd/.user_uploaded/media_1786368940875.png";
const buf = fs.readFileSync(filePath);

let pos = 8;
let width = 0, height = 0, colorType = 0;
const idatChunks = [];

while (pos < buf.length) {
  const length = buf.readUInt32BE(pos);
  const type = buf.toString("ascii", pos + 4, pos + 8);
  if (type === "IHDR") {
    width = buf.readUInt32BE(pos + 8);
    height = buf.readUInt32BE(pos + 12);
    colorType = buf[pos + 17];
  } else if (type === "IDAT") {
    idatChunks.push(buf.subarray(pos + 8, pos + 8 + length));
  }
  pos += 12 + length;
}

const decompressed = zlib.inflateSync(Buffer.concat(idatChunks));
const bpp = colorType === 6 ? 4 : 3;
const stride = 1 + width * bpp;

// Unfilter PNG
const uncompressed = Buffer.alloc(width * height * bpp);

function paeth(a, b, c) {
  const p = a + b - c;
  const pa = Math.abs(p - a);
  const pb = Math.abs(p - b);
  const pc = Math.abs(p - c);
  if (pa <= pb && pa <= pc) return a;
  if (pb <= pc) return b;
  return c;
}

for (let y = 0; y < height; y++) {
  const filterType = decompressed[y * stride];
  const rowStart = y * stride + 1;
  const outRowStart = y * width * bpp;
  const prevOutRowStart = (y - 1) * width * bpp;

  for (let x = 0; x < width * bpp; x++) {
    const raw = decompressed[rowStart + x];
    const a = x >= bpp ? uncompressed[outRowStart + x - bpp] : 0;
    const b = y > 0 ? uncompressed[prevOutRowStart + x] : 0;
    const c = y > 0 && x >= bpp ? uncompressed[prevOutRowStart + x - bpp] : 0;

    let val = 0;
    if (filterType === 0) val = raw;
    else if (filterType === 1) val = (raw + a) & 0xff;
    else if (filterType === 2) val = (raw + b) & 0xff;
    else if (filterType === 3) val = (raw + Math.floor((a + b) / 2)) & 0xff;
    else if (filterType === 4) val = (raw + paeth(a, b, c)) & 0xff;

    uncompressed[outRowStart + x] = val;
  }
}

function getUnfilteredPixel(x, y) {
  const offset = (y * width + x) * bpp;
  return {
    r: uncompressed[offset],
    g: uncompressed[offset + 1],
    b: uncompressed[offset + 2],
    a: bpp === 4 ? uncompressed[offset + 3] : 255
  };
}

console.log("\n=== RIGHT SIDE VERTICAL COLOR PROFILE (x = 120 / 179) ===");
for (let y = 10; y < height - 10; y += 15) {
  const { r, g, b } = getUnfilteredPixel(120, y);
  const hex = `#${r.toString(16).padStart(2, "0")}${g.toString(16).padStart(2, "0")}${b.toString(16).padStart(2, "0")}`;
  console.log(`y=${String(y).padStart(3, "0")} (${(y/height*100).toFixed(1)}%): RGB(${String(r).padStart(3, "0")}, ${String(g).padStart(3, "0")}, ${String(b).padStart(3, "0")}) -> ${hex}`);
}

console.log("\n=== TOP RIGHT REGION (x = 140 / 179, y = 20..150) ===");
for (let y = 20; y <= 150; y += 10) {
  const { r, g, b } = getUnfilteredPixel(140, y);
  const hex = `#${r.toString(16).padStart(2, "0")}${g.toString(16).padStart(2, "0")}${b.toString(16).padStart(2, "0")}`;
  console.log(`y=${String(y).padStart(3, "0")}: RGB(${String(r).padStart(3, "0")}, ${String(g).padStart(3, "0")}, ${String(b).padStart(3, "0")}) -> ${hex}`);
}

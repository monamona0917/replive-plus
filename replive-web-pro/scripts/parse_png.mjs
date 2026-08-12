import fs from "node:fs";
import zlib from "node:zlib";

const filePath = "C:/Users/Fu/.gemini/antigravity/brain/1cb2791b-b636-419f-b48a-24a9b8d472dd/.user_uploaded/media_1786368940875.png";
const buf = fs.readFileSync(filePath);

// Simple PNG parser to get width, height and uncompressed IDAT
let pos = 8; // skip signature
let width = 0, height = 0, bitDepth = 0, colorType = 0;
const idatChunks = [];

while (pos < buf.length) {
  const length = buf.readUInt32BE(pos);
  const type = buf.toString("ascii", pos + 4, pos + 8);
  if (type === "IHDR") {
    width = buf.readUInt32BE(pos + 8);
    height = buf.readUInt32BE(pos + 12);
    bitDepth = buf[pos + 16];
    colorType = buf[pos + 17];
  } else if (type === "IDAT") {
    idatChunks.push(buf.subarray(pos + 8, pos + 8 + length));
  }
  pos += 12 + length;
}

console.log(`PNG Dimensions: ${width}x${height}, colorType: ${colorType}`);

const decompressed = zlib.inflateSync(Buffer.concat(idatChunks));
const bytesPerPixel = colorType === 6 ? 4 : colorType === 2 ? 3 : 1;
const scanlineLength = 1 + width * bytesPerPixel;

function getPixel(x, y) {
  const rowStart = y * scanlineLength + 1; // skip filter byte (assuming 0 / none for simple reading or filter handling)
  const offset = rowStart + x * bytesPerPixel;
  return {
    r: decompressed[offset],
    g: decompressed[offset + 1],
    b: decompressed[offset + 2],
    a: bytesPerPixel === 4 ? decompressed[offset + 3] : 255
  };
}

console.log("\n--- Vertical slice of colors along the right edge (x = width - 4) ---");
for (let y = 0; y < height; y += Math.max(1, Math.floor(height / 20))) {
  const { r, g, b } = getPixel(Math.min(width - 4, width - 1), y);
  const hex = `#${r.toString(16).padStart(2, "0")}${g.toString(16).padStart(2, "0")}${b.toString(16).padStart(2, "0")}`;
  console.log(`y=${String(y).padStart(2, "0")} (${(y/height*100).toFixed(1)}%): RGB(${r}, ${g}, ${b}) -> ${hex}`);
}

console.log("\n--- Upper right background (x = width - 8, y = 0..height/2) ---");
for (let y = 0; y < height / 2; y += Math.max(1, Math.floor(height / 20))) {
  const { r, g, b } = getPixel(Math.max(0, width - 8), y);
  const hex = `#${r.toString(16).padStart(2, "0")}${g.toString(16).padStart(2, "0")}${b.toString(16).padStart(2, "0")}`;
  console.log(`y=${String(y).padStart(2, "0")} (${(y/height*100).toFixed(1)}%): RGB(${r}, ${g}, ${b}) -> ${hex}`);
}

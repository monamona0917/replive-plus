import os
from PIL import Image

src_path = r"C:/Users/Fu/.gemini/antigravity/brain/1cb2791b-b636-419f-b48a-24a9b8d472dd/.user_uploaded/media_1786368940875.png"
img = Image.open(src_path).convert("RGB")
w, h = img.size
print(f"Original comparison crop size: {w}x{h}")
pix = img.load()

print("\n--- Right edge colors (x = w - 3) from top (y=0) to bottom (y=h-1) ---")
for y in range(0, h, max(1, h // 20)):
    r, g, b = pix[w - 3, y]
    print(f"y={y:02d} ({y/h*100:4.1f}%): RGB({r:3d}, {g:3d}, {b:3d}) -> #{r:02x}{g:02x}{b:02x}")

print("\n--- Upper right background behind F top-bar (x = w - 6) ---")
for y in range(0, h // 2, max(1, h // 20)):
    r, g, b = pix[w - 6, y]
    print(f"y={y:02d} ({y/h*100:4.1f}%): RGB({r:3d}, {g:3d}, {b:3d}) -> #{r:02x}{g:02x}{b:02x}")

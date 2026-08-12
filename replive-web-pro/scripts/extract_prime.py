import os
from PIL import Image

src_path = r"C:/Users/Fu/.gemini/antigravity/brain/1cb2791b-b636-419f-b48a-24a9b8d472dd/.user_uploaded/media_1786418090823.jpg"
img = Image.open(src_path).convert("RGBA")
w, h = img.size
print(f"Original image size: {w}x{h}")

pixels = img.load()

# Let's find gold/yellow pixels of the sparkles (R > 180, G > 140, B < 120)
gold_pixels = []
for y in range(h):
    for x in range(w // 2): # left half
        r, g, b, a = pixels[x, y]
        # Gold/yellow detection
        if r > 160 and g > 130 and b < 110:
            gold_pixels.append((x, y))

if gold_pixels:
    min_x = min(p[0] for p in gold_pixels)
    max_x = max(p[0] for p in gold_pixels)
    min_y = min(p[1] for p in gold_pixels)
    max_y = max(p[1] for p in gold_pixels)
    print(f"Gold sparkles bounding box: ({min_x}, {min_y}) to ({max_x}, {max_y}), size: {max_x - min_x + 1}x{max_y - min_y + 1}")
    
    pad = 4
    crop_box = (max(0, min_x - pad), max(0, min_y - pad), min(w, max_x + pad + 1), min(h, max_y + pad + 1))
    cropped = img.crop(crop_box)
    
    out_public = r"D:/Tencentt/Tencent Files/1528760842/文件/MobileFile/nsy_chat_live-master/replive-web-pro/public"
    cropped.save(f"{out_public}/prime-icon-original.png")
    print(f"Saved prime-icon-original.png with size {cropped.size}")
    
    # Analyze exact colors
    center_color = pixels[(min_x + max_x)//2, (min_y + max_y)//2]
    print(f"Center gold sample: {center_color}")
else:
    print("No gold pixels detected with initial threshold, scanning all non-background pixels...")

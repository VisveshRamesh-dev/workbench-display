"""Generate placeholder spec images, one per real part code.

Each image is clearly labelled with its own part number and uses a distinct
colour, so a demo scan visibly matches the part. Replace these with the real
spec images when they arrive (keep the same file names, or update config.json).
"""
from PIL import Image, ImageDraw, ImageFont
import os

# code, subtitle, colour
parts = [
    ("82301GB030MWU", "Door Garnish - Front Left (FL)", "#1F3A68"),
    ("82301GW000BJU", "Door Garnish - 82301 variant",   "#2A5A3E"),
    ("87310GB000ABP", "Part 87310 - Assembly Spec",      "#6B3A1F"),
    ("86600DY740ABP", "Part 86600 - Assembly Spec",      "#5A2A6B"),
    ("87410BM000ABP", "Demo sample - 87410",             "#0D5B63"),
    ("87510BM000ABP", "Demo sample - 87510",             "#7A1F4B"),
    ("87610BM000ABP", "Demo sample - 87610",             "#4B5A1F"),
]

W, H = 1600, 1000


def load_font(size):
    for path in (
        "C:/Windows/Fonts/arialbd.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
    ):
        try:
            return ImageFont.truetype(path, size)
        except OSError:
            continue
    return ImageFont.load_default()


def load_font_reg(size):
    for path in (
        "C:/Windows/Fonts/arial.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    ):
        try:
            return ImageFont.truetype(path, size)
        except OSError:
            continue
    return ImageFont.load_default()


title_font = load_font(96)
body_font = load_font_reg(48)
small_font = load_font_reg(36)

os.makedirs("assets", exist_ok=True)

for code, desc, color in parts:
    img = Image.new("RGB", (W, H), "white")
    d = ImageDraw.Draw(img)

    d.rectangle([0, 0, W, 150], fill=color)
    d.text((40, 28), code, fill="white", font=title_font)
    d.text((40, 168), desc, fill="#333", font=body_font)
    d.line([(40, 245), (W - 40, 245)], fill="#ccc", width=2)

    cx, cy = W // 2, 570
    d.rounded_rectangle([cx - 500, cy - 200, cx + 500, cy + 200], radius=40, outline=color, width=6)
    d.rectangle([cx - 380, cy - 40, cx - 240, cy + 40], outline=color, width=4)
    d.text((cx - 400, cy + 60), "SWITCH", fill=color, font=small_font)
    for i in range(8):
        d.ellipse([cx + 200 + i * 30, cy - 60, cx + 220 + i * 30, cy - 40], outline=color, width=2)
    d.text((cx + 200, cy + 60), "GRILLE", fill=color, font=small_font)
    d.rectangle([cx - 100, cy - 120, cx + 100, cy - 90], outline=color, width=4)
    d.text((cx - 60, cy - 160), "ARM REST", fill=color, font=small_font)

    y = 840
    for step in [
        "1. Verify part orientation matches diagram",
        "2. Fit switch bezel  -  torque 1.2 Nm",
        "3. Snap-fit speaker grille  -  press until 4 clicks",
        "4. Attach arm rest with 2 x M6 screws  -  torque 8 Nm",
    ]:
        d.text((60, y), step, fill="#111", font=small_font)
        y += 42

    img.save(f"assets/{code}.png", "PNG")
    print(f"wrote assets/{code}.png")

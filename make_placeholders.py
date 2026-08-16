"""Generate 4 placeholder spec images."""
from PIL import Image, ImageDraw, ImageFont
import os

parts = [
    ("SB390GKG", "Door Garnish - Front Left",  "#1F3A68"),
    ("SB391GKG", "Door Garnish - Front Right", "#2A5A3E"),
    ("SB392GKG", "Door Garnish - Rear Left",   "#6B3A1F"),
    ("SB393GKG", "Door Garnish - Rear Right",  "#5A2A6B"),
]

W, H = 1600, 1000
try:
    title_font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 90)
    body_font  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 46)
    small_font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 34)
except OSError:
    title_font = body_font = small_font = ImageFont.load_default()

os.makedirs("assets", exist_ok=True)

for code, desc, color in parts:
    img = Image.new("RGB", (W, H), "white")
    d = ImageDraw.Draw(img)

    d.rectangle([0, 0, W, 140], fill=color)
    d.text((40, 30), code, fill="white", font=title_font)
    d.text((40, 155), desc, fill="#333", font=body_font)
    d.line([(40, 230), (W - 40, 230)], fill="#ccc", width=2)

    cx, cy = W // 2, 560
    d.rounded_rectangle([cx - 500, cy - 200, cx + 500, cy + 200], radius=40, outline=color, width=6)
    d.rectangle([cx - 380, cy - 40, cx - 240, cy + 40], outline=color, width=4)
    d.text((cx - 400, cy + 60), "SWITCH", fill=color, font=small_font)
    for i in range(8):
        d.ellipse([cx + 200 + i*30, cy - 60, cx + 220 + i*30, cy - 40], outline=color, width=2)
    d.text((cx + 200, cy + 60), "GRILLE", fill=color, font=small_font)
    d.rectangle([cx - 100, cy - 120, cx + 100, cy - 90], outline=color, width=4)
    d.text((cx - 60, cy - 160), "ARM REST", fill=color, font=small_font)

    y = 830
    for step in [
        "1. Verify part orientation matches diagram",
        "2. Fit switch bezel  -  torque 1.2 Nm",
        "3. Snap-fit speaker grille  -  press until 4 clicks",
        "4. Attach arm rest with 2 x M6 screws  -  torque 8 Nm",
    ]:
        d.text((60, y), step, fill="#111", font=small_font)
        y += 40

    img.save(f"assets/{code}.png", "PNG")
    print(f"wrote assets/{code}.png")

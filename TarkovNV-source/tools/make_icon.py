"""Generates assets/tarkovnv.ico - a night-vision goggle mark in TarkovNV colours."""
import sys
from PIL import Image, ImageDraw

BG = (10, 11, 9, 255)
FRAME = (69, 70, 58, 255)
NV = (139, 255, 69, 255)
NV_DK = (31, 64, 16, 255)


def draw(size):
    s = size * 4  # supersample
    im = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    r = s * 0.16
    d.rounded_rectangle([0, 0, s - 1, s - 1], radius=r, fill=BG, outline=FRAME, width=max(4, s // 40))
    # goggle housing
    cy = s * 0.53
    lr = s * 0.205
    for cx in (s * 0.305, s * 0.695):
        d.ellipse([cx - lr - s * 0.05, cy - lr - s * 0.05, cx + lr + s * 0.05, cy + lr + s * 0.05], fill=FRAME)
    d.rectangle([s * 0.3, cy - s * 0.07, s * 0.7, cy + s * 0.07], fill=FRAME)
    for cx in (s * 0.305, s * 0.695):
        d.ellipse([cx - lr, cy - lr, cx + lr, cy + lr], fill=NV_DK)
        ir = lr * 0.72
        d.ellipse([cx - ir, cy - ir, cx + ir, cy + ir], fill=NV)
        hr = lr * 0.22
        d.ellipse([cx - ir * 0.45 - hr, cy - ir * 0.45 - hr, cx - ir * 0.45 + hr, cy - ir * 0.45 + hr], fill=(220, 255, 200, 255))
    # mount on top
    d.rectangle([s * 0.44, s * 0.14, s * 0.56, cy - lr * 0.6], fill=FRAME)
    d.rectangle([s * 0.36, s * 0.12, s * 0.64, s * 0.2], fill=FRAME)
    return im.resize((size, size), Image.LANCZOS)


if __name__ == "__main__":
    out = sys.argv[1]
    sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
    imgs = [draw(z) for z in sizes]
    imgs[-1].save(out, format="ICO", sizes=[(z, z) for z in sizes], append_images=imgs[:-1])
    draw(256).save(out.replace(".ico", ".png"))

#!/usr/bin/env python3
"""app/AppIcon.svg → app/AppIcon.png (1024) · app/AppIcon.icns · engine/web/icon.png (Playwright + Chromium 필요)"""
import asyncio, io, os, struct
from playwright.async_api import async_playwright

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SVG = os.path.join(ROOT, "app", "AppIcon.svg")

async def render(size):
    async with async_playwright() as p:
        b = await p.chromium.launch()
        pg = await b.new_page(viewport={"width": size, "height": size}, device_scale_factor=1)
        svg = open(SVG, encoding="utf-8").read()
        await pg.set_content(f'<html><body style="margin:0;background:transparent">'
                             f'<img src="data:image/svg+xml;base64,{__import__("base64").b64encode(svg.encode()).decode()}" '
                             f'width="{size}" height="{size}" style="display:block"></body></html>')
        await pg.wait_for_timeout(200)
        png = await pg.screenshot(omit_background=True, clip={"x": 0, "y": 0, "width": size, "height": size})
        await b.close()
        return png

async def main():
    sizes = {16: None, 32: None, 64: None, 128: None, 256: None, 512: None, 1024: None}
    for s in sizes:
        sizes[s] = await render(s)
    open(os.path.join(ROOT, "app", "AppIcon.png"), "wb").write(sizes[1024])
    open(os.path.join(ROOT, "engine", "web", "icon.png"), "wb").write(sizes[256])
    # icns: PNG 를 그대로 담는 형식들
    entries = [(b"icp4", 16), (b"icp5", 32), (b"icp6", 64), (b"ic07", 128), (b"ic08", 256), (b"ic09", 512), (b"ic10", 1024),
               (b"ic11", 32), (b"ic12", 64), (b"ic13", 256), (b"ic14", 512)]
    body = b"".join(t + struct.pack(">I", len(sizes[s]) + 8) + sizes[s] for t, s in entries)
    open(os.path.join(ROOT, "app", "AppIcon.icns"), "wb").write(b"icns" + struct.pack(">I", len(body) + 8) + body)
    print("✔ app/AppIcon.png · app/AppIcon.icns · engine/web/icon.png")

asyncio.run(main())

#!/usr/bin/env python3
"""Write the update feed (latest.json).
usage: feed.py <feed.json> <version> <zip path> [notes...]"""
import datetime, hashlib, json, os, sys

path, ver, zpath, *notes = sys.argv[1:]
data = open(zpath, "rb").read()
json.dump({"version": ver, "date": datetime.date.today().isoformat(), "zip": os.path.basename(zpath),
           "sha256": hashlib.sha256(data).hexdigest(), "size": len(data), "notes": notes},
          open(path, "w"), ensure_ascii=False, indent=2)
print(f"✔ {os.path.basename(zpath)} ({len(data)} bytes) → {os.path.basename(path)} v{ver}")

#!/usr/bin/env python3
"""Writes catalogue.json and icon/<id>.png for the CRX files in <dir>/crx/.

Run by tools/fetch-extensions.sh. One entry per extension, the newest version on disk:
id, name, version, description, size and the CRX's path in the store.
"""
import io
import json
import os
import struct
import sys
import zipfile


def zip_part(data):
    # CRX3: "Cr24", version 3, header length, header, then the zip.
    magic, version, header_len = data[:4], *struct.unpack("<II", data[4:12])
    if magic != b"Cr24" or version != 3:
        raise ValueError("not a CRX3 file")
    return zipfile.ZipFile(io.BytesIO(data[12 + header_len:]))


def message(z, manifest, text):
    # "__MSG_name__" is looked up in the default locale; keys aren't case sensitive.
    if not text.startswith("__MSG_") or not text.endswith("__"):
        return text
    key = text[6:-2].lower()
    path = "_locales/%s/messages.json" % manifest.get("default_locale", "en")
    messages = json.loads(z.read(path).decode("utf-8-sig"))
    for k, v in messages.items():
        if k.lower() == key:
            return v["message"]
    return text


def version_key(v):
    return [int(p) for p in v.split(".")]


def main(root):
    newest = {}
    for name in sorted(os.listdir(os.path.join(root, "crx"))):
        if not name.endswith(".crx"):
            continue
        ext_id, version = name[:-4].split("_", 1)
        if ext_id not in newest or version_key(version) > version_key(newest[ext_id][0]):
            newest[ext_id] = (version, name)

    catalogue = []
    for ext_id, (version, name) in sorted(newest.items()):
        path = os.path.join(root, "crx", name)
        data = open(path, "rb").read()
        z = zip_part(data)
        manifest = json.loads(z.read("manifest.json").decode("utf-8-sig"))
        icons = manifest.get("icons", {})
        if icons:
            biggest = icons[max(icons, key=int)]
            with open(os.path.join(root, "icon", ext_id + ".png"), "wb") as f:
                f.write(z.read(biggest.lstrip("/")))
        catalogue.append({
            "id": ext_id,
            "name": message(z, manifest, manifest["name"]),
            "version": version,
            "description": message(z, manifest, manifest.get("description", "")),
            "size": len(data),
            "crx": "crx/" + name,
            "icon": "icon/%s.png" % ext_id if icons else "",
        })
        print("%-16s %-34s %s" % (version, catalogue[-1]["name"][:34], ext_id))

    with open(os.path.join(root, "catalogue.json"), "w") as f:
        json.dump(catalogue, f, indent=2)
        f.write("\n")


if __name__ == "__main__":
    main(sys.argv[1])

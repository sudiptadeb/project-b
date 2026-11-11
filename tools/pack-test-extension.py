#!/usr/bin/env python3
"""Packs tools/test-extension/ as a CRX3 into the mock store's folder.

    tools/pack-test-extension.py [version]      # 1.0 unless named

The version replaces the manifest's, so packing 1.0 and later 1.1 gives the store an
update to offer. The package is signed with tools/test-extension.pem, a key that exists
only so the extension keeps one id. Then the store's catalogue is written again.
"""
import hashlib
import io
import json
import os
import struct
import subprocess
import sys
import zipfile

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
SOURCE = os.path.join(ROOT, "tools/test-extension")
KEY = os.path.join(ROOT, "tools/test-extension.pem")
STORE = os.path.join(ROOT, ".tools/extensions")


def field(number, data):
    # A protobuf length-delimited field.
    out, key, length = bytearray(), number << 3 | 2, len(data)
    for value in (key, length):
        while value > 0x7F:
            out.append(value & 0x7F | 0x80)
            value >>= 7
        out.append(value)
    return bytes(out) + data


def main(version):
    archive = io.BytesIO()
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as z:
        for name in sorted(os.listdir(SOURCE)):
            data = open(os.path.join(SOURCE, name), "rb").read()
            if name == "manifest.json":
                manifest = json.loads(data)
                manifest["version"] = version
                data = json.dumps(manifest, indent=2).encode()
            z.writestr(zipfile.ZipInfo(name), data)
    archive = archive.getvalue()

    # CRX3: "Cr24", 3, the header's length, the header, the zip. The header holds the
    # public key, the signature, and the id, which is the key's hash. See
    # components/crx_file/crx3.proto in Chromium.
    public_key = subprocess.check_output(
        ["openssl", "rsa", "-in", KEY, "-pubout", "-outform", "DER"], stderr=subprocess.DEVNULL)
    crx_id = hashlib.sha256(public_key).digest()[:16]
    signed_data = field(1, crx_id)
    to_sign = b"CRX3 SignedData\x00" + struct.pack("<I", len(signed_data)) + signed_data + archive
    signature = subprocess.run(
        ["openssl", "dgst", "-sha256", "-sign", KEY], input=to_sign, check=True,
        stdout=subprocess.PIPE).stdout
    header = field(2, field(1, public_key) + field(2, signature)) + field(10000, signed_data)

    ext_id = "".join(chr(ord("a") + int(c, 16)) for c in crx_id.hex())
    os.makedirs(os.path.join(STORE, "crx"), exist_ok=True)
    os.makedirs(os.path.join(STORE, "icon"), exist_ok=True)
    path = os.path.join(STORE, "crx", "%s_%s.crx" % (ext_id, version))
    with open(path, "wb") as f:
        f.write(b"Cr24" + struct.pack("<II", 3, len(header)) + header + archive)
    print("packed", os.path.relpath(path, ROOT), flush=True)
    subprocess.check_call(
        [sys.executable, "-I", os.path.join(ROOT, "tools/extension_catalogue.py"), STORE])


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "1.0")

"""Open a few pages in the packaged macOS app and report what each did, and any renderer kill.

Usage: macprobe.py <path to project_b binary>
Each page sets its title to the result; the page's server records it too.
"""
import http.server
import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request

BIN = sys.argv[1]
results = {}

JS = {
    "badge": "navigator.setAppBadge(3).then(()=>'resolved',e=>'rej:'+e.name)",
    "badgeClear": "navigator.clearAppBadge().then(()=>'resolved',e=>'rej:'+e.name)",
    "speechLocal": "SpeechRecognition.available({langs:['en-US'],processLocally:true}).then(r=>'got:'+r,e=>'rej:'+e.name)",
    "languageModel": "(async()=>{const a=await LanguageModel.availability();if(a!=='available'&&a!=='downloadable')return 'availability:'+a;"
                     "const m=await LanguageModel.create();return 'availability:'+a+' answer:'+(await m.prompt('What is 2+2?')).slice(0,80)})().catch(e=>'rej:'+e.name+':'+e.message)",
    "translator": "Promise.race([Translator.availability({sourceLanguage:'en',targetLanguage:'fr'}).then(r=>'got:'+r,e=>'rej:'+e.name),"
                  "new Promise(r=>setTimeout(()=>r('no answer in 4s'),4000))])",
    "share": "Promise.resolve(typeof navigator.share)",
    "installedApps": "Promise.resolve(typeof navigator.getInstalledRelatedApps)",
    "paymentRequest": "Promise.resolve(typeof PaymentRequest)",
    "clipboardWrite": "navigator.clipboard.writeText('x').then(()=>'resolved',e=>'rej:'+e.name)",
    "geolocation": "new Promise(r=>navigator.geolocation.getCurrentPosition(()=>r('got'),e=>r('err:'+e.code),{timeout:3000}))",
    "eyeDropper": "Promise.resolve(typeof EyeDropper)",
    "hid": "navigator.hid ? Promise.race([navigator.hid.getDevices().then(d=>'devices:'+d.length,e=>'rej:'+e.name),new Promise(r=>setTimeout(()=>r('no answer in 3s'),3000))]) : Promise.resolve('absent')",
    "queryLocalFonts": "typeof queryLocalFonts=='function' ? queryLocalFonts().then(f=>'fonts:'+f.length,e=>'rej:'+e.name) : Promise.resolve('absent')",
    "webauthn": "navigator.credentials.create({publicKey:{challenge:new Uint8Array(16),rp:{name:'x'},user:{id:new Uint8Array(4),name:'u',displayName:'u'},pubKeyCredParams:[{type:'public-key',alg:-7}],timeout:2000}}).then(()=>'created',e=>'rej:'+e.name+':'+e.message)",
}


def page(name):
    return ("<title>%s:pending</title><script>"
            "Promise.resolve().then(()=>%s).then(r=>{document.title='%s:'+r;fetch('/r?%s='+encodeURIComponent(r))},"
            "e=>{document.title='%s:throw:'+e;fetch('/r?%s='+encodeURIComponent('throw:'+e))})</script>" % (name, JS[name], name, name, name, name))


class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/r?"):
            k, _, v = self.path[3:].partition("=")
            results[k] = urllib.request.unquote(v)
            body = b""
        else:
            n = self.path.strip("/"); body = page(n).encode() if n in JS else b""
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), H)
threading.Thread(target=srv.serve_forever, daemon=True).start()
port = srv.server_address[1]

sys.path.insert(0, sys.argv[2])
os.environ["PB_BIN"] = BIN
import drive_first_app as d
d.OUT = tempfile.mkdtemp(prefix="pb-macprobe-out-")
d.BIN = BIN
for name in JS:
    profile = tempfile.mkdtemp(prefix="pb-macprobe-")
    app = d.App(profile)
    url = "http://127.0.0.1:%d/%s" % (port, name)
    d.focus_bar(app)
    app.cmd("type", url)
    app.cmd("key", "enter")
    deadline = time.time() + 15
    while time.time() < deadline and name not in results:
        time.sleep(0.3)
    time.sleep(2)
    try:
        app.close()
    except Exception:
        app.proc.kill()
    log = open(os.path.join(d.OUT, "app.log"))
    kills = [l.strip() for l in log if "bad Mojo message" in l]
    open(os.path.join(d.OUT, "app.log"), "w").close()
    print("%-15s %-50s %s" % (name, results.get(name, "(no report)"), " | ".join(kills)[:300]))
    sys.stdout.flush()
    shutil.rmtree(profile, ignore_errors=True)

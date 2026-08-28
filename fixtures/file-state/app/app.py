import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

STATE = Path("/data/value")
VERSION = os.getenv("APP_VERSION", "unknown")


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self.reply(200, "ok")
        elif self.path == "/value" and STATE.exists():
            self.reply(200, STATE.read_text(encoding="utf-8"))
        else:
            self.reply(404, "missing")

    def do_POST(self):
        if self.path != "/seed":
            self.reply(404, "missing")
            return
        length = int(self.headers.get("Content-Length", "0"))
        STATE.write_bytes(self.rfile.read(length))
        self.reply(201, "seeded")

    def reply(self, status, body):
        data = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "text/plain")
        self.send_header("X-App-Version", VERSION)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, fmt, *args):
        print(f"{VERSION}: {fmt % args}", flush=True)


ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()

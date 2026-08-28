import os
import subprocess
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def sql(statement):
    return subprocess.run(
        ["psql", "-h", "postgres", "-U", "upgradeproof", "-d", "upgradeproof", "-At", "-c", statement],
        check=True,
        text=True,
        capture_output=True,
    ).stdout.strip()


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        try:
            if self.path == "/health":
                sql("SELECT 1")
                self.reply(200, "ok")
            elif self.path == "/users":
                self.reply(200, sql("SELECT name FROM users WHERE id=1"))
            else:
                self.reply(404, "missing")
        except subprocess.CalledProcessError as exc:
            self.reply(503, exc.stderr)

    def do_POST(self):
        if self.path != "/seed":
            self.reply(404, "missing")
            return
        try:
            sql("CREATE TABLE IF NOT EXISTS users (id integer PRIMARY KEY, name text NOT NULL); INSERT INTO users(id,name) VALUES (1,'Ada') ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name")
            self.reply(201, "seeded")
        except subprocess.CalledProcessError as exc:
            self.reply(503, exc.stderr)

    def reply(self, status, body):
        data = body.encode()
        self.send_response(status)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()

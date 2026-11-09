"""GitHub de mentira (só o pedaço da API que o programa usa), para os testes."""
import base64
import hashlib
import json
import threading
import http.server
import socketserver
from urllib.parse import urlparse, parse_qs, unquote

TOKEN = "TESTE"


class Repo:
    def __init__(self):
        self.ramos = {"main": {"index.html": b"<html>site</html>"}}
        self.commits = 0


class GitHubFalso:
    def __init__(self, repos=("automec135/automec135.github.io",)):
        self.repos = {r: Repo() for r in repos}
        self.pedidos = []
        gh = self

        class H(http.server.BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def _resp(self, st, obj=None, cru=None, tipo="application/json"):
                corpo = cru if cru is not None else json.dumps(obj or {}).encode()
                self.send_response(st)
                self.send_header("Content-Type", tipo)
                self.send_header("Access-Control-Allow-Origin", "*")
                self.send_header("Access-Control-Allow-Headers", "Accept, Authorization")
                self.send_header("Content-Length", str(len(corpo)))
                self.end_headers()
                self.wfile.write(corpo)

            def do_OPTIONS(self):
                self._resp(204, cru=b"")

            def _auth(self):
                return self.headers.get("Authorization") == "Bearer " + TOKEN

            def _rota(self):
                u = urlparse(self.path)
                partes = [unquote(p) for p in u.path.strip("/").split("/")]
                return u, partes

            def do_GET(self):
                gh.pedidos.append(("GET", self.path))
                u, p = self._rota()
                q = parse_qs(u.query)
                publico = len(p) >= 5 and p[3] == "contents" and "raw" in (self.headers.get("Accept") or "")
                if not publico and not self._auth():
                    return self._resp(401, {"message": "Bad credentials"})
                if len(p) < 3 or p[0] != "repos":
                    return self._resp(404, {"message": "Not Found"})
                nome = p[1] + "/" + p[2]
                r = gh.repos.get(nome)
                if not r:
                    return self._resp(404, {"message": "Not Found"})
                if len(p) == 3:
                    return self._resp(200, {"default_branch": "main", "permissions": {"push": True}})
                if p[3:5] == ["git", "ref"] and p[5] == "heads":
                    ramo = "/".join(p[6:])
                    if ramo not in r.ramos:
                        return self._resp(404, {"message": "Not Found"})
                    return self._resp(200, {"object": {"sha": "sha-" + ramo}})
                if p[3] == "contents":
                    caminho = "/".join(p[4:])
                    ramo = q.get("ref", ["main"])[0]
                    arqs = r.ramos.get(ramo, {})
                    if caminho not in arqs:
                        return self._resp(404, {"message": "Not Found"})
                    c = arqs[caminho]
                    if publico:
                        return self._resp(200, cru=c, tipo="text/plain; charset=utf-8")
                    return self._resp(200, {"sha": hashlib.sha1(c).hexdigest(), "encoding": "base64",
                                            "content": base64.encodebytes(c).decode()})
                return self._resp(404, {"message": "Not Found"})

            def _corpo(self):
                n = int(self.headers.get("Content-Length") or 0)
                return json.loads(self.rfile.read(n) or b"{}")

            def do_POST(self):
                gh.pedidos.append(("POST", self.path))
                if not self._auth():
                    return self._resp(401, {"message": "Bad credentials"})
                u, p = self._rota()
                r = gh.repos.get(p[1] + "/" + p[2])
                d = self._corpo()
                if p[3:5] == ["git", "refs"]:
                    ramo = d["ref"].replace("refs/heads/", "")
                    r.ramos[ramo] = dict(r.ramos["main"])
                    return self._resp(201, {"ref": d["ref"]})
                return self._resp(404, {"message": "Not Found"})

            def do_PUT(self):
                gh.pedidos.append(("PUT", self.path))
                if not self._auth():
                    return self._resp(401, {"message": "Bad credentials"})
                u, p = self._rota()
                r = gh.repos.get(p[1] + "/" + p[2])
                if not r:
                    return self._resp(404, {"message": "Not Found"})
                d = self._corpo()
                caminho = "/".join(p[4:])
                ramo = d.get("branch", "main")
                arqs = r.ramos.setdefault(ramo, {})
                if caminho in arqs and d.get("sha") != hashlib.sha1(arqs[caminho]).hexdigest():
                    return self._resp(409, {"message": "sha does not match"})
                if caminho not in arqs and d.get("sha"):
                    return self._resp(422, {"message": "sha wasn't supplied"})
                arqs[caminho] = base64.b64decode(d["content"])
                r.commits += 1
                return self._resp(200, {"content": {"sha": hashlib.sha1(arqs[caminho]).hexdigest()}})

        class S(socketserver.ThreadingMixIn, socketserver.TCPServer):
            daemon_threads = True
            allow_reuse_address = True

        self.srv = S(("127.0.0.1", 0), H)
        self.porta = self.srv.server_address[1]
        self.url = f"http://127.0.0.1:{self.porta}"
        threading.Thread(target=self.srv.serve_forever, daemon=True).start()

    def arquivo(self, repo, ramo, caminho):
        return self.repos[repo].ramos.get(ramo, {}).get(caminho)

    def parar(self):
        self.srv.shutdown()

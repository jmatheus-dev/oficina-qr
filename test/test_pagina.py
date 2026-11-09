"""Testa a página pública (site/index.html) com a API simulada e tira capturas."""
import json
import re
import sys
import threading
import http.server
import socketserver
from pathlib import Path
from playwright.sync_api import sync_playwright

RAIZ = Path(__file__).resolve().parent.parent
SAIDA = Path(sys.argv[1] if len(sys.argv) > 1 else "/tmp/capturas")
SAIDA.mkdir(parents=True, exist_ok=True)
CSV_URL = "https://docs.google.com/spreadsheets/d/e/2PACX-TESTE/pub?gid=0&single=true&output=csv"

# cópia da página apontando para a API de teste
teste_dir = SAIDA / "site"
teste_dir.mkdir(exist_ok=True)
html = (RAIZ / "site" / "index.html").read_text(encoding="utf-8")
assert "api.github.com/repos/automec135/automec135.github.io/contents/historico.csv" in html
(teste_dir / "index.html").write_text(html, encoding="utf-8")
import shutil
for png in (RAIZ / "site").glob("*.png"):
    shutil.copy(png, teste_dir / png.name)

import csv, io, hashlib


def pid(codigo):
    return hashlib.sha256(("automec135|" + codigo).encode()).hexdigest()[:20]


LINHAS = [
    ["id", "modelo", "ano", "data", "km", "itens", "outros", "proxima_troca_km", "obs"],
    ["_", "2026-10-21T17:43", "", "", "", "", "", "", ""],
    [pid("EXPL6789"), "Fiat Strada", "2019", "2025-01-20", "140100", "Óleo do motor", "<script>alert(1)</script>", "147100", ""],
    [pid("EXPL6789"), "Fiat Strada", "2019", "2025-11-14", "153273", "Óleo do motor; Filtro de óleo", "", "160273", "Óleo 5W30 sintético"],
    [pid("EXPL6789"), "Fiat Strada", "2019", "2025-05-02", "145010", "", "Troca da correia dentada e do tensor,\nbomba \"d'água\"", "", ""],
    [pid("SEMSERV2"), "", "", "", "", "", "", "", ""],
    [pid("SOOUTRO3"), "Gol", "2012", "2025-03-03", "98000", "", "Embreagem nova", "", ""],
]
def para_csv(linhas, fim="\r\n"):
    b = io.StringIO()
    csv.writer(b, lineterminator=fim).writerows(linhas)
    return b.getvalue()


# planilha antiga (Google): tudo; programa novo (GitHub): um serviço novo do mesmo carro + repetido
CSV_TEXTO = para_csv(LINHAS)
import base64, hmac as _hmac
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def cifrar_placa(placa, codigo):
    # igual ao dados.CifrarPlaca do programa
    chave = hashlib.sha256(("placa|" + codigo).encode()).digest()
    nonce = _hmac.new(chave, placa.encode(), hashlib.sha256).digest()[:12]
    return base64.b64encode(nonce + AESGCM(chave).encrypt(nonce, placa.encode(), None)).decode()


CAB_NOVO = LINHAS[0] + ["proxima_troca_data", "proxima_correia_km", "proxima_correia_data", "garantia", "placa_cifrada"]
def novo(l, *extra):
    return l + list(extra) + [""] * (len(CAB_NOVO) - len(l) - len(extra))
CSV_APP = para_csv([CAB_NOVO, novo(["_", "2026-10-22T09:00", "", "", "", "", "", "", ""]),
                    novo(LINHAS[3]),  # repetido nas duas fontes: deve aparecer uma vez só
                    novo([pid("EXPL6789"), "Fiat Strada", "2019", "2026-10-22", "160500", "", "Mangueira de ar", "", ""], "", "", "", "", cifrar_placa("BRA2E19", "EXPL6789")),
                    novo([pid("SOAPP777"), "Toyota Corolla", "2015", "2026-10-22", "98765", "Óleo do motor; Filtro de óleo", "Mangueira de ar", "108765", ""], "2027-04-22", "", "", "", cifrar_placa("PKZ7C10", "SOAPP777"))], "\n")



class Silencioso(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **k):
        super().__init__(*a, directory=str(teste_dir), **k)

    def log_message(self, *a):
        pass


srv = socketserver.TCPServer(("127.0.0.1", 0), Silencioso)
porta = srv.server_address[1]
threading.Thread(target=srv.serve_forever, daemon=True).start()
BASE = f"http://127.0.0.1:{porta}/"

falhas = []


def checar(cond, msg):
    if not cond:
        falhas.append(msg)
        print("FALHOU:", msg)


with sync_playwright() as p:
    nav = p.chromium.launch()

    def nova(largura=390, altura=844, escuro=False, api_cai=False, github="api", google=True):
        ctx = nav.new_context(viewport={"width": largura, "height": altura}, device_scale_factor=2,
                              color_scheme="dark" if escuro else "light", locale="pt-BR")
        pg = ctx.new_page()
        erros = []
        pg.on("pageerror", lambda e: erros.append(str(e)))
        pg.on("console", lambda m: erros.append(m.text) if m.type == "error" and "fonts.g" not in (m.location or {}).get("url", "") else None)
        pg.route("https://fonts.googleapis.com/**", lambda r: r.abort())
        pg.route("https://fonts.gstatic.com/**", lambda r: r.abort())

        cab = {"Access-Control-Allow-Origin": "*", "Access-Control-Allow-Headers": "Accept",
               "Content-Type": "text/csv; charset=utf-8"}

        def api(route):
            if api_cai or not google:
                return route.abort()
            route.fulfill(status=200, body=CSV_TEXTO.encode("utf-8"), headers=cab)

        def gh_api(route):
            if route.request.method == "OPTIONS":
                return route.fulfill(status=204, headers=cab)
            if api_cai or github not in ("api",):
                return route.abort() if github != "404" else route.fulfill(status=404, body="{}", headers=cab)
            route.fulfill(status=200, body=CSV_APP.encode("utf-8"), headers=cab)

        def gh_raw(route):
            if api_cai or github not in ("raw",):
                return route.abort()
            route.fulfill(status=200, body=CSV_APP.encode("utf-8"), headers=cab)
        pg.route("https://docs.google.com/spreadsheets/**", api)
        pg.route("https://api.github.com/**", gh_api)
        pg.route("https://raw.githubusercontent.com/**", gh_raw)
        pg.on("dialog", lambda d: (falhas.append("alert disparou (XSS)"), d.dismiss()))
        return ctx, pg, erros

    # 1. histórico completo
    ctx, pg, erros = nova()
    pg.goto(BASE + "?c=expl6789")
    pg.wait_for_selector(".tempo li")
    pg.wait_for_timeout(800)
    checar(pg.inner_text("h1").startswith("Fiat Strada"), "título com modelo")
    checar(pg.get_attribute(".desenho", "aria-label") == "Desenho de uma picape pequena", "Strada desenhada como picape")
    checar(pg.inner_text(".placa .letras") == "BRA2E19", "placa aberta com o código do QR")
    checar("22/10/2026, às 09:00" in pg.inner_text("#rodape"), "data de atualização mais nova das fontes")
    checar("EXPL6789" in pg.inner_text("#rodape .codigo"), "código do histórico no rodapé")
    checar(pg.inner_text(".prox-km").replace("\u00a0", " ") == "160.273 km" and pg.inner_text(".prox-data") == "14/05/2026", "próxima troca por KM e data")
    checar(pg.locator(".tempo > li").count() == 4, "4 serviços (planilha + programa, sem repetir)")
    checar(pg.locator(".tempo > li:not([hidden])").count() == 3, "mostra 3 e esconde o resto")
    checar(pg.inner_text(".tempo > li:first-child").find("Mangueira de ar") >= 0, "serviço novo do programa aparece primeiro")
    checar(pg.locator(".tempo > li.oleo").count() == 2, "2 trocas de óleo marcadas")
    pg.click("button:has-text('Ver todos os 4 serviços')")
    checar(pg.locator(".tempo > li:not([hidden])").count() == 4, "Ver todos abre o resto")
    checar("bomba \"d'água\"" in pg.inner_text(".tempo"), "CSV com aspas e quebra de linha")
    checar("<script>" in pg.inner_text(".tempo"), "texto escapado, não executado")
    wa = pg.get_attribute(".flutua a.botao", "href")
    checar(wa.startswith("https://wa.me/557998513926?text=") and "EXPL6789" in wa, "link do WhatsApp com código")
    checar("troca%20de%20%C3%B3leo" in wa, "WhatsApp pede troca de óleo quando está atrasada")
    checar(pg.evaluate("document.documentElement.scrollWidth <= innerWidth"), "sem rolagem lateral 390px")
    checar(pg.inner_text(".selo-topo") == "Manutenção atrasada" and pg.inner_text(".flutua .quando") == "Atrasada", "selo de atrasada no topo e no cartão")
    checar("Garantia de 90 dias: até 20/01/2027" in pg.inner_text(".tempo > li:first-child"), "garantia do serviço recente")
    checar("terminou em" in pg.inner_text(".tempo > li:last-child"), "garantia vencida nos antigos")
    checar("Mangueira de ar" in pg.inner_text(".feito") and "Serviço concluído" in pg.inner_text(".feito"), "último serviço em verde")
    checar(pg.locator(".saude li").count() == 7 and "sem registro aqui" in pg.inner_text(".saude li:has-text('Filtro de ar')"), "saúde do carro com 7 itens")
    checar(pg.locator(".avaliar a[href*='writereview']").count() == 1, "avaliação no Google")
    with pg.expect_download() as dl:
        pg.click("button:has-text('Me lembre da próxima troca')")
    ics = Path(dl.value.path()).read_text(encoding="utf-8")
    checar("BEGIN:VEVENT" in ics and "DTSTART;VALUE=DATE:" in ics and "EXPL6789" in ics and "TRIGGER:-P1D" in ics, "lembrete .ics para a agenda")
    checar("calendar.google.com" in pg.get_attribute("a:has-text('Agenda do Google')", "href"), "link da Agenda do Google")
    checar(pg.locator("button:has-text('Indicar')").count() == 1 and pg.locator("button:has-text('PDF')").count() == 1, "botões indicar e PDF")
    pg.screenshot(path=str(SAIDA / "1-historico.png"), full_page=True)
    pg.emulate_media(media="print")
    checar(pg.is_visible(".cert") and "EXPL6789" in pg.inner_text(".cert"), "certificado aparece no PDF com código de verificação")
    checar(not pg.is_visible(".acoes"), "botões não saem no PDF")
    pg.pdf(path=str(SAIDA / "certificado.pdf"))
    pg.emulate_media(media="screen")
    checar(not erros, f"erros no console: {erros}")
    ctx.close()

    # código errado não abre a placa
    ctx, pg, erros = nova()
    pg.goto(BASE + "?c=expl6789")
    pg.wait_for_selector(".tempo li")
    ok = pg.evaluate("""async () => (await abrirPlaca('""" + cifrar_placa("BRA2E19", "EXPL6789") + """', 'ERRADO12')) === ''""")
    checar(ok, "placa não abre com outro código")
    ctx.close()

    # 2. escuro
    ctx, pg, erros = nova(escuro=True)
    pg.goto(BASE + "?c=EXPL6789")
    pg.wait_for_selector(".tempo li")
    pg.wait_for_timeout(1600)
    pg.screenshot(path=str(SAIDA / "2-escuro.png"), full_page=True)
    ctx.close()

    # 3. não encontrado
    ctx, pg, erros = nova()
    pg.goto(BASE + "?c=ZZZZZZZZ")
    pg.wait_for_selector("text=Veículo não encontrado")
    pg.screenshot(path=str(SAIDA / "3-nao-encontrado.png"), full_page=True)
    ctx.close()

    # 4. sem internet / API fora
    ctx, pg, erros = nova(api_cai=True)
    pg.goto(BASE + "?c=EXPL6789")
    pg.wait_for_selector("text=Não deu para carregar o histórico")
    checar(pg.locator("button:has-text('Tentar de novo')").count() == 1, "botão tentar de novo")
    pg.screenshot(path=str(SAIDA / "4-sem-internet.png"), full_page=True)
    ctx.close()

    # 5. sem código
    ctx, pg, erros = nova()
    pg.goto(BASE)
    pg.wait_for_selector("text=aponte a câmera")
    pg.screenshot(path=str(SAIDA / "5-sem-codigo.png"), full_page=True)
    ctx.close()

    # 6. veículo sem serviços e veículo só com "outros"
    ctx, pg, erros = nova()
    pg.goto(BASE + "?c=SEMSERV2")
    pg.wait_for_selector("text=Ainda não há serviços")
    checar(pg.inner_text("h1") == "Seu veículo", "nome padrão sem modelo")
    pg.goto(BASE + "?c=abc")
    pg.wait_for_selector("text=Veículo não encontrado")
    pg.goto(BASE + "?c=SOOUTRO3")
    pg.wait_for_selector(".tempo li")
    pg.wait_for_timeout(1500)
    checar("Último serviço" in pg.inner_text(".flutua"), "cartão de último serviço sem troca de óleo")
    checar(pg.get_attribute(".desenho", "aria-label") == "Desenho de um carro hatch", "Gol desenhado como hatch")
    checar(pg.locator(".placa").count() == 0, "sem placa quando não veio do programa")
    pg.screenshot(path=str(SAIDA / "6-so-outros.png"), full_page=True)
    ctx.close()

    # 7. tela estreita (320px) e computador
    ctx, pg, erros = nova(largura=320, altura=640)
    pg.goto(BASE + "?c=EXPL6789")
    pg.wait_for_selector(".tempo li")
    pg.wait_for_timeout(1500)
    checar(pg.evaluate("document.documentElement.scrollWidth <= innerWidth"), "sem rolagem lateral 320px")
    pg.screenshot(path=str(SAIDA / "7-320px.png"), full_page=True)
    ctx.close()
    ctx, pg, erros = nova(largura=1280, altura=800)
    pg.goto(BASE + "?c=EXPL6789")
    pg.wait_for_selector(".tempo li")
    pg.wait_for_timeout(1500)
    pg.screenshot(path=str(SAIDA / "8-computador.png"), full_page=False)
    ctx.close()

    # 8b. só o programa novo (GitHub) e pelo endereço reserva (raw)
    ctx, pg, erros = nova(github="raw", google=False)
    pg.goto(BASE + "?c=SOAPP777")
    pg.wait_for_selector(".tempo li")
    checar(pg.inner_text("h1").startswith("Toyota Corolla"), "carro só do programa novo (via reserva raw)")
    checar(pg.get_attribute(".desenho", "aria-label") == "Desenho de um sedã médio" and pg.inner_text(".placa .letras") == "PKZ7C10", "Corolla: sedã e placa")
    checar(pg.inner_text(".prox-data") == "22/04/2027", "data da próxima troca vem do programa")
    ctx.close()
    # 8c. GitHub ainda sem dados (404) e planilha ok
    ctx, pg, erros = nova(github="404")
    pg.goto(BASE + "?c=SOOUTRO3")
    pg.wait_for_selector(".tempo li")
    checar(pg.inner_text("h1").startswith("Gol"), "funciona com o GitHub ainda vazio (404)")
    ctx.close()

    # 9. demonstração
    ctx, pg, erros = nova()
    pg.goto(BASE + "?c=DEMO")
    pg.wait_for_timeout(300)
    pg.screenshot(path=str(SAIDA / "9-carregando.png"))
    pg.wait_for_selector(".tempo li")
    pg.wait_for_timeout(1600)
    pg.screenshot(path=str(SAIDA / "9-demo.png"), full_page=True)
    ctx.close()
    nav.close()

srv.shutdown()
print("Capturas em", SAIDA)
print("FALHAS:" if falhas else "OK — página passou em todos os testes", *falhas, sep="\n  ")
sys.exit(1 if falhas else 0)

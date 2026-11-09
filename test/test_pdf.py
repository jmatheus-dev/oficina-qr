"""Teste do modo PDF: o balcão salva o PDF do Zeus numa pasta e o programa
registra e imprime sozinho (versão Linux do mesmo código)."""
import glob
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request
from pathlib import Path

from playwright.sync_api import sync_playwright

RAIZ = Path(__file__).resolve().parent.parent
BIN = sys.argv[1] if len(sys.argv) > 1 else "/tmp/oficinaqr"
SAIDA = Path(sys.argv[2] if len(sys.argv) > 2 else "/tmp/capturas-pdf")
SAIDA.mkdir(parents=True, exist_ok=True)
PDF = RAIZ / "app/internal/pdfzeus/testdata/orcamento-17-mini.pdf"
if not PDF.exists():
    print("pulado: PDF de exemplo do Zeus não incluído (dados de cliente)")
    sys.exit(0)
PORTA = 47997
URL = f"http://127.0.0.1:{PORTA}/"
falhas = []


def checar(cond, msg):
    print(("ok   " if cond else "FALHOU ") + msg)
    if not cond:
        falhas.append(msg)


def esperar(cond, segundos=15, passo=0.25):
    fim = time.time() + segundos
    while time.time() < fim:
        v = cond()
        if v:
            return v
        time.sleep(passo)
    return None


tmp = Path(tempfile.mkdtemp(prefix="oficinaqr-pdf-"))
pasta, imp, entrada = tmp / "dados", tmp / "impressoes", tmp / "entrada"
imp.mkdir(parents=True)
env = dict(os.environ, OFICINAQR_PASTA=str(pasta), OFICINAQR_SEM_JANELA="1", OFICINAQR_GITHUB_API="http://127.0.0.1:9",
           OFICINAQR_ESPERA_MS="400", OFICINAQR_IMPRESSOES=str(imp))
env.pop("OFICINAQR_ZEUS_TESTE", None)
proc = subprocess.Popen([BIN, "--porta", str(PORTA)], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
import atexit
atexit.register(lambda: proc.poll() is None and proc.kill())


def ping():
    try:
        return urllib.request.urlopen(URL + "api/ping", timeout=2).read() == b"oficinaqr"
    except Exception:
        return False


checar(bool(esperar(ping)), "programa abriu")
impressas = lambda: sorted(glob.glob(str(imp / "*.png")))

with sync_playwright() as p:
    nav = p.chromium.launch()
    pg = nav.new_context(viewport={"width": 1366, "height": 900}, locale="pt-BR").new_page()
    erros = []
    pg.on("pageerror", lambda e: erros.append(str(e)))
    pg.on("console", lambda m: erros.append(m.text) if m.type == "error" else None)
    pg.goto(URL + "#/config")
    pg.wait_for_selector("h1:has-text('Configurações')")
    sec = pg.locator("section:has(h2:has-text('Zeus'))")
    checar(sec.locator("input[value=pdf]").is_checked(), "modo PDF vem marcado em instalação nova")
    checar(not sec.locator("text=Servidor").is_visible(), "campos do banco escondidos no modo PDF")
    pg.select_option("select#c-imp", "Arquivo PNG (teste)")
    sec.locator("label.interruptor:has-text('Ler o Zeus automaticamente')").click()
    sec.locator("div.campo:has(label:has-text('Pasta onde o balcão')) input").fill(str(entrada))
    sec.locator("button:has-text('Salvar e testar')").click()
    pg.wait_for_selector("text=Pasta lida")
    cfg = json.loads((pasta / "config.json").read_text())
    checar(cfg["zeus"]["modo"] == "pdf" and cfg["zeus"]["pasta_pdf"] == str(entrada) and cfg["zeus"]["ativo"], "configuração do modo PDF gravada")
    checar(entrada.is_dir(), "pasta de entrada criada sozinha")

    # o balcão salva o PDF do orçamento na pasta
    shutil.copy(PDF, entrada / "orcamento-17.pdf.pdf")
    pg.request.post(URL + "api/zeus/ler", headers={"X-Oficina": "1"})
    def pend():
        d = json.loads((pasta / "dados.json").read_text())
        return [l for l in (d.get("lancamentos") or []) if l["status"] == "pendente"]
    checar(bool(esperar(lambda: pend(), 15)), "PDF lido e posto para revisão")
    time.sleep(1)
    checar(len(impressas()) == 0, "nada impresso antes da revisão")
    l = pend()[0]
    checar(l["placa"] == "TST1234" and l.get("modelo") == "Fiat Uno 1.0", f"placa e modelo tirados do PDF ({l['placa']} {l.get('modelo')})")
    pg.goto(URL + "#/revisar/" + "pdf%3Aorcamento-17")
    pg.wait_for_selector("h1:has-text('Revisar serviço do Zeus')")
    checar("há " in pg.inner_text(".zeus-info") and "Usar a data de hoje" in pg.inner_text(".zeus-info"), "avisa que a data do orçamento é antiga")
    pg.click("button:has-text('Usar a data de hoje')")
    pg.click("button:has-text('Confirmar e imprimir etiqueta')")
    pg.wait_for_selector("text=Pronto! Etiqueta impressa.")
    checar(len(impressas()) == 1, "imprimiu depois de confirmar")
    d = json.loads((pasta / "dados.json").read_text())
    sv = [s for s in d["servicos"] if s.get("zeus_ref") == "pdf:orcamento-17"]
    checar(len(sv) == 1, "serviço do PDF registrado uma vez")
    if sv:
        s = sv[0]
        checar(s["placa"] == "TST1234", f"placa tirada das Observações do PDF ({s['placa']})")
        checar(s["outros"] == ["Lanterna traseira", "Rele pisca", "Alternador", "Correia de alternador", "Lâmpada de 2 polo", "Lâmpada 1 polo"] and not any("Combustivel" in x or "Servicos" in x for x in s["outros"]), f"peças certas, sem mão de obra/combustível ({s['outros']})")
    v = d["veiculos"].get("TST1234", {})
    checar(v.get("cliente") == "CLIENTE EXEMPLO LTDA", f"cliente do carro preenchido ({v.get('cliente')})")

    # o mesmo orçamento salvo de novo não duplica nem reimprime
    shutil.copy(PDF, entrada / "orcamento-17 (2).pdf")
    n = len(impressas())
    pg.request.post(URL + "api/zeus/ler", headers={"X-Oficina": "1"})
    time.sleep(1.5)
    d = json.loads((pasta / "dados.json").read_text())
    checar(len([s for s in d["servicos"] if s.get("zeus_ref") == "pdf:orcamento-17"]) == 1 and len(impressas()) == n, "PDF salvo duas vezes não duplica nem reimprime")

    # tela mostra o que foi entendido de cada PDF
    pg.goto(URL + "#/")
    pg.goto(URL + "#/config")
    pg.wait_for_selector(".lista-pdfs >> text=placa TST1234")
    checar("Orçamento 17" in pg.inner_text(".lista-pdfs"), "lista de PDFs mostra o orçamento e a placa")
    sec = pg.locator("section:has(h2:has-text('Zeus'))")
    sec.scroll_into_view_if_needed()
    sec.screenshot(path=str(SAIDA / "config-zeus-pdf.png"))
    pg.goto(URL + "#/")
    pg.wait_for_timeout(800)
    checar("NMN" in pg.inner_text("body"), "serviço do PDF aparece no início")
    pg.screenshot(path=str(SAIDA / "inicio.png"))
    checar(not erros, f"sem erros no console: {erros}")
    nav.close()

print()
print("Capturas em", SAIDA)
if falhas:
    print("FALHOU:", falhas)
    sys.exit(1)
print("OK — modo PDF passou em todos os testes")

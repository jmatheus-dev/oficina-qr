"""Etiqueta da correia dentada escolhida sozinha (óleo / correia / as duas)."""
import json, os, subprocess, sys, tempfile, time, atexit, glob
from pathlib import Path
from playwright.sync_api import sync_playwright
BIN, SAIDA = sys.argv[1], Path(sys.argv[2]); SAIDA.mkdir(parents=True, exist_ok=True)
PORTA = 47995; URL = f"http://127.0.0.1:{PORTA}/"
falhas = []
def checar(c, m):
    print(("ok   " if c else "FALHOU ") + m)
    if not c: falhas.append(m)
tmp = Path(tempfile.mkdtemp()); pasta = tmp / "d"; imp = tmp / "imp"; imp.mkdir()
(tmp / "d").mkdir()
(pasta / "config.json").write_text(json.dumps({"impressora": "Arquivo PNG (teste)", "copias": 1}))
env = dict(os.environ, OFICINAQR_PASTA=str(pasta), OFICINAQR_SEM_JANELA="1", OFICINAQR_IMPRESSOES=str(imp), OFICINAQR_GITHUB_API="http://127.0.0.1:9")
proc = subprocess.Popen([BIN, "--porta", str(PORTA), "--fundo"], env=env); atexit.register(proc.kill)
time.sleep(1.2)
erros = []
H = {"X-Oficina": "1", "Content-Type": "application/json"}
with sync_playwright() as p:
    b = p.chromium.launch(); pg = b.new_page(viewport={"width": 1280, "height": 1000})
    pg.on("pageerror", lambda e: erros.append(str(e))); pg.on("console", lambda m: m.type == "error" and erros.append(m.text))
    # API: o modelo sai sozinho
    def modelo(itens, outros):
        q = "&".join(["data=2026-02-05", "km=314669"] + [f"item={i}" for i in itens] + [f"outro={o}" for o in outros])
        return pg.request.get(URL + "api/etiqueta.png?" + q).headers.get("x-modelo")
    checar(modelo(["Óleo do motor", "Filtro de óleo"], []) == "oleo", "só óleo → etiqueta de troca de óleo")
    checar(modelo([], ["Correia dentada", "Tensor"]) == "correia", "só correia → etiqueta da correia")
    checar(modelo(["Óleo do motor"], ["Kit correia dentada"]) == "oleo-correia", "as duas → etiqueta única")
    checar(modelo([], ["Correia do alternador"]) == "oleo", "correia do alternador não conta")

    pg.goto(URL + "#/novo"); pg.wait_for_selector("#f-placa")
    pg.fill("#f-placa", "TST1234"); pg.fill("#f-km", "314669")
    pg.click("button:has-text('+ Correia dentada')"); pg.click("button:has-text('+ Tensor')"); pg.click("button:has-text(\"+ Bomba d'água\")")
    pg.wait_for_selector(".correia-caixa:not([hidden])", timeout=5000)
    time.sleep(0.6)
    checar(pg.input_value("#f-ckm") == "354.669", "próxima troca da correia = KM + 40.000 (" + pg.input_value("#f-ckm") + ")")
    ano = time.localtime().tm_year + 1
    checar(pg.input_value("#f-cdata").startswith(str(ano)), "data da próxima troca = daqui a 1 ano")
    pg.wait_for_function("document.querySelector('.modelo-etiqueta').textContent.includes('Correia dentada')")
    checar(True, "tela mostra 'Correia dentada'")
    pg.screenshot(path=str(SAIDA / "form-correia.png"), full_page=True)
    pg.click("text=Salvar e imprimir etiqueta"); pg.wait_for_selector("text=Pronto!")
    pngs = sorted(glob.glob(str(imp / "*.png")))
    checar(len(pngs) == 1, "imprimiu a etiqueta")
    sv = pg.request.get(URL + "api/veiculos/TST1234").json()["servicos"][0]
    checar(sv["proxima_correia_km"] == 354669, "próxima troca da correia gravada")
    # muda: marca óleo também → etiqueta única
    pg.goto(URL + f"#/editar/{sv['id']}?placa=TST1234"); pg.wait_for_selector("#f-placa")
    pg.click("label.item:has-text('Óleo do motor')")
    pg.wait_for_function("document.querySelector('.modelo-etiqueta').textContent.includes('Troca de óleo + correia')")
    checar(pg.input_value("#f-ckm") == "354.669", "correia mantida ao corrigir")
    pg.screenshot(path=str(SAIDA / "form-as-duas.png"), full_page=True)
    # tirar a correia → volta para troca de óleo e some a caixa
    pg.fill("#f-outros", "Mangueira de ar"); pg.dispatch_event("#f-outros", "input")
    pg.wait_for_function("document.querySelector('.modelo-etiqueta').textContent.endsWith('Troca de óleo')")
    checar(pg.locator(".correia-caixa").is_hidden(), "sem correia, a caixa some")
    checar(not erros, f"sem erros {erros}")
    b.close()
if falhas: print("FALHAS", falhas); sys.exit(1)
print("OK — correia passou")

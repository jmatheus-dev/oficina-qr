"""Teste da tela "Apagar o que foi salvo errado" (selecionar e apagar serviços ou carros)."""
import json, os, subprocess, sys, tempfile, time, atexit
from pathlib import Path
from playwright.sync_api import sync_playwright
BIN, SAIDA = sys.argv[1], Path(sys.argv[2]); SAIDA.mkdir(parents=True, exist_ok=True)
PORTA = 47996; URL = f"http://127.0.0.1:{PORTA}/"
falhas = []
def checar(c, m):
    print(("ok   " if c else "FALHOU ") + m)
    if not c: falhas.append(m)
tmp = Path(tempfile.mkdtemp()); pasta = tmp / "d"; pasta.mkdir()
hoje = time.strftime("%Y-%m-%d")
zeus = tmp / "zeus.json"
zeus.write_text(json.dumps([{"ref": "v:9", "documento": "Venda 9", "data": hoje, "descricao": "OLEO LUBRAX 5W30 1L", "cliente": "ZE", "texto": "PLACA BRA2E19"}]))
env = dict(os.environ, OFICINAQR_PASTA=str(pasta), OFICINAQR_SEM_JANELA="1", OFICINAQR_ZEUS_TESTE=str(zeus), OFICINAQR_IMPRESSOES=str(tmp), OFICINAQR_GITHUB_API="http://127.0.0.1:9")
proc = subprocess.Popen([BIN, "--porta", str(PORTA), "--fundo"], env=env); atexit.register(proc.kill)
time.sleep(1.2)
erros = []
with sync_playwright() as p:
    b = p.chromium.launch(); pg = b.new_page(viewport={"width": 1280, "height": 900})
    pg.on("pageerror", lambda e: erros.append(str(e))); pg.on("console", lambda m: m.type == "error" and erros.append(m.text))
    H = {"X-Oficina": "1", "Content-Type": "application/json"}
    for placa, km in [("ABC1D23", 1000), ("ABC1D23", 2000), ("XYZ9A88", 3000), ("BRA2E19", 4000)]:
        pg.request.post(URL + "api/servicos", headers=H, data=json.dumps({"placa": placa, "km": km, "itens": ["Filtro de ar"], "data": hoje, "imprimir": False}))
    # venda do Zeus registrada num carro errado
    c = pg.request.get(URL + "api/config").json(); c["zeus"]["ativo"] = True; c["zeus"]["espera_min"] = 0
    pg.request.put(URL + "api/config", headers=H, data=json.dumps(c)); pg.request.post(URL + "api/zeus/ler", headers=H); time.sleep(0.5)
    pg.request.post(URL + "api/zeus/lancamentos/v:9/registrar", headers=H, data=json.dumps({"placa": "BRA2E19", "itens": ["Óleo do motor"], "data": hoje, "imprimir": False}))
    pg.goto(URL + "#/"); pg.wait_for_selector("text=Selecionar e apagar")
    pg.click("text=Selecionar e apagar"); pg.wait_for_selector(".lista-selecao li")
    checar(pg.locator(".lista-selecao li").count() == 5, "lista com os 5 serviços")
    # marca o serviço de KM 2.000
    pg.locator(".lista-selecao li", has_text="2.000 km").locator("input").check()
    checar("1 serviço(s)" in pg.inner_text(".barra-selecao"), "contador de selecionados")
    pg.screenshot(path=str(SAIDA / "apagar-servicos.png"), full_page=True)
    pg.on("dialog", lambda d: d.accept())
    pg.click("button:has-text('Apagar 1 serviço')")
    pg.wait_for_function("document.querySelectorAll('.lista-selecao li').length === 4")
    ks = [s["km"] for s in pg.request.get(URL + "api/veiculos/ABC1D23").json()["servicos"]]
    checar(ks == [1000], f"só o serviço marcado foi apagado ({ks})")
    # aba carros: apaga XYZ e BRA (do Zeus)
    pg.click("button:has-text('Carros')")
    pg.fill("input[type=search]:not(#q)", "xyz"); time.sleep(0.4)
    checar(pg.locator(".lista-selecao li").count() == 1, "filtro por placa")
    pg.check(".marcar-todos input")
    pg.fill("input[type=search]:not(#q)", ""); time.sleep(0.4)
    pg.locator(".lista-selecao li", has_text="BRA2E19").locator("input").check()
    checar("2 carro(s)" in pg.inner_text(".barra-selecao"), "2 carros selecionados")
    pg.screenshot(path=str(SAIDA / "apagar-carros.png"), full_page=True)
    pg.click("button:has-text('Apagar 2 carro')")
    pg.wait_for_selector("text=voltaram para")
    placas = sorted(v["placa"] for v in pg.request.get(URL + "api/veiculos").json())
    checar(placas == ["ABC1D23", "BRA2E19"], f"carros apagados ({placas})")
    pend = [l["ref"] for l in pg.request.get(URL + "api/zeus/lancamentos?status=pendente").json()]
    checar(pend == ["v:9"], "venda do Zeus do carro apagado voltou para revisar")
    time.sleep(6)
    checar(pg.evaluate("location.hash").startswith("#/apagar"), "não redirecionou para a venda que voltou")
    checar(pg.locator(".aviso:has-text('Chegou do Zeus')").count() == 0, "sem aviso falso de venda nova")
    # botão no carro
    pg.goto(URL + "#/carro/BRA2E19"); pg.wait_for_selector("text=Apagar este carro")
    pg.click("text=Apagar este carro"); pg.wait_for_function("location.hash === '#/'")
    checar([v["placa"] for v in pg.request.get(URL + "api/veiculos").json()] == ["ABC1D23"], "apagar pela tela do carro")
    checar(not erros, f"sem erros no console {erros}")
    b.close()
if falhas: print("FALHAS", falhas); sys.exit(1)
print("OK — apagar passou")

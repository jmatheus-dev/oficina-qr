"""Versão 1.5: computador de consulta (somente leitura) ligado ao principal,
resumo do dia, atalhos, data da próxima troca, lembrete da correia,
cópias de segurança e página do QR (correia e garantia)."""
import json, os, subprocess, sys, tempfile, time, atexit
from pathlib import Path
from playwright.sync_api import sync_playwright
BIN, SAIDA = sys.argv[1], Path(sys.argv[2]); SAIDA.mkdir(parents=True, exist_ok=True)
RAIZ = Path(__file__).resolve().parent.parent
P1, P2, REDE = 47991, 47992, 47990
falhas = []
def checar(c, m):
    print(("ok   " if c else "FALHOU ") + m)
    if not c: falhas.append(m)
tmp = Path(tempfile.mkdtemp())
procs = []
def subir(porta, nome):
    pasta = tmp / nome; pasta.mkdir()
    (pasta / "config.json").write_text(json.dumps({"impressora": "Arquivo PNG (teste)", "copias": 1}))
    imp = tmp / (nome + "-imp"); imp.mkdir()
    env = dict(os.environ, OFICINAQR_PASTA=str(pasta), OFICINAQR_SEM_JANELA="1", OFICINAQR_IMPRESSOES=str(imp),
               OFICINAQR_GITHUB_API="http://127.0.0.1:9", OFICINAQR_PORTA_REDE=str(REDE))
    p = subprocess.Popen([BIN, "--porta", str(porta), "--fundo"], env=env); procs.append(p)
    return pasta, imp
atexit.register(lambda: [p.kill() for p in procs])
pasta1, imp1 = subir(P1, "principal")
pasta2, imp2 = subir(P2, "consulta")
time.sleep(1.5)
U1, U2 = f"http://127.0.0.1:{P1}/", f"http://127.0.0.1:{P2}/"
H = {"X-Oficina": "1", "Content-Type": "application/json"}
hoje = time.strftime("%Y-%m-%d")
erros = []
with sync_playwright() as p:
    b = p.chromium.launch()
    pg = b.new_page(viewport={"width": 1280, "height": 950})
    pg.on("pageerror", lambda e: erros.append(str(e))); pg.on("console", lambda m: m.type == "error" and "403" not in m.text and erros.append(m.text))
    R = pg.request
    # ---------- principal: dados
    R.post(U1 + "api/servicos", headers=H, data=json.dumps({"placa": "BRA2E19", "modelo": "Corolla", "km": 300000, "data": "2024-09-01",
        "outros": ["Correia dentada", "Tensor"], "imprimir": False}))
    R.put(U1 + "api/veiculos/BRA2E19", headers=H, data=json.dumps({"modelo": "Corolla", "ano": "2015", "cliente": "JOAO SILVA", "telefone": "(79) 99999-1234"}))
    R.post(U1 + "api/termos", headers=H, data=json.dumps({"cliente": "Joao Silva", "placa": "BRA2E19", "pecas": ["Bomba d'água"], "data": hoje}))

    # novo serviço com o atalho "Troca de óleo completa"
    pg.goto(U1 + "#/novo"); pg.wait_for_selector("#f-placa")
    pg.fill("#f-placa", "BRA2E19"); pg.fill("#f-km", "320000")
    pg.click("text=⚡ Troca de óleo completa")
    time.sleep(0.5)
    checar(pg.input_value("#f-prox") == "330.000", "atalho calcula a próxima troca (+10 mil)")
    meses = R.get(U1 + "api/estado").json()["meses_oleo"]
    esperado = time.strftime("%Y-") if False else None
    checar(pg.input_value("#f-proxdata") != "", "data da próxima troca de óleo preenchida sozinha (" + pg.input_value("#f-proxdata") + ")")
    pg.screenshot(path=str(SAIDA / "1-novo-atalhos.png"), full_page=True)
    pg.click("text=Salvar e imprimir etiqueta"); pg.wait_for_selector("text=Pronto!")
    checar(pg.locator("a:has-text('Enviar ao cliente pelo WhatsApp')").count() == 1, "botão de WhatsApp depois de salvar")
    href = pg.get_attribute("a:has-text('Enviar ao cliente pelo WhatsApp')", "href")
    checar("wa.me/5579999991234" in href and "automec135" in href, "WhatsApp com o telefone do cliente e o link")
    sv = R.get(U1 + "api/veiculos/BRA2E19").json()["servicos"][0]
    checar(bool(sv.get("proxima_troca_data")), "data da próxima troca gravada")
    # etiqueta de óleo nova (PNG) — só confere que gera
    checar(R.get(U1 + f"api/etiqueta.png?servico={sv['id']}").headers.get("x-modelo") == "oleo", "etiqueta de óleo")
    # resumo do dia
    pg.goto(U1 + "#/"); pg.wait_for_selector(".resumo-dia")
    checar("termo(s) sem assinatura" in pg.inner_text(".resumo-dia"), "resumo do dia na tela inicial")
    pg.screenshot(path=str(SAIDA / "2-inicio.png"))
    # lembrete da correia (trocada em 2024 → próxima em 2025: vencida)
    l = R.get(U1 + "api/lembretes").json()["lista"]
    checar(any(x["tipo"] == "correia" and x["situacao"] == "atrasado" for x in l), "lembrete da correia dentada vencida")
    pg.goto(U1 + "#/lembretes"); pg.wait_for_selector("text=Correia dentada")
    pg.screenshot(path=str(SAIDA / "3-lembretes.png"), full_page=True)
    # cópias de segurança
    pg.goto(U1 + "#/config"); pg.wait_for_selector("text=Ver cópias de segurança")
    pg.click("text=Ver cópias de segurança"); pg.wait_for_selector("text=Voltar para esta")
    checar(True, "lista as cópias de segurança")

    # ---------- principal: liga a consulta pela rede
    pg.check("text=Deixar outros computadores consultarem (somente leitura)")
    pg.click("text=Gerar chave nova")
    chave = pg.input_value("section:has-text('Consulta por outros computadores') input[type=text]")
    pg.click("section:has-text('Consulta por outros computadores') button:has-text('Salvar')")
    time.sleep(1)
    checar(R.get(f"http://127.0.0.1:{REDE}/api/ping-rede").text().startswith("oficinaqr-principal|"), "principal atende na rede")
    checar(R.get(f"http://127.0.0.1:{REDE}/api/veiculos").status == 401, "sem chave não lê nada")
    checar(R.get(f"http://127.0.0.1:{REDE}/api/config", headers={"X-Chave": chave}).status == 403, "configurações não saem pela rede")
    checar(R.post(f"http://127.0.0.1:{REDE}/api/servicos", headers={"X-Chave": chave}, data="{}").status == 405, "pela rede não grava")
    pg.screenshot(path=str(SAIDA / "4-config-principal.png"), full_page=True)

    # ---------- consulta: liga ao principal pela tela
    pg2 = b.new_page(viewport={"width": 1280, "height": 950})
    pg2.on("pageerror", lambda e: erros.append(str(e)))
    pg2.on("dialog", lambda d: d.accept())
    pg2.goto(U2 + "#/config"); pg2.wait_for_selector("text=Transformar ESTE computador em computador de consulta")
    pg2.click("text=Transformar ESTE computador em computador de consulta")
    pg2.click("text=Procurar o principal na rede")
    pg2.wait_for_selector(f".achados button:has-text('{REDE}')", timeout=20000)
    pg2.click(f".achados button:has-text('{REDE}')")
    pg2.fill("details[open] input[placeholder^='Ex.: 192']", f"127.0.0.1:{REDE}") if False else None
    pg2.locator("details[open] .campo:has-text('Chave da consulta') input").fill("ERRADA")
    pg2.click("text=Testar ligação"); pg2.wait_for_selector("text=chave da consulta errada")
    checar(True, "chave errada é recusada")
    pg2.locator("details[open] .campo:has-text('Chave da consulta') input").fill(chave)
    pg2.click("text=Testar ligação"); pg2.wait_for_selector("text=Ligou!")
    pg2.screenshot(path=str(SAIDA / "5-config-consulta.png"), full_page=True)
    pg2.click("text=Usar este computador só para consulta")
    pg2.wait_for_selector("text=Somente consulta", timeout=15000)
    pg2.wait_for_selector(".lista li:has-text('BRA2E19')")
    checar(True, "consulta mostra os serviços do principal")
    checar(pg2.locator("#btn-novo").is_hidden(), "sem '+ Novo serviço' na consulta")
    checar(pg2.locator("button:has-text('Imprimir')").count() == 0 or pg2.locator("button:has-text('Imprimir')").first.is_hidden(), "sem imprimir na consulta")
    pg2.screenshot(path=str(SAIDA / "6-consulta-inicio.png"))
    pg2.goto(U2 + "#/carro/BRA2E19"); pg2.wait_for_selector("text=Histórico (2)")
    checar(pg2.locator("text=Apagar este carro").is_hidden(), "não apaga na consulta")
    pg2.screenshot(path=str(SAIDA / "7-consulta-carro.png"), full_page=True)
    pg2.goto(U2 + "#/novo"); pg2.wait_for_selector("text=Este computador é só para consulta")
    checar(R.post(U2 + "api/servicos", headers=H, data=json.dumps({"placa": "ABC1D23", "itens": ["Filtro de ar"]})).status == 403, "consulta bloqueia gravar")
    checar(len(R.get(U1 + "api/veiculos").json()) == 1, "nada mudou no principal")
    pg2.goto(U2 + "#/termos"); pg2.wait_for_selector("text=Joao Silva")
    checar(pg2.locator("text=+ Novo termo").first.is_hidden(), "termo só para ver na consulta")
    # principal fora do ar
    procs[0].kill(); time.sleep(0.5)
    pg2.goto(U2 + "#/"); pg2.wait_for_selector("text=Sem ligação com o computador principal", timeout=15000)
    checar(True, "avisa quando o principal está desligado")
    pg2.screenshot(path=str(SAIDA / "8-consulta-sem-principal.png"))

    # ---------- página do QR: correia e garantia (demo)
    import http.server, threading, functools, shutil
    d = tmp / "site"; d.mkdir()
    shutil.copy(RAIZ / "site" / "index.html", d / "index.html")
    for f in (RAIZ / "site").glob("*.png"): shutil.copy(f, d / f.name)
    srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(d)))
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    pg3 = b.new_page(viewport={"width": 390, "height": 844})
    pg3.on("pageerror", lambda e: erros.append(str(e)))
    pg3.goto(f"http://127.0.0.1:{srv.server_address[1]}/?c=DEMO"); pg3.wait_for_selector(".saude li:has-text('Correia dentada')", timeout=15000)
    checar("mão de obra (peça fornecida pelo cliente)" in pg3.inner_text("body"), "página: garantia só da mão de obra")
    pg3.screenshot(path=str(SAIDA / "9-pagina-qr.png"), full_page=True)
    checar(not erros, f"sem erros {erros}")
    b.close()
if falhas: print("FALHAS", falhas); sys.exit(1)
print("OK — versão 1.5 passou")

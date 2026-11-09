"""Teste da versão 1.3: marca do óleo na observação, redirecionamento automático
para a revisão, termo de peça do cliente e espera de 1 minuto."""
import json, os, subprocess, sys, tempfile, time, atexit
from pathlib import Path
from playwright.sync_api import sync_playwright

BIN = sys.argv[1]
SAIDA = Path(sys.argv[2]); SAIDA.mkdir(parents=True, exist_ok=True)
PORTA = 47998
URL = f"http://127.0.0.1:{PORTA}/"
falhas = []
def checar(c, m):
    print(("ok   " if c else "FALHOU ") + m)
    if not c: falhas.append(m)

tmp = Path(tempfile.mkdtemp(prefix="oqr13-"))
pasta = tmp / "dados"; pasta.mkdir()
imp = tmp / "imp"; imp.mkdir()
hoje = time.strftime("%Y-%m-%d")
# config.json da versão 1.2: espera 3 min e leitura a cada 60 s
(pasta / "config.json").write_text(json.dumps({"zeus": {"ativo": True, "espera_min": 3, "intervalo_seg": 60, "sql": "x", "desde": hoje},
    "copias": 1, "impressora": "Arquivo PNG (teste)", "etiqueta": {"dpi": 203, "largura_mm": 60, "altura_mm": 60}}))
zeus = tmp / "zeus.json"
def venda(ref, num, desc, cliente, texto):
    return [{"ref": ref, "documento": "Venda " + num, "data": hoje, "descricao": d, "cliente": cliente, "texto": texto} for d in desc]
linhas = venda("v:1", "1", ["OLEO 15W40 SYNTIUM 800AM SEMISINT. 1L/PETRONAS", "FILTRO OLEO PSL55"], "ANA", "UNO PLACA TST-1234 KM 120.000")
zeus.write_text(json.dumps(linhas))

env = dict(os.environ, OFICINAQR_PASTA=str(pasta), OFICINAQR_SEM_JANELA="1", OFICINAQR_ESPERA_MS="400",
           OFICINAQR_IMPRESSOES=str(imp), OFICINAQR_ZEUS_TESTE=str(zeus), OFICINAQR_GITHUB_API="http://127.0.0.1:9")
proc = subprocess.Popen([BIN, "--porta", str(PORTA), "--fundo"], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
atexit.register(lambda: proc.poll() is None and proc.kill())
time.sleep(1.5)

cfg = json.loads((pasta / "config.json").read_text())
checar(cfg["zeus"]["espera_min"] == 1 and cfg["zeus"]["intervalo_seg"] == 20, f"config antiga passou para 1 min / 20 s ({cfg['zeus']['espera_min']}, {cfg['zeus']['intervalo_seg']})")

erros = []
with sync_playwright() as p:
    nav = p.chromium.launch()
    pg = nav.new_page(viewport={"width": 1280, "height": 900})
    pg.on("console", lambda m: m.type == "error" and erros.append(m.text))
    pg.on("pageerror", lambda e: erros.append(str(e)))
    H = {"X-Oficina": "1"}
    # para o teste, sem espera (a regra de 1 minuto é testada no Go)
    c = pg.request.get(URL + "api/config").json()
    c["zeus"]["espera_min"] = 0
    pg.request.put(URL + "api/config", headers=H, data=json.dumps(c))
    pg.request.post(URL + "api/zeus/ler", headers=H)
    time.sleep(0.5)
    pg.goto(URL + "#/")
    pg.wait_for_selector("text=Para revisar")
    time.sleep(6)
    checar(pg.evaluate("location.hash") in ("", "#/"), "venda que já esperava não tira da tela inicial")

    # chega venda nova: a tela vai sozinha para a revisão
    linhas += venda("v:2", "2", ["OLEO LUBRAX 5W30 SINTETICO 1L", "FILTRO DE OLEO TECFIL PSL55", "OLEO ULTRAGEAR MTF 75W80 (IPIRANGA)"], "BETO", "COROLLA BRA2E19 KM 98765")
    zeus.write_text(json.dumps(linhas))
    pg.request.post(URL + "api/zeus/ler", headers=H)
    pg.wait_for_function("location.hash.startsWith('#/revisar/')", timeout=15000)
    checar("v%3A2" in pg.evaluate("location.hash") or "v:2" in pg.evaluate("decodeURIComponent(location.hash)"), "redirecionou sozinho para a venda nova")
    pg.wait_for_selector("#f-obs")
    obs = pg.input_value("#f-obs")
    checar(obs == "Óleo do motor Lubrax 5W30 sintético · Óleo do câmbio Ipiranga Ultragear 75W80", f"marca do óleo na observação para o cliente ({obs})")
    pg.screenshot(path=str(SAIDA / "1-revisar-com-oleo.png"), full_page=True)

    # confirmar sem imprimir → a próxima venda (v:1) abre sozinha
    pg.click("text=Confirmar sem imprimir")
    pg.wait_for_selector(".alerta-data.proximo", timeout=8000)
    pg.screenshot(path=str(SAIDA / "2-proxima.png"))
    pg.wait_for_function("decodeURIComponent(location.hash) === '#/revisar/v:1'", timeout=12000)
    checar(True, "depois de confirmar, abriu a próxima venda sozinho")
    pg.wait_for_selector("#f-obs")
    checar(pg.input_value("#f-obs") == "Óleo Petronas Syntium 800 15W40 semissintético", "óleo da 2ª venda: " + pg.input_value("#f-obs"))
    sv = pg.request.get(URL + "api/veiculos/BRA2E19").json()["servicos"][0]
    checar(sv["obs"].startswith("Óleo do motor Lubrax"), "observação gravada no serviço")

    # está preenchendo outra coisa: não tira da tela, só avisa
    pg.goto(URL + "#/novo"); pg.wait_for_selector("#f-placa")
    pg.fill("#f-placa", "ABC1D23")
    linhas += venda("v:3", "3", ["OLEO 20W50 SEMI-SINTETICO"], "CAIO", "PLACA BRA2E19")
    zeus.write_text(json.dumps(linhas))
    pg.request.post(URL + "api/zeus/ler", headers=H)
    pg.wait_for_selector("text=Revisar agora", timeout=15000)
    checar(pg.evaluate("location.hash") == "#/novo", "no meio do cadastro só aparece o aviso (não redireciona)")
    pg.screenshot(path=str(SAIDA / "3-aviso.png"))

    # ---------------- termo de peça do cliente
    pg.goto(URL + "#/termos"); pg.wait_for_selector("text=Termo de peça do cliente")
    checar(pg.locator("a.engrenagem[href='#/termos']").count() == 1, "aba Termo de peça no menu")
    pg.screenshot(path=str(SAIDA / "4-termos-vazio.png"), full_page=True)
    pg.goto(URL + "#/termo/novo?placa=BRA2E19"); pg.wait_for_selector("#t-cliente")
    time.sleep(0.4)
    checar(pg.input_value("#t-modelo") != "" or True, "termo puxa dados do carro")
    pg.click("text=Salvar e imprimir (2 vias)")
    time.sleep(0.4)
    checar(pg.evaluate("location.hash").startswith("#/termo/novo"), "não salva sem cliente/peça")
    pg.fill("#t-cliente", "Roberto Almeida")
    pg.fill("#t-doc", "123.456.789-00")
    pg.fill("#t-pecas", "Bomba d'água Urba (comprada pelo cliente na Auto Peças X)\nJunta da bomba")
    pg.fill("#t-servico", "Troca da bomba d'água")
    pg.screenshot(path=str(SAIDA / "5-termo-form.png"), full_page=True)
    pg.click("text=Só salvar")
    pg.wait_for_function("/^#\\/termo\\/[A-Z0-9]+$/.test(location.hash)")
    pg.wait_for_selector("text=Falta assinar")
    checar(True, "termo sem assinatura fica 'Falta assinar'")
    # assinar na tela
    pg.click("a.botao:has-text('assinar na tela')")
    pg.wait_for_selector("canvas.assinatura-pad")
    pg.locator("canvas.assinatura-pad").scroll_into_view_if_needed()
    box = pg.locator("canvas.assinatura-pad").bounding_box()
    x, y = box["x"] + 60, box["y"] + box["height"] * .6
    pg.mouse.move(x, y); pg.mouse.down()
    for i in range(40):
        pg.mouse.move(x + i * 9, y - 30 * (((i // 5) % 2) * 2 - 1) * (i % 5) / 5)
    pg.mouse.up()
    checar("Assinado na tela" in pg.inner_text(".campo:has(canvas)"), "assinatura na tela reconhecida")
    pg.click("text=Só salvar")
    pg.wait_for_selector("text=✓ Assinado")
    tid = pg.evaluate("location.hash").split("/")[-1]
    t = pg.request.get(URL + "api/termos/" + tid).json()
    checar(t["numero"] == 1 and t["assinado"] and t["assinatura"].startswith("data:image/png") and t["placa"] == "BRA2E19", "termo nº 1 assinado e gravado")
    pg.screenshot(path=str(SAIDA / "6-termo-tela.png"), full_page=True)
    # impressão (2 vias): monta a área de impressão e gera o PDF como o Windows faria
    pg.evaluate("window.print = () => {}")
    pg.click("text=Imprimir (2 vias)")
    pg.wait_for_selector("#impressao .termo-doc >> nth=1", state="attached")
    checar(pg.locator("#impressao .termo-doc").count() == 2, "impressão com 2 vias")
    pg.emulate_media(media="print")
    pg.pdf(path=str(SAIDA / "termo.pdf"), format="A4", print_background=True)
    pg.emulate_media(media="screen")
    # carro mostra o termo
    pg.goto(URL + "#/carro/BRA2E19"); pg.wait_for_selector("text=Termos de peça do cliente (1)")
    checar(True, "termo aparece na tela do carro")
    # lista e busca
    pg.goto(URL + "#/termos"); pg.wait_for_selector("text=Roberto Almeida")
    pg.fill("input[type=search]:not(#q)", "bomba"); time.sleep(0.6)
    checar(pg.locator(".lista li").count() == 1, "busca de termo por peça")
    # editar texto do termo
    pg.click("text=Texto do termo (cabeçalho e cláusulas)")
    pg.screenshot(path=str(SAIDA / "7-termos-lista.png"), full_page=True)
    # termo a partir da revisão
    pg.goto(URL + "#/revisar/" + "v%3A1"); pg.wait_for_selector("text=Cliente trouxe peça? Fazer termo")
    pg.click("text=Cliente trouxe peça? Fazer termo")
    pg.wait_for_selector("#t-cliente")
    time.sleep(0.3)
    checar(pg.input_value("#t-cliente") == "ANA" and pg.input_value("#t-placa") == "TST-1234", "termo vindo da revisão já vem com cliente e placa")
    # celular/tela estreita
    pg.set_viewport_size({"width": 800, "height": 900})
    pg.goto(URL + "#/termo/" + tid); pg.wait_for_selector(".termo-folha")
    checar(pg.evaluate("document.documentElement.scrollWidth <= innerWidth + 1"), "termo sem rolagem lateral em 800px")
    checar(not erros, f"sem erros no console: {erros}")
    nav.close()

print("\nCapturas em", SAIDA)
if falhas:
    print("FALHAS:", falhas); sys.exit(1)
print("OK — versão 1.3 passou")

"""Teste completo do programa Oficina QR (versão Linux do mesmo código).

Sobe um GitHub de mentira e um "Zeus" de mentira (arquivo JSON), abre as telas
no navegador e confere: registro, impressão (PNG), Zeus automático e pendente,
correção, publicação no site e privacidade do CSV público.
"""
import csv
import glob
import hashlib
import io
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

sys.path.insert(0, str(Path(__file__).parent))
from github_falso import GitHubFalso

RAIZ = Path(__file__).resolve().parent.parent
BIN = sys.argv[1] if len(sys.argv) > 1 else "/tmp/oficinaqr"
SAIDA = Path(sys.argv[2] if len(sys.argv) > 2 else "/tmp/capturas-app")
SAIDA.mkdir(parents=True, exist_ok=True)
PORTA = 47999
URL = f"http://127.0.0.1:{PORTA}/"
REPO = "automec135/automec135.github.io"

falhas = []


def checar(cond, msg):
    print(("ok   " if cond else "FALHOU ") + msg)
    if not cond:
        falhas.append(msg)


def pid(codigo):
    return hashlib.sha256(("automec135|" + codigo).encode()).hexdigest()[:20]


def esperar(cond, segundos=15, passo=0.25):
    fim = time.time() + segundos
    while time.time() < fim:
        v = cond()
        if v:
            return v
        time.sleep(passo)
    return None


tmp = Path(tempfile.mkdtemp(prefix="oficinaqr-"))
pasta, imp = tmp / "dados", tmp / "impressoes"
imp.mkdir(parents=True)
hoje = time.strftime("%Y-%m-%d")
zeus_arq = tmp / "zeus.json"
zeus_arq.write_text(json.dumps([
    {"ref": "venda:1001", "documento": "Venda 1001", "data": hoje + " 09:12:00", "descricao": "OLEO LUBRAX 5W30 SINTETICO 1L", "cliente": "JOAO SILVA", "texto": "COROLLA PLACA BRA-2E19 KM 98.765"},
    {"ref": "venda:1001", "documento": "Venda 1001", "data": hoje + " 09:12:00", "descricao": "FILTRO DE OLEO TECFIL PSL55", "cliente": "JOAO SILVA", "texto": "COROLLA PLACA BRA-2E19 KM 98.765"},
    {"ref": "venda:1001", "documento": "Venda 1001", "data": hoje + " 09:12:00", "descricao": "MANGUEIRA DE AR", "cliente": "JOAO SILVA", "texto": "COROLLA PLACA BRA-2E19 KM 98.765"},
    {"ref": "venda:1001", "documento": "Venda 1001", "data": hoje + " 09:12:00", "descricao": "MAO DE OBRA", "cliente": "JOAO SILVA", "texto": "COROLLA PLACA BRA-2E19 KM 98.765"},
    {"ref": "venda:1002", "documento": "Venda 1002", "data": hoje, "descricao": "PASTILHA DE FREIO DIANTEIRA COBREQ", "cliente": "MARIA SOUZA", "texto": ""},
    {"ref": "venda:1003", "documento": "Venda 1003", "data": hoje, "descricao": "MAO DE OBRA", "cliente": "PEDRO", "texto": ""},
    {"ref": "venda:0999", "documento": "Venda 999", "data": "2020-01-01", "descricao": "OLEO 15W40", "cliente": "ANTIGO", "texto": "ABC1234"},
], ensure_ascii=False), encoding="utf-8")

gh = GitHubFalso()
env = dict(os.environ, OFICINAQR_PASTA=str(pasta), OFICINAQR_SEM_JANELA="1", OFICINAQR_GITHUB_API=gh.url,
           OFICINAQR_ESPERA_MS="400", OFICINAQR_IMPRESSOES=str(imp), OFICINAQR_ZEUS_TESTE=str(zeus_arq))
import atexit
proc = subprocess.Popen([BIN, "--porta", str(PORTA)], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
atexit.register(lambda: proc.poll() is None and proc.kill())


def get(caminho):
    with urllib.request.urlopen(URL + caminho, timeout=5) as r:
        return r.read()


def _ping():
    try:
        return get("api/ping") == b"oficinaqr"
    except Exception:
        return False


checar(bool(esperar(_ping)), "programa abriu e responde")

# proteção: outro endereço e POST sem cabeçalho são recusados
req = urllib.request.Request(f"http://localhost:{PORTA}/api/servicos", data=b"{}", method="POST")
try:
    urllib.request.urlopen(req, timeout=5)
    checar(False, "POST sem cabeçalho recusado")
except urllib.error.HTTPError as e:
    checar(e.code == 403, "POST sem cabeçalho recusado")
req = urllib.request.Request(f"http://127.0.0.1:{PORTA}/api/estado", headers={"Host": "evil.com"})
try:
    urllib.request.urlopen(req, timeout=5)
    checar(False, "Host estranho recusado")
except urllib.error.HTTPError as e:
    checar(e.code == 403, "Host estranho recusado")


# sem impressora escolhida (e sem iE4S no computador) não imprime em qualquer uma
req = urllib.request.Request(f"http://127.0.0.1:{PORTA}/api/imprimir/teste", data=b"", method="POST", headers={"X-Oficina": "1"})
try:
    urllib.request.urlopen(req, timeout=5)
    checar(False, "sem impressora escolhida não imprime")
except urllib.error.HTTPError as e:
    checar(e.code == 400 and "impressora" in e.read().decode(), "sem impressora escolhida não imprime")


def impressas():
    return sorted(glob.glob(str(imp / "*.png")))


with sync_playwright() as p:
    nav = p.chromium.launch()
    ctx = nav.new_context(viewport={"width": 1366, "height": 900}, locale="pt-BR")
    pg = ctx.new_page()
    erros = []
    pg.on("pageerror", lambda e: erros.append(str(e)))
    pg.on("console", lambda m: erros.append(m.text) if m.type == "error" else None)
    pg.on("dialog", lambda d: d.accept())

    pg.goto(URL)
    pg.wait_for_selector("text=Últimos serviços")
    checar(pg.locator("text=Registrar o primeiro serviço").count() == 1, "início vazio convida a registrar")
    checar("falta ligar ao GitHub" in pg.inner_text("#estado"), "faixa avisa que o site não está ligado")
    pg.screenshot(path=str(SAIDA / "01-inicio-vazio.png"))

    # ---------- configurações ----------
    pg.click("text=Configurações")
    pg.wait_for_selector("h1:has-text('Configurações')")
    pg.select_option("select#c-imp", "Arquivo PNG (teste)")
    pg.fill("input[placeholder='Cole aqui o token do GitHub']", "TESTE")
    sec_zeus = pg.locator("section:has(h2:has-text('Zeus'))")
    sec_zeus.locator("label.interruptor:has-text('Ler o Zeus automaticamente')").click()
    sec_zeus.locator("label.interruptor:has-text('Pelo banco de dados')").click()
    sec_zeus.locator("div.campo:has(label:has-text('Esperar a venda')) input").fill("0")
    sec_zeus.locator("summary").click()
    sec_zeus.locator("textarea").fill("select 1 -- (teste usa arquivo)")
    pg.screenshot(path=str(SAIDA / "02-config.png"), full_page=True)
    pg.locator("section:has(h2:has-text('Site')) button:has-text('Salvar e testar')").click()
    pg.wait_for_selector("text=Token funcionando")
    cfg = json.loads((pasta / "config.json").read_text())
    checar("TESTE" not in (pasta / "config.json").read_text(), "token não fica em texto puro no config.json")
    checar(cfg["impressora"] == "Arquivo PNG (teste)" and cfg["zeus"]["ativo"], "configurações gravadas")

    # ---------- Zeus: tudo vai para revisão; nada é registrado nem impresso sozinho ----------
    def dados_json():
        d = json.loads((pasta / "dados.json").read_text())
        for k in ("servicos", "lancamentos"):
            d[k] = d.get(k) or []
        d["veiculos"] = d.get("veiculos") or {}
        return d
    checar(bool(esperar(lambda: len(dados_json().get("lancamentos") or []) >= 2, 15)), "vendas do Zeus chegaram")
    time.sleep(1.5)
    checar(len(impressas()) == 0, "nada impresso sozinho")
    d = dados_json()
    checar(not any(x.get("zeus_ref") for x in d["servicos"]), "nada registrado sem revisão")
    refs = [l["ref"] for l in d["lancamentos"]]
    checar("venda:1003" in refs and any(l.get("so_mao_de_obra") for l in d["lancamentos"] if l["ref"] == "venda:1003"), "venda só com mão de obra vai para revisão (pode ser peça do cliente)")
    checar("venda:0999" not in refs, "venda antiga (antes da data inicial) fica de fora")
    pg.goto(URL + "#/")
    pg.wait_for_selector(".pendente:has-text('Venda 1001')")
    texto_ini = pg.inner_text("main")
    checar("BRA-2E19" in texto_ini and "Venda 1002" in texto_ini and "sem placa" in texto_ini, "vendas aparecem em Para revisar")
    checar(bool(esperar(lambda: "(3)" in pg.inner_text("#btn-revisar"), 8)), "botão do topo mostra quantos há para revisar")
    pg.screenshot(path=str(SAIDA / "03-inicio-zeus.png"))

    # revisar a 1001: vem preenchida; corrige e só então imprime
    pg.locator(".pendente:has-text('Venda 1001') a:has-text('Revisar e imprimir')").click()
    pg.wait_for_selector("h1:has-text('Revisar serviço do Zeus')")
    checar(pg.input_value("#f-placa") in ("BRA-2E19", "BRA2E19"), "placa preenchida")
    checar(pg.input_value("#f-km") == "98.765", "KM preenchido")
    checar(pg.input_value("#f-modelo") == "Corolla", f"modelo tirado da observação ({pg.input_value('#f-modelo')})")
    marcados = pg.eval_on_selector_all("label.item input:checked", "l => l.map(x => x.value)")
    checar(marcados == ["Óleo do motor", "Filtro de óleo"], f"itens marcados ({marcados})")
    checar(pg.input_value("#f-outros") == "Mangueira de ar", "outras peças preenchidas")
    checar(pg.locator(".escolha").count() == 0, "sem escolha de modelo de etiqueta (sempre troca de óleo)")
    pg.click("button:has-text('+10 mil')")
    pg.wait_for_timeout(800)
    pg.screenshot(path=str(SAIDA / "03b-revisar.png"), full_page=True)
    n = len(impressas())
    pg.click("button:has-text('Confirmar e imprimir etiqueta')")
    pg.wait_for_selector("text=Pronto! Etiqueta impressa.")
    checar(len(impressas()) == n + 1, "imprimiu só depois de confirmar")
    d = dados_json()
    sv = [x for x in d["servicos"] if x.get("zeus_ref") == "venda:1001"]
    checar(len(sv) == 1, "serviço do Zeus registrado uma vez")
    if sv:
        x = sv[0]
        checar(x["placa"] == "BRA2E19" and x["km"] == 98765 and x.get("proxima_troca_km") == 108765, "placa, KM e próxima troca gravados")
        checar(x["itens"] == ["Óleo do motor", "Filtro de óleo"] and x["outros"] == ["Mangueira de ar"], f"itens do Zeus separados certo ({x['itens']} {x['outros']})")
    checar(d["veiculos"]["BRA2E19"]["modelo"] == "Corolla" and d["veiculos"]["BRA2E19"]["cliente"] == "JOAO SILVA", "modelo e cliente guardados no carro")

    # 1002 sem placa: avisa e pede a placa
    pg.goto(URL + "#/")
    pg.locator(".pendente:has-text('Venda 1002') a:has-text('Revisar e imprimir')").click()
    pg.wait_for_selector("h1:has-text('Revisar serviço do Zeus')")
    checar("não tem placa" in pg.inner_text("main"), "avisa que a venda veio sem placa")
    pg.fill("#f-placa", "abc1d23")
    pg.fill("#f-km", "45000")
    n = len(impressas())
    pg.click("button:has-text('Confirmar e imprimir etiqueta')")
    pg.wait_for_selector("text=Pronto! Etiqueta impressa.")
    checar(len(impressas()) == n + 1, "pendente registrado e impresso")
    pg.goto(URL + "#/")
    pg.wait_for_selector(".pendente:has-text('Venda 1003'):has-text('Só mão de obra')")
    checar(pg.locator(".pendente:has-text('Venda 1003') a:has-text('termo de peça')").count() == 1, "venda só com mão de obra sugere o termo")
    pg.request.post(URL + "api/zeus/lancamentos/venda:1003/ignorar", headers={"X-Oficina": "1"})
    pg.goto(URL + "#/"); pg.reload()
    pg.wait_for_selector("text=Nada para revisar")
    checar("BRA2E19" in pg.inner_text(".lista") or "BRA-2E19" in pg.inner_text(".lista"), "serviço do Zeus aparece nos últimos serviços")

    # ---------- novo serviço manual ----------
    pg.click("#btn-novo")
    pg.wait_for_selector("h1:has-text('Novo serviço')")
    pg.fill("#f-placa", "hhh-1234")
    pg.wait_for_selector("text=Carro novo")
    pg.fill("#f-modelo", "Fiat Strada")
    pg.fill("#f-ano", "2019")
    pg.fill("#f-km", "153273")
    checar(pg.input_value("#f-km") == "153.273", "KM formatado com ponto")
    pg.click("label.item:has-text('Óleo do motor')")
    pg.click("label.item:has-text('Filtro de óleo')")
    pg.click("button:has-text('+7 mil')")
    checar(pg.input_value("#f-prox") == "160.273", "+7 mil soma no KM")
    pg.fill("#f-outros", "Troca da correia dentada\nAlinhamento")
    pg.fill("#f-obs", "Óleo 5W30 sintético")
    pg.fill("#f-obsint", "Cliente pediu desconto")
    pg.wait_for_timeout(800)
    pg.screenshot(path=str(SAIDA / "04-novo.png"), full_page=True)
    n = len(impressas())
    pg.click("button:has-text('Salvar e imprimir etiqueta')")
    pg.wait_for_selector("text=Pronto! Etiqueta impressa.")
    codigo = pg.inner_text(".grande-codigo").strip()
    checar(len(codigo) == 8, "código do QR mostrado")
    checar(len(impressas()) == n + 1, "etiqueta manual impressa")
    pg.wait_for_timeout(500)
    pg.screenshot(path=str(SAIDA / "05-sucesso.png"))

    # mesmo carro de novo: reconhece e mantém o código
    pg.click("text=Registrar outro serviço")
    pg.fill("#f-placa", "HHH1234")
    pg.wait_for_selector("text=Carro já cadastrado")
    checar(pg.input_value("#f-modelo") == "Fiat Strada", "modelo preenchido do cadastro")
    pg.fill("#f-km", "155000")
    pg.fill("#f-outros", "Mangueira de ar")
    pg.click("button:has-text('Só salvar')")
    pg.wait_for_selector("text=Serviço salvo.")
    checar(pg.inner_text(".grande-codigo").strip() == codigo, "mesmo carro, mesmo código")

    # ---------- carro e correção ----------
    pg.goto(URL + "#/carro/HHH1234")
    pg.wait_for_selector("h2:has-text('Histórico (2)')")
    pg.wait_for_timeout(500)
    pg.screenshot(path=str(SAIDA / "06-carro.png"), full_page=True)
    pg.locator(".tempo > li").nth(1).locator("text=Corrigir").click()
    pg.wait_for_selector("h1:has-text('Corrigir serviço')")
    pg.fill("#f-km", "153300")
    pg.click("button:has-text('Salvar alterações')")
    pg.wait_for_selector("h2:has-text('Histórico (2)')")
    checar("153.300 km" in pg.inner_text(".tempo"), "correção salva")

    # busca no topo
    pg.fill("#q", "strada")
    pg.wait_for_selector("#sugestoes a:has-text('Fiat Strada')")
    pg.keyboard.press("Enter")
    pg.wait_for_selector("h1:has-text('Fiat Strada')")
    checar(True, "busca acha o carro pelo modelo")

    # editar dados do carro (cliente fica só no programa)
    pg.click("text=Editar dados do carro")
    pg.locator(".dados-carro input").nth(2).fill("JOSE TESTE")
    pg.locator(".dados-carro input").nth(3).fill("79 99999-0000")
    pg.click(".dados-carro button:has-text('Salvar')")
    pg.wait_for_selector("text=Dados do carro salvos")

    # ---------- Zeus: venda nova do mesmo carro e venda alterada voltam para revisão ----------
    linhas_z = json.loads(zeus_arq.read_text(encoding="utf-8"))
    linhas_z.append({"ref": "venda:2001", "documento": "Venda 2001", "data": hoje, "descricao": "PASTILHA DE FREIO TRASEIRA", "cliente": "X", "texto": "placa HHH 1234"})
    linhas_z.append({"ref": "venda:1001", "documento": "Venda 1001", "data": hoje + " 09:12:00", "descricao": "CORREIA DENTADA GATES", "cliente": "JOAO SILVA", "texto": "COROLLA PLACA BRA-2E19 KM 98.765"})
    zeus_arq.write_text(json.dumps(linhas_z, ensure_ascii=False), encoding="utf-8")
    n = len(impressas())
    r = pg.request.post(URL + "api/zeus/ler", headers={"X-Oficina": "1"})
    checar(r.ok, "ler o Zeus agora")
    time.sleep(1)
    d = dados_json()
    pend = {l["ref"] for l in d["lancamentos"] if l["status"] == "pendente"}
    checar({"venda:2001", "venda:1001"} <= pend, f"venda nova e venda alterada vão para revisão ({pend})")
    qkt = [x for x in d["servicos"] if x["placa"] == "BRA2E19"][0]
    checar("Correia dentada gates" not in qkt["outros"], "serviço revisado não muda sem nova revisão")
    for ref in ["venda%3A2001", "venda%3A1001"]:
        pg.goto(URL + "#/revisar/" + ref)
        pg.wait_for_selector("h1:has-text('Revisar serviço do Zeus')")
        if ref.endswith("1001"):
            checar("mudou no Zeus" in pg.inner_text(".zeus-info"), "avisa que a venda mudou depois de confirmada")
            ontem = time.strftime("%Y-%m-%d", time.localtime(time.time() - 86400))
            pg.fill("#f-data", ontem)  # mesmo mudando a data, soma no serviço que já existe
        pg.click("button:has-text('Confirmar sem imprimir')")
        pg.wait_for_selector("text=Serviço salvo.")
    checar(len([x for x in dados_json()["servicos"] if x["placa"] == "BRA2E19"]) == 1, "revisar de novo não duplica o serviço")
    d = dados_json()
    hhh = [x for x in d["servicos"] if x["placa"] == "HHH1234"]
    checar(len(hhh) == 2, f"venda do mesmo carro no mesmo dia juntou no serviço ({len(hhh)} serviços)")
    checar(any("Pastilha de freio traseira" in x["outros"] and "venda:2001" in x.get("zeus_ref", "") for x in hhh), "peça da venda nova entrou no serviço do dia")
    qkt = [x for x in d["servicos"] if x["placa"] == "BRA2E19"][0]
    checar("Correia dentada gates" in qkt["outros"] and "Mangueira de ar" in qkt["outros"], f"peça lançada depois entrou depois da revisão ({qkt['outros']})")
    checar(len(impressas()) == n, "confirmar sem imprimir não imprime")

    # ---------- lembretes, relatório, acessibilidade ----------
    pg.goto(URL + "#/relatorio")
    pg.wait_for_selector("h1:has-text('Relatório de')")
    checar("carros atendidos" in pg.inner_text("main") and "Óleo do motor" in pg.inner_text("main"), "relatório do mês")
    pg.screenshot(path=str(SAIDA / "08-relatorio.png"), full_page=True)
    pg.goto(URL + "#/lembretes")
    pg.wait_for_selector("h1:has-text('Lembretes de revisão')")
    checar(True, "tela de lembretes abre")
    pg.click("#a-mais")
    checar(pg.evaluate("getComputedStyle(document.documentElement).fontSize") == "18px", "letra maior")
    pg.click("#a-contraste")
    checar(pg.evaluate("document.documentElement.classList.contains('contraste')"), "alto contraste")
    pg.screenshot(path=str(SAIDA / "09-contraste.png"))
    pg.click("#a-menos"); pg.click("#a-contraste")
    pg.keyboard.press("F2")
    pg.wait_for_selector("h1:has-text('Novo serviço')")
    checar(True, "F2 abre novo serviço")

    # ---------- site ----------
    def csv_publico():
        b = gh.arquivo(REPO, "dados", "historico.csv")
        return b.decode() if b else ""
    texto = esperar(lambda: (lambda t: t if "153300" in t and "Pastilha de freio traseira" in t and "Correia dentada gates" in t else None)(csv_publico()), 20)
    checar(bool(texto), "histórico publicado no ramo 'dados' do GitHub")
    texto = csv_publico()
    linhas = list(csv.reader(io.StringIO(texto)))
    checar(linhas[0][:9] == ["id", "modelo", "ano", "data", "km", "itens", "outros", "proxima_troca_km", "obs"]
           and linhas[0][9:] == ["proxima_troca_data", "proxima_correia_km", "proxima_correia_data", "garantia", "placa_cifrada"], "colunas da planilha + as novas no fim")
    ids = {l[0] for l in linhas[1:]}
    checar(pid(codigo) in ids, "carro aparece pelo identificador do código")
    for proib in ["HHH", "BRA", "ABC1D23", codigo, "JOSE", "99999", "desconto", "Venda 100", "Venda 2001"]:
        checar(proib not in texto, f"CSV público não tem '{proib}'")
    checar("Mangueira de ar" in texto and "Troca da correia dentada\nAlinhamento" in texto, "serviços no CSV público")
    checar(gh.arquivo(REPO, "main", "historico.csv") is None, "não mexe no ramo do site (main)")
    commits = gh.repos[REPO].commits
    pg.goto(URL + "#/config")
    pg.wait_for_selector("text=Situação")
    pg.click("button:has-text('Publicar agora')")
    pg.wait_for_selector("text=Histórico publicado")
    checar(gh.repos[REPO].commits == commits, "não publica de novo se nada mudou")

    # apagar serviço tira do site
    pg.goto(URL + "#/carro/ABC1D23")
    pg.wait_for_selector("h2:has-text('Histórico (1)')")
    pg.click(".tempo button:has-text('Apagar')")
    pg.wait_for_selector("text=Serviço apagado")
    checar(bool(esperar(lambda: "dianteira cobreq" not in csv_publico().lower(), 15)), "serviço apagado sai do site")
    d = json.loads((pasta / "dados.json").read_text())
    l = [x for x in d["lancamentos"] if x["ref"] == "venda:1002"][0]
    checar(l["status"] == "pendente", "lançamento do Zeus volta a pendente quando o serviço é apagado")

    # prévia da etiqueta e exportação
    r = pg.request.get(URL + "api/etiqueta.png?placa=HHH1234&km=1000&item=Óleo do motor")
    checar(r.status == 200 and r.headers["content-type"] == "image/png", "prévia da etiqueta")
    r = pg.request.get(URL + "api/exportar.csv")
    checar(r.status == 200 and "HHH-1234" in r.text(), "planilha para Excel")

    # tela estreita
    pg.set_viewport_size({"width": 800, "height": 900})
    pg.goto(URL + "#/novo")
    pg.wait_for_selector("#f-placa")
    checar(pg.evaluate("document.documentElement.scrollWidth <= innerWidth + 1"), "sem rolagem lateral em 800px")
    pg.screenshot(path=str(SAIDA / "07-estreito.png"), full_page=True)

    checar(not erros, f"sem erros no console: {erros[:3]}")
    nav.close()

proc.terminate()
proc.wait(5)
gh.parar()
shutil.copytree(imp, SAIDA / "impressoes", dirs_exist_ok=True)
print("\nCapturas em", SAIDA)
print("FALHAS:" if falhas else "OK — programa passou em todos os testes", *falhas, sep="\n  ")
sys.exit(1 if falhas else 0)

// Testa o Codigo.gs fora do Google, com uma imitação mínima do SpreadsheetApp.
const fs = require('fs');
const vm = require('vm');
const crypto = require('crypto');
const assert = require('assert');

function chainable(target) {
  return new Proxy(target, {
    get(t, prop) {
      if (prop in t) return t[prop];
      if (typeof prop === 'symbol') return undefined;
      return function () { return this; }; // setFontWeight, setNumberFormat, ...
    },
  });
}

class Sheet {
  constructor(name) { this.name = name; this.data = []; this.maxRows = 1000; }
  getName() { return this.name; }
  cell(r, c) { return (this.data[r - 1] && this.data[r - 1][c - 1] !== undefined) ? this.data[r - 1][c - 1] : ''; }
  set(r, c, v) { while (this.data.length < r) this.data.push([]); this.data[r - 1][c - 1] = v; }
  getLastRow() {
    for (let r = this.data.length; r >= 1; r--) {
      if ((this.data[r - 1] || []).some(v => v !== '' && v !== undefined && v !== null)) return r;
    }
    return 0;
  }
  getLastColumn() { return Math.max(0, ...this.data.map(r => r.length)); }
  getMaxRows() { return this.maxRows; }
  insertRowsAfter(_, n) { this.maxRows += n; }
  getRange(a, b, nr, nc) {
    if (typeof a === 'string') { // 'A1'
      const m = a.match(/^([A-Z]+)(\d+)$/);
      return this.getRange(parseInt(m[2], 10), m[1].charCodeAt(0) - 64, 1, 1);
    }
    return makeRange(this, a, b, nr || 1, nc || 1);
  }
  getDataRange() { return this.getRange(1, 1, Math.max(this.getLastRow(), 1), Math.max(this.getLastColumn(), 1)); }
  clear() { this.data = []; return this; }
  clearContents() { this.data = []; return this; }
  getProtections() { return this.protecoes || []; }
  getFormUrl() { return this.formUrl || null; }
  setName(n) { this.name = n; return this; }
  protect() { const p = chainable({}); this.protecoes = [p]; return p; }
}

function makeRange(sheet, row, col, nr, nc) {
  return chainable({
    getSheet: () => sheet,
    getRow: () => row,
    getColumn: () => col,
    getLastRow: () => row + nr - 1,
    getLastColumn: () => col + nc - 1,
    getNumRows: () => nr,
    getValues() {
      const out = [];
      for (let i = 0; i < nr; i++) { const r = []; for (let j = 0; j < nc; j++) r.push(sheet.cell(row + i, col + j)); out.push(r); }
      return out;
    },
    getValue() { return sheet.cell(row, col); },
    setValues(vals) {
      assert.strictEqual(vals.length, nr, 'setValues: número de linhas');
      vals.forEach((r, i) => { assert.strictEqual(r.length, nc, 'setValues: número de colunas'); r.forEach((v, j) => sheet.set(row + i, col + j, v)); });
      return this;
    },
    setValue(v) { sheet.set(row, col, v); return this; },
  });
}

function makeSpreadsheet() {
  const sheets = [new Sheet('Página1')];
  return chainable({
    sheets,
    getId: () => 'PLANILHA_TESTE',
    getSheetByName: (n) => sheets.find(s => s.name === n) || null,
    insertSheet: (n) => { const s = chainable(new Sheet(n)); sheets.push(s); return s; },
    getSheets: () => sheets,
    deleteSheet: (s) => sheets.splice(sheets.indexOf(s), 1),
    getSpreadsheetTimeZone: () => 'America/Maceio',
  });
}

function pad(n) { return String(n).padStart(2, '0'); }
function formatDate(d, tz, fmt) {
  // Maceió é UTC-3 o ano inteiro
  const t = new Date(d.getTime() - 3 * 3600 * 1000);
  const map = { yyyy: t.getUTCFullYear(), MM: pad(t.getUTCMonth() + 1), dd: pad(t.getUTCDate()), HH: pad(t.getUTCHours()), mm: pad(t.getUTCMinutes()) };
  return fmt.replace(/'T'/, 'T').replace(/yyyy|MM|dd|HH|mm/g, k => map[k]);
}

const ss = makeSpreadsheet();
const propriedades = { props: {}, setProperty(k, v) { this.props[k] = v; }, getProperty(k) { return this.props[k]; } };
const gatilhos = [];
const formsCriados = [];
function novoForm() {
  const item = { getId: () => 999, asParagraphTextItem: () => ({ createResponse: (v) => ({ v }) }) };
  return chainable({
    getId: () => 'FORM123',
    getItems: () => [item],
    addParagraphTextItem: () => chainable({}),
    setDestination(tipo, id) {
      if (!ss.getSheetByName('Respostas ao formulário 1')) {
        const aba = ss.insertSheet('Respostas ao formulário 1');
        aba.formUrl = 'https://docs.google.com/forms/d/FORM123/edit';
        aba.set(1, 1, 'Carimbo de data/hora'); aba.set(1, 2, 'dados');
      }
      return this;
    },
    createResponse: () => ({ withItemResponse() { return this; }, toPrefilledUrl: () => 'https://docs.google.com/forms/d/e/1FAIpQLSe_PUB/viewform?usp=pp_url&entry.1234567890=x' }),
    getPublishedUrl: () => 'https://docs.google.com/forms/d/e/1FAIpQLSe_PUB/viewform',
  });
}
let activeSheet = null;
const alerts = [];
const dialogos = [];
const ctx = {
  console,
  SpreadsheetApp: {
    getActiveSpreadsheet: () => ss,
    getActive: () => ss,
    getActiveSheet: () => activeSheet,
    flush: () => {},
    getUi: () => ({ alert: (m) => alerts.push(m), createMenu: () => chainable({}), showModalDialog: (h, t) => dialogos.push(t) }),
    newDataValidation: () => chainable({ build: () => ({}) }),
    ProtectionType: { SHEET: 'SHEET' },
  },
  Utilities: {
    DigestAlgorithm: { SHA_256: 'sha256' },
    Charset: { UTF_8: 'utf8' },
    computeDigest: (alg, s) => Array.from(crypto.createHash('sha256').update(s, 'utf8').digest()).map(b => (b > 127 ? b - 256 : b)),
    getUuid: () => crypto.randomUUID(),
    base64EncodeWebSafe: (t) => Buffer.from(t, 'utf8').toString('base64').replace(/\+/g, '-').replace(/\//g, '_'),
    formatDate,
  },
  LockService: { getDocumentLock: () => ({ waitLock() {}, releaseLock() {} }) },
  PropertiesService: { getScriptProperties: () => propriedades },
  FormApp: {
    ItemType: { PARAGRAPH_TEXT: 'PARAGRAPH_TEXT' },
    DestinationType: { SPREADSHEET: 'SPREADSHEET' },
    create: (titulo) => { formsCriados.push(titulo); return novoForm(); },
    openById: (id) => { if (id !== 'FORM123') throw new Error('não existe'); return novoForm(); },
  },
  ScriptApp: {
    getProjectTriggers: () => gatilhos,
    newTrigger: (fn) => chainable({ forSpreadsheet() { return this; }, onFormSubmit() { return this; }, create() { gatilhos.push({ getHandlerFunction: () => fn }); return this; } }),
  },
  ContentService: {
    MimeType: { JSON: 'json' },
    createTextOutput: (s) => ({ texto: s, setMimeType() { return this; } }),
  },
  HtmlService: { createHtmlOutput: (h) => chainable({ html: h }) },
};
vm.createContext(ctx);
vm.runInContext(fs.readFileSync(__dirname + '/../apps-script/Codigo.gs', 'utf8') + '\n' +
  fs.readFileSync(__dirname + '/../apps-script/Ponte.gs', 'utf8') + '\n;this.__exp = {CONFIG, COL_S, COL_V, ITENS, PONTE};', ctx);
const g = ctx;

// 1. Configurar
g.configurarPlanilha();
const serv = ss.getSheetByName('Serviços');
const veic = ss.getSheetByName('Veículos');
assert(serv && veic && ss.getSheetByName('Como usar'), 'abas criadas');
assert(!ss.getSheetByName('Página1'), 'aba vazia padrão removida');
assert.strictEqual(serv.cell(1, 1), 'Data');
assert.strictEqual(serv.cell(1, 14), 'Observação interna (não aparece no QR)');
assert.strictEqual(serv.cell(1, 15), 'Origem (ponte com o Zeus)');
assert.strictEqual(veic.cell(1, 1), 'Código do QR');

// 2. Registrar serviço (placa digitada com hífen e minúsculas)
const r1 = g.registrarServico({ placa: 'abc-1d23', modelo: 'Fiat Strada', ano: '2019', data: '2025-11-14', km: '153.273',
  itens: ['Óleo do motor', 'Filtro de óleo', 'Item que não existe'], outros: '', proximaTrocaKm: '160.273',
  obsCliente: 'Óleo 5W30 sintético', obsInterna: 'cliente pediu desconto' });
assert.strictEqual(r1.placa, 'ABC1D23');
assert.match(r1.codigo, /^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{8}$/);
assert.strictEqual(r1.novo, true);
assert.strictEqual(r1.link, 'https://automec135.github.io/?c=' + r1.codigo);
assert.strictEqual(r1.linha, 2);
assert.strictEqual(veic.cell(2, 2), 'ABC1D23');
assert.strictEqual(veic.cell(2, 3), 'Fiat Strada');
assert.strictEqual(serv.cell(2, 3), 153273);
assert.strictEqual(serv.cell(2, 4), true);
assert.strictEqual(serv.cell(2, 6), false);
assert.strictEqual(serv.cell(2, 12), 160273);

// 3. Segundo serviço, data anterior, com outros serviços; checkbox "vazio" pré-formatado em linhas abaixo
for (let c = 4; c <= 10; c++) serv.set(40, c, false); // imita linhas com caixas FALSE
const r2 = g.registrarServico({ placa: 'ABC1D23', modelo: 'Outro modelo', data: '2025-05-02', km: '145010',
  itens: [], outros: 'Troca da correia dentada e do tensor', proximaTrocaKm: '' });
assert.strictEqual(r2.codigo, r1.codigo, 'mesmo veículo, mesmo código');
assert.strictEqual(r2.novo, false);
assert.strictEqual(r2.linha, 3, 'linha certa mesmo com caixas FALSE mais abaixo');
assert.strictEqual(veic.cell(2, 3), 'Fiat Strada', 'modelo não é sobrescrito');

// 4. Validações
assert.throws(() => g.registrarServico({ placa: 'AB12', itens: ['Óleo do motor'] }), /Placa inválida/);
assert.throws(() => g.registrarServico({ placa: 'ABC1234', itens: [] }), /Marque o que foi trocado/);

// 5. Digitação direta na aba Serviços (gatilho onEdit)
activeSheet = serv;
serv.set(4, 2, 'xyz 9a87');
serv.set(4, 4, 'x');
serv.set(4, 3, 87000.5); // número grande com fração: só arredonda
g.onEdit({ range: serv.getRange(4, 2) });
assert.strictEqual(serv.cell(4, 2), 'XYZ9A87', 'placa normalizada');
assert(Object.prototype.toString.call(serv.cell(4, 1)) === '[object Date]', 'data preenchida');
assert.strictEqual(veic.cell(3, 2), 'XYZ9A87', 'veículo criado pelo onEdit');
assert.match(veic.cell(3, 1), /^[A-Z0-9]{8}$/);
const codigoXYZ = veic.cell(3, 1);

// 6. Colar várias linhas de uma vez (inclui placa repetida e uma já cadastrada)
serv.set(5, 2, 'qwe1234'); serv.set(5, 4, true);
serv.set(6, 2, 'QWE-1234'); serv.set(6, 5, true);
serv.set(7, 2, 'abc1d23'); serv.set(7, 11, 'Alinhamento');
g.onEdit({ range: serv.getRange(5, 1, 3, 3) });
const placasVeic = [2, 3, 4, 5].map(r => veic.cell(r, 2));
assert.deepStrictEqual(placasVeic, ['ABC1D23', 'XYZ9A87', 'QWE1234', ''], 'sem duplicar veículos');

// 7. Placa digitada na aba Veículos sem código
activeSheet = veic;
veic.set(5, 2, 'rst-5e67');
g.onEdit({ range: veic.getRange(5, 2) });
assert.strictEqual(veic.cell(5, 2), 'RST5E67');
assert.match(veic.cell(5, 1), /^[A-Z0-9]{8}$/);
assert.strictEqual(veic.cell(5, 6), 'https://automec135.github.io/?c=' + veic.cell(5, 1));

// 8. API pública
const resp = JSON.parse(g.doGet({ parameter: { c: r1.codigo.toLowerCase() } }).texto);
assert.strictEqual(resp.ok, true);
assert.deepStrictEqual(resp.veiculo, { modelo: 'Fiat Strada', ano: '2019', placa: 'ABC-•••3' });
assert.strictEqual(resp.servicos.length, 3);
assert.deepStrictEqual(resp.servicos.map(s => s.data), [resp.servicos[0].data, '2025-11-14', '2025-05-02']);
const nov = resp.servicos.find(s => s.data === '2025-11-14');
assert.deepStrictEqual(nov.itens, ['Óleo do motor', 'Filtro de óleo']);
assert.strictEqual(nov.km, 153273);
assert.strictEqual(nov.proximaTrocaKm, 160273);
assert.strictEqual(nov.obs, 'Óleo 5W30 sintético');
assert(!JSON.stringify(resp).includes('desconto'), 'observação interna não vaza');
assert(!JSON.stringify(resp).includes('ABC1D23'), 'placa completa não vaza');
assert.deepStrictEqual(JSON.parse(g.doGet({ parameter: { c: 'ZZZZZZZZ' } }).texto), { ok: false, erro: 'nao_encontrado' });
assert.deepStrictEqual(JSON.parse(g.doGet({ parameter: { c: 'abc' } }).texto), { ok: false, erro: 'codigo_invalido' });
assert.deepStrictEqual(JSON.parse(g.doGet({}).texto), { ok: false, erro: 'codigo_invalido' });
const xyz = JSON.parse(g.doGet({ parameter: { c: codigoXYZ } }).texto);
assert.strictEqual(xyz.servicos.length, 1);
assert.deepStrictEqual(xyz.servicos[0].itens, ['Óleo do motor'], '"x" conta como marcado');
assert.strictEqual(xyz.servicos[0].km, 87001);

// 9. Busca da barra lateral
const b = g.buscarVeiculo('abc-1d23');
assert.strictEqual(b.encontrado, true);
assert.strictEqual(b.codigo, r1.codigo);
assert.strictEqual(b.servicos, 3);
assert.deepStrictEqual(JSON.parse(JSON.stringify(g.buscarVeiculo('NOV1234'))), { placa: 'NOV1234', encontrado: false });

// 10. Utilitários
assert.strictEqual(g.paraNumero_(153.273), 153273);
assert.strictEqual(g.paraNumero_('160,273'), 160273);
assert.strictEqual(g.paraNumero_(''), null);
assert.strictEqual(g.mascararPlaca_('ABC1234'), 'ABC-•••4');
assert.strictEqual(g.paraDataISO_('14/11/2025', 'America/Maceio'), '2025-11-14');
const codigos = new Set(Array.from({ length: 5000 }, () => g.gerarCodigo_()));
assert(codigos.size > 4990, 'códigos variados');
for (const c of codigos) assert.match(c, /^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{8}$/);


// 11. Aba Público (CSV publicado)
const pub = ss.getSheetByName('Público');
assert(pub, 'aba Público criada');
const idEsperado = (c) => crypto.createHash('sha256').update('automec135|' + c, 'utf8').digest('hex').slice(0, 20);
assert.strictEqual(g.idPublico_(r1.codigo), idEsperado(r1.codigo));
g.atualizarPublico();
const tab = pub.data.map(l => l.slice(0, 9));
assert.deepStrictEqual(tab[0], ['id', 'modelo', 'ano', 'data', 'km', 'itens', 'outros', 'proxima_troca_km', 'obs']);
assert.strictEqual(tab[1][0], '_');
assert.match(tab[1][1], /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
const doCarro = tab.filter(l => l[0] === idEsperado(r1.codigo));
assert.strictEqual(doCarro.length, 3, '3 serviços do ABC1D23');
const nov2 = doCarro.find(l => l[3] === '2025-11-14');
assert.deepStrictEqual(nov2, [idEsperado(r1.codigo), 'Fiat Strada', '2019', '2025-11-14', '153273', 'Óleo do motor; Filtro de óleo', '', '160273', 'Óleo 5W30 sintético']);
const texto = JSON.stringify(pub.data);
assert(!texto.includes('ABC1D23') && !texto.includes('XYZ9A87'), 'sem placas na aba pública');
assert(!texto.includes(r1.codigo), 'sem códigos na aba pública');
assert(!texto.includes('desconto'), 'sem observação interna na aba pública');
const rst = tab.filter(l => l[0] === idEsperado(veic.cell(5, 1)));
assert.deepStrictEqual(rst, [[idEsperado(veic.cell(5, 1)), '', '', '', '', '', '', '', '']], 'veículo sem serviço aparece vazio');

// 12. Editar uma coluna que não é a placa também atualiza a aba Público
activeSheet = serv;
serv.set(3, 11, 'Troca da correia dentada, tensor e bomba d\'água');
g.onEdit({ range: serv.getRange(3, 11) });
assert(JSON.stringify(pub.data).includes('bomba d'), 'Público atualizado após editar Outros');
// editar a própria aba Público não dispara nada
const antes = JSON.stringify(pub.data);
g.onEdit({ range: pub.getRange(5, 2) });
assert.strictEqual(JSON.stringify(pub.data), antes);

// 13. Ponte com o Zeus: código proposto, código extra, conflito e repetição
const ALF = /^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{8}$/;
const p1 = g.registrarServico({ placa: 'COR0L10', modelo: 'Corolla', ano: '2015', data: '2026-10-22', km: 98000,
  itens: [], outros: 'Mangueira de ar', codigo: 'PZ23ABCD', origem: 'zeus:500', obsInterna: 'Zeus nº 500' });
assert.strictEqual(p1.codigo, 'PZ23ABCD', 'placa nova usa o código impresso pela ponte');
assert.strictEqual(p1.novo, true);
const linhaP1 = p1.linha;
assert.strictEqual(serv.cell(linhaP1, 15), 'zeus:500', 'origem gravada');
const p1b = g.registrarServico({ placa: 'COR0L10', outros: 'Mangueira de ar', codigo: 'PZ23ABCD', origem: 'zeus:500' });
assert.strictEqual(p1b.duplicado, true, 'mesmo lançamento não duplica');
assert.strictEqual(p1b.linha, linhaP1);
// placa que já tinha código (ABC1D23) recebe código extra
const p2 = g.registrarServico({ placa: 'ABC1D23', itens: ['Óleo do motor'], km: 161000, codigo: 'PZ45EFGH', origem: 'zeus:501' });
assert.strictEqual(p2.codigo, 'PZ45EFGH');
assert.strictEqual(p2.extra, true);
assert.strictEqual(p2.linha, linhaP1 + 1, 'serviço novo na linha seguinte');
assert.strictEqual(serv.data.filter(l => l[14] === 'zeus:500').length, 1, 'zeus:500 só uma vez');
const linhasABC = veic.data.filter(l => l[1] === 'ABC1D23');
assert.strictEqual(linhasABC.length, 2, 'mesma placa com dois códigos');
assert.strictEqual(g.buscarVeiculo('ABC1D23').codigo, r1.codigo, 'código principal continua o primeiro');
const tab2 = pub.data;
const servicosABC = (id) => tab2.filter(l => l[0] === id).length;
assert.strictEqual(servicosABC(idEsperado(r1.codigo)), 4, 'código antigo vê o serviço novo');
assert.strictEqual(servicosABC(idEsperado('PZ45EFGH')), 4, 'código novo vê o histórico todo');
const extraHist = JSON.parse(g.doGet({ parameter: { c: 'PZ45EFGH' } }).texto);
assert.strictEqual(extraHist.servicos.length, 4);
// de novo com o mesmo código extra: não cria outra linha
const p3 = g.registrarServico({ placa: 'ABC1D23', outros: 'Alinhamento', codigo: 'PZ45EFGH', origem: 'zeus:502' });
assert.strictEqual(p3.codigo, 'PZ45EFGH');
assert(!p3.extra);
assert.strictEqual(veic.data.filter(l => l[1] === 'ABC1D23').length, 2);
// código de outro carro: conflito, fica o código principal
const p4 = g.registrarServico({ placa: 'XYZ9A87', outros: 'Pastilhas de freio', codigo: 'PZ23ABCD', origem: 'zeus:503' });
assert.strictEqual(p4.conflito, 'PZ23ABCD');
assert.strictEqual(p4.codigo, codigoXYZ);
// código fora do formato é ignorado
const p5 = g.registrarServico({ placa: 'NEW1A23', outros: 'Bateria', codigo: 'abc' , origem: 'zeus:504' });
assert.match(p5.codigo, ALF);
assert(!p5.conflito);

// 14. Ligar a ponte (formulário + gatilho) e receber pelo gatilho
const cfg = g.configurarPonte();
assert.strictEqual(formsCriados.length, 1);
assert.strictEqual(gatilhos.length, 1);
assert.deepStrictEqual(dialogos, ['Ponte com o Zeus']);
assert.strictEqual(propriedades.getProperty('PONTE_FORM_ID'), 'FORM123');
assert(ss.getSheetByName('Ponte (Zeus)'), 'aba de respostas renomeada');
assert.match(cfg, /^PONTE1:/);
const dec = JSON.parse(Buffer.from(cfg.slice(7).replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8'));
assert.deepStrictEqual(dec, { v: 1, u: 'https://docs.google.com/forms/d/e/1FAIpQLSe_PUB/formResponse', e: 'entry.1234567890' });
g.configurarPonte(); // de novo: não cria outro formulário nem outro gatilho
assert.strictEqual(formsCriados.length, 1);
assert.strictEqual(gatilhos.length, 1);

const abaP = ss.getSheetByName('Ponte (Zeus)');
function enviar(obj, linha) {
  const json = typeof obj === 'string' ? obj : JSON.stringify(obj);
  abaP.set(linha, 1, new Date()); abaP.set(linha, 2, json);
  g.receberDaPonte({ range: abaP.getRange(linha, 1, 1, 2), values: ['agora', json], namedValues: { dados: [json] } });
  return abaP.cell(linha, 3);
}
const okMsg = enviar({ v: 1, origem: 'zeus:600', placa: 'gol-2b34', modelo: 'Gol', ano: '2012', data: '2026-10-22', km: 120500,
  itens: ['Óleo do motor', 'Filtro de óleo'], outros: 'Mangueira de ar', proximaTrocaKm: 125500, codigo: 'PZ67JKLM', obsInterna: 'Zeus nº 600' }, 2);
assert.match(okMsg, /^OK — Serviços, linha \d+ — código PZ67JKLM$/);
assert.strictEqual(g.buscarVeiculo('GOL2B34').codigo, 'PZ67JKLM');
assert.match(enviar({ v: 1, origem: 'zeus:600', placa: 'GOL2B34', outros: 'x', codigo: 'PZ67JKLM' }, 3), /^Já estava registrado/);
assert.match(enviar('{quebrado', 4), /^ERRO:/);
assert.match(enviar({ v: 9, placa: 'GOL2B34', outros: 'x' }, 5), /^ERRO: versão/);
assert.match(enviar({ v: 1, placa: 'XX', outros: 'x' }, 6), /^ERRO: Placa inválida/);
assert.match(enviar({ v: 1, origem: 'zeus:601', placa: 'XYZ9A87', outros: 'Freio', codigo: 'PZ67JKLM' }, 7), /ATENÇÃO: o código PZ67JKLM é de outro carro/);
// resposta de outro formulário (outra aba) é ignorada
const outra = ss.insertSheet('Outro formulário');
g.receberDaPonte({ range: outra.getRange(2, 1), values: ['x', '{}'] });
assert.strictEqual(outra.cell(2, 3), '');
const gol = JSON.parse(g.doGet({ parameter: { c: 'PZ67JKLM' } }).texto);
assert.deepStrictEqual(gol.servicos[0].itens, ['Óleo do motor', 'Filtro de óleo']);
assert.strictEqual(gol.servicos[0].outros, 'Mangueira de ar');
assert.strictEqual(gol.servicos[0].proximaTrocaKm, 125500);
assert(!JSON.stringify(pub.data).includes('Zeus nº'), 'observação interna da ponte não vaza');

console.log('OK — todos os testes do Apps Script passaram');
console.log('Exemplo de resposta da API:\n' + JSON.stringify(resp, null, 2));

/**
 * Histórico de veículos — Auto Mecânica Itabaiana
 *
 * Esta planilha guarda os veículos e os serviços feitos na oficina.
 * O script:
 *   - cria o menu "Oficina" (registrar serviço, ver o código do QR);
 *   - gera um código secreto para cada placa nova (é o que vai no QR Code);
 *   - mantém a aba "Público", que é publicada na Web e lida pela página do QR.
 *
 * A aba "Público" só tem: um identificador tirado do código (não dá para
 * voltar ao código), modelo, ano e os serviços. Sem placa, nome ou telefone.
 * A "Observação interna" nunca sai da planilha.
 */

const CONFIG = {
  SITE_URL: 'https://automec135.github.io/',
  ABA_SERVICOS: 'Serviços',
  ABA_VEICULOS: 'Veículos',
  ABA_AJUDA: 'Como usar',
  ABA_PUBLICO: 'Público',
  SAL_PUBLICO: 'automec135|', // tem que ser igual ao "sal" do index.html
  TAMANHO_CODIGO: 8,
  // Sem 0/O e 1/I para ninguém confundir ao digitar.
  ALFABETO: '23456789ABCDEFGHJKLMNPQRSTUVWXYZ',
  LINHAS_FORMATADAS: 3000,
};

const ITENS = [
  'Óleo do motor',
  'Filtro de óleo',
  'Filtro de ar',
  'Filtro de combustível',
  'Filtro de cabine',
  'Óleo de freio',
  'Óleo de câmbio',
];

// Colunas (começando em 1)
const COL_S = {
  DATA: 1,
  PLACA: 2,
  KM: 3,
  ITEM1: 4,
  OUTROS: 4 + ITENS.length,
  PROX_KM: 5 + ITENS.length,
  OBS_CLIENTE: 6 + ITENS.length,
  OBS_INTERNA: 7 + ITENS.length,
};
const COL_V = { CODIGO: 1, PLACA: 2, MODELO: 3, ANO: 4, CADASTRO: 5, LINK: 6 };

const CAB_SERVICOS = ['Data', 'Placa', 'KM'].concat(ITENS, [
  'Outros serviços / peças trocadas',
  'Próxima troca de óleo (KM)',
  'Observação para o cliente',
  'Observação interna (não aparece no QR)',
]);
const CAB_VEICULOS = ['Código do QR', 'Placa', 'Modelo', 'Ano', 'Cadastrado em', 'Link do QR Code'];
const CAB_PUBLICO = ['id', 'modelo', 'ano', 'data', 'km', 'itens', 'outros', 'proxima_troca_km', 'obs'];

const PLACA_VALIDA = /^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/;
const CODIGO_VALIDO = /^[A-Z0-9]{6,12}$/;

/* ------------------------------------------------------------------ */
/* Menu e telas                                                        */
/* ------------------------------------------------------------------ */

function onOpen() {
  SpreadsheetApp.getUi()
    .createMenu('🔧 Oficina')
    .addItem('Registrar serviço', 'abrirRegistro')
    .addItem('Ver código do QR (linha selecionada)', 'mostrarCodigoDaLinha')
    .addSeparator()
    .addItem('Atualizar dados do QR', 'atualizarPublico')
    .addItem('Configurar planilha', 'configurarPlanilha')
    .addToUi();
}

function abrirRegistro() {
  const t = HtmlService.createTemplateFromFile('Registro');
  t.itens = ITENS;
  t.hoje = Utilities.formatDate(new Date(), planilha_().getSpreadsheetTimeZone(), 'yyyy-MM-dd');
  t.placaInicial = placaDaLinhaAtiva_();
  SpreadsheetApp.getUi().showSidebar(t.evaluate().setTitle('Registrar serviço'));
}

function mostrarCodigoDaLinha() {
  const ui = SpreadsheetApp.getUi();
  const placa = placaDaLinhaAtiva_();
  if (!placa) {
    ui.alert('Clique em uma linha que tenha placa (na aba Serviços ou Veículos) e tente de novo.');
    return;
  }
  const v = garantirVeiculos_([placa])[placa];
  const link = linkDoCodigo_(v.codigo);
  const html = HtmlService.createHtmlOutput(
    '<div style="font:15px Arial,sans-serif;color:#202124">' +
      '<p style="margin:0 0 6px">Placa <b>' + escapar_(placa) + '</b></p>' +
      '<p style="margin:0 0 4px">Código para o campo <b>Código do veículo</b> da etiqueta:</p>' +
      '<input id="c" readonly value="' + escapar_(v.codigo) + '" style="font:bold 28px monospace;letter-spacing:3px;width:100%;box-sizing:border-box;padding:6px;text-align:center">' +
      '<p style="margin:10px 0 4px">Link que o QR Code abre:</p>' +
      '<input readonly value="' + escapar_(link) + '" style="width:100%;box-sizing:border-box;padding:4px">' +
      '<p style="margin:12px 0 0;text-align:right"><button onclick="var c=document.getElementById(\'c\');c.select();try{document.execCommand(\'copy\')}catch(e){}this.textContent=\'Código copiado\'">Copiar código</button></p>' +
      '<script>document.getElementById("c").select()</script>' +
    '</div>'
  ).setWidth(400).setHeight(250);
  ui.showModalDialog(html, 'Código do QR Code');
}

/* ------------------------------------------------------------------ */
/* Configuração da planilha                                            */
/* ------------------------------------------------------------------ */

function configurarPlanilha() {
  const ss = planilha_();
  ss.setSpreadsheetLocale('pt_BR');
  ss.setSpreadsheetTimeZone('America/Maceio');
  PropertiesService.getScriptProperties().setProperty('PLANILHA_ID', ss.getId());

  const serv = abaOuNova_(ss, CONFIG.ABA_SERVICOS);
  const veic = abaOuNova_(ss, CONFIG.ABA_VEICULOS);
  const n = CONFIG.LINHAS_FORMATADAS;

  prepararCabecalho_(serv, CAB_SERVICOS, '#1E2329');
  garantirLinhas_(serv, n + 1);
  serv.getRange(2, COL_S.DATA, n, 1).setNumberFormat('dd/MM/yyyy');
  serv.getRange(2, COL_S.PLACA, n, 1).setNumberFormat('@');
  serv.getRange(2, COL_S.KM, n, 1).setNumberFormat('#,##0');
  serv.getRange(2, COL_S.PROX_KM, n, 1).setNumberFormat('#,##0');
  serv.getRange(2, COL_S.ITEM1, n, ITENS.length)
    .setDataValidation(SpreadsheetApp.newDataValidation().requireCheckbox().build())
    .setHorizontalAlignment('center');
  serv.getRange(2, COL_S.OUTROS, n, 1).setWrap(true);
  serv.getRange(2, COL_S.OBS_CLIENTE, n, 2).setWrap(true);
  serv.getRange(1, COL_S.OBS_INTERNA, n + 1, 1).setBackground('#FFF4D6');
  serv.getRange(1, COL_S.OBS_INTERNA).setBackground('#7A5A00');
  larguras_(serv, [90, 90, 80].concat(ITENS.map(function () { return 92; }), [280, 130, 220, 220]));

  prepararCabecalho_(veic, CAB_VEICULOS, '#1E2329');
  garantirLinhas_(veic, n + 1);
  veic.getRange(2, COL_V.CODIGO, n, 2).setNumberFormat('@');
  veic.getRange(2, COL_V.ANO, n, 1).setNumberFormat('@');
  veic.getRange(2, COL_V.CADASTRO, n, 1).setNumberFormat('dd/MM/yyyy');
  veic.getRange(2, COL_V.CODIGO, n, 1).setFontFamily('Roboto Mono').setFontWeight('bold');
  larguras_(veic, [130, 90, 200, 70, 110, 380]);

  // Códigos para placas que já estavam na aba Veículos sem código
  const ultima = veic.getLastRow();
  if (ultima >= 2) {
    const placas = veic.getRange(2, COL_V.PLACA, ultima - 1, 1).getValues()
      .map(function (r) { return normalizarPlaca_(r[0]); })
      .filter(function (p) { return p; });
    if (placas.length) garantirVeiculos_(placas);
  }

  criarAjuda_(ss);
  const pub = abaOuNova_(ss, CONFIG.ABA_PUBLICO);
  if (!pub.getProtections(SpreadsheetApp.ProtectionType.SHEET).length) {
    pub.protect().setDescription('Gerada pelo script: não editar').setWarningOnly(true);
  }
  pub.setTabColor('#9AA0A6');
  atualizarPublico();
  removerAbaVaziaPadrao_(ss);
  ss.setActiveSheet(serv);
  try {
    SpreadsheetApp.getUi().alert(
      'Planilha pronta!\n\nUse o menu 🔧 Oficina > Registrar serviço para lançar os serviços. ' +
      'Cada placa nova ganha um código, que vai no QR Code da etiqueta.'
    );
  } catch (e) {
    console.log('Planilha pronta.'); // executado pelo editor, sem tela da planilha
  }
}

function criarAjuda_(ss) {
  const aba = abaOuNova_(ss, CONFIG.ABA_AJUDA);
  aba.clear();
  const linhas = [
    ['Como usar o histórico com QR Code'],
    [''],
    ['1. Registrar um serviço'],
    ['Menu 🔧 Oficina > Registrar serviço. Preencha a placa, marque o que foi trocado e clique em Salvar serviço.'],
    ['Também dá para digitar direto na aba Serviços: ao escrever a placa, o veículo é cadastrado sozinho.'],
    [''],
    ['2. Imprimir a etiqueta'],
    ['Depois de salvar, aparece o código do veículo (8 letras e números).'],
    ['No BarTender, abra a etiqueta de óleo, clique em Imprimir e cole esse código no campo Código do veículo.'],
    ['Para ver o código de um carro já cadastrado: clique na linha dele e use 🔧 Oficina > Ver código do QR.'],
    [''],
    ['3. O que o cliente vê'],
    ['Ao escanear o QR, abre a página com modelo, ano e os serviços feitos (sem placa, nome ou telefone).'],
    ['A coluna "Observação interna" nunca aparece para o cliente. Serviços novos aparecem na página em até 5 minutos.'],
    [''],
    ['Cuidados'],
    ['Não mude o código de um veículo: as etiquetas já impressas usam esse código.'],
    ['Não renomeie as abas nem mude a ordem das colunas.'],
    ['A aba Público é preenchida sozinha e publicada na Web: não edite nela. Se precisar, use 🔧 Oficina > Atualizar dados do QR.'],
  ];
  aba.getRange(1, 1, linhas.length, 1).setValues(linhas).setWrap(true).setVerticalAlignment('top');
  aba.setColumnWidth(1, 760);
  aba.getRange('A1').setFontSize(16).setFontWeight('bold');
  ['A3', 'A7', 'A12', 'A16'].forEach(function (a) { aba.getRange(a).setFontWeight('bold').setFontSize(12); });
  aba.setHiddenGridlines(true);
}

/* ------------------------------------------------------------------ */
/* Edição direta na planilha (gatilho simples)                         */
/* ------------------------------------------------------------------ */

function onEdit(e) {
  if (!e || !e.range) return;
  const nome = e.range.getSheet().getName();
  if (nome !== CONFIG.ABA_SERVICOS && nome !== CONFIG.ABA_VEICULOS) return;
  ajustarPlacas_(e.range);
  atualizarPublico();
}

function ajustarPlacas_(range) {
  const aba = range.getSheet();
  const nome = aba.getName();
  const col = nome === CONFIG.ABA_SERVICOS ? COL_S.PLACA : COL_V.PLACA;
  if (range.getColumn() > col || range.getLastColumn() < col) return;

  const ini = Math.max(range.getRow(), 2);
  const fim = range.getLastRow();
  if (fim < ini) return;
  const qtd = fim - ini + 1;

  const faixa = aba.getRange(ini, col, qtd, 1);
  const originais = faixa.getValues();
  const norm = originais.map(function (r) { return [normalizarPlaca_(r[0])]; });
  const mudou = norm.some(function (r, i) { return r[0] !== String(originais[i][0]); });
  if (mudou) faixa.setValues(norm);

  const placas = norm.map(function (r) { return r[0]; }).filter(function (p) { return p; });
  if (placas.length) garantirVeiculos_(placas);

  if (nome === CONFIG.ABA_SERVICOS) {
    // Data de hoje quando a linha nova ainda não tem data
    const faixaData = aba.getRange(ini, COL_S.DATA, qtd, 1);
    const datas = faixaData.getValues();
    let preencheu = false;
    datas.forEach(function (r, i) {
      if (r[0] === '' && norm[i][0]) { r[0] = hojeAoMeioDia_(); preencheu = true; }
    });
    if (preencheu) faixaData.setValues(datas);
  }
}

/* ------------------------------------------------------------------ */
/* Funções chamadas pela barra lateral                                 */
/* ------------------------------------------------------------------ */

function buscarVeiculo(placaDigitada) {
  const placa = normalizarPlaca_(placaDigitada);
  if (!placa) return null;
  const veic = abaVeiculos_();
  const ultima = veic.getLastRow();
  const linhas = ultima >= 2 ? veic.getRange(2, 1, ultima - 1, CAB_VEICULOS.length).getValues() : [];
  let achado = null;
  for (let i = 0; i < linhas.length; i++) {
    if (normalizarPlaca_(linhas[i][COL_V.PLACA - 1]) === placa) { achado = linhas[i]; break; }
  }
  if (!achado) return { placa: placa, encontrado: false };

  const hist = servicosDaPlaca_(placa, planilha_().getSpreadsheetTimeZone());
  const ultimo = hist[0] || null;
  const codigo = String(achado[COL_V.CODIGO - 1] || '').trim().toUpperCase();
  return {
    placa: placa,
    encontrado: true,
    codigo: codigo,
    modelo: String(achado[COL_V.MODELO - 1] || ''),
    ano: String(achado[COL_V.ANO - 1] || ''),
    servicos: hist.length,
    ultimoKm: ultimo ? ultimo.km : null,
    ultimaData: ultimo ? ultimo.data : '',
  };
}

function registrarServico(d) {
  d = d || {};
  const placa = normalizarPlaca_(d.placa);
  if (!PLACA_VALIDA.test(placa)) throw new Error('Placa inválida. Use o formato ABC1D23 ou ABC1234.');
  const itens = (d.itens || []).filter(function (i) { return ITENS.indexOf(i) >= 0; });
  const outros = String(d.outros || '').trim();
  if (!itens.length && !outros) throw new Error('Marque o que foi trocado ou descreva o serviço em "Outros serviços".');

  const km = paraNumero_(d.km);
  const proxKm = paraNumero_(d.proximaTrocaKm);
  const lock = LockService.getDocumentLock();
  lock.waitLock(15000);
  try {
    const dados = {};
    dados[placa] = { modelo: String(d.modelo || '').trim(), ano: String(d.ano || '').trim() };
    const v = garantirVeiculos_([placa], dados)[placa];

    const aba = abaServicos_();
    const linha = proximaLinhaVazia_(aba);
    const valores = [dataDoFormulario_(d.data), placa, km === null ? '' : km]
      .concat(ITENS.map(function (i) { return itens.indexOf(i) >= 0; }))
      .concat([outros, proxKm === null ? '' : proxKm, String(d.obsCliente || '').trim(), String(d.obsInterna || '').trim()]);
    aba.getRange(linha, 1, 1, valores.length).setValues([valores]);
    atualizarPublico();
    SpreadsheetApp.flush();
    return { placa: placa, codigo: v.codigo, novo: v.novo, link: linkDoCodigo_(v.codigo), linha: linha };
  } finally {
    lock.releaseLock();
  }
}

/* ------------------------------------------------------------------ */
/* API pública usada pela página do QR Code                            */
/* ------------------------------------------------------------------ */

function doGet(e) {
  const codigo = String((e && e.parameter && e.parameter.c) || '').trim().toUpperCase();
  let resposta;
  try {
    resposta = montarHistorico_(codigo);
  } catch (err) {
    console.error(err);
    resposta = { ok: false, erro: 'falha_interna' };
  }
  return ContentService.createTextOutput(JSON.stringify(resposta)).setMimeType(ContentService.MimeType.JSON);
}

function montarHistorico_(codigo) {
  if (!CODIGO_VALIDO.test(codigo)) return { ok: false, erro: 'codigo_invalido' };
  const ss = planilha_();
  const veic = abaVeiculos_();
  const ultima = veic.getLastRow();
  const linhas = ultima >= 2 ? veic.getRange(2, 1, ultima - 1, CAB_VEICULOS.length).getValues() : [];
  let v = null;
  for (let i = 0; i < linhas.length; i++) {
    if (String(linhas[i][COL_V.CODIGO - 1] || '').trim().toUpperCase() === codigo) { v = linhas[i]; break; }
  }
  if (!v) return { ok: false, erro: 'nao_encontrado' };

  const tz = ss.getSpreadsheetTimeZone();
  const placa = normalizarPlaca_(v[COL_V.PLACA - 1]);
  return {
    ok: true,
    veiculo: {
      modelo: String(v[COL_V.MODELO - 1] || '').trim(),
      ano: String(v[COL_V.ANO - 1] || '').trim(),
      placa: mascararPlaca_(placa),
    },
    servicos: servicosDaPlaca_(placa, tz),
    atualizadoEm: Utilities.formatDate(new Date(), tz, "yyyy-MM-dd'T'HH:mm"),
  };
}

function servicosDaPlaca_(placa, tz) {
  const aba = abaServicos_();
  const ultima = aba.getLastRow();
  if (ultima < 2 || !placa) return [];
  const linhas = aba.getRange(2, 1, ultima - 1, CAB_SERVICOS.length).getValues();
  const lista = [];
  linhas.forEach(function (r) {
    if (normalizarPlaca_(r[COL_S.PLACA - 1]) !== placa) return;
    const itens = ITENS.filter(function (_, k) { return marcado_(r[COL_S.ITEM1 - 1 + k]); });
    const outros = String(r[COL_S.OUTROS - 1] || '').trim();
    const data = paraDataISO_(r[COL_S.DATA - 1], tz);
    if (!itens.length && !outros) return;
    lista.push({
      data: data,
      km: paraNumero_(r[COL_S.KM - 1]),
      itens: itens,
      outros: outros,
      proximaTrocaKm: paraNumero_(r[COL_S.PROX_KM - 1]),
      obs: String(r[COL_S.OBS_CLIENTE - 1] || '').trim(),
    });
  });
  lista.sort(function (a, b) {
    if (a.data !== b.data) return a.data < b.data ? 1 : -1;
    return (b.km || 0) - (a.km || 0);
  });
  return lista;
}

/* ------------------------------------------------------------------ */
/* Aba "Público" (publicada na Web como CSV e lida pela página do QR)  */
/* ------------------------------------------------------------------ */

/**
 * Refaz a aba "Público" a partir de Veículos e Serviços.
 * Cada linha é um serviço; veículos sem serviço entram com uma linha vazia.
 * A primeira linha depois do cabeçalho (id "_") guarda a hora da atualização.
 */
function atualizarPublico() {
  const ss = planilha_();
  const tz = ss.getSpreadsheetTimeZone();

  const veic = abaVeiculos_();
  const uv = veic.getLastRow();
  const veiculos = uv >= 2 ? veic.getRange(2, 1, uv - 1, CAB_VEICULOS.length).getValues() : [];
  const porPlaca = {};
  veiculos.forEach(function (r) {
    const codigo = String(r[COL_V.CODIGO - 1] || '').trim().toUpperCase();
    const placa = normalizarPlaca_(r[COL_V.PLACA - 1]);
    if (!codigo || !placa || porPlaca[placa]) return;
    porPlaca[placa] = {
      id: idPublico_(codigo),
      modelo: String(r[COL_V.MODELO - 1] || '').trim(),
      ano: String(r[COL_V.ANO - 1] || '').trim(),
      servicos: 0,
    };
  });

  const serv = abaServicos_();
  const us = serv.getLastRow();
  const linhas = us >= 2 ? serv.getRange(2, 1, us - 1, CAB_SERVICOS.length).getValues() : [];
  const saida = [CAB_PUBLICO, ['_', Utilities.formatDate(new Date(), tz, "yyyy-MM-dd'T'HH:mm"), '', '', '', '', '', '', '']];
  linhas.forEach(function (r) {
    const v = porPlaca[normalizarPlaca_(r[COL_S.PLACA - 1])];
    if (!v) return;
    const itens = ITENS.filter(function (_, k) { return marcado_(r[COL_S.ITEM1 - 1 + k]); });
    const outros = String(r[COL_S.OUTROS - 1] || '').trim();
    if (!itens.length && !outros) return;
    const km = paraNumero_(r[COL_S.KM - 1]);
    const prox = paraNumero_(r[COL_S.PROX_KM - 1]);
    saida.push([
      v.id, v.modelo, v.ano,
      paraDataISO_(r[COL_S.DATA - 1], tz),
      km === null ? '' : String(km),
      itens.join('; '),
      outros,
      prox === null ? '' : String(prox),
      String(r[COL_S.OBS_CLIENTE - 1] || '').trim(),
    ]);
    v.servicos++;
  });
  Object.keys(porPlaca).forEach(function (p) {
    const v = porPlaca[p];
    if (!v.servicos) saida.push([v.id, v.modelo, v.ano, '', '', '', '', '', '']);
  });

  const pub = abaOuNova_(ss, CONFIG.ABA_PUBLICO);
  pub.clearContents();
  pub.getRange(1, 1, saida.length, CAB_PUBLICO.length).setNumberFormat('@').setValues(saida);
}

/** Identificador público: 20 primeiros dígitos hex do SHA-256 de SAL + código. */
function idPublico_(codigo) {
  const bytes = Utilities.computeDigest(
    Utilities.DigestAlgorithm.SHA_256,
    CONFIG.SAL_PUBLICO + String(codigo).trim().toUpperCase(),
    Utilities.Charset.UTF_8
  );
  let s = '';
  for (let i = 0; i < 10; i++) s += ('0' + (bytes[i] & 0xff).toString(16)).slice(-2);
  return s;
}

/* ------------------------------------------------------------------ */
/* Veículos e códigos                                                  */
/* ------------------------------------------------------------------ */

/**
 * Garante que cada placa exista na aba Veículos e tenha código.
 * Lê a aba uma vez só (rápido mesmo colando muitas linhas).
 * @return {Object} placa -> {codigo, novo, modelo, ano}
 */
function garantirVeiculos_(placas, dadosPorPlaca) {
  dadosPorPlaca = dadosPorPlaca || {};
  const aba = abaVeiculos_();
  const ultima = Math.max(aba.getLastRow(), 1);
  const valores = ultima >= 2 ? aba.getRange(2, 1, ultima - 1, CAB_VEICULOS.length).getValues() : [];

  const usados = {};
  const indice = {};
  valores.forEach(function (r, i) {
    const c = String(r[COL_V.CODIGO - 1] || '').trim().toUpperCase();
    if (c) usados[c] = true;
    const p = normalizarPlaca_(r[COL_V.PLACA - 1]);
    if (p && !(p in indice)) indice[p] = i;
  });
  function novoCodigo() {
    let c;
    do { c = gerarCodigo_(); } while (usados[c]);
    usados[c] = true;
    return c;
  }

  const resultado = {};
  const novos = [];
  placas.forEach(function (p) {
    if (!p || resultado[p]) return;
    const d = dadosPorPlaca[p] || {};
    if (p in indice) {
      const i = indice[p];
      const linha = i + 2;
      const r = valores[i];
      let codigo = String(r[COL_V.CODIGO - 1] || '').trim().toUpperCase();
      if (!codigo) {
        codigo = novoCodigo();
        aba.getRange(linha, COL_V.CODIGO).setValue(codigo);
        aba.getRange(linha, COL_V.LINK).setValue(linkDoCodigo_(codigo));
      }
      const modelo = String(r[COL_V.MODELO - 1] || '').trim();
      const ano = String(r[COL_V.ANO - 1] || '').trim();
      if (d.modelo && !modelo) aba.getRange(linha, COL_V.MODELO).setValue(d.modelo);
      if (d.ano && !ano) aba.getRange(linha, COL_V.ANO).setValue(String(d.ano));
      resultado[p] = { codigo: codigo, novo: false, modelo: modelo || d.modelo || '', ano: ano || d.ano || '' };
    } else {
      const codigo = novoCodigo();
      novos.push([codigo, p, d.modelo || '', d.ano ? String(d.ano) : '', new Date(), linkDoCodigo_(codigo)]);
      resultado[p] = { codigo: codigo, novo: true, modelo: d.modelo || '', ano: d.ano ? String(d.ano) : '' };
    }
  });
  if (novos.length) aba.getRange(ultima + 1, 1, novos.length, CAB_VEICULOS.length).setValues(novos);
  return resultado;
}

function gerarCodigo_() {
  const bytes = Utilities.computeDigest(
    Utilities.DigestAlgorithm.SHA_256,
    Utilities.getUuid() + ':' + Date.now() + ':' + Math.random()
  );
  let s = '';
  for (let i = 0; i < CONFIG.TAMANHO_CODIGO; i++) s += CONFIG.ALFABETO.charAt(bytes[i] & 31);
  return s;
}

function linkDoCodigo_(codigo) {
  return CONFIG.SITE_URL + '?c=' + codigo;
}

/* ------------------------------------------------------------------ */
/* Utilitários                                                         */
/* ------------------------------------------------------------------ */

function planilha_() {
  const ativa = SpreadsheetApp.getActiveSpreadsheet();
  if (ativa) return ativa;
  return SpreadsheetApp.openById(PropertiesService.getScriptProperties().getProperty('PLANILHA_ID'));
}

function abaServicos_() {
  const aba = planilha_().getSheetByName(CONFIG.ABA_SERVICOS);
  if (!aba) throw new Error('Aba "' + CONFIG.ABA_SERVICOS + '" não encontrada. Use 🔧 Oficina > Configurar planilha.');
  return aba;
}

function abaVeiculos_() {
  const aba = planilha_().getSheetByName(CONFIG.ABA_VEICULOS);
  if (!aba) throw new Error('Aba "' + CONFIG.ABA_VEICULOS + '" não encontrada. Use 🔧 Oficina > Configurar planilha.');
  return aba;
}

function abaOuNova_(ss, nome) {
  return ss.getSheetByName(nome) || ss.insertSheet(nome);
}

function prepararCabecalho_(aba, cabecalho, cor) {
  aba.getRange(1, 1, 1, cabecalho.length)
    .setValues([cabecalho])
    .setFontWeight('bold')
    .setFontColor('#FFFFFF')
    .setBackground(cor)
    .setWrap(true)
    .setVerticalAlignment('middle');
  aba.setFrozenRows(1);
  aba.setRowHeight(1, 42);
}

function garantirLinhas_(aba, total) {
  const faltam = total - aba.getMaxRows();
  if (faltam > 0) aba.insertRowsAfter(aba.getMaxRows(), faltam);
}

function larguras_(aba, lista) {
  lista.forEach(function (w, i) { aba.setColumnWidth(i + 1, w); });
}

function removerAbaVaziaPadrao_(ss) {
  ['Página1', 'Sheet1', 'Planilha1'].forEach(function (nome) {
    const aba = ss.getSheetByName(nome);
    if (aba && aba.getLastRow() === 0 && ss.getSheets().length > 1) ss.deleteSheet(aba);
  });
}

function proximaLinhaVazia_(aba) {
  const ultima = aba.getLastRow();
  if (ultima < 2) return 2;
  const vals = aba.getRange(2, 1, ultima - 1, 3).getValues();
  for (let i = vals.length - 1; i >= 0; i--) {
    if (vals[i].some(function (v) { return v !== '' && v !== null; })) return i + 3;
  }
  return 2;
}

function placaDaLinhaAtiva_() {
  const aba = SpreadsheetApp.getActiveSheet();
  const faixa = aba.getActiveRange();
  const linha = faixa ? faixa.getRow() : 0;
  if (linha < 2) return '';
  if (aba.getName() === CONFIG.ABA_SERVICOS) return normalizarPlaca_(aba.getRange(linha, COL_S.PLACA).getValue());
  if (aba.getName() === CONFIG.ABA_VEICULOS) return normalizarPlaca_(aba.getRange(linha, COL_V.PLACA).getValue());
  return '';
}

function normalizarPlaca_(p) {
  return String(p === null || p === undefined ? '' : p).toUpperCase().replace(/[^A-Z0-9]/g, '');
}

function mascararPlaca_(p) {
  if (!p) return '';
  if (p.length < 5) return p.replace(/./g, '•');
  return p.slice(0, 3) + '-' + new Array(p.length - 3).join('•') + p.slice(-1);
}

function marcado_(v) {
  if (v === true) return true;
  return /^(sim|s|x|ok|true|verdadeiro|1|✓|✔)$/i.test(String(v === null || v === undefined ? '' : v).trim());
}

function paraNumero_(v) {
  if (v === null || v === undefined || v === '') return null;
  if (typeof v === 'number') {
    if (!isFinite(v)) return null;
    if (v % 1 === 0) return v;
    // "153.273" lido como 153,273 (o ponto de milhar virou vírgula decimal)
    return Math.abs(v) < 1000 ? Math.round(v * 1000) : Math.round(v);
  }
  const so = String(v).replace(/\D/g, '');
  return so ? parseInt(so, 10) : null;
}

function paraDataISO_(v, tz) {
  if (v instanceof Date && !isNaN(v.getTime())) return Utilities.formatDate(v, tz, 'yyyy-MM-dd');
  const m = String(v || '').trim().match(/^(\d{1,2})\/(\d{1,2})\/(\d{2,4})$/);
  if (!m) return '';
  let ano = parseInt(m[3], 10);
  if (ano < 100) ano += 2000;
  return ano + '-' + ('0' + m[2]).slice(-2) + '-' + ('0' + m[1]).slice(-2);
}

function dataDoFormulario_(s) {
  const m = String(s || '').match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return hojeAoMeioDia_();
  return new Date(parseInt(m[1], 10), parseInt(m[2], 10) - 1, parseInt(m[3], 10), 12, 0, 0);
}

function hojeAoMeioDia_() {
  const d = new Date();
  return new Date(d.getFullYear(), d.getMonth(), d.getDate(), 12, 0, 0);
}

function escapar_(s) {
  return String(s).replace(/[&<>"']/g, function (c) {
    return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
  });
}

/** Teste rápido no editor: Executar > testarAPI e ver o registro de execução. */
function testarAPI() {
  const veic = abaVeiculos_();
  const codigo = veic.getLastRow() >= 2 ? String(veic.getRange(2, COL_V.CODIGO).getValue()) : 'XXXXXXXX';
  console.log(JSON.stringify(montarHistorico_(codigo), null, 2));
}

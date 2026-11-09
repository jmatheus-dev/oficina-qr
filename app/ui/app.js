'use strict';
/* Oficina QR — telas do programa (tudo em português, feito para usar rápido). */

const tela = document.getElementById('tela');
let ESTADO = { itens: [] };
let ATUALIZACAO = null;

/* ---------------------------------------------------------- ajudantes --- */
function el(tag, attrs, ...filhos) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === null || v === undefined || v === false) continue;
    if (k === 'class') n.className = v;
    else if (k.startsWith('on') && typeof v === 'function') n.addEventListener(k.slice(2), v);
    else if (k === 'value') n.value = v;
    else if (k === 'checked') n.checked = !!v;
    else n.setAttribute(k, v === true ? '' : v);
  }
  for (const f of filhos.flat(Infinity)) if (f !== null && f !== undefined && f !== false) n.append(f);
  return n;
}
const $ = (s, r = document) => r.querySelector(s);
const milhar = (n) => (n || n === 0) ? Number(n).toLocaleString('pt-BR') : '';
const hoje = () => { const d = new Date(); d.setMinutes(d.getMinutes() - d.getTimezoneOffset()); return d.toISOString().slice(0, 10); };
function dataBR(iso) { const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso || ''); return m ? `${m[3]}/${m[2]}/${m[1]}` : ''; }
function horaBR(iso) { if (!iso || iso.startsWith('0001')) return ''; const d = new Date(iso); return isNaN(d) ? '' : d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }); }
function quando(iso) {
  if (!iso || iso.startsWith('0001')) return '';
  const d = new Date(iso); if (isNaN(d)) return '';
  const h = new Date(); const ontem = new Date(h); ontem.setDate(h.getDate() - 1);
  if (d.toDateString() === h.toDateString()) return 'hoje às ' + horaBR(iso);
  if (d.toDateString() === ontem.toDateString()) return 'ontem às ' + horaBR(iso);
  return d.toLocaleDateString('pt-BR') + ' ' + horaBR(iso);
}
function soPlaca(s) { return String(s || '').toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 7); }
const placaValida = (p) => /^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/.test(p);
function placaBonita(p) { p = soPlaca(p); return p.length === 7 && /\d/.test(p[4]) ? p.slice(0, 3) + '-' + p.slice(3) : p; }
const numero = (v) => { const s = String(v ?? '').replace(/\D/g, ''); return s ? parseInt(s, 10) : 0; };

async function api(caminho, opcoes = {}) {
  const o = { method: opcoes.method || 'GET', headers: { 'X-Oficina': '1' } };
  if (opcoes.corpo !== undefined) { o.body = JSON.stringify(opcoes.corpo); o.headers['Content-Type'] = 'application/json'; }
  let r;
  try { r = await fetch('/api/' + caminho, o); } catch (e) { throw new Error('O programa não está respondendo. Feche e abra de novo.'); }
  const tipo = r.headers.get('Content-Type') || '';
  const dados = tipo.includes('json') ? await r.json() : await r.text();
  if (!r.ok) throw new Error((dados && dados.erro) || dados || ('Erro ' + r.status));
  return dados;
}

function aviso(texto, erro) {
  const a = el('div', { class: 'aviso' + (erro ? ' erro' : ''), role: erro ? 'alert' : 'status' }, texto);
  $('#avisos').append(a);
  setTimeout(() => a.remove(), erro ? 9000 : 4500);
}

async function ocupado(botao, fn) {
  const t = botao.textContent; botao.disabled = true; botao.textContent = 'Aguarde…';
  try { return await fn(); } finally { botao.disabled = false; botao.textContent = t; }
}

function placaEl(p, grande) { return el('span', { class: 'placa' + (grande ? ' grande' : '') }, placaBonita(p)); }

/* ------------------------------------------------------------ estado --- */
let pendentesAntes = null;
// aviso sonoro curto quando chega lançamento do Zeus sem placa
function bip() {
  try {
    const ctx = new (window.AudioContext || window.webkitAudioContext)();
    [0, 0.18].forEach((t) => {
      const o = ctx.createOscillator(), g = ctx.createGain();
      o.frequency.value = 880; g.gain.value = 0.08;
      o.connect(g); g.connect(ctx.destination);
      o.start(ctx.currentTime + t); o.stop(ctx.currentTime + t + 0.12);
    });
  } catch (e) { /* sem som, tudo bem */ }
}
async function atualizarEstado() {
  try { ESTADO = await api('estado'); } catch (e) {
    $('#estado').replaceChildren(el('span', { class: 'chip' }, el('span', { class: 'bola erro' }), e.message)); return;
  }
  const s = ESTADO.site || {}, z = ESTADO.zeus || {};
  const chips = [];
  document.body.classList.toggle('leitura', !!ESTADO.somente_leitura);
  if (ESTADO.somente_leitura) {
    chips.push(chip('', '🔒 Somente consulta — dados do computador principal' + (ESTADO.principal ? ' (' + ESTADO.principal + ')' : ''), '#/config'));
    if (ESTADO.erro_principal) {
      chips.push(chip('erro', ESTADO.erro_principal, '#/config'));
      $('#estado').replaceChildren(...chips);
      return;
    }
  }
  if (ESTADO.somente_leitura) { /* o site e a impressora são do principal */ }
  else if (!s.configurado) chips.push(chip('atencao', 'Site: falta ligar ao GitHub', '#/config'));
  else if (s.erro) chips.push(chip('erro', 'Site: erro ao publicar — ' + s.erro, '#/config'));
  else if (s.pendente || s.publicando) chips.push(chip('atencao', 'Site: atualizando…'));
  else chips.push(chip('ok', 'Site atualizado' + (s.ultimo && !s.ultimo.startsWith('0001') ? ' ' + quando(s.ultimo) : '')));
  if (!z.ativo) chips.push(chip('', 'Zeus: desligado', '#/config'));
  else if (z.erro) chips.push(chip('erro', 'Zeus: ' + z.erro, '#/config'));
  else chips.push(chip('ok', 'Zeus lido ' + (quando(z.ultima) || '…')));
  if (z.esperando) chips.push(chip('atencao', `${z.esperando} venda(s) sendo lançada(s) no Zeus`));
  if (z.pendentes) chips.push(chip('atencao', `${z.pendentes} serviço(s) do Zeus para revisar`, '#/'));
  const rev = $('#btn-revisar');
  if (rev) { rev.textContent = z.pendentes ? `Para revisar (${z.pendentes})` : 'Para revisar'; rev.classList.toggle('tem', !!z.pendentes); }
  if (ATUALIZACAO && ATUALIZACAO.nova) chips.push(chip('atencao', `Versão nova ${ATUALIZACAO.versao} disponível — clique para atualizar`, '#/config?atualizar=1'));
  if (pendentesAntes !== null && (z.pendentes || 0) > pendentesAntes) bip();
  pendentesAntes = z.pendentes || 0;
  if (ESTADO.somente_leitura) { /* impressão só no principal */ }
  else if (ESTADO.erro_impress) chips.push(chip('erro', 'Impressora: ' + ESTADO.erro_impress, '#/config'));
  else chips.push(chip(ESTADO.impressora ? 'ok' : 'atencao', 'Impressora: ' + (ESTADO.impressora || 'escolha em Configurações'), '#/config'));
  $('#estado').replaceChildren(...chips);
  document.title = (z.pendentes ? `(${z.pendentes}) ` : '') + 'Oficina QR';
}
function chip(tipo, texto, link) {
  const c = el('span', { class: 'chip' }, el('span', { class: 'bola ' + tipo }), link ? el('a', { href: link }, texto) : texto);
  return c;
}

/* ------------------------------------------------------------- busca --- */
(function busca() {
  const q = $('#q'), sug = $('#sugestoes');
  let t, ativo = -1, lista = [];
  async function procurar() {
    const v = q.value.trim();
    if (!v) { sug.hidden = true; return; }
    try { lista = await api('veiculos?q=' + encodeURIComponent(v)); } catch (e) { return; }
    ativo = -1;
    const p = soPlaca(v);
    const itens = lista.slice(0, 8).map((c) => el('a', { href: '#/carro/' + c.placa },
      placaEl(c.placa), el('span', {}, el('strong', {}, c.modelo || 'Modelo não informado'), c.ano ? ' ' + c.ano : ''),
      el('span', { class: 'cinza pequeno', style: 'margin-left:auto' }, c.servicos ? `${c.servicos} serviço(s)` : 'sem serviços')));
    if (placaValida(p) && !lista.some((c) => c.placa === p)) {
      itens.push(el('a', { href: '#/novo?placa=' + p }, el('strong', {}, '+ Novo serviço para '), placaEl(p)));
    }
    if (!itens.length) itens.push(el('div', { class: 'cinza', style: 'padding:12px 16px' }, 'Nenhum carro encontrado.'));
    sug.replaceChildren(...itens);
    sug.hidden = false;
  }
  q.addEventListener('input', () => { clearTimeout(t); t = setTimeout(procurar, 150); });
  q.addEventListener('keydown', (e) => {
    const links = [...sug.querySelectorAll('a')];
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      ativo = Math.max(0, Math.min(links.length - 1, ativo + (e.key === 'ArrowDown' ? 1 : -1)));
      links.forEach((a, i) => a.classList.toggle('ativo', i === ativo));
    } else if (e.key === 'Escape') { sug.hidden = true; }
  });
  $('#busca').addEventListener('submit', (e) => {
    e.preventDefault();
    const links = [...sug.querySelectorAll('a')];
    const alvo = links[ativo >= 0 ? ativo : 0];
    if (alvo) location.hash = alvo.getAttribute('href');
  });
  sug.addEventListener('click', () => { sug.hidden = true; q.value = ''; });
  document.addEventListener('click', (e) => { if (!$('#busca').contains(e.target)) sug.hidden = true; });
})();

/* ------------------------------------------------------------ rotas --- */
const rotas = [
  [/^#?\/?$/, telaInicio],
  [/^#\/novo(?:\?(.*))?$/, (m) => telaNovo(new URLSearchParams(m[1] || ''))],
  [/^#\/carro\/([A-Z0-9]+)(?:\?.*)?$/, (m) => telaCarro(m[1])],
  [/^#\/editar\/([A-Z0-9]+)(?:\?.*)?$/, (m) => telaEditar(m[1])],
  [/^#\/config(?:\?.*)?$/, telaConfig],
  [/^#\/revisar\/(.+)$/, (m) => telaRevisar(decodeURIComponent(m[1]))],
  [/^#\/lembretes$/, telaLembretes],
  [/^#\/relatorio(?:\?(.*))?$/, (m) => telaRelatorio(new URLSearchParams(m[1] || ''))],
  [/^#\/zeus$/, telaZeusAvancado],
  [/^#\/apagar(?:\?(.*))?$/, (m) => telaApagar(new URLSearchParams(m[1] || ''))],
  [/^#\/termos(?:\?(.*))?$/, (m) => telaTermos(new URLSearchParams(m[1] || ''))],
  [/^#\/termo\/novo(?:\?(.*))?$/, (m) => telaTermoForm(null, new URLSearchParams(m[1] || ''))],
  [/^#\/termo\/([A-Z0-9]+)\/editar$/, (m) => telaTermoForm(m[1], new URLSearchParams())],
  [/^#\/termo\/([A-Z0-9]+)(?:\?.*)?$/, (m) => telaTermo(m[1])],
];
let rotaAtual = '';
async function navegar() {
  const h = location.hash || '#/';
  rotaAtual = h;
  telaSucessoAberta = false;
  pararContagem();
  window.scrollTo(0, 0);
  if (ESTADO.somente_leitura && /^#\/(novo|editar|revisar|apagar|termo\/novo)|^#\/termo\/[A-Z0-9]+\/editar/.test(h)) {
    tela.replaceChildren(el('section', { class: 'cartao destaque' }, el('h1', {}, 'Este computador é só para consulta'),
      el('p', {}, 'Para registrar, corrigir, imprimir ou apagar, use o computador principal. Aqui dá para ver carros, histórico, termos, lembretes e relatório.'),
      el('a', { class: 'botao', href: '#/' }, 'Voltar ao início')));
    return;
  }
  for (const [re, f] of rotas) {
    const m = re.exec(h);
    if (m) {
      tela.replaceChildren(el('div', { class: 'carregando' }, 'Carregando…'));
      try { await f(m); } catch (e) { tela.replaceChildren(el('div', { class: 'cartao' }, el('h2', {}, 'Algo deu errado'), el('p', {}, e.message), el('a', { class: 'botao', href: '#/' }, 'Voltar ao início'))); }
      return;
    }
  }
  location.hash = '#/';
}
window.addEventListener('hashchange', navegar);
// links da internet abrem no navegador do computador (onde já tem login no GitHub etc.)
document.addEventListener('click', async (e) => {
  const a = e.target.closest && e.target.closest('a[href^="https://"]');
  if (!a) return;
  e.preventDefault();
  try { await api('abrir', { method: 'POST', corpo: { url: a.href } }); aviso('Abrindo no navegador…'); }
  catch (er) { window.open(a.href, '_blank', 'noopener'); }
});
// clicar num link para a tela em que já estamos recarrega a tela (ex.: "Registrar outro serviço")
document.addEventListener('click', (e) => {
  const a = e.target.closest && e.target.closest('a[href^="#"]');
  if (a && a.getAttribute('href') === location.hash && !e.defaultPrevented) { e.preventDefault(); navegar(); }
});

/* ------------------------------------------------------------ início --- */
async function telaInicio() {
  const [pend, recentes, hojeR] = await Promise.all([api('zeus/lancamentos?status=pendente'), api('servicos/recentes'), api('hoje').catch(() => null)]);
  const z = ESTADO.zeus || {};
  const leitura = !!ESTADO.somente_leitura;
  const num = (n, t, link) => el(link ? 'a' : 'div', { class: 'numero-grande' + (link ? ' link' : ''), href: link || null }, el('strong', {}, String(n)), el('span', {}, t));
  const resumo = hojeR ? el('section', { class: 'cartao resumo-dia' }, el('h2', {}, 'Hoje'),
    el('div', { class: 'numeros' }, num(hojeR.servicos, 'serviço(s) registrado(s)'), num(hojeR.impressos, 'com etiqueta impressa'),
      num(hojeR.carros_novos, 'carro(s) novo(s)'), num(hojeR.pendentes, 'para revisar', '#/'),
      num(hojeR.termos_sem_assinatura, 'termo(s) sem assinatura', '#/termos'))) : null;

  const colZeus = el('section', { class: 'cartao destaque' },
    el('div', { class: 'linha-titulo' }, el('h2', {}, `Para revisar${pend.length ? ` (${pend.length})` : ''}`),
      z.ativo ? el('button', { class: 'botao leve pequeno escrita', onclick: async (e) => {
        await ocupado(e.target, async () => {
          try { const r = await api('zeus/ler', { method: 'POST' }); aviso(r.novos ? `${r.novos} serviço(s) novo(s) do Zeus` : 'Nada novo no Zeus'); navegar(); } catch (er) { aviso(er.message, true); }
        });
      } }, 'Ler o Zeus agora') : null));
  if (leitura) {
    colZeus.append(el('p', { class: 'cinza pequeno' }, 'Quem revisa e imprime é o computador principal. Aqui só aparece o que está esperando lá.'));
    if (!pend.length) colZeus.append(el('p', { class: 'vazio' }, 'Nada esperando revisão no computador principal.'));
    else pend.forEach((l) => colZeus.append(cartaoPendente(l)));
  } else if (!z.ativo) {
    colZeus.append(el('p', { class: 'vazio' }, 'A leitura do Zeus está desligada. Quando estiver ligada, cada venda paga no Zeus aparece aqui para você conferir antes de imprimir a etiqueta. ',
      el('a', { href: '#/config' }, 'Ligar em Configurações')));
  } else if (!pend.length) {
    colZeus.append(el('p', { class: 'vazio' }, 'Nada para revisar. Cada venda paga no Zeus aparece aqui: você confere, corrige se precisar e só então imprime a etiqueta.'));
  } else {
    colZeus.append(el('p', { class: 'cinza pequeno' }, 'Nada é impresso sozinho. Abra cada um, confira placa, KM e peças, e imprima.'));
    pend.forEach((l) => colZeus.append(cartaoPendente(l)));
  }
  if (!leitura) colZeus.append(el('p', { class: 'pequeno', style: 'margin:10px 0 0' }, el('a', { href: '#/', onclick: (e) => { e.preventDefault(); mostrarIgnorados(colZeus); } }, 'Ver lançamentos ignorados')));

  const colRec = el('section', { class: 'cartao' },
    el('div', { class: 'linha-titulo' }, el('h2', {}, 'Últimos serviços'),
      el('span', { style: 'display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end' },
        recentes.length ? el('a', { class: 'botao leve pequeno', href: '#/apagar', title: 'Escolher serviços ou carros salvos errado e apagar' }, '🗑 Selecionar e apagar') : null,
        el('a', { class: 'botao vermelho pequeno', href: '#/novo' }, '+ Novo serviço'))));
  if (!recentes.length) {
    colRec.append(el('div', { class: 'vazio' }, el('p', {}, 'Nenhum serviço registrado ainda.'),
      el('a', { class: 'botao vermelho grande', href: '#/novo' }, 'Registrar o primeiro serviço')));
  } else {
    colRec.append(el('ul', { class: 'lista' }, recentes.map((s) => el('li', {},
      placaEl(s.placa),
      el('a', { class: 'meio', href: '#/carro/' + s.placa, style: 'color:inherit;text-decoration:none' },
        el('div', { class: 'titulo' }, s.modelo || 'Modelo não informado', ' ', el('span', { class: 'cinza pequeno' }, '· ' + dataBR(s.data) + (s.km ? ' · ' + milhar(s.km) + ' km' : ''))),
        el('div', { class: 'linha2' }, [...s.itens, ...s.outros].join(', ') || (s.proxima_troca_km ? 'Próxima troca ' + milhar(s.proxima_troca_km) + ' km' : ''))),
      s.origem === 'zeus' ? el('span', { class: 'etiq zeus', title: 'Veio do Zeus' }, 'Zeus') : null,
      el('button', { class: 'botao leve pequeno escrita', title: 'Imprimir a etiqueta de novo', onclick: (e) => imprimir(e.target, s.id) }, 'Imprimir')))));
  }
  const falta = [];
  if (!ESTADO.impressora) falta.push(['Escolher a impressora de etiquetas e imprimir um teste', '#/config']);
  if (!(ESTADO.site || {}).configurado) falta.push(['Ligar o site (colar o token do GitHub)', '#/config']);
  if (!z.ativo) falta.push(['Ligar a leitura do Zeus', '#/config']);
  const passos = falta.length ? el('section', { class: 'cartao', style: 'border-left:6px solid #E8A100' },
    el('h2', {}, 'Para terminar de configurar'),
    el('ol', { class: 'passos', style: 'margin-bottom:0' }, falta.map(([t, h]) => el('li', {}, el('a', { href: h }, t))))) : null;
  tela.replaceChildren(leitura ? '' : (passos || ''), resumo || '', el('div', { class: 'colunas' }, colRec, colZeus));
}

async function mostrarIgnorados(onde) {
  const l = await api('zeus/lancamentos?status=ignorado');
  const caixa = el('div', { style: 'margin-top:12px' }, el('h3', {}, 'Ignorados'));
  if (!l.length) caixa.append(el('p', { class: 'cinza' }, 'Nenhum.'));
  l.forEach((x) => caixa.append(el('div', { class: 'pendente', style: 'border-left-color:#999' },
    el('div', { class: 'cab' }, el('strong', {}, x.documento), el('span', { class: 'cinza' }, dataBR(x.data))),
    el('div', { class: 'produtos' }, x.produtos.join(' · ')),
    el('button', { class: 'botao leve pequeno', onclick: async () => { naoRedirecionar.add(x.ref); await api(`zeus/lancamentos/${encodeURIComponent(x.ref)}/voltar`, { method: 'POST' }); navegar(); } }, 'Voltar para a lista'))));
  onde.append(caixa);
}

// observação dizendo que o serviço não foi feito / foi cancelado
const reNaoFeito = /n[aã]o\s+(foi\s+)?(finaliz|feito|realiz|autoriz|aprov|fez)|cancel|desist|devolv|s[oó]\s+or[cç]amento|or[cç]amento\s+feito/i;
function diasAtras(iso) { const d = new Date(iso + 'T12:00:00'); return isNaN(d) ? 0 : Math.round((Date.now() - d) / 864e5); }

function cartaoPendente(l) {
  const carro = [l.modelo, l.ano].filter(Boolean).join(' ');
  const velho = diasAtras(l.data) > 30;
  return el('div', { class: 'pendente' },
    el('div', { class: 'cab' }, el('strong', {}, l.documento), el('span', { class: 'cinza' }, dataBR(l.data), l.cliente ? ' · ' + l.cliente : '')),
    el('div', { class: 'carro-linha' }, l.placa ? placaEl(l.placa) : el('span', { class: 'etiq aviso-etiq' }, 'sem placa na observação'),
      el('strong', {}, carro || ''), l.km ? el('span', { class: 'cinza' }, ' · ' + milhar(l.km) + ' km') : null),
    velho ? el('div', { class: 'alerta-data' }, `⚠ Venda de ${dataBR(l.data)} (há ${diasAtras(l.data)} dias)`) : null,
    reNaoFeito.test(l.texto || '') ? el('div', { class: 'alerta-data' }, '⚠ A observação diz que o serviço pode não ter sido feito. Confira antes de confirmar.') : null,
    l.servico_id ? el('div', { class: 'alerta-data' }, '↻ Esta venda mudou no Zeus depois de confirmada.') : null,
    l.so_mao_de_obra ? el('div', { class: 'alerta-data' }, '🔧 Só mão de obra nesta venda. O cliente trouxe a peça? ',
      el('a', { class: 'escrita', href: '#/termo/novo?ref=' + encodeURIComponent(l.ref) }, 'Fazer o termo de peça do cliente')) : null,
    el('div', {}, l.itens.map((i) => el('span', { class: 'etiq padrao' }, i)), l.outros.map((o) => el('span', { class: 'etiq' }, o))),
    l.texto ? el('div', { class: 'produtos' }, 'Obs. no Zeus: ', l.texto) : null,
    el('div', { class: 'acoes' },
      el('a', { class: 'botao vermelho', href: '#/revisar/' + encodeURIComponent(l.ref) }, 'Revisar e imprimir'),
      el('button', { class: 'link-botao escrita', onclick: async () => {
        if (!confirm('Ignorar este lançamento? Ele não vai para o histórico de nenhum carro.')) return;
        await api(`zeus/lancamentos/${encodeURIComponent(l.ref)}/ignorar`, { method: 'POST' }); atualizarEstado(); navegar();
      } }, 'Ignorar')));
}

/* --------------------------------------------------- revisar do Zeus --- */
async function telaRevisar(ref) {
  let l = null;
  try { l = await api('zeus/lancamento/' + encodeURIComponent(ref)); } catch (e) { /* sumiu */ }
  if (!l || l.status !== 'pendente') { aviso('Esse lançamento já foi revisado ou ignorado.'); location.hash = '#/'; return; }
  let carroCad = null;
  if (l.placa) { try { carroCad = (await api('veiculos/' + l.placa + '?opcional=1')).veiculo; } catch (e) { /* novo */ } }
  let doCliente = [];
  if (l.cliente) { try { doCliente = (await api('veiculos?q=' + encodeURIComponent(l.cliente))).filter((v) => (v.cliente || '').toUpperCase() === l.cliente.toUpperCase()); } catch (e) { /* nada */ } }
  const inicial = {
    placa: l.placa || '', modelo: (carroCad && carroCad.modelo) || l.modelo || '', ano: (carroCad && carroCad.ano) || l.ano || '',
    data: l.data, km: l.km || '', itens: l.itens, outros: l.outros,
    obs: l.oleo || '', // marca do óleo que saiu no Zeus
    obs_interna: `${l.documento}${l.cliente ? ' · ' + l.cliente : ''}: ${l.produtos.join('; ')}`,
  };
  const antes = (f, previa) => {
    const caixa = el('div', { class: 'zeus-info' },
      el('div', {}, el('strong', {}, l.documento), ' · ', dataBR(l.data), l.cliente ? ' · ' + l.cliente : '', l.telefone ? ' · ' + l.telefone : ''),
      l.texto ? el('div', {}, 'Observação no Zeus: ', el('em', {}, l.texto)) : null,
      l.oleo ? el('div', {}, '🛢 Óleo da venda (já está na observação para o cliente): ', el('strong', {}, l.oleo)) : null,
      el('details', {}, el('summary', {}, `Como está no Zeus (${l.produtos.length} itens)`), el('ul', { class: 'pequeno' }, l.produtos.map((p) => el('li', {}, p)))));
    const dias = diasAtras(l.data);
    if (dias > 30) {
      caixa.append(el('div', { class: 'alerta-data' }, `⚠ A data da venda é ${dataBR(l.data)} (há ${dias} dias). Confira se é isso mesmo. `,
        el('button', { type: 'button', class: 'botao leve pequeno', onclick: () => { f.data.value = hoje(); previa(); } }, 'Usar a data de hoje')));
    }
    if (!l.placa) caixa.append(el('div', { class: 'alerta-data' }, '⚠ A observação no Zeus não tem placa. Digite a placa abaixo.'));
    if (reNaoFeito.test(l.texto || '')) caixa.append(el('div', { class: 'alerta-data' }, '⚠ A observação diz que o serviço pode não ter sido feito ou foi cancelado. Se não foi feito, clique em “Ignorar”.'));
    if (l.so_mao_de_obra) caixa.append(el('div', { class: 'alerta-data' }, '🔧 Esta venda só tem mão de obra: escreva abaixo, em “Outros serviços / peças”, o que foi feito. Se o cliente trouxe a peça, ',
      el('a', { href: '#/termo/novo?ref=' + encodeURIComponent(l.ref) }, 'faça o termo de peça do cliente'), ' (a garantia fica só da mão de obra).'));
    if (l.servico_id) caixa.append(el('div', { class: 'alerta-data' }, '↻ Esta venda já tinha sido confirmada e mudou no Zeus (ex.: ganhou peça). O que você confirmar agora é somado ao serviço que já existe; a etiqueta só sai se você pedir.'));
    if (doCliente.length) {
      caixa.append(el('div', { class: 'carros-cliente' }, el('span', { class: 'rotulo' }, 'Carros deste cliente (clique para escolher):'),
        doCliente.map((v) => el('button', { type: 'button', class: 'botao leve pequeno', onclick: () => {
          f.placa.value = placaBonita(v.placa); f.placa.dispatchEvent(new Event('input')); f.modelo.value = v.modelo || f.modelo.value; f.ano.value = v.ano || f.ano.value; previa();
        } }, placaBonita(v.placa), ' ', v.modelo || ''))));
    }
    return caixa;
  };
  tela.replaceChildren(formularioServico(inicial, async (d) => {
    const r = await api(`zeus/lancamentos/${encodeURIComponent(l.ref)}/registrar`, { method: 'POST', corpo: d });
    telaSucesso(r, d.imprimir, true);
    atualizarEstado();
  }, { titulo: 'Revisar serviço do Zeus', textoImprimir: 'Confirmar e imprimir etiqueta', textoSalvar: 'Confirmar sem imprimir', antes,
    extra: el('span', {}, el('a', { class: 'link-botao', href: '#/' }, 'Voltar'), ' · ',
      el('a', { class: 'link-botao', href: '#/termo/novo?ref=' + encodeURIComponent(l.ref), title: 'O cliente comprou a peça por conta própria' }, 'Cliente trouxe peça? Fazer termo'), ' · ',
      el('button', { type: 'button', class: 'link-botao', onclick: async () => {
        if (!confirm('Ignorar este lançamento? Ele não vai para o histórico de nenhum carro.')) return;
        await api(`zeus/lancamentos/${encodeURIComponent(l.ref)}/ignorar`, { method: 'POST' }); atualizarEstado(); location.hash = '#/';
      } }, 'Ignorar')) }));
}

async function imprimir(botao, id, modelo, copias) {
  await ocupado(botao, async () => {
    try { await api('imprimir', { method: 'POST', corpo: { servico: id, modelo: modelo || '', copias: copias || 0 } }); aviso('Etiqueta enviada para a impressora'); }
    catch (e) { aviso('A etiqueta não saiu: ' + e.message, true); }
    atualizarEstado();
  });
}

/* ------------------------------------------------------- novo serviço --- */
function formularioServico(inicial, aoSalvar, opcoes = {}) {
  const v = Object.assign({ placa: '', modelo: '', ano: '', data: hoje(), km: '', itens: [], outros: [], proxima_troca_km: '', obs: '', obs_interna: '' }, inicial);
  const f = {};
  f.placa = el('input', { type: 'text', class: 'placa-campo', id: 'f-placa', maxlength: 8, placeholder: 'ABC1D23', value: placaBonita(v.placa), autocomplete: 'off', required: true });
  f.modelo = el('input', { type: 'text', id: 'f-modelo', value: v.modelo, placeholder: 'Ex.: Toyota Corolla' });
  f.ano = el('input', { type: 'text', id: 'f-ano', value: v.ano, inputmode: 'numeric', maxlength: 9, placeholder: '2019' });
  f.data = el('input', { type: 'date', id: 'f-data', value: v.data, max: hoje() });
  f.km = el('input', { type: 'text', id: 'f-km', inputmode: 'numeric', value: v.km ? milhar(v.km) : '', placeholder: 'Ex.: 153.273' });
  f.prox = el('input', { type: 'text', id: 'f-prox', inputmode: 'numeric', value: v.proxima_troca_km ? milhar(v.proxima_troca_km) : '', placeholder: 'Ex.: 163.273' });
  f.outros = el('textarea', { id: 'f-outros', placeholder: 'Uma coisa por linha. Ex.:\nMangueira de ar\nPastilhas de freio dianteiras' }, (v.outros || []).join('\n'));
  f.obs = el('input', { type: 'text', id: 'f-obs', value: v.obs, placeholder: 'Ex.: Óleo 5W30 sintético' });
  // correia dentada: aparece sozinho quando o serviço tem correia dentada
  f.cKM = el('input', { type: 'text', id: 'f-ckm', inputmode: 'numeric', value: v.proxima_correia_km ? milhar(v.proxima_correia_km) : '', placeholder: 'Ex.: 354.669' });
  f.cData = el('input', { type: 'date', id: 'f-cdata', value: v.proxima_correia_data || '' });
  let correiaMexida = !!(v.proxima_correia_km || v.proxima_correia_data);
  const caixaCorreia = el('div', { class: 'campo correia-caixa', hidden: true },
    el('span', { class: 'rotulo' }, '⚙ Correia dentada — próxima troca'),
    el('div', { class: 'campos' },
      el('div', { class: 'campo c6' }, el('label', { for: 'f-ckm' }, 'KM da próxima troca'), f.cKM),
      el('div', { class: 'campo c6' }, el('label', { for: 'f-cdata' }, 'Data da próxima troca'), f.cData)),
    el('span', { class: 'ajuda' }, 'Preenchido sozinho com o padrão (Configurações > Etiqueta). Pode mudar.'));
  const nomeModelo = el('p', { class: 'modelo-etiqueta', 'aria-live': 'polite' });
  let modeloAtual = 'oleo';
  function padraoCorreia(forcar) {
    if (correiaMexida && !forcar) return;
    const c = ESTADO.correia || { km: 40000, meses: 12 };
    const km = numero(f.km.value);
    f.cKM.value = km ? milhar(km + c.km) : '';
    const d = new Date((f.data.value || hoje()) + 'T12:00:00');
    if (!isNaN(d)) { d.setMonth(d.getMonth() + c.meses); f.cData.value = d.toISOString().slice(0, 10); }
  }
  f.cKM.addEventListener('input', () => { correiaMexida = true; const n = numero(f.cKM.value); f.cKM.value = n ? milhar(n) : ''; previa(); });
  f.cData.addEventListener('input', () => { correiaMexida = true; previa(); });
  const adicionar = (nome) => () => {
    const linhas = f.outros.value.split('\n').map((s) => s.trim()).filter(Boolean);
    if (!linhas.some((l) => l.toLowerCase() === nome.toLowerCase())) linhas.push(nome);
    f.outros.value = linhas.join('\n'); previa();
  };
  const atalhosCorreia = el('div', { class: 'somas' }, ['Correia dentada', 'Tensor', "Bomba d'água"].map((n) =>
    el('button', { type: 'button', class: 'botao leve pequeno', onclick: adicionar(n) }, '+ ' + n)));
  f.obsInt = el('input', { type: 'text', id: 'f-obsint', value: v.obs_interna, placeholder: 'Só a oficina vê' });
  const infoCarro = el('div', { 'aria-live': 'polite' });
  const erroPlaca = el('div', { class: 'erro-campo' });
  const marcados = new Set(v.itens || []);
  const itens = el('div', { class: 'itens' }, (ESTADO.itens || []).map((nome) => {
    const cb = el('input', { type: 'checkbox', value: nome, checked: marcados.has(nome) });
    const lab = el('label', { class: 'item' + (marcados.has(nome) ? ' marcado' : '') }, cb, el('span', { class: 'caixa' }), nome);
    cb.addEventListener('change', () => { lab.classList.toggle('marcado', cb.checked); previa(); });
    return lab;
  }));
  // data da próxima troca de óleo: data do serviço + meses (Configurações), se trocou o óleo
  f.proxData = el('input', { type: 'date', id: 'f-proxdata', value: v.proxima_troca_data || '' });
  let proxDataMexida = !!v.proxima_troca_data;
  f.proxData.addEventListener('input', () => { proxDataMexida = true; previa(); });
  const temOleo = () => !!itens.querySelector('input[value="Óleo do motor"]:checked');
  function autoProxData() {
    if (proxDataMexida) return;
    if (!temOleo()) { f.proxData.value = ''; return; }
    const d = new Date((f.data.value || hoje()) + 'T12:00:00');
    if (isNaN(d)) return;
    d.setMonth(d.getMonth() + (ESTADO.meses_oleo || 6));
    f.proxData.value = d.toISOString().slice(0, 10);
  }
  itens.addEventListener('change', () => { autoProxData(); previa(); });
  f.data.addEventListener('input', autoProxData);
  const marcar = (nomes) => nomes.forEach((n) => {
    const cb = itens.querySelector(`input[value="${n}"]`);
    if (cb && !cb.checked) { cb.checked = true; cb.dispatchEvent(new Event('change', { bubbles: true })); }
  });
  const atalhos = el('div', { class: 'somas atalhos' },
    el('button', { type: 'button', class: 'botao leve pequeno', title: 'Marca óleo do motor e filtro de óleo e calcula a próxima troca', onclick: () => {
      marcar(['Óleo do motor', 'Filtro de óleo']);
      const k = numero(f.km.value);
      if (k && !numero(f.prox.value)) f.prox.value = milhar(k + 10000);
      autoProxData(); previa();
    } }, '⚡ Troca de óleo completa'),
    el('button', { type: 'button', class: 'botao leve pequeno', title: 'Correia dentada, tensor e bomba d\'água', onclick: () => {
      ['Correia dentada', 'Tensor', "Bomba d'água"].forEach((n) => adicionar(n)());
    } }, '⚡ Kit correia completo'));
  const somar = (n) => () => { const k = numero(f.km.value); if (!k) { aviso('Preencha o KM atual primeiro.', true); f.km.focus(); return; } f.prox.value = milhar(k + n); previa(); };

  const img = el('img', { class: 'etiqueta', alt: 'Prévia da etiqueta' });

  function coletar() {
    return {
      placa: soPlaca(f.placa.value), modelo: f.modelo.value.trim(), ano: f.ano.value.trim(), data: f.data.value || hoje(),
      km: numero(f.km.value), itens: [...itens.querySelectorAll('input:checked')].map((c) => c.value),
      outros: f.outros.value.split('\n').map((s) => s.trim()).filter(Boolean), proxima_troca_km: numero(f.prox.value),
      obs: f.obs.value.trim(), obs_interna: f.obsInt.value.trim(),
      proxima_troca_data: f.proxData.value || '',
      proxima_correia_km: caixaCorreia.hidden ? 0 : numero(f.cKM.value), proxima_correia_data: caixaCorreia.hidden ? '' : f.cData.value,
    };
  }
  let tPrev, numPrevia = 0;
  function previa() {
    clearTimeout(tPrev);
    tPrev = setTimeout(() => {
      const d = coletar();
      const q = new URLSearchParams({ placa: d.placa, data: d.data, km: d.km || '', proxima: d.proxima_troca_km || '', modelo: d.modelo, proxima_data: d.proxima_troca_data || '',
        proxima_correia_km: d.proxima_correia_km || '', proxima_correia_data: d.proxima_correia_data || '' });
      d.itens.forEach((i) => q.append('item', i)); d.outros.forEach((o) => q.append('outro', o));
      const pedido = ++numPrevia;
      fetch('/api/etiqueta.png?' + q).then(async (r) => {
        if (!r.ok || pedido !== numPrevia) return;
        const m = r.headers.get('X-Modelo') || 'oleo';
        const blob = await r.blob();
        if (pedido !== numPrevia) return;
        if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src);
        img.src = URL.createObjectURL(blob);
        nomeModelo.textContent = 'Etiqueta escolhida sozinha: ' + decodeURIComponent((r.headers.get('X-Modelo-Nome') || '').replace(/\+/g, ' '));
        const temCorreia = m !== 'oleo';
        if (temCorreia && caixaCorreia.hidden) { caixaCorreia.hidden = false; if (!f.cKM.value && !f.cData.value) padraoCorreia(true); previa(); }
        else if (!temCorreia) caixaCorreia.hidden = true;
        modeloAtual = m;
      }).catch(() => {});
    }, 250);
  }
  let ultimaPlaca = '';
  async function conferirPlaca() {
    const p = soPlaca(f.placa.value);
    erroPlaca.textContent = '';
    if (p === ultimaPlaca) return;
    ultimaPlaca = p;
    infoCarro.replaceChildren();
    if (!placaValida(p)) return;
    try {
      const c = await api('veiculos/' + p + '?opcional=1');
      if (!c.veiculo) throw new Error('novo');
      if (!f.modelo.value) f.modelo.value = c.veiculo.modelo || '';
      if (!f.ano.value) f.ano.value = c.veiculo.ano || '';
      const ult = c.servicos[0];
      infoCarro.replaceChildren(el('div', { class: 'encontrado' }, el('strong', {}, 'Carro já cadastrado. '),
        `${c.servicos.length} serviço(s)` + (ult ? `, o último em ${dataBR(ult.data)}${ult.km ? ' com ' + milhar(ult.km) + ' km' : ''}.` : '.'),
        ' ', el('a', { href: '#/carro/' + p, target: '_self' }, 'Ver histórico')));
    } catch (e) {
      infoCarro.replaceChildren(el('div', { class: 'ajuda' }, 'Carro novo: vai ganhar um código de QR na hora de salvar.'));
    }
    previa();
  }
  f.placa.addEventListener('input', () => { const pos = f.placa.value; f.placa.value = soPlaca(pos); conferirPlaca(); previa(); });
  f.placa.addEventListener('blur', () => { f.placa.value = placaBonita(f.placa.value); });
  f.placa.addEventListener('focus', () => { f.placa.value = soPlaca(f.placa.value); });
  for (const c of [f.km, f.prox]) c.addEventListener('input', () => { const n = numero(c.value); c.value = n ? milhar(n) : ''; if (c === f.km && !caixaCorreia.hidden) padraoCorreia(); previa(); });
  f.data.addEventListener('input', () => { if (!caixaCorreia.hidden) padraoCorreia(); });
  for (const c of [f.modelo, f.data, f.outros, f.ano]) c.addEventListener('input', previa);

  async function salvar(botao, imprimirJa) {
    const d = coletar();
    if (!placaValida(d.placa)) { erroPlaca.textContent = 'Placa inválida. Use o formato ABC1D23 ou ABC1234.'; f.placa.focus(); return; }
    if (!d.itens.length && !d.outros.length && !d.proxima_troca_km) { aviso('Marque o que foi trocado ou escreva o serviço feito.', true); return; }
    if (d.proxima_troca_km && d.km && d.proxima_troca_km <= d.km) { aviso('A próxima troca precisa ser maior que o KM atual.', true); f.prox.focus(); return; }
    await ocupado(botao, async () => {
      try { await aoSalvar(Object.assign(d, { imprimir: imprimirJa, modelo_etiqueta: modeloAtual })); } catch (e) { aviso(e.message, true); }
    });
  }
  const btnSalvarImp = el('button', { type: 'button', class: 'botao vermelho grande', 'data-principal': '1', title: 'Atalho: Ctrl+Enter', onclick: (e) => salvar(e.target, true) }, opcoes.textoImprimir || 'Salvar e imprimir etiqueta');
  const btnSalvar = el('button', { type: 'button', class: 'botao leve', onclick: (e) => salvar(e.target, false) }, opcoes.textoSalvar || 'Só salvar');

  const form = el('div', { class: 'colunas' },
    el('section', { class: 'cartao destaque' },
      el('h1', {}, opcoes.titulo || 'Novo serviço'),
      opcoes.antes ? opcoes.antes(f, previa) : null,
      el('div', { class: 'campos' },
        el('div', { class: 'campo c5' }, el('label', { for: 'f-placa' }, 'Placa'), f.placa, erroPlaca),
        el('div', { class: 'campo c4' }, el('label', { for: 'f-modelo' }, 'Modelo'), f.modelo),
        el('div', { class: 'campo c3' }, el('label', { for: 'f-ano' }, 'Ano'), f.ano),
        el('div', { class: 'campo' }, infoCarro),
        el('div', { class: 'campo c6' }, el('label', { for: 'f-data' }, 'Data do serviço'), f.data),
        el('div', { class: 'campo c6' }, el('label', { for: 'f-km' }, 'KM atual'), f.km),
        el('div', { class: 'campo' }, el('span', { class: 'rotulo' }, 'O que foi trocado'), atalhos, itens),
        el('div', { class: 'campo' }, el('label', { for: 'f-outros' }, 'Outros serviços / peças'), f.outros, atalhosCorreia),
        caixaCorreia,
        el('div', { class: 'campo c6' }, el('label', { for: 'f-prox' }, 'Próxima troca de óleo (KM)'), f.prox,
          el('div', { class: 'somas' }, [5000, 7000, 10000].map((n) => el('button', { type: 'button', class: 'botao leve pequeno', onclick: somar(n) }, `+${n / 1000} mil`)))),
        el('div', { class: 'campo c6' }, el('label', { for: 'f-proxdata' }, 'Próxima troca de óleo (data)'), f.proxData,
          el('span', { class: 'ajuda' }, `Preenchida sozinha: data + ${ESTADO.meses_oleo || 6} meses. O que vier primeiro, KM ou data.`)),
        el('div', { class: 'campo' }, el('label', { for: 'f-obs' }, 'Observação para o cliente'), f.obs, el('span', { class: 'ajuda' }, 'Aparece na página do QR Code.')),
        el('div', { class: 'campo' }, el('label', { for: 'f-obsint' }, 'Observação interna'), f.obsInt, el('span', { class: 'ajuda' }, 'Não aparece para o cliente.'))),
      el('div', { class: 'acoes' }, btnSalvarImp, btnSalvar, opcoes.extra || null)),
    el('aside', { class: 'cartao previa' }, el('h2', {}, 'Etiqueta'), img, nomeModelo,
      el('p', { class: 'ajuda', style: 'text-align:center' }, 'Só troca de óleo → etiqueta de troca de óleo. Só correia dentada → etiqueta da correia. As duas → uma etiqueta só com as duas. As peças e serviços aparecem para o cliente ao escanear o QR Code.')));
  setTimeout(() => { (v.placa ? f.km : f.placa).focus(); autoProxData(); conferirPlaca(); previa(); }, 0);
  return form;
}

async function telaNovo(params) {
  const placa = soPlaca(params.get('placa') || '');
  tela.replaceChildren(formularioServico({ placa }, async (d) => {
    const r = await api('servicos', { method: 'POST', corpo: d });
    telaSucesso(r, d.imprimir);
    atualizarEstado();
  }));
}

// WhatsApp para o cliente com o link do histórico (sem telefone: escolhe o contato no WhatsApp)
function whatsCliente(v, link) {
  let tel = String(v.telefone || '').replace(/\D/g, '');
  if (tel.length >= 10 && !tel.startsWith('55')) tel = '55' + tel;
  if (tel.length < 12) tel = '';
  const nome = String(v.cliente || '').trim().split(/\s+/)[0] || '';
  const Nome = nome ? nome[0].toUpperCase() + nome.slice(1).toLowerCase() : '';
  const msg = `Olá${Nome ? ', ' + Nome : ''}! Aqui é da Auto Mecânica Itabaiana. O serviço do seu ${v.modelo || 'carro'} (${placaBonita(v.placa)}) está registrado. ` +
    `Pelo link você vê o histórico, a garantia e a próxima troca: ${link}`;
  return el('a', { class: 'botao verde', href: `https://wa.me/${tel}?text=${encodeURIComponent(msg)}`, title: tel ? 'Abre o WhatsApp com a mensagem pronta' : 'Sem telefone cadastrado: escolha o contato no WhatsApp' },
    '📲 Enviar ao cliente pelo WhatsApp');
}

function telaSucesso(r, pediuImpressao, doZeus) {
  const sv = r.servico, v = r.veiculo;
  telaSucessoAberta = true;
  if (doZeus) setTimeout(proximoDoZeus, 300);
  if (pediuImpressao && r.erro_impressao) aviso('Salvo, mas a etiqueta não saiu: ' + r.erro_impressao, true);
  else aviso(r.impresso ? 'Serviço salvo e etiqueta impressa' : 'Serviço salvo');
  tela.replaceChildren(el('div', { class: 'colunas' },
    el('section', { class: 'cartao destaque sucesso' },
      el('h1', {}, r.impresso ? 'Pronto! Etiqueta impressa.' : 'Serviço salvo.'),
      el('p', {}, placaEl(v.placa, true)),
      el('p', { class: 'cinza' }, `${v.modelo || ''} ${v.ano || ''}`),
      el('p', {}, 'Código do QR deste carro:'),
      el('p', { class: 'codigo grande-codigo' }, v.codigo),
      el('p', { class: 'pequeno cinza' }, 'O histórico no site atualiza sozinho em até 1 minuto.'),
      el('div', { class: 'acoes', style: 'justify-content:center' },
        el('button', { class: 'botao preto', onclick: (e) => imprimir(e.target, sv.id) }, r.impresso ? 'Imprimir de novo' : 'Imprimir etiqueta'),
        r.link ? whatsCliente(v, r.link) : null,
        el('a', { class: 'botao leve', href: '#/carro/' + v.placa }, 'Ver histórico do carro'),
        el('a', { class: 'botao vermelho', href: '#/novo' }, 'Registrar outro serviço'))),
    el('aside', { class: 'cartao' }, el('h2', {}, 'Etiqueta'),
      el('img', { class: 'etiqueta', alt: 'Etiqueta', src: `/api/etiqueta.png?servico=${sv.id}&tipo=${r.modelo_etiqueta || ''}&t=${Date.now()}` }))));
}

async function telaEditar(id) {
  // procura o serviço pelo id em todos os carros recentes não é preciso: o servidor tem o carro
  const lista = await api('servicos/recentes');
  let s = lista.find((x) => x.id === id);
  if (!s) {
    const placa = new URLSearchParams(location.hash.split('?')[1] || '').get('placa');
    if (placa) { const c = await api('veiculos/' + placa); s = c.servicos.find((x) => x.id === id); }
  }
  if (!s) throw new Error('Serviço não encontrado.');
  const c = await api('veiculos/' + s.placa);
  tela.replaceChildren(formularioServico(Object.assign({}, s, { modelo: c.veiculo.modelo, ano: c.veiculo.ano }), async (d) => {
    const r = await api('servicos/' + id, { method: 'PUT', corpo: d });
    aviso('Alterações salvas');
    if (r.erro_impressao) aviso('A etiqueta não saiu: ' + r.erro_impressao, true);
    location.hash = '#/carro/' + r.servico.placa;
  }, { titulo: 'Corrigir serviço', textoImprimir: 'Salvar e imprimir de novo', textoSalvar: 'Salvar alterações',
    extra: el('a', { class: 'link-botao', href: '#/carro/' + s.placa }, 'Cancelar') }));
}

/* ------------------------------------------------------------- carro --- */
async function telaCarro(placa) {
  const [c, termos] = await Promise.all([api('veiculos/' + placa), api('termos?placa=' + encodeURIComponent(placa)).catch(() => [])]);
  const v = c.veiculo;
  let modelo = '';
  const img = el('img', { class: 'etiqueta', alt: 'Etiqueta do último serviço' });
  const ult = c.servicos[0];
  const atualizarImg = () => { if (ult) img.src = `/api/etiqueta.png?servico=${ult.id}&tipo=${modelo}&t=${Date.now()}`; };
  atualizarImg();
  const copias = el('input', { type: 'number', min: 1, max: 20, value: 1, style: 'width:80px', 'aria-label': 'Cópias' });

  const dadosCarro = el('div', { class: 'dados-carro' },
    el('div', {}, el('span', {}, 'Modelo'), v.modelo || '—'), el('div', {}, el('span', {}, 'Ano'), v.ano || '—'),
    el('div', {}, el('span', {}, 'Cliente'), v.cliente || '—'), el('div', {}, el('span', {}, 'Telefone'), v.telefone || '—'),
    el('div', {}, el('span', {}, 'Código do QR'), el('span', { class: 'codigo', style: 'display:inline;font-size:1rem;color:inherit;text-transform:none' }, v.codigo)));

  function editarCarro() {
    const m = el('input', { type: 'text', value: v.modelo }), a = el('input', { type: 'text', value: v.ano }),
      cl = el('input', { type: 'text', value: v.cliente || '' }), t = el('input', { type: 'text', value: v.telefone || '' });
    dadosCarro.replaceChildren(el('div', { class: 'campos', style: 'width:100%' },
      el('div', { class: 'campo c4' }, el('label', {}, 'Modelo'), m), el('div', { class: 'campo c3' }, el('label', {}, 'Ano'), a),
      el('div', { class: 'campo c5' }, el('label', {}, 'Cliente'), cl), el('div', { class: 'campo c5' }, el('label', {}, 'Telefone'), t),
      el('div', { class: 'campo' }, el('span', { class: 'ajuda' }, 'Nome e telefone do cliente ficam só no programa (não vão para o site). Com o nome igual ao do Zeus, os lançamentos desse cliente já vêm com a placa.')),
      el('div', { class: 'campo' }, el('div', { class: 'acoes', style: 'margin:0' },
        el('button', { class: 'botao preto', onclick: async (e) => {
          await ocupado(e.target, async () => { await api('veiculos/' + v.placa, { method: 'PUT', corpo: { modelo: m.value, ano: a.value, cliente: cl.value, telefone: t.value } }); aviso('Dados do carro salvos'); navegar(); });
        } }, 'Salvar'),
        el('button', { class: 'link-botao', onclick: navegar }, 'Cancelar')))));
  }

  const hist = el('section', { class: 'cartao' }, el('div', { class: 'linha-titulo' }, el('h2', {}, `Histórico (${c.servicos.length})`),
    el('a', { class: 'botao vermelho', href: '#/novo?placa=' + v.placa }, '+ Novo serviço para este carro')));
  if (!c.servicos.length) hist.append(el('p', { class: 'vazio' }, 'Nenhum serviço ainda.'));
  else hist.append(el('ol', { class: 'tempo' }, c.servicos.map((s) => el('li', { class: s.itens.includes('Óleo do motor') ? 'oleo' : null },
    el('div', { class: 'quando' }, dataBR(s.data), s.km ? el('span', { class: 'cinza' }, ' · ' + milhar(s.km) + ' km') : null,
      s.origem === 'zeus' ? el('span', { class: 'etiq zeus', style: 'margin-left:8px' }, 'Zeus') : null),
    s.itens.length ? el('div', {}, s.itens.map((i) => el('span', { class: 'etiq padrao' }, '✓ ' + i))) : null,
    s.outros.length ? el('div', { class: 'outros' }, s.outros.join('\n')) : null,
    s.proxima_troca_km ? el('div', { class: 'obs' }, 'Próxima troca de óleo: ', el('strong', {}, milhar(s.proxima_troca_km) + ' km')) : null,
    s.proxima_correia_km || s.proxima_correia_data ? el('div', { class: 'obs' }, 'Próxima troca da correia dentada: ', el('strong', {}, [s.proxima_correia_km ? milhar(s.proxima_correia_km) + ' km' : '', dataBR(s.proxima_correia_data)].filter(Boolean).join(' ou '))) : null,
    s.obs ? el('div', { class: 'obs' }, 'Obs. para o cliente: ' + s.obs) : null,
    s.obs_interna ? el('div', { class: 'obs' }, '🔒 Interna: ' + s.obs_interna) : null,
    el('div', { class: 'acoes' },
      el('button', { class: 'botao leve pequeno escrita', onclick: (e) => imprimir(e.target, s.id, modelo, numero(copias.value)) }, 'Imprimir etiqueta'),
      el('a', { class: 'botao leve pequeno', href: `#/editar/${s.id}?placa=${v.placa}` }, 'Corrigir'),
      el('button', { class: 'link-botao pequeno escrita', onclick: async () => {
        if (!confirm(`Apagar o serviço de ${dataBR(s.data)}? Ele some também da página do QR.`)) return;
        await api('servicos/' + s.id, { method: 'DELETE' }); aviso('Serviço apagado'); navegar();
      } }, 'Apagar'))))));

  tela.replaceChildren(
    el('section', { class: 'cartao destaque' }, el('div', { class: 'carro-topo' },
      el('div', {}, placaEl(v.placa, true), el('h1', { style: 'margin-top:12px' }, v.modelo || 'Modelo não informado', v.ano ? el('span', { class: 'cinza' }, ' ' + v.ano) : ''),
        dadosCarro,
        el('div', { class: 'acoes' },
          el('button', { class: 'botao leve pequeno escrita', onclick: editarCarro }, 'Editar dados do carro'),
          el('a', { class: 'botao leve pequeno', href: c.link, target: '_blank', rel: 'noopener' }, 'Abrir página do cliente'),
          whatsCliente(v, c.link),
          el('button', { class: 'link-botao pequeno escrita', onclick: async () => {
            if (!confirm(`Apagar o carro ${placaBonita(v.placa)} e os ${c.servicos.length} serviço(s) dele? Use isso só para carro salvo errado: a etiqueta com QR Code dele deixa de mostrar o histórico.`)) return;
            try {
              const r = await api('apagar', { method: 'POST', corpo: { carros: [v.placa] } });
              r.voltaram.forEach((ref) => naoRedirecionar.add(ref));
              aviso(`Carro ${placaBonita(v.placa)} apagado` + (r.voltaram.length ? `. ${r.voltaram.length} venda(s) do Zeus voltaram para “Para revisar”.` : ''));
              atualizarEstado(); location.hash = '#/';
            } catch (e) { aviso(e.message, true); }
          } }, 'Apagar este carro'))),
      ult ? el('div', { style: 'min-width:260px;text-align:center' }, img,
        el('div', { class: 'acoes', style: 'justify-content:center' }, el('label', {}, 'Cópias ', copias),
          el('button', { class: 'botao vermelho escrita', onclick: (e) => imprimir(e.target, ult.id, modelo, numero(copias.value)) }, 'Imprimir etiqueta'))) : null)),
    hist,
    el('section', { class: 'cartao' }, el('div', { class: 'linha-titulo' }, el('h2', {}, `Termos de peça do cliente (${termos.length})`),
      el('a', { class: 'botao leve pequeno', href: '#/termo/novo?placa=' + v.placa }, '+ Termo de peça do cliente')),
      termos.length ? listaTermos(termos) : el('p', { class: 'cinza pequeno', style: 'margin:0' }, 'Quando o cliente trouxer a peça comprada por ele, faça o termo: a oficina responde só pela mão de obra.')));
}

/* ------------------------------------------------------ configurações --- */
function chaveNova() {
  const a = '23456789ABCDEFGHJKLMNPQRSTUVWXYZ';
  const n = new Uint32Array(8); crypto.getRandomValues(n);
  return [...n].map((x) => a[x % a.length]).join('');
}

// lista das cópias de segurança, com "Voltar para esta"
function caixaCopias() {
  const caixa = el('div', {});
  const abrir = el('button', { type: 'button', class: 'botao leve pequeno', onclick: async () => {
    let l = [];
    try { l = await api('copias'); } catch (e) { aviso(e.message, true); return; }
    if (!l.length) { caixa.replaceChildren(el('p', { class: 'cinza pequeno' }, 'Nenhuma cópia ainda.')); return; }
    caixa.replaceChildren(el('div', { class: 'tabela-rolagem', style: 'max-height:280px' }, el('table', { class: 'dados' },
      el('tr', {}, ['Cópia', 'Quando', 'Carros', 'Serviços', 'Termos', ''].map((t) => el('th', {}, t))),
      l.map((c) => el('tr', {},
        el('td', {}, c.nome.startsWith('antes-de-') ? 'Antes de ' + c.nome.split('-')[2] : 'Fim do dia', c.onde === 'extra' ? ' (pasta extra)' : ''),
        el('td', {}, quando(c.hora)), el('td', {}, c.carros), el('td', {}, c.servicos), el('td', {}, c.termos),
        el('td', {}, el('button', { class: 'botao leve pequeno escrita', onclick: async (e) => {
          if (!confirm(`Voltar os dados para a cópia de ${quando(c.hora)} (${c.carros} carros, ${c.servicos} serviços)?\n\nO que foi feito depois dela some do programa e do site. Antes, o programa guarda como está agora.`)) return;
          await ocupado(e.target, async () => {
            try { await api('copias/restaurar', { method: 'POST', corpo: { nome: c.nome, onde: c.onde } }); aviso('Dados restaurados'); atualizarEstado(); location.hash = '#/'; }
            catch (er) { aviso(er.message, true); }
          });
        } }, 'Voltar para esta')))))));
  } }, 'Ver cópias de segurança');
  caixa.append(abrir);
  return caixa;
}

// escolher: este computador é o principal ou só consulta
function blocoModoConsulta(cfg) {
  const end = el('input', { type: 'text', value: cfg.principal || '', placeholder: 'Ex.: 192.168.1.20', autocomplete: 'off' });
  const chave = el('input', { type: 'text', value: cfg.modo_computador === 'consulta' ? (cfg.chave_rede || '') : '', autocomplete: 'off', style: 'font-family:Consolas,monospace;letter-spacing:2px;text-transform:uppercase' });
  const achados = el('div', { class: 'achados' });
  const resultado = el('div', { 'aria-live': 'polite' });
  const testar = async () => {
    const r = await api('consulta/testar', { method: 'POST', corpo: { principal: end.value.trim(), chave: chave.value.trim().toUpperCase() } });
    end.value = r.principal;
    resultado.replaceChildren(el('div', { class: 'encontrado' }, el('strong', {}, 'Ligou! '), `Computador principal na versão ${r.versao}` + (r.hoje ? ` · ${r.hoje.servicos} serviço(s) hoje` : '') + '.'));
    return r;
  };
  return el('div', { class: 'campos', style: 'margin-top:12px' },
    el('div', { class: 'campo' }, el('p', { class: 'cinza', style: 'margin:0' }, 'No modo consulta, este computador NÃO registra, NÃO imprime etiqueta, NÃO lê o Zeus e NÃO publica o site: só mostra os dados do computador principal pela rede (carros, histórico, termos, lembretes e relatório).')),
    el('div', { class: 'campo c6' }, el('label', {}, 'Endereço do computador principal'), end),
    el('div', { class: 'campo c6' }, el('label', {}, 'Chave da consulta (está nas Configurações do principal)'), chave),
    el('div', { class: 'campo' }, el('div', { class: 'acoes', style: 'margin:0' },
      el('button', { type: 'button', class: 'botao leve', onclick: (e) => ocupado(e.target, async () => {
        achados.replaceChildren(el('span', { class: 'cinza' }, 'Procurando na rede…'));
        const l = await api('consulta/procurar', { method: 'POST' }).catch(() => []);
        achados.replaceChildren(...(l.length ? l.map((a) => el('button', { type: 'button', class: 'botao leve pequeno', onclick: () => { end.value = a.endereco; } }, `${a.nome} (${a.endereco})`))
          : [el('span', { class: 'cinza pequeno' }, 'Nenhum achado. No principal, ligue “Deixar outros computadores consultarem”, ou digite o endereço.')]));
      }) }, 'Procurar o principal na rede'),
      el('button', { type: 'button', class: 'botao leve', onclick: (e) => ocupado(e.target, async () => { try { await testar(); } catch (er) { resultado.replaceChildren(el('div', { class: 'alerta-data' }, er.message)); } }) }, 'Testar ligação'))),
    el('div', { class: 'campo' }, achados, resultado),
    el('div', { class: 'campo' }, el('div', { class: 'acoes', style: 'margin:0' },
      el('button', { type: 'button', class: 'botao vermelho', onclick: (e) => ocupado(e.target, async () => {
        try { await testar(); } catch (er) { resultado.replaceChildren(el('div', { class: 'alerta-data' }, er.message)); return; }
        if (!confirm('Usar este computador só para CONSULTA? Ele para de ler o Zeus, imprimir e publicar o site. Dá para voltar depois em Configurações.')) return;
        try {
          await api('config', { method: 'PUT', corpo: { modo_computador: 'consulta', principal: end.value.trim(), chave_rede: chave.value.trim().toUpperCase(), rede_ligada: false } });
          location.hash = '#/'; location.reload();
        } catch (er) { aviso(er.message, true); }
      }) }, 'Usar este computador só para consulta'))));
}

// Configurações no computador de consulta: só o modo e o básico
async function telaConfigConsulta() {
  const cfg = await api('config');
  const iniciar = el('input', { type: 'checkbox', checked: cfg.iniciar_com_windows });
  tela.replaceChildren(el('h1', {}, 'Configurações'),
    el('section', { class: 'cartao config-secao destaque' }, el('h2', {}, '🔒 Este computador é só para consulta'),
      el('p', {}, 'Os dados vêm do computador principal ', el('strong', {}, cfg.principal || '?'), '. Para registrar, imprimir etiqueta ou mudar algo, use o principal.'),
      ESTADO.erro_principal ? el('div', { class: 'alerta-data' }, ESTADO.erro_principal) : el('div', { class: 'encontrado' }, 'Ligado ao principal (versão ' + (ESTADO.versao_principal || '?') + ').'),
      el('details', { style: 'margin-top:12px' }, el('summary', { style: 'cursor:pointer' }, 'Mudar o endereço ou a chave'), blocoModoConsulta(cfg)),
      el('div', { class: 'acoes' },
        el('button', { class: 'botao leve', onclick: async (e) => {
          if (!confirm('Voltar este computador a ser PRINCIPAL? Só faça isso se o outro não for mais usado: os dois juntos publicando o site apagam o que o outro registrou.')) return;
          await ocupado(e.target, async () => { await api('config', { method: 'PUT', corpo: { modo_computador: 'principal' } }); location.hash = '#/'; location.reload(); });
        } }, 'Voltar a ser o computador principal'))),
    el('section', { class: 'cartao config-secao' }, el('h2', {}, '💻 Programa'),
      el('label', { class: 'interruptor' }, iniciar, 'Abrir junto com o Windows'),
      el('div', { class: 'acoes' },
        el('button', { class: 'botao preto', onclick: (e) => ocupado(e.target, async () => { await api('config', { method: 'PUT', corpo: { iniciar_com_windows: iniciar.checked, modo_computador: 'consulta' } }); aviso('Salvo'); }) }, 'Salvar'),
        el('a', { class: 'botao leve', href: '/api/registro', target: '_blank' }, 'Ver registro de erros'),
        el('button', { class: 'botao leve', onclick: async () => {
          if (!confirm('Fechar o programa?')) return;
          await api('sair', { method: 'POST' }); document.body.replaceChildren(el('main', {}, el('div', { class: 'cartao' }, el('h1', {}, 'Programa fechado'), el('p', {}, 'Pode fechar esta janela.'))));
        } }, 'Fechar o programa'))));
}

async function telaConfig() {
  if (ESTADO.somente_leitura) return telaConfigConsulta();
  const [cfg, imps] = await Promise.all([api('config'), api('impressoras').catch(() => ({ impressoras: [], padrao: '' }))]);
  if (/atualizar=1/.test(location.hash)) setTimeout(async () => {
    const alvo = [...document.querySelectorAll('.rotulo')].find((x) => x.textContent === 'Atualização do programa');
    if (alvo) { alvo.scrollIntoView({ block: 'center' }); alvo.parentElement.querySelector('button').click(); }
  }, 300);
  const s = ESTADO.site || {}, z = ESTADO.zeus || {};
  const ce = cfg.etiqueta;

  // impressora
  const selImp = el('select', { id: 'c-imp' }, el('option', { value: '' }, '— escolha —'),
    imps.impressoras.map((n) => el('option', { value: n, selected: n === cfg.impressora ? '' : null }, n + (n === imps.padrao ? ' (padrão do Windows)' : ''))));
  if (!cfg.impressora) { const ie4s = imps.impressoras.find((n) => /ie4s|idprt/i.test(n)); if (ie4s) selImp.value = ie4s; }
  const copias = el('input', { type: 'number', min: 1, max: 20, value: cfg.copias || 1 });
  const aoSalvar = el('input', { type: 'checkbox', checked: cfg.imprimir_ao_salvar });
  const tamDriver = el('input', { type: 'checkbox', checked: cfg.tamanho_do_driver });
  const ajX = el('input', { type: 'number', step: 0.5, value: ce.ajuste_x_mm || 0 });
  const ajY = el('input', { type: 'number', step: 0.5, value: ce.ajuste_y_mm || 0 });
  const larg = el('input', { type: 'number', step: 1, value: ce.largura_mm });
  const alt = el('input', { type: 'number', step: 1, value: ce.altura_mm });
  // etiqueta
  const titulo = el('input', { type: 'text', value: ce.titulo });
  const insta = el('input', { type: 'text', value: ce.instagram });
  const fone = el('input', { type: 'text', value: ce.telefone });
  const corKM = el('input', { type: 'number', min: 1000, step: 1000, value: ce.correia_km || 40000 });
  const corMeses = el('input', { type: 'number', min: 1, max: 120, value: ce.correia_meses || 12 });
  const oleoAntigo = el('input', { type: 'checkbox', checked: !!ce.oleo_modelo_antigo });
  // consulta pela rede (este é o principal)
  const redeLigada = el('input', { type: 'checkbox', checked: !!cfg.rede_ligada });
  const chaveRede = el('input', { type: 'text', value: cfg.chave_rede || '', autocomplete: 'off', style: 'font-family:Consolas,monospace;letter-spacing:2px;text-transform:uppercase' });
  const infoRede = el('div', { class: 'ajuda' });
  api('rede').then((r) => infoRede.replaceChildren('Endereço deste computador na rede: ',
    ...(r.ips.length ? r.ips.map((ip, i) => [i ? ' ou ' : '', el('strong', {}, ip)]) : [el('strong', {}, 'não achei')]),
    ` (nome: ${r.nome}). O outro computador procura sozinho; se não achar, digite este endereço lá.`)).catch(() => {});
  // site
  const token = el('input', { type: 'password', placeholder: cfg.tem_token ? 'Token já guardado (cole outro para trocar)' : 'Cole aqui o token do GitHub', autocomplete: 'off' });
  const repo = el('input', { type: 'text', value: cfg.site.repo });
  const repoBkp = el('input', { type: 'text', value: cfg.site.repo_backup || '', placeholder: 'automec135/oficina-dados (privado, opcional)' });
  // zeus
  const zc = cfg.zeus;
  const zAtivo = el('input', { type: 'checkbox', checked: zc.ativo });
  const zHost = el('input', { type: 'text', value: zc.host });
  const zPorta = el('input', { type: 'number', value: zc.porta || 5432 });
  const zBanco = el('input', { type: 'text', value: zc.banco || '' });
  const zUser = el('input', { type: 'text', value: zc.usuario || '', autocomplete: 'off' });
  const zSenha = el('input', { type: 'password', placeholder: cfg.tem_senha ? 'Senha já guardada' : 'Senha do banco', autocomplete: 'new-password' });
  const zInt = el('input', { type: 'number', min: 15, value: zc.intervalo_seg || 20 });
  const zDesde = el('input', { type: 'date', value: zc.desde || hoje() });
  const zAutoReg = el('input', { type: 'checkbox', checked: zc.auto_registrar });
  const zAutoImp = el('input', { type: 'checkbox', checked: zc.auto_imprimir });
  const zEspera = el('input', { type: 'number', min: 0, max: 60, value: zc.espera_min ?? 1 });
  const zModoAtual = zc.modo || (zc.sql ? 'banco' : 'pdf');
  const zModoPDF = el('input', { type: 'radio', name: 'zmodo', value: 'pdf', checked: zModoAtual === 'pdf' });
  const zModoBanco = el('input', { type: 'radio', name: 'zmodo', value: 'banco', checked: zModoAtual === 'banco' });
  const zPasta = el('input', { type: 'text', value: zc.pasta_pdf || '', placeholder: 'C:\\OficinaQR\\entrada' });
  const zSQL = el('textarea', { style: 'font-family:Consolas,monospace;font-size:.85rem;min-height:140px', placeholder: 'Consulta que lista os produtos das vendas (preenchida pelo suporte)' }, zc.sql || '');
  const iniciar = el('input', { type: 'checkbox', checked: cfg.iniciar_com_windows });
  const pastaCopia = el('input', { type: 'text', value: cfg.pasta_copia || '', placeholder: 'Ex.: C:\\Users\\Usuario\\OneDrive\\OficinaQR-copias' });
  const mesesLembrete = el('input', { type: 'number', min: 1, max: 24, value: cfg.meses_lembrete || 6 });
  const infoAtual = el('div', { class: 'ajuda' }, 'Versão ' + (ESTADO.versao || ''));

  function juntar() {
    const c = {
      impressora: selImp.value, copias: numero(copias.value) || 1, imprimir_ao_salvar: aoSalvar.checked, whatsapp: cfg.whatsapp,
      tamanho_do_driver: tamDriver.checked,
      etiqueta: Object.assign({}, ce, { titulo: titulo.value, instagram: insta.value, telefone: fone.value,
        correia_km: numero(corKM.value) || 40000, correia_meses: numero(corMeses.value) || 12, oleo_modelo_antigo: oleoAntigo.checked,
        ajuste_x_mm: parseFloat(ajX.value) || 0, ajuste_y_mm: parseFloat(ajY.value) || 0, largura_mm: parseFloat(larg.value) || 60, altura_mm: parseFloat(alt.value) || 60 }),
      site: Object.assign({}, cfg.site, { repo: repo.value.trim(), repo_backup: repoBkp.value.trim() }),
      zeus: { ativo: zAtivo.checked, host: zHost.value.trim(), porta: numero(zPorta.value) || 5432, banco: zBanco.value.trim(), usuario: zUser.value.trim(),
        sql: zSQL.value, intervalo_seg: numero(zInt.value) || 20, desde: zDesde.value, auto_registrar: zAutoReg.checked, auto_imprimir: zAutoImp.checked,
        espera_min: Math.max(0, parseInt(zEspera.value, 10) || 0),
        modo: zModoBanco.checked ? 'banco' : 'pdf', pasta_pdf: zPasta.value.trim() },
      iniciar_com_windows: iniciar.checked,
      pasta_copia: pastaCopia.value.trim(), meses_lembrete: numero(mesesLembrete.value) || 6,
      rede_ligada: redeLigada.checked, modo_computador: 'principal',
    };
    if (chaveRede.value.trim().toUpperCase() !== (cfg.chave_rede || '')) c.chave_rede = chaveRede.value.trim().toUpperCase();
    if (token.value.trim()) c.token = token.value.trim();
    if (zSenha.value) c.senha = zSenha.value;
    return c;
  }
  async function salvarTudo(botao, depois) {
    await ocupado(botao, async () => {
      try {
        const r = await api('config', { method: 'PUT', corpo: juntar() });
        token.value = ''; zSenha.value = '';
        if (r.tem_token) token.placeholder = 'Token já guardado (cole outro para trocar)';
        if (r.tem_senha) zSenha.placeholder = 'Senha já guardada';
        aviso('Configurações salvas'); await atualizarEstado(); if (depois) await depois();
      }
      catch (e) { aviso(e.message, true); }
    });
  }
  const botaoSalvar = () => el('button', { class: 'botao preto', onclick: (e) => salvarTudo(e.target) }, 'Salvar');
  const inter = (cb, texto) => el('label', { class: 'interruptor' }, cb, texto);
  const campo = (cls, rot, ctl, ajuda) => el('div', { class: 'campo ' + cls }, el('label', {}, rot), ctl, ajuda ? el('span', { class: 'ajuda' }, ajuda) : null);
  // ---- Zeus: modo PDF
  const listaPDFs = el('div', { class: 'lista-pdfs' });
  async function listarPDFs() {
    try {
      const r = await api('zeus/pdfs');
      if (!r.pdfs.length) { listaPDFs.replaceChildren(el('p', { class: 'cinza pequeno' }, 'Nenhum PDF na pasta ainda.')); return; }
      listaPDFs.replaceChildren(el('p', { class: 'rotulo' }, 'Últimos PDFs na pasta'),
        ...r.pdfs.map((p) => el('div', { class: 'produtos', style: 'margin:4px 0' },
          el('strong', {}, p.arquivo), ' (' + p.quando + ') — ',
          p.erro ? el('span', { style: 'color:#b42318' }, 'não entendi: ' + p.erro)
            : [p.documento, ' · ', p.cliente || 'sem cliente', ' · ', p.produtos + ' produto(s)', ' · ',
              p.placas && p.placas.length === 1 ? el('strong', {}, 'placa ' + p.placas[0])
                : el('span', { style: 'color:#b54708' }, p.placas && p.placas.length > 1 ? 'mais de uma placa na observação' : 'sem placa na observação')])));
    } catch (er) { listaPDFs.replaceChildren(el('p', { class: 'cinza pequeno' }, er.message)); }
  }
  const blocoPDF = el('div', { class: 'campos' },
    el('div', { class: 'campo' }, el('p', { class: 'cinza', style: 'margin:0' },
      'No Zeus: Imprimir > modelo ', el('strong', {}, 'Mini'), ' > Visualizar Impressão > botão ', el('strong', {}, 'PDF'),
      ' > salvar nesta pasta. Escreva a placa nas Observações (ex.: "PLACA ABC1D23 KM 98765"). O programa lê o PDF sozinho; com placa, registra e imprime a etiqueta.')),
    campo('c8', 'Pasta onde o balcão salva os PDFs', zPasta, 'Deixe vazio para usar C:\\OficinaQR\\entrada.'),
    el('div', { class: 'campo c4' }, el('span', { class: 'rotulo' }, '\u00a0'),
      el('button', { class: 'botao leve', onclick: async (e) => ocupado(e.target, async () => {
        try { await api('zeus/abrir-pasta', { method: 'POST' }); } catch (er) { aviso(er.message, true); }
      }) }, 'Abrir a pasta')),
    el('div', { class: 'campo' }, listaPDFs));
  // ---- Zeus: modo banco (campos antigos)
  const blocoBanco = el('div', { class: 'campos' },
    el('div', { class: 'campo' }, el('p', { class: 'cinza', style: 'margin:0' }, 'O programa só LÊ o banco do Zeus (nunca altera estoque nem financeiro). Quando uma venda tem a placa na observação, o serviço entra no histórico e a etiqueta sai sozinha.')),
    campo('c4', 'Servidor', zHost), campo('c3', 'Porta', zPorta), campo('c5', 'Banco de dados', zBanco),
    campo('c6', 'Usuário', zUser), campo('c6', 'Senha', zSenha, 'Fica guardada cifrada neste computador.'),
    campo('c4', 'Ler lançamentos a partir de', zDesde),
    campo('c4', 'Esperar a venda parar de mudar (minutos)', zEspera, 'Evita imprimir enquanto o balcão ainda lança peças. 0 = na hora.'),
    el('details', { class: 'campo' }, el('summary', {}, 'Avançado: consulta do Zeus'), zSQL,
      el('p', { class: 'ajuda' }, 'Colunas: ref, documento, data, descricao, cliente, texto. $1 = data inicial. ', el('a', { href: '#/zeus' }, 'Abrir o explorador do banco'))));
  const mostrarModo = () => { blocoPDF.style.display = zModoPDF.checked ? '' : 'none'; blocoBanco.style.display = zModoBanco.checked ? '' : 'none'; };
  zModoPDF.addEventListener('change', mostrarModo); zModoBanco.addEventListener('change', mostrarModo);
  mostrarModo(); listarPDFs();

  const estadoSite = !s.configurado ? 'Ainda não ligado' : s.erro ? 'Erro: ' + s.erro : s.ultimo && !s.ultimo.startsWith('0001') ? 'Atualizado ' + quando(s.ultimo) : 'Ligado';

  tela.replaceChildren(el('h1', {}, 'Configurações'),
    el('section', { class: 'cartao config-secao' }, el('h2', {}, '🖨 Impressora e etiqueta'),
      el('div', { class: 'campos' },
        campo('c6', 'Impressora', selImp, 'Escolha a iDPRT iE4S.'),
        campo('c3', 'Cópias por etiqueta', copias),
        el('div', { class: 'campo c3' }, el('span', { class: 'rotulo' }, 'Ao salvar'), inter(aoSalvar, 'Já imprimir'), el('span', { class: 'ajuda' }, 'Botão principal imprime.')),
        campo('c3', 'Largura (mm)', larg), campo('c3', 'Altura (mm)', alt),
        campo('c3', 'Mover → (mm)', ajX, 'Se sair torta, ajuste aqui.'), campo('c3', 'Mover ↓ (mm)', ajY),
        el('div', { class: 'campo' }, inter(tamDriver, 'Usar o tamanho de papel configurado no driver da impressora'),
          el('span', { class: 'ajuda' }, 'Só marque se a etiqueta sair pulando ou cortada: aí o tamanho vem do driver (Preferências de impressão do Windows).')),
        campo('c4', 'Título', titulo), campo('c4', 'Instagram', insta), campo('c4', 'Telefone / WhatsApp', fone),
        campo('c6', 'Correia dentada: próxima troca em + KM', corKM, 'Padrão da etiqueta da correia (dá para mudar em cada serviço).'),
        campo('c6', 'Correia dentada: próxima troca em + meses', corMeses),
        el('div', { class: 'campo' }, inter(oleoAntigo, 'Etiqueta de óleo no modelo antigo (igual ao BarTender, sem a data da próxima troca)'))),
      el('div', { class: 'acoes' }, botaoSalvar(),
        el('button', { class: 'botao leve', onclick: (e) => salvarTudo(e.target, async () => {
          try { await api('imprimir/teste', { method: 'POST' }); aviso('Etiqueta de teste enviada'); } catch (er) { aviso('Não imprimiu: ' + er.message, true); }
        }) }, 'Salvar e imprimir teste'))),

    el('section', { class: 'cartao config-secao' }, el('h2', {}, '🌐 Site do QR Code'),
      el('p', {}, 'Situação: ', el('strong', {}, estadoSite)),
      el('p', { class: 'cinza' }, 'O programa manda o histórico (só modelo, ano e serviços — sem placa, nome ou telefone) para o site automec135.github.io. Para isso ele precisa de um token do GitHub:'),
      el('ol', { class: 'passos' },
        el('li', {}, 'Entre no GitHub com a conta ', el('strong', {}, 'automec135'), ' e abra ', el('a', { href: 'https://github.com/settings/personal-access-tokens/new', target: '_blank', rel: 'noopener' }, 'github.com/settings/personal-access-tokens/new'), '.'),
        el('li', {}, 'Nome: ', el('em', {}, 'Oficina QR'), '. Validade: a maior possível (ou “No expiration”).'),
        el('li', {}, 'Em “Repository access”, escolha ', el('em', {}, 'Only select repositories'), ' e marque ', el('strong', {}, 'automec135.github.io'), ' (e o repositório da cópia de segurança, se tiver).'),
        el('li', {}, 'Em “Permissions” › “Repository permissions”, ponha ', el('strong', {}, 'Contents: Read and write'), '.'),
        el('li', {}, 'Clique em “Generate token”, copie e cole abaixo.')),
      el('div', { class: 'campos' },
        campo('c6', 'Token do GitHub', token, 'Fica guardado cifrado neste computador.'),
        campo('c6', 'Repositório do site', repo),
        campo('c6', 'Cópia de segurança completa (opcional)', repoBkp, 'Repositório PRIVADO onde vai uma cópia de todos os dados (com placas). Deixe vazio se não quiser.')),
      el('div', { class: 'acoes' }, botaoSalvar(),
        el('button', { class: 'botao leve', onclick: (e) => salvarTudo(e.target, async () => {
          try { await api('site/testar', { method: 'POST' }); aviso('Token funcionando'); } catch (er) { aviso(er.message, true); }
        }) }, 'Salvar e testar'),
        el('button', { class: 'botao leve', onclick: async (e) => ocupado(e.target, async () => {
          try { await api('site/publicar', { method: 'POST' }); aviso('Histórico publicado'); atualizarEstado(); } catch (er) { aviso(er.message, true); }
        }) }, 'Publicar agora'))),

    el('section', { class: 'cartao config-secao' }, el('h2', {}, '🔗 Ligação com o Zeus'),
      el('p', {}, 'Situação: ', el('strong', {}, !z.ativo ? 'Desligado' : z.erro ? 'Erro: ' + z.erro : 'Lido ' + (quando(z.ultima) || '—'))),
      el('div', { class: 'campos' },
        el('div', { class: 'campo' }, inter(zAtivo, 'Ler o Zeus automaticamente')),
        el('div', { class: 'campo' }, el('span', { class: 'rotulo' }, 'Como o programa recebe o que foi lançado no Zeus'),
          el('label', { class: 'interruptor' }, zModoPDF, 'Pelos PDFs (botão PDF da impressão do Zeus) — não precisa de senha'),
          el('label', { class: 'interruptor' }, zModoBanco, 'Pelo banco de dados do Zeus (precisa de um usuário de leitura da empresa do Zeus)'))),
      blocoPDF, blocoBanco,
      el('div', { class: 'campos' },
        el('div', { class: 'campo c8' }, el('span', { class: 'rotulo' }, 'Revisão'), el('p', { class: 'ajuda', style: 'margin:0' },
          'Nada do Zeus é registrado nem impresso sozinho: cada venda paga aparece em “Para revisar”, você confere e manda imprimir.')),
        campo('c4', 'Ler a cada (segundos)', zInt)),
      el('div', { class: 'acoes' }, botaoSalvar(),
        el('button', { class: 'botao leve', onclick: (e) => salvarTudo(e.target, async () => {
          if (zModoPDF.checked) {
            try { const r = await api('zeus/ler', { method: 'POST' }); aviso(r.novos ? `${r.novos} lançamento(s) novo(s)` : 'Pasta lida — nada novo'); await listarPDFs(); } catch (er) { aviso(er.message, true); }
            return;
          }
          try { const r = await api('zeus/testar', { method: 'POST' }); aviso('Conectou: ' + r.versao.split(',')[0]); } catch (er) { aviso(er.message, true); }
        }) }, 'Salvar e testar'))),

    el('section', { class: 'cartao config-secao' }, el('h2', {}, '🖥 Consulta por outros computadores'),
      el('p', { class: 'cinza', style: 'margin-top:0' }, 'Este é o computador principal (registra, imprime e lê o Zeus). Outros computadores da oficina podem só CONSULTAR os dados dele, sem mudar nada.'),
      el('div', { class: 'campos' },
        el('div', { class: 'campo' }, inter(redeLigada, 'Deixar outros computadores consultarem (somente leitura)')),
        campo('c6', 'Chave da consulta', chaveRede, 'O outro computador precisa desta chave. Troque se alguém que não devia souber.'),
        el('div', { class: 'campo c6' }, el('span', { class: 'rotulo' }, '\u00a0'), el('button', { class: 'botao leve', onclick: () => { chaveRede.value = chaveNova(); } }, 'Gerar chave nova')),
        el('div', { class: 'campo' }, infoRede),
        el('div', { class: 'campo' }, el('span', { class: 'ajuda' }, 'Na primeira vez, o Windows pode perguntar se libera o Oficina QR na rede: marque “Redes privadas” e clique em Permitir.'))),
      el('div', { class: 'acoes' }, el('button', { class: 'botao preto', onclick: (e) => {
        if (redeLigada.checked && !chaveRede.value.trim()) chaveRede.value = chaveNova();
        salvarTudo(e.target);
      } }, 'Salvar')),
      el('details', { style: 'margin-top:14px' }, el('summary', { style: 'cursor:pointer' }, 'Transformar ESTE computador em computador de consulta'),
        blocoModoConsulta(cfg))),

    el('section', { class: 'cartao config-secao' }, el('h2', {}, '💻 Programa'),
      el('div', { class: 'campos' },
        el('div', { class: 'campo' }, inter(iniciar, 'Abrir junto com o Windows (fica lendo o Zeus mesmo com a janela fechada)')),
        campo('c8', 'Cópia de segurança extra (pasta)', pastaCopia, 'Uma cópia dos dados por dia vai também para esta pasta. Numa pasta do OneDrive, a cópia fica guardada na internet.'),
        campo('c4', 'Troca de óleo a cada (meses)', mesesLembrete, 'Vale para a data na etiqueta, os lembretes e a página do QR Code.'),
        el('div', { class: 'campo' }, el('span', { class: 'ajuda' }, 'Dados guardados em: ', el('code', {}, ESTADO.pasta || ''), ' (com uma cópia por dia na pasta “copias”).')),
        el('div', { class: 'campo' }, el('span', { class: 'rotulo' }, 'Cópias de segurança'), caixaCopias(),
          el('span', { class: 'ajuda' }, 'Se algo foi apagado ou mudado sem querer, volte para uma cópia. Antes de voltar, o programa guarda como está agora (dá para desfazer).')),
        el('div', { class: 'campo' }, el('span', { class: 'rotulo' }, 'Atualização do programa'), infoAtual,
          el('div', { class: 'acoes', style: 'margin:6px 0 0' },
            el('button', { class: 'botao leve', onclick: (e) => ocupado(e.target, async () => {
              const r = await api('atualizacao?agora=1'); ATUALIZACAO = r; mostrarAtualizacao(r, infoAtual); atualizarEstado();
            }) }, 'Procurar versão nova'))),
        el('div', { class: 'campo' }, el('span', { class: 'rotulo' }, 'Atalhos de teclado'), el('p', { class: 'ajuda', style: 'margin:0' },
          'F2 = novo serviço · F3 = buscar carro · F4 = para revisar · Ctrl+Enter = salvar e imprimir (na tela do serviço) · botões A− / A+ / ◐ no topo = tamanho da letra e alto contraste.'))),
      el('div', { class: 'acoes' }, botaoSalvar(),
        el('a', { class: 'botao leve', href: '/api/exportar.csv', download: '' }, 'Baixar planilha com todos os serviços'),
        el('a', { class: 'botao leve', href: '/api/registro', target: '_blank' }, 'Ver registro de erros'),
        el('button', { class: 'botao leve', onclick: async () => {
          if (!confirm('Fechar o programa por completo? A leitura do Zeus para até abrir de novo.')) return;
          await api('sair', { method: 'POST' }); document.body.replaceChildren(el('main', {}, el('div', { class: 'cartao' }, el('h1', {}, 'Programa fechado'), el('p', {}, 'Pode fechar esta janela. Para abrir de novo, use o atalho Oficina QR.'))));
        } }, 'Fechar o programa'))));
}

function mostrarAtualizacao(r, onde) {
  if (r.erro) { onde.replaceChildren('Versão ' + r.atual + ' — não consegui procurar atualização: ' + r.erro); return; }
  if (!r.nova) { onde.replaceChildren('Versão ' + r.atual + ' — é a mais nova.'); return; }
  onde.replaceChildren(el('strong', {}, `Versão nova ${r.versao} disponível`), ' (você está na ' + r.atual + '). ',
    r.notas ? el('div', { class: 'pequeno', style: 'white-space:pre-line' }, r.notas) : null,
    el('button', { class: 'botao vermelho pequeno', style: 'margin-top:6px', onclick: (e) => instalarAtualizacao(e.target) }, 'Atualizar agora'));
}
async function instalarAtualizacao(botao) {
  if (!confirm('Baixar e instalar a versão nova? O programa fecha e abre de novo sozinho em alguns segundos.')) return;
  await ocupado(botao, async () => {
    try { await api('atualizacao/instalar', { method: 'POST' }); } catch (e) { aviso(e.message, true); return; }
    document.body.replaceChildren(el('main', {}, el('div', { class: 'cartao' }, el('h1', {}, 'Atualizando…'), el('p', {}, 'O programa está abrindo a versão nova. Esta janela recarrega sozinha.'))));
    for (let i = 0; i < 60; i++) {
      await new Promise((r) => setTimeout(r, 1000));
      try { const e = await fetch('/api/estado').then((x) => x.json()); if (e.versao) { location.href = '/#/'; location.reload(); return; } } catch (er) { /* ainda abrindo */ }
    }
  });
}

/* ------------------------------------------------------------ lembretes --- */
async function telaLembretes() {
  const r = await api('lembretes');
  const lista = el('div');
  const linha = (l) => el('div', { class: 'pendente' + (l.avisado ? ' avisado' : ''), style: l.situacao === 'atrasado' ? 'border-left-color:var(--vermelho)' : 'border-left-color:#E8A100' },
    el('div', { class: 'cab' }, el('span', {}, placaEl(l.placa), ' ', el('strong', {}, [l.modelo, l.ano].filter(Boolean).join(' ') || 'Modelo não informado')),
      el('span', { class: 'cinza' }, `${l.cliente || 'cliente sem nome'}${l.telefone ? ' · ' + l.telefone : ''}`)),
    l.tipo === 'correia'
      ? el('div', {}, el('span', { class: 'etiq padrao' }, 'Correia dentada'), ` Trocada em ${dataBR(l.ultima)}${l.km ? ' com ' + milhar(l.km) + ' km' : ''} — próxima: ${dataBR(l.proxima_data)}${l.proxima_km ? ' ou ' + milhar(l.proxima_km) + ' km' : ''}`,
        l.avisado ? el('span', { class: 'etiq' }, ' avisado em ' + dataBR(l.avisado_em)) : null)
      : el('div', {}, el('span', { class: 'etiq' }, 'Troca de óleo'), ` Última troca: ${dataBR(l.ultima)}${l.km ? ' com ' + milhar(l.km) + ' km' : ''} — há ${l.meses} meses`,
      l.proxima_km ? ` · próxima marcada para ${milhar(l.proxima_km)} km` : '',
      l.avisado ? el('span', { class: 'etiq' }, ' avisado em ' + dataBR(l.avisado_em)) : null),
    el('div', { class: 'acoes' },
      l.whatsapp ? el('a', { class: 'botao verde', href: l.whatsapp }, 'Mandar lembrete no WhatsApp') : el('span', { class: 'cinza pequeno' }, 'Sem telefone: preencha em “Editar dados do carro”.'),
      el('button', { class: 'botao leve pequeno escrita', onclick: async (e) => { await ocupado(e.target, async () => { await api(`lembretes/${l.placa}/avisado`, { method: 'POST' }); aviso('Marcado como avisado'); navegar(); }); } }, l.avisado ? 'Avisei de novo hoje' : 'Marcar como avisado'),
      el('button', { class: 'botao leve pequeno', onclick: async () => { try { await navigator.clipboard.writeText(l.mensagem); aviso('Mensagem copiada'); } catch (er) { prompt('Copie a mensagem:', l.mensagem); } } }, 'Copiar mensagem'),
      el('a', { class: 'link-botao pequeno', href: '#/carro/' + l.placa }, 'Ver carro')));
  const atrasados = r.lista.filter((l) => !l.avisado && l.situacao === 'atrasado');
  const chegando = r.lista.filter((l) => !l.avisado && l.situacao === 'chegando');
  const avisados = r.lista.filter((l) => l.avisado);
  const grupo = (titulo, l, vazio) => [el('h2', { style: 'margin-top:18px' }, `${titulo} (${l.length})`), l.length ? l.map(linha) : el('p', { class: 'cinza' }, vazio)];
  lista.append(...grupo(`Atrasados (óleo há mais de ${r.meses} meses ou correia vencida)`, atrasados, 'Ninguém atrasado.'),
    ...grupo('Chegando a hora (no próximo mês)', chegando, 'Ninguém no próximo mês.'),
    ...grupo('Já avisados', avisados, 'Nenhum ainda.'));
  tela.replaceChildren(el('section', { class: 'cartao' }, el('h1', {}, 'Lembretes de revisão'),
    el('p', { class: 'cinza' }, `Troca de óleo feita há ${r.meses - 1} meses ou mais, e correia dentada com a próxima troca vencida ou nos próximos 30 dias. O botão abre o WhatsApp com a mensagem pronta; depois marque como avisado. O tempo muda em Configurações > Programa.`),
    lista));
}

/* ------------------------------------------------------------ relatório --- */
async function telaRelatorio(params) {
  const mes = params.get('mes') || hoje().slice(0, 7);
  const r = await api('relatorio?mes=' + mes);
  const [a, m] = mes.split('-').map(Number);
  const nomeMes = new Date(a, m - 1, 15).toLocaleDateString('pt-BR', { month: 'long', year: 'numeric' });
  const outroMes = (d) => { const x = new Date(a, m - 1 + d, 15); return `${x.getFullYear()}-${String(x.getMonth() + 1).padStart(2, '0')}`; };
  const num = (v, t) => el('div', { class: 'numero-grande' }, el('strong', {}, String(v)), el('span', {}, t));
  const barras = (l) => {
    const max = Math.max(1, ...l.map((x) => x.n));
    return el('div', { class: 'barras' }, l.map((x) => el('div', { class: 'barra' }, el('span', { class: 'nome' }, x.nome),
      el('span', { class: 'trilho' }, el('span', { class: 'cheio', style: `width:${Math.round(x.n / max * 100)}%` })), el('span', { class: 'n' }, x.n))));
  };
  tela.replaceChildren(
    el('section', { class: 'cartao' },
      el('div', { class: 'linha-titulo' }, el('h1', { style: 'margin:0;text-transform:capitalize' }, 'Relatório de ' + nomeMes),
        el('span', {}, el('a', { class: 'botao leve pequeno', href: '#/relatorio?mes=' + outroMes(-1) }, '◀ Mês anterior'), ' ',
          el('a', { class: 'botao leve pequeno', href: '#/relatorio?mes=' + outroMes(1) }, 'Próximo mês ▶'))),
      el('div', { class: 'numeros' }, num(r.servicos, 'serviços'), num(r.carros, 'carros atendidos'), num(r.carros_novos, 'carros novos'), num(r.do_zeus, 'vieram do Zeus'))),
    el('div', { class: 'colunas' },
      el('section', { class: 'cartao' }, el('h2', {}, 'Troca de óleo e filtros'), r.itens.length ? barras(r.itens) : el('p', { class: 'cinza' }, 'Nenhum no mês.')),
      el('section', { class: 'cartao' }, el('h2', {}, 'Peças e serviços que mais saíram'), r.outros.length ? barras(r.outros) : el('p', { class: 'cinza' }, 'Nenhum no mês.'))),
    el('section', { class: 'cartao' }, el('h2', {}, `Clientes que não voltam há mais de 6 meses (${r.sem_voltar.length})`),
      r.sem_voltar.length ? el('ul', { class: 'lista' }, r.sem_voltar.slice(0, 40).map((x) => el('li', {}, placaEl(x.placa),
        el('a', { class: 'meio', href: '#/carro/' + x.placa, style: 'color:inherit;text-decoration:none' },
          el('div', { class: 'titulo' }, x.modelo || 'Modelo não informado'), el('div', { class: 'linha2' }, `${x.cliente || ''} · última visita ${dataBR(x.ultimo)}`)))))
        : el('p', { class: 'cinza' }, 'Nenhum.'),
      el('p', { class: 'ajuda' }, 'Para chamar de volta, use a tela Lembretes.')));
}

/* --------------------------------------------- explorador do Zeus --- */
async function telaZeusAvancado() {
  const saida = el('div');
  const texto = el('input', { type: 'text', placeholder: 'Ex.: MANGUEIRA, ou o nome de um cliente' });
  const sql = el('textarea', { style: 'font-family:Consolas,monospace;font-size:.85rem;min-height:120px', placeholder: 'select * from ... limit 20' });
  function tabela(cols, linhas) {
    return el('div', { class: 'tabela-rolagem' }, el('table', { class: 'dados' }, el('thead', {}, el('tr', {}, cols.map((c) => el('th', {}, c)))),
      el('tbody', {}, linhas.map((l) => el('tr', {}, l.map((v) => el('td', { title: v }, v)))))));
  }
  async function rodar(botao, fn) { await ocupado(botao, async () => { try { await fn(); } catch (e) { saida.replaceChildren(el('p', { class: 'erro-campo' }, e.message)); } }); }
  tela.replaceChildren(el('h1', {}, 'Explorador do banco do Zeus'),
    el('p', { class: 'cinza' }, 'Só leitura. Serve para descobrir onde o Zeus guarda as vendas e montar a consulta da ligação.'),
    el('section', { class: 'cartao' },
      el('div', { class: 'acoes', style: 'margin-top:0' },
        el('button', { class: 'botao leve', onclick: (e) => rodar(e.target, async () => {
          const t = await api('zeus/tabelas');
          saida.replaceChildren(el('h2', {}, `${t.length} tabelas`), tabela(['esquema', 'tabela', 'linhas (aprox.)'], t.map((x) => [x.esquema, x.nome, milhar(x.linhas)])));
        }) }, 'Listar tabelas')),
      el('div', { class: 'campos', style: 'margin-top:14px' }, el('div', { class: 'campo c8' }, el('label', {}, 'Procurar um texto em todas as tabelas'), texto),
        el('div', { class: 'campo c4', style: 'justify-content:flex-end' }, el('button', { class: 'botao leve', onclick: (e) => rodar(e.target, async () => {
          const a = await api('zeus/procurar', { method: 'POST', corpo: { texto: texto.value } });
          saida.replaceChildren(el('h2', {}, `Achado em ${a.length} coluna(s)`), tabela(['tabela', 'coluna', 'exemplos'], a.map((x) => [x.tabela, x.coluna, x.exemplo.join(' | ')])));
        }) }, 'Procurar'))),
      el('div', { class: 'campo', style: 'margin-top:14px' }, el('label', {}, 'Consulta (SELECT)'), sql,
        el('span', { class: 'ajuda' }, 'Se tiver $1, roda como a consulta da ligação (com os últimos 7 dias) e mostra os lançamentos montados.')),
      el('div', { class: 'acoes' }, el('button', { class: 'botao preto', onclick: (e) => rodar(e.target, async () => {
        const r = await api('zeus/consulta', { method: 'POST', corpo: { sql: sql.value } });
        if (r.lancamentos) saida.replaceChildren(el('h2', {}, `${r.linhas} linha(s) → ${r.lancamentos.length} lançamento(s)`), el('pre', { class: 'codigo-bloco' }, JSON.stringify(r.lancamentos, null, 1)));
        else saida.replaceChildren(el('h2', {}, `${r.linhas.length} linha(s)`), tabela(r.colunas, r.linhas));
      }) }, 'Rodar'), el('a', { class: 'link-botao', href: '#/config' }, 'Voltar'))),
    el('section', { class: 'cartao' }, saida));
}

/* ------------------------------------- apagar registros salvos errado --- */
async function telaApagar(params) {
  const aba = params.get('aba') === 'carros' ? 'carros' : 'servicos';
  const marcar = new Set((params.get('placa') || params.get('id') || '').split(',').filter(Boolean));
  const [servicos, carros] = await Promise.all([api('servicos/recentes?limite=300'), api('veiculos?limite=5000')]);
  carros.sort((a, b) => String(b.criado_em || '').localeCompare(String(a.criado_em || '')));
  const escolhidos = { servicos: new Set(aba === 'servicos' ? marcar : []), carros: new Set(aba === 'carros' ? marcar : []) };
  let atual = aba, filtro = '';
  const busca = el('input', { type: 'search', placeholder: 'Filtrar por placa, modelo ou cliente', 'aria-label': 'Filtrar' });
  const lista = el('ul', { class: 'lista lista-selecao' });
  const todos = el('input', { type: 'checkbox', 'aria-label': 'Selecionar todos da lista' });
  const voltarZeus = el('input', { type: 'checkbox', checked: true });
  const btn = el('button', { class: 'botao vermelho', disabled: true }, 'Apagar selecionados');
  const resumo = el('span', { class: 'cinza', 'aria-live': 'polite' });
  const bServ = el('button', { type: 'button' }, 'Últimos serviços'), bCarros = el('button', { type: 'button' }, 'Carros');
  const norm = (s) => String(s || '').toUpperCase().normalize('NFD').replace(/[̀-ͯ]/g, '');

  function linhas() {
    const f = norm(filtro), fp = f.replace(/[^A-Z0-9]/g, '');
    if (atual === 'servicos') {
      return servicos.filter((s) => !f || (fp && s.placa.includes(fp)) || norm(s.modelo + ' ' + [...s.itens, ...s.outros].join(' ')).includes(f))
        .map((s) => ({ chave: s.id, conteudo: [placaEl(s.placa),
          el('div', { class: 'meio' }, el('div', { class: 'titulo' }, s.modelo || 'Modelo não informado', ' ', el('span', { class: 'cinza pequeno' }, '· serviço de ' + dataBR(s.data) + (s.km ? ' · ' + milhar(s.km) + ' km' : ''))),
            el('div', { class: 'linha2' }, [...s.itens, ...s.outros].join(', ') || '—'),
            el('div', { class: 'linha2' }, 'Salvo ' + quando(s.criado_em))),
          s.origem === 'zeus' ? el('span', { class: 'etiq zeus' }, 'Zeus') : null] }));
    }
    return carros.filter((c) => !f || (fp && c.placa.includes(fp)) || norm(c.modelo + ' ' + c.cliente).includes(f))
      .map((c) => ({ chave: c.placa, conteudo: [placaEl(c.placa),
        el('div', { class: 'meio' }, el('div', { class: 'titulo' }, c.modelo || 'Modelo não informado', c.ano ? ' ' + c.ano : '', c.cliente ? el('span', { class: 'cinza pequeno' }, ' · ' + c.cliente) : ''),
          el('div', { class: 'linha2' }, `${c.servicos} serviço(s)` + (c.ultimo ? ', o último em ' + dataBR(c.ultimo) : '') + ' · cadastrado ' + quando(c.criado_em))),
        el('a', { class: 'link-botao pequeno', href: '#/carro/' + c.placa }, 'Ver')] }));
  }
  function desenhar() {
    bServ.setAttribute('aria-pressed', atual === 'servicos'); bCarros.setAttribute('aria-pressed', atual === 'carros');
    const sel = escolhidos[atual];
    const ls = linhas();
    lista.replaceChildren(...(ls.length ? ls.map((l) => {
      const cb = el('input', { type: 'checkbox', checked: sel.has(l.chave), 'aria-label': 'Selecionar' });
      const li = el('li', { class: sel.has(l.chave) ? 'marcado' : null }, el('label', { class: 'marcar' }, cb), ...l.conteudo);
      cb.addEventListener('change', () => { cb.checked ? sel.add(l.chave) : sel.delete(l.chave); li.classList.toggle('marcado', cb.checked); contar(); });
      return li;
    }) : [el('li', { class: 'cinza' }, 'Nada encontrado.')]));
    todos.checked = ls.length > 0 && ls.every((l) => sel.has(l.chave));
    contar();
  }
  function contar() {
    const ns = escolhidos.servicos.size, nc = escolhidos.carros.size;
    const partes = [];
    if (ns) partes.push(`${ns} serviço(s)`);
    if (nc) partes.push(`${nc} carro(s)`);
    resumo.textContent = partes.length ? 'Selecionado: ' + partes.join(' e ') : 'Nada selecionado';
    btn.disabled = !(ns || nc);
    btn.textContent = ns || nc ? `Apagar ${partes.join(' e ')}` : 'Apagar selecionados';
  }
  todos.addEventListener('change', () => { const sel = escolhidos[atual]; linhas().forEach((l) => (todos.checked ? sel.add(l.chave) : sel.delete(l.chave))); desenhar(); });
  bServ.addEventListener('click', () => { atual = 'servicos'; desenhar(); });
  bCarros.addEventListener('click', () => { atual = 'carros'; desenhar(); });
  let t; busca.addEventListener('input', () => { clearTimeout(t); t = setTimeout(() => { filtro = busca.value.trim(); desenhar(); }, 150); });
  btn.addEventListener('click', async () => {
    const ids = [...escolhidos.servicos], placas = [...escolhidos.carros];
    const servDosCarros = carros.filter((c) => escolhidos.carros.has(c.placa)).reduce((n, c) => n + c.servicos, 0);
    let msg = 'Apagar de vez?\n';
    if (ids.length) msg += `\n• ${ids.length} serviço(s)`;
    if (placas.length) msg += `\n• ${placas.length} carro(s): ${placas.map(placaBonita).join(', ')} — com ${servDosCarros} serviço(s) deles. A etiqueta com QR Code desses carros deixa de mostrar o histórico.`;
    msg += '\n\nSome também da página do QR Code. Fica só na cópia de segurança do dia.';
    if (!confirm(msg)) return;
    await ocupado(btn, async () => {
      try {
        const r = await api('apagar', { method: 'POST', corpo: { servicos: ids, carros: placas, ignorar_zeus: !voltarZeus.checked } });
        r.voltaram.forEach((ref) => naoRedirecionar.add(ref));
        aviso(`Apagado: ${r.servicos} serviço(s)` + (r.carros ? ` e ${r.carros} carro(s)` : '') + (r.voltaram.length ? `. ${r.voltaram.length} venda(s) do Zeus voltaram para “Para revisar”.` : '.'));
        atualizarEstado();
        const q = new URLSearchParams({ aba: atual });
        if (location.hash === '#/apagar?' + q) navegar(); else location.hash = '#/apagar?' + q;
      } catch (e) { aviso(e.message, true); }
    });
  });
  tela.replaceChildren(el('section', { class: 'cartao destaque' },
    el('h1', {}, 'Apagar o que foi salvo errado'),
    el('p', { class: 'cinza', style: 'margin-top:0' }, 'Marque os serviços ou os carros e clique em apagar. Apagar um carro apaga também todos os serviços dele. Para só corrigir (placa, KM, peças), use “Corrigir” na tela do carro.'),
    el('div', { class: 'linha-titulo', style: 'flex-wrap:wrap' }, el('div', { class: 'escolha', role: 'group', 'aria-label': 'O que apagar' }, bServ, bCarros), busca),
    el('div', { class: 'barra-selecao' }, el('label', { class: 'marcar-todos' }, todos, ' Selecionar todos da lista'), resumo),
    lista,
    el('label', { class: 'interruptor', style: 'margin-top:16px' }, voltarZeus, 'Se veio do Zeus, a venda volta para “Para revisar” (desligado: a venda fica ignorada)'),
    el('div', { class: 'acoes' }, btn, el('a', { class: 'link-botao', href: '#/' }, 'Voltar'))));
  busca.style.maxWidth = '360px';
  desenhar();
}

/* ------------------------------------- redirecionamento automático --- */
// Quando chega venda nova do Zeus (1 minuto depois de terminada no balcão),
// a tela vai sozinha para "Revisar" dela — a não ser que alguém esteja
// preenchendo outra coisa; aí só aparece um aviso com o botão para abrir.
let telaSucessoAberta = false;
let refsPendentes = null; // refs que já estavam esperando revisão
let chaveNovos = '';
let buscandoNovos = false;
const naoRedirecionar = new Set();

function podeSairDaTela() {
  const q = $('#q');
  if (document.activeElement === q && q.value.trim()) return false;
  if (telaSucessoAberta) return true;
  const h = location.hash || '#/';
  return /^#?\/?$|^#\/(lembretes|relatorio|termos)(\?.*)?$|^#\/carro\//.test(h);
}

async function verNovosDoZeus() {
  if (ESTADO.somente_leitura) return; // quem revisa é o computador principal
  const z = ESTADO.zeus || {};
  const chave = (z.pendentes || 0) + '|' + (ESTADO.alteracao || '');
  if (buscandoNovos || chave === chaveNovos) return;
  buscandoNovos = true;
  try {
    const lista = await api('zeus/lancamentos?status=pendente');
    chaveNovos = chave;
    const antes = refsPendentes;
    refsPendentes = new Set(lista.map((l) => l.ref));
    if (!antes) return; // primeira vez: o que já estava esperando não redireciona
    const novos = lista.filter((l) => !antes.has(l.ref) && !naoRedirecionar.has(l.ref))
      .sort((a, b) => String(a.recebido_em || '').localeCompare(String(b.recebido_em || '')));
    lista.forEach((l) => naoRedirecionar.delete(l.ref));
    if (novos.length) redirecionarPara(novos[0], novos.length);
  } catch (e) { /* tenta na próxima */ } finally { buscandoNovos = false; }
}

function descreverVenda(l) {
  return [l.documento, [l.modelo, l.placa ? placaBonita(l.placa) : ''].filter(Boolean).join(' '), l.cliente].filter(Boolean).join(' — ');
}

function redirecionarPara(l, qtd) {
  const destino = '#/revisar/' + encodeURIComponent(l.ref);
  const mais = qtd > 1 ? ` (+${qtd - 1} na lista)` : '';
  if (location.hash === destino) return;
  if (podeSairDaTela()) {
    location.hash = destino;
    aviso('Chegou do Zeus: ' + descreverVenda(l) + mais);
    try { window.focus(); } catch (e) { /* tudo bem */ }
    return;
  }
  // está preenchendo algo: não tira a pessoa da tela
  const a = el('div', { class: 'aviso', role: 'status', style: 'border-left-color:#E8A100' },
    'Chegou do Zeus: ' + descreverVenda(l) + mais + ' ',
    el('a', { href: destino, class: 'botao vermelho pequeno', style: 'margin-left:8px', onclick: () => a.remove() }, 'Revisar agora'),
    el('button', { class: 'link-botao', style: 'color:#fff;margin-left:10px', onclick: () => a.remove() }, 'Depois'));
  $('#avisos').append(a);
  setTimeout(() => a.remove(), 60000);
}

// depois de confirmar uma venda do Zeus: se tem outra esperando, abre ela sozinha
let contagem = null;
function pararContagem() { if (contagem) { clearInterval(contagem.t); contagem.caixa.remove(); contagem = null; } }
async function proximoDoZeus() {
  let lista = [];
  try { lista = await api('zeus/lancamentos?status=pendente'); } catch (e) { return; }
  if (!lista.length || !telaSucessoAberta) return;
  lista.sort((a, b) => String(a.recebido_em || '').localeCompare(String(b.recebido_em || '')));
  const l = lista[0];
  let s = 6;
  const txt = el('span', {});
  const caixa = el('div', { class: 'alerta-data proximo' }, txt, ' ',
    el('a', { class: 'botao vermelho pequeno', href: '#/revisar/' + encodeURIComponent(l.ref) }, 'Abrir agora'),
    el('button', { type: 'button', class: 'botao leve pequeno', onclick: (e) => { e.stopPropagation(); pararContagem(); } }, 'Ficar aqui'));
  const sec = tela.querySelector('.sucesso');
  if (!sec) return;
  sec.prepend(caixa);
  const escrever = () => { txt.textContent = `Ainda ${lista.length === 1 ? 'tem 1 venda' : 'tem ' + lista.length + ' vendas'} do Zeus para revisar. Abrindo ${descreverVenda(l)} em ${s} s…`; };
  escrever();
  contagem = { caixa, t: setInterval(() => {
    s--; escrever();
    if (s <= 0) { pararContagem(); location.hash = '#/revisar/' + encodeURIComponent(l.ref); }
  }, 1000) };
  // mexeu na tela (ex.: imprimir de novo): para a contagem
  tela.addEventListener('pointerdown', (e) => { if (!caixa.contains(e.target)) pararContagem(); }, { once: true });
}

/* ------------------------------------------- termo de peça do cliente --- */
// O cliente comprou a peça por conta própria: assina que a oficina responde
// só pela mão de obra. Fica guardado no programa e sai impresso em 2 vias.
let MODELO_TERMO = null;
async function modeloTermo(recarregar) {
  if (!MODELO_TERMO || recarregar) MODELO_TERMO = await api('termo-modelo');
  return MODELO_TERMO;
}
const numTermo = (n) => 'Nº ' + String(n || 0).padStart(4, '0');
function dataExtenso(iso) {
  const d = new Date((iso || hoje()) + 'T12:00:00');
  return isNaN(d) ? dataBR(iso) : d.toLocaleDateString('pt-BR', { day: 'numeric', month: 'long', year: 'numeric' });
}
function etiqTermo(t) {
  return t.assinado ? el('span', { class: 'etiq', style: 'background:#E3F3E6;color:#1E7B34' }, '✓ Assinado' + (t.assinado_papel && !t.assinatura ? ' no papel' : ''))
    : el('span', { class: 'etiq aviso-etiq' }, '✍ Falta assinar');
}

function listaTermos(lista) {
  return el('ul', { class: 'lista' }, lista.map((t) => el('li', {},
    el('a', { class: 'meio', href: '#/termo/' + t.id, style: 'color:inherit;text-decoration:none' },
      el('div', { class: 'titulo' }, numTermo(t.numero), ' · ', t.cliente, ' ', el('span', { class: 'cinza pequeno' }, '· ' + dataBR(t.data))),
      el('div', { class: 'linha2' }, (t.modelo ? t.modelo + ' · ' : '') + t.pecas.join(', '))),
    t.placa ? placaEl(t.placa) : null,
    etiqTermo(t),
    el('a', { class: 'botao leve pequeno', href: '#/termo/' + t.id }, 'Abrir'))));
}

async function telaTermos(params) {
  const q = params.get('q') || '';
  const [lista, modelo] = await Promise.all([api('termos?q=' + encodeURIComponent(q)), modeloTermo()]);
  const busca = el('input', { type: 'search', value: q, placeholder: 'Buscar por cliente, placa, peça ou número', 'aria-label': 'Buscar termo' });
  let t;
  busca.addEventListener('input', () => { clearTimeout(t); t = setTimeout(async () => {
    const l = await api('termos?q=' + encodeURIComponent(busca.value.trim()));
    caixaLista.replaceChildren(l.length ? listaTermos(l) : el('p', { class: 'vazio' }, 'Nenhum termo encontrado.'));
  }, 200); });
  const faltam = lista.filter((x) => !x.assinado).length;
  const caixaLista = el('div', {}, lista.length ? listaTermos(lista)
    : el('div', { class: 'vazio' }, el('p', {}, 'Nenhum termo feito ainda.'), el('a', { class: 'botao vermelho', href: '#/termo/novo' }, 'Fazer o primeiro termo')));

  // texto do termo (editável)
  const cab = el('input', { type: 'text', value: modelo.cabecalho });
  const cl = el('textarea', { style: 'min-height:220px' }, modelo.clausulas);
  const textoTermo = el('details', { class: 'cartao escrita' }, el('summary', { style: 'cursor:pointer;font-weight:800' }, 'Texto do termo (cabeçalho e cláusulas)'),
    el('div', { class: 'campos', style: 'margin-top:14px' },
      el('div', { class: 'campo' }, el('label', {}, 'Cabeçalho (nome da oficina, cidade, contato)'), cab),
      el('div', { class: 'campo' }, el('label', {}, 'Cláusulas — uma por linha (o programa numera)'), cl,
        el('span', { class: 'ajuda' }, 'Vale para os termos novos e para as próximas impressões. Se apagar tudo, volta o texto padrão.')),
      el('div', { class: 'campo' }, el('div', { class: 'acoes', style: 'margin:0' },
        el('button', { class: 'botao preto', onclick: async (e) => {
          await ocupado(e.target, async () => {
            try { MODELO_TERMO = Object.assign({}, MODELO_TERMO, await api('termo-modelo', { method: 'PUT', corpo: { cabecalho: cab.value, clausulas: cl.value } })); cab.value = MODELO_TERMO.cabecalho; cl.value = MODELO_TERMO.clausulas; aviso('Texto do termo salvo'); }
            catch (er) { aviso(er.message, true); }
          });
        } }, 'Salvar texto'),
        el('button', { class: 'link-botao', onclick: () => { cab.value = modelo.cabecalho_padrao; cl.value = modelo.clausulas_padrao; aviso('Texto padrão de volta na tela. Clique em “Salvar texto” para usar.'); } }, 'Voltar ao texto padrão')))));

  tela.replaceChildren(
    el('section', { class: 'cartao destaque' },
      el('div', { class: 'linha-titulo' }, el('h1', { style: 'margin:0' }, 'Termo de peça do cliente'),
        el('a', { class: 'botao vermelho', href: '#/termo/novo' }, '+ Novo termo')),
      el('p', { class: 'cinza', style: 'margin-top:0' }, 'Quando o cliente compra a peça por conta própria, ele assina este termo: a oficina responde só pela mão de obra. O cliente pode assinar na tela ou no papel (sai em 2 vias: oficina e cliente).'),
      faltam ? el('div', { class: 'alerta-data' }, `✍ ${faltam} termo(s) ainda sem assinatura.`) : null,
      el('div', { style: 'margin:12px 0' }, busca),
      caixaLista),
    textoTermo);
}

function padAssinatura(inicial, aoMudar) {
  const cv = el('canvas', { class: 'assinatura-pad', width: 900, height: 260, role: 'img', 'aria-label': 'Área para o cliente assinar com o mouse, caneta ou dedo' });
  const ctx = cv.getContext('2d');
  let vazio = true, desenhando = false, ultimo = null;
  const pincel = () => { ctx.lineWidth = 3.4; ctx.lineCap = 'round'; ctx.lineJoin = 'round'; ctx.strokeStyle = '#111'; ctx.fillStyle = '#111'; };
  pincel();
  if (inicial) { const im = new Image(); im.onload = () => { ctx.drawImage(im, 0, 0, cv.width, cv.height); vazio = false; }; im.src = inicial; }
  const pos = (e) => { const r = cv.getBoundingClientRect(); return [(e.clientX - r.left) * cv.width / r.width, (e.clientY - r.top) * cv.height / r.height]; };
  cv.addEventListener('pointerdown', (e) => {
    e.preventDefault(); cv.setPointerCapture(e.pointerId); desenhando = true; ultimo = pos(e);
    ctx.beginPath(); ctx.arc(ultimo[0], ultimo[1], 1.7, 0, Math.PI * 2); ctx.fill();
    if (vazio) { vazio = false; aoMudar && aoMudar(); }
  });
  cv.addEventListener('pointermove', (e) => {
    if (!desenhando) return;
    const p = pos(e);
    ctx.beginPath(); ctx.moveTo(ultimo[0], ultimo[1]); ctx.lineTo(p[0], p[1]); ctx.stroke(); ultimo = p;
  });
  const fim = () => { desenhando = false; };
  cv.addEventListener('pointerup', fim); cv.addEventListener('pointercancel', fim);
  return {
    el: cv,
    limpar() { ctx.clearRect(0, 0, cv.width, cv.height); vazio = true; pincel(); aoMudar && aoMudar(); },
    vazio: () => vazio,
    png: () => (vazio ? '' : cv.toDataURL('image/png')),
  };
}

async function telaTermoForm(id, params) {
  let t = { data: hoje(), pecas: [] };
  if (id) t = await api('termos/' + id);
  else {
    const placa = soPlaca(params.get('placa') || ''), ref = params.get('ref');
    if (ref) {
      try {
        const l = await api('zeus/lancamento/' + encodeURIComponent(ref));
        Object.assign(t, { cliente: l.cliente || '', telefone: l.telefone || '', placa: l.placa || '', km: l.km || '', modelo: l.modelo || '', ano: l.ano || '', data: l.data || hoje(), zeus_ref: l.ref });
      } catch (e) { /* sem a venda: começa em branco */ }
    }
    if (placa) t.placa = placa;
    if (t.placa) {
      try {
        const c = await api('veiculos/' + t.placa + '?opcional=1');
        if (c.veiculo) {
          const v = c.veiculo;
          t.modelo = t.modelo || v.modelo || ''; t.ano = t.ano || v.ano || '';
          t.cliente = t.cliente || v.cliente || ''; t.telefone = t.telefone || v.telefone || '';
          if (!t.km && c.servicos[0] && c.servicos[0].km) t.km = c.servicos[0].km;
        }
      } catch (e) { /* carro novo */ }
    }
  }
  const f = {
    cliente: el('input', { type: 'text', id: 't-cliente', value: t.cliente || '', autocomplete: 'off', placeholder: 'Nome completo do cliente' }),
    doc: el('input', { type: 'text', id: 't-doc', value: t.documento || '', autocomplete: 'off', placeholder: '000.000.000-00' }),
    fone: el('input', { type: 'text', id: 't-fone', value: t.telefone || '', autocomplete: 'off', placeholder: '(79) 99999-9999' }),
    placa: el('input', { type: 'text', id: 't-placa', class: 'placa-campo', maxlength: 8, value: placaBonita(t.placa || ''), autocomplete: 'off', placeholder: 'ABC1D23' }),
    modelo: el('input', { type: 'text', id: 't-modelo', value: t.modelo || '', placeholder: 'Ex.: Fiat Uno' }),
    ano: el('input', { type: 'text', id: 't-ano', value: t.ano || '', maxlength: 9, inputmode: 'numeric' }),
    km: el('input', { type: 'text', id: 't-km', value: t.km ? milhar(t.km) : '', inputmode: 'numeric' }),
    data: el('input', { type: 'date', id: 't-data', value: t.data || hoje() }),
    pecas: el('textarea', { id: 't-pecas', placeholder: 'Uma peça por linha. Ex.:\nBomba d\'água (comprada na loja X)\nJogo de pastilhas dianteiras' }, (t.pecas || []).join('\n')),
    servico: el('textarea', { id: 't-servico', style: 'min-height:70px', placeholder: 'Ex.: Troca da bomba d\'água' }, t.servico || ''),
    obs: el('input', { type: 'text', id: 't-obs', value: t.obs || '', placeholder: 'Opcional. Ex.: peça usada / sem nota fiscal' }),
    papel: el('input', { type: 'checkbox', checked: !!t.assinado_papel }),
  };
  f.km.addEventListener('input', () => { const n = numero(f.km.value); f.km.value = n ? milhar(n) : ''; });
  f.placa.addEventListener('input', () => { f.placa.value = soPlaca(f.placa.value); });
  f.placa.addEventListener('blur', async () => {
    f.placa.value = placaBonita(f.placa.value);
    const p = soPlaca(f.placa.value);
    if (!placaValida(p)) return;
    try {
      const c = await api('veiculos/' + p + '?opcional=1');
      if (c.veiculo) {
        if (!f.modelo.value) f.modelo.value = c.veiculo.modelo || '';
        if (!f.ano.value) f.ano.value = c.veiculo.ano || '';
        if (!f.cliente.value) f.cliente.value = c.veiculo.cliente || '';
        if (!f.fone.value) f.fone.value = c.veiculo.telefone || '';
      }
    } catch (e) { /* carro novo */ }
  });
  const situacao = el('div', { class: 'ajuda', 'aria-live': 'polite' });
  const pad = padAssinatura(t.assinatura || '', () => atualizarSituacao());
  function atualizarSituacao() {
    situacao.textContent = !pad.vazio() ? '✓ Assinado na tela.' : f.papel.checked ? '✓ Assinado no papel.' : 'Ainda sem assinatura: o cliente pode assinar aqui ou na via impressa.';
  }
  f.papel.addEventListener('change', atualizarSituacao);
  setTimeout(atualizarSituacao, 50);

  function coletar() {
    return {
      cliente: f.cliente.value.trim(), documento: f.doc.value.trim(), telefone: f.fone.value.trim(),
      placa: soPlaca(f.placa.value), modelo: f.modelo.value.trim(), ano: f.ano.value.trim(), km: numero(f.km.value),
      data: f.data.value || hoje(), pecas: f.pecas.value.split('\n').map((s) => s.trim()).filter(Boolean),
      servico: f.servico.value.trim(), obs: f.obs.value.trim(), assinatura: pad.png(), assinado_papel: f.papel.checked,
      zeus_ref: t.zeus_ref || '', assinado_em: t.assinado_em,
    };
  }
  async function salvar(botao, imprimirDepois) {
    const d = coletar();
    if (!d.cliente) { aviso('Escreva o nome do cliente.', true); f.cliente.focus(); return; }
    if (!d.pecas.length) { aviso('Escreva a peça (ou peças) que o cliente trouxe.', true); f.pecas.focus(); return; }
    if (d.placa && !placaValida(d.placa)) { aviso('Placa inválida. Use o formato ABC1D23 ou ABC1234 (ou deixe em branco).', true); f.placa.focus(); return; }
    if (!d.assinatura) delete d.assinado_em; // tirou a assinatura da tela
    await ocupado(botao, async () => {
      try {
        const salvo = await api(id ? 'termos/' + id : 'termos', { method: id ? 'PUT' : 'POST', corpo: d });
        aviso(`Termo ${numTermo(salvo.numero)} salvo`);
        location.hash = '#/termo/' + salvo.id;
        if (imprimirDepois) setTimeout(async () => { try { imprimirTermo(await api('termos/' + salvo.id)); } catch (e) { aviso(e.message, true); } }, 400);
      } catch (e) { aviso(e.message, true); }
    });
  }
  const campo = (cls, rot, input, ajuda) => el('div', { class: 'campo ' + cls }, el('label', { for: input.id }, rot), input, ajuda ? el('span', { class: 'ajuda' }, ajuda) : null);
  tela.replaceChildren(el('section', { class: 'cartao destaque' },
    el('h1', {}, id ? `Corrigir termo ${numTermo(t.numero)}` : 'Novo termo de peça do cliente'),
    el('p', { class: 'cinza', style: 'margin-top:0' }, 'O cliente comprou a peça por conta própria. Com este termo ele declara que a oficina é responsável só pela mão de obra.'),
    el('div', { class: 'campos' },
      campo('c6', 'Cliente *', f.cliente), campo('c3', 'CPF ou RG', f.doc), campo('c3', 'Telefone', f.fone),
      campo('c3', 'Placa', f.placa), campo('c4', 'Modelo', f.modelo), campo('c2', 'Ano', f.ano), campo('c3', 'KM', f.km),
      campo('c4', 'Data', f.data),
      campo('c6', 'Peça(s) que o cliente trouxe *', f.pecas, 'Uma por linha. Se souber, escreva marca e onde foi comprada.'),
      campo('c6', 'Serviço que a oficina vai fazer (mão de obra)', f.servico),
      campo('', 'Observação', f.obs),
      el('div', { class: 'campo' }, el('span', { class: 'rotulo' }, 'Assinatura do cliente'),
        el('div', { class: 'assinatura-caixa' }, pad.el, el('div', { class: 'assinatura-linha' }, 'Assine aqui com o mouse, a caneta ou o dedo')),
        el('div', { class: 'acoes', style: 'margin-top:6px' },
          el('button', { type: 'button', class: 'botao leve pequeno', onclick: () => pad.limpar() }, 'Limpar assinatura'),
          el('label', { class: 'interruptor' }, f.papel, 'O cliente assinou no papel')),
        situacao)),
    el('div', { class: 'acoes' },
      el('button', { type: 'button', class: 'botao vermelho grande', 'data-principal': '1', title: 'Atalho: Ctrl+Enter', onclick: (e) => salvar(e.target, true) }, 'Salvar e imprimir (2 vias)'),
      el('button', { type: 'button', class: 'botao leve', onclick: (e) => salvar(e.target, false) }, 'Só salvar'),
      el('a', { class: 'link-botao', href: id ? '#/termo/' + id : '#/termos' }, 'Cancelar'))));
  setTimeout(() => (f.cliente.value ? f.pecas : f.cliente).focus(), 0);
}

function documentoTermo(t, m, via) {
  const nomeOficina = (m.cabecalho || '').split(/\s+[—–-]\s+/)[0] || 'Oficina';
  const cidade = /itabaiana/i.test(m.cabecalho || '') ? 'Itabaiana/SE' : '';
  const clausulas = String(m.clausulas || '').split('\n').map((s) => s.trim()).filter(Boolean);
  const veiculo = [[t.modelo, t.ano].filter(Boolean).join(' '), t.placa ? 'placa ' + placaBonita(t.placa) : '', t.km ? milhar(t.km) + ' km' : ''].filter(Boolean).join(' · ');
  const linha = (rot, val) => el('div', {}, el('span', {}, rot), val || '—');
  return el('article', { class: 'termo-doc' },
    el('header', { class: 'termo-cab' }, el('img', { src: 'logo.png', alt: '' }),
      el('div', {}, el('strong', {}, m.cabecalho || ''), via ? el('div', { class: 'termo-via' }, via) : null)),
    el('h1', {}, 'Termo de responsabilidade'),
    el('p', { class: 'termo-sub' }, 'Peça fornecida pelo cliente — a oficina responde somente pela mão de obra'),
    el('p', { class: 'termo-num' }, numTermo(t.numero), ' · ', dataBR(t.data)),
    el('div', { class: 'termo-dados' },
      linha('Cliente', t.cliente), linha('CPF / RG', t.documento), linha('Telefone', t.telefone), linha('Veículo', veiculo)),
    el('h3', {}, 'Peça(s) fornecida(s) pelo cliente'),
    el('ul', {}, (t.pecas || []).map((p) => el('li', {}, p))),
    t.servico ? [el('h3', {}, 'Serviço a ser feito pela oficina (mão de obra)'), el('p', {}, t.servico)] : null,
    t.obs ? el('p', {}, el('strong', {}, 'Observação: '), t.obs) : null,
    el('h3', {}, 'Declaração do cliente'),
    el('ol', { class: 'termo-clausulas' }, clausulas.map((c) => el('li', {}, c))),
    el('p', {}, 'Por estar de acordo, assino este termo em duas vias de igual teor.'),
    el('p', { class: 'termo-local' }, (cidade ? cidade + ', ' : '') + dataExtenso(t.data) + '.'),
    el('div', { class: 'termo-assinaturas' },
      el('div', {}, t.assinatura ? el('img', { class: 'termo-assin-img', src: t.assinatura, alt: 'Assinatura do cliente' }) : el('div', { class: 'termo-assin-vazia' }),
        el('div', { class: 'termo-linha' }), el('strong', {}, t.cliente || 'Cliente'), el('div', {}, t.documento ? 'CPF/RG: ' + t.documento : 'Cliente')),
      el('div', {}, el('div', { class: 'termo-assin-vazia' }), el('div', { class: 'termo-linha' }), el('strong', {}, nomeOficina), el('div', {}, 'Oficina'))),
    t.assinatura && t.assinado_em ? el('p', { class: 'termo-rodape' }, `Assinado pelo cliente na tela do computador da oficina em ${dataBR(String(t.assinado_em).slice(0, 10))} às ${horaBR(t.assinado_em)}.`) : null);
}

async function imprimirTermo(t) {
  const m = await modeloTermo();
  document.getElementById('impressao')?.remove();
  const area = el('div', { id: 'impressao' }, documentoTermo(t, m, 'Via da oficina'), documentoTermo(t, m, 'Via do cliente'));
  document.body.append(area);
  await Promise.all([...area.querySelectorAll('img')].map((i) => (i.decode ? i.decode().catch(() => {}) : null)));
  const tirar = () => setTimeout(() => area.remove(), 500);
  window.addEventListener('afterprint', tirar, { once: true });
  document.title = 'Termo ' + numTermo(t.numero) + ' - ' + (t.cliente || '');
  window.print();
  atualizarEstado();
}

async function telaTermo(id) {
  const [t, m] = await Promise.all([api('termos/' + id), modeloTermo()]);
  const acoes = el('div', { class: 'acoes', style: 'margin-top:0' },
    el('button', { class: 'botao vermelho', 'data-principal': '1', onclick: () => imprimirTermo(t) }, 'Imprimir (2 vias)'),
    el('a', { class: 'botao leve', href: `#/termo/${t.id}/editar` }, t.assinado ? 'Corrigir' : 'Corrigir / assinar na tela'),
    !t.assinado ? el('button', { class: 'botao leve escrita', onclick: async (e) => {
      await ocupado(e.target, async () => {
        try { await api('termos/' + t.id, { method: 'PUT', corpo: Object.assign({}, t, { assinado_papel: true }) }); aviso('Marcado como assinado no papel'); navegar(); }
        catch (er) { aviso(er.message, true); }
      });
    } }, 'O cliente assinou no papel') : null,
    t.placa ? el('a', { class: 'botao leve', href: '#/carro/' + t.placa }, 'Ver o carro') : null,
    el('button', { class: 'link-botao escrita', onclick: async () => {
      if (!confirm(`Apagar o termo ${numTermo(t.numero)} de ${t.cliente}? Isso não tem volta.`)) return;
      await api('termos/' + t.id, { method: 'DELETE' }); aviso('Termo apagado'); location.hash = '#/termos';
    } }, 'Apagar'));
  tela.replaceChildren(
    el('section', { class: 'cartao destaque' },
      el('div', { class: 'linha-titulo' }, el('h1', { style: 'margin:0' }, `Termo ${numTermo(t.numero)}`), etiqTermo(t)),
      !t.assinado ? el('div', { class: 'alerta-data' }, '✍ Falta a assinatura do cliente. Imprima e peça para ele assinar, ou clique em “Corrigir / assinar na tela”.') : null,
      acoes, el('p', { class: 'pequeno', style: 'margin:12px 0 0' }, el('a', { href: '#/termos' }, '← Todos os termos'))),
    el('div', { class: 'termo-folha' }, documentoTermo(t, m, '')));
}

/* ------------------------------------------------------------ início --- */
let ultimaAlteracao = '';
async function vigiar() {
  await atualizarEstado();
  await verNovosDoZeus();
  // se chegou coisa nova (ex.: do Zeus) e estamos no início, atualiza a lista
  if (ESTADO.alteracao && ESTADO.alteracao !== ultimaAlteracao) {
    const primeira = !ultimaAlteracao;
    ultimaAlteracao = ESTADO.alteracao;
    if (!primeira && (location.hash === '' || location.hash === '#/') && !tela.contains(document.activeElement)) navegar();
  }
}
/* ---------------------------------------------- acessibilidade e atalhos --- */
(function acessibilidade() {
  const raiz = document.documentElement;
  let tam = 16, contraste = false;
  try { tam = parseInt(localStorage.getItem('oqr-letra'), 10) || 16; contraste = localStorage.getItem('oqr-contraste') === '1'; } catch (e) { /* sem memória */ }
  const aplicar = () => {
    raiz.style.fontSize = tam + 'px';
    raiz.classList.toggle('contraste', contraste);
    $('#a-contraste').setAttribute('aria-pressed', contraste ? 'true' : 'false');
    try { localStorage.setItem('oqr-letra', String(tam)); localStorage.setItem('oqr-contraste', contraste ? '1' : '0'); } catch (e) { /* tudo bem */ }
  };
  $('#a-menos').addEventListener('click', () => { tam = Math.max(14, tam - 2); aplicar(); });
  $('#a-mais').addEventListener('click', () => { tam = Math.min(26, tam + 2); aplicar(); });
  $('#a-contraste').addEventListener('click', () => { contraste = !contraste; aplicar(); });
  aplicar();
})();
document.addEventListener('keydown', (e) => {
  if (e.key === 'F2') { e.preventDefault(); if (!ESTADO.somente_leitura) location.hash = '#/novo'; }
  else if (e.key === 'F3') { e.preventDefault(); $('#q').focus(); }
  else if (e.key === 'F4') { e.preventDefault(); location.hash = '#/'; }
  else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    const b = document.querySelector('[data-principal]');
    if (b && !b.disabled) { e.preventDefault(); b.click(); }
  }
});

(async () => {
  api('atualizacao').then((r) => { ATUALIZACAO = r; atualizarEstado(); }).catch(() => {});
  await atualizarEstado();
  ultimaAlteracao = ESTADO.alteracao || '';
  navegar();
  setInterval(vigiar, 5000);
})();

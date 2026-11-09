// Pacote servidor junta tudo: guarda os dados, atende as telas (que abrem numa
// janela do Edge/Chrome) e liga o Zeus, a impressora e o site.
package servidor

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"oficinaqr/internal/atualizar"
	"oficinaqr/internal/dados"
	"oficinaqr/internal/etiqueta"
	"oficinaqr/internal/impressora"
	"oficinaqr/internal/itens"
	"oficinaqr/internal/registro"
	"oficinaqr/internal/sistema"
	"oficinaqr/internal/site"
	"oficinaqr/internal/zeus"
)

const Versao = "1.6.1"

type Servidor struct {
	Banco    *dados.Banco
	cfg      *configs
	Site     *site.Publicador
	Monitor  *zeus.Monitor
	telas    fs.FS
	porta    int
	Parar    chan struct{}
	pararUma sync.Once
	// para os testes trocarem a impressão
	Imprimir func(impressora.Trabalho) error

	muImp     sync.Mutex
	ultimoErr string
	// depois de atualizar: caminho do programa para abrir de novo ao fechar
	Reabrir string

	muAviso sync.Mutex
	avisos  []string
	refs    []string
	// última vez que a janela do programa perguntou o estado (a janela está aberta?)
	ultimaTela atomic.Int64
	// para os testes: abrir a janela do programa
	AbrirJanela func(url string) error

	mux       *http.ServeMux
	muRede    sync.Mutex
	rede      *http.Server
	proxy     *httputil.ReverseProxy
	proxyPara string
}

func Novo(pasta string, telas fs.FS) (*Servidor, error) {
	b, err := dados.Abrir(pasta)
	if err != nil {
		return nil, err
	}
	cs, err := abrirConfig(pasta)
	if err != nil {
		return nil, err
	}
	s := &Servidor{Banco: b, cfg: cs, telas: telas, Parar: make(chan struct{}), Imprimir: impressora.Imprimir, AbrirJanela: sistema.AbrirJanela}
	s.Site = site.Novo(b, cs.Site)
	s.Site.Opcoes = s.opcoesPublico
	// para testes
	if v := os.Getenv("OFICINAQR_GITHUB_API"); v != "" {
		s.Site.API = v
		atualizar.API = v
	}
	if v, _ := strconv.Atoi(os.Getenv("OFICINAQR_ESPERA_MS")); v > 0 {
		s.Site.Espera = time.Duration(v) * time.Millisecond
	}
	s.Monitor = zeus.NovoMonitor(b, cs.Zeus)
	// nada do Zeus é registrado nem impresso sozinho: tudo espera a revisão
	s.Monitor.AoChegar = nil
	s.Monitor.AoAtualizar = s.revisarDeNovo
	s.Monitor.AoNovo = s.avisarNovo
	b.CopiaExtra = cs.Ler().PastaCopia
	// versão 1.2.1: passa os nomes dos serviços já gravados pelas regras novas (uma vez)
	if n, err := b.RevisarNomes(2, itens.Revisar); err != nil {
		registro.Erro("revisar nomes antigos: %v", err)
	} else if n > 0 {
		registro.Info("nomes de %d serviço(s) antigos acertados pelas regras novas", n)
	}
	// 1.2.2: modelo (sem o nome do dono) e cliente genérico dos que esperam revisão
	if err := b.RecalcularPendentes(3, zeus.LimparNome); err != nil {
		registro.Erro("recalcular pendentes: %v", err)
	}
	if n, err := b.RevisarNomes(3, itens.Revisar); err != nil {
		registro.Erro("revisar nomes antigos: %v", err)
	} else if n > 0 {
		registro.Info("nomes de %d serviço(s) antigos acertados pelas regras novas", n)
	}
	return s, nil
}

// Rodar começa os trabalhos de fundo (site e Zeus), protegidos contra panes.
func (s *Servidor) Rodar() {
	registro.Proteger("publicação do site", s.Parar, func() { s.Site.Rodar(s.Parar) })
	registro.Proteger("leitura do Zeus", s.Parar, func() { s.Monitor.Rodar(s.Parar) })
	s.ajustarRede()
}

func (s *Servidor) Fechar() {
	s.pararUma.Do(func() {
		close(s.Parar)
		s.muRede.Lock()
		if s.rede != nil {
			s.rede.Close()
		}
		s.muRede.Unlock()
	})
}

/* ---------------------------------------------------------- etiqueta --- */

func (s *Servidor) dadosEtiqueta(sv dados.Servico, v dados.Veiculo) etiqueta.Dados {
	return etiqueta.Dados{Codigo: v.Codigo, Data: sv.Data, KM: sv.KM, ProximaKM: sv.ProximaKM,
		Itens: sv.Itens, Outros: sv.Outros, Modelo: v.Modelo, Placa: dados.MascararPlaca(v.Placa),
		Correia: etiqueta.DetectarCorreia(sv.Itens, sv.Outros), ProximaCorreiaKM: sv.ProximaCorreiaKM, ProximaCorreiaData: sv.ProximaCorreiaData,
		ProximaData: dados.ProximaDataOleo(sv, s.cfg.Ler().MesesLembrete)}
}

func (s *Servidor) configEtiqueta() etiqueta.Config {
	c := s.cfg.Ler().Etiqueta
	if c.DPI <= 0 {
		c = etiqueta.ConfigPadrao()
	}
	return c
}

// impressoraEscolhida devolve a impressora configurada. Se nenhuma foi
// escolhida ainda, procura a iDPRT/iE4S e guarda. Nunca usa a impressora
// padrão do Windows sem perguntar (podia sair etiqueta numa folha A4).
func (s *Servidor) impressoraEscolhida() (string, error) {
	if n := s.cfg.Ler().Impressora; n != "" {
		return n, nil
	}
	lista, _, err := impressora.Listar()
	if err != nil {
		return "", err
	}
	for _, n := range lista {
		b := strings.ToLower(n)
		if strings.Contains(b, "ie4s") || strings.Contains(b, "idprt") || strings.Contains(b, "teclabel") {
			if err := s.cfg.Mudar(func(c *Config) { c.Impressora = n }, nil, nil); err == nil {
				registro.Info("impressora escolhida sozinha: %s", n)
			}
			return n, nil
		}
	}
	return "", errors.New("escolha a impressora de etiquetas em Configurações")
}

func (s *Servidor) trabalho(img *image.Gray, copias int, nome string) (impressora.Trabalho, error) {
	ce := s.configEtiqueta()
	cfg := s.cfg.Ler()
	imp, err := s.impressoraEscolhida()
	if err != nil {
		return impressora.Trabalho{}, err
	}
	t := impressora.Trabalho{Impressora: imp, Imagem: img, DPI: ce.DPI, LarguraMM: ce.LarguraMM,
		AlturaMM: ce.AlturaMM, Copias: copias, Nome: nome}
	if cfg.TamanhoDoDriver { // deixa o driver decidir o tamanho do papel
		t.LarguraMM, t.AlturaMM = 0, 0
	}
	return t, nil
}

// imprimirServico desenha e imprime. modelo "" = escolhe sozinho.
func (s *Servidor) imprimirServico(id, modelo string, copias int) (string, error) {
	sv, v, ok := s.Banco.Servico(id)
	if !ok {
		return "", dados.ErrNaoAchado
	}
	d := s.dadosEtiqueta(sv, v)
	// a etiqueta é escolhida sozinha: troca de óleo, correia dentada ou as duas
	modelo = etiqueta.ModeloPara(d)
	img, err := etiqueta.Desenhar(modelo, d, s.configEtiqueta())
	if err != nil {
		return modelo, err
	}
	if copias < 1 {
		copias = s.cfg.Ler().Copias
	}
	s.muImp.Lock()
	defer s.muImp.Unlock()
	t, err := s.trabalho(img, copias, "Etiqueta "+dados.MascararPlaca(v.Placa))
	if err == nil {
		err = s.Imprimir(t)
	}
	if err != nil {
		s.ultimoErr = err.Error()
		registro.Erro("impressão da etiqueta de %s: %v", v.Placa, err)
		return modelo, err
	}
	s.ultimoErr = ""
	registro.Info("etiqueta %s impressa para %s (%d cópia(s))", modelo, v.Placa, copias)
	s.Banco.ContarImpressao(id)
	return modelo, nil
}

/* -------------------------------------------------------------- Zeus --- */

// registrarDoZeus: lançamento novo com placa — registra e imprime sozinho.
func (s *Servidor) registrarDoZeus(l dados.Lancamento) {
	sv, jaImpresso, err := s.registrarLancamento(l, dados.NovoServico{Placa: l.Placa, Data: l.Data, KM: l.KM, Itens: l.Itens, Outros: l.Outros})
	if err != nil {
		registro.Erro("registrar lançamento %s do Zeus: %v", l.Ref, err)
		return
	}
	if s.cfg.Zeus().AutoImprim && !jaImpresso {
		s.imprimirServico(sv.ID, "", 0)
	}
}

// atualizarDoZeus: a venda ganhou peças depois de registrada — junta no
// serviço (o site atualiza; a etiqueta não é impressa de novo sozinha).
func (s *Servidor) atualizarDoZeus(l dados.Lancamento) {
	if l.ServicoID == "" {
		return
	}
	if _, err := s.Banco.MesclarServico(l.ServicoID, l.Itens, l.Outros, l.Ref, l.KM, ""); err != nil {
		registro.Erro("atualizar serviço com o lançamento %s: %v", l.Ref, err)
	}
}

// registrarLancamento vira serviço. Se o carro já tem serviço no mesmo dia,
// junta nele em vez de criar outro (ex.: lançado à mão e também no Zeus, ou
// duas vendas do mesmo carro). jaImpresso diz se esse serviço já teve etiqueta.
func (s *Servidor) registrarLancamento(l dados.Lancamento, n dados.NovoServico) (dados.Servico, bool, error) {
	n.Origem = "zeus"
	n.ZeusRef = l.Ref
	if n.Modelo == "" && n.Ano == "" {
		if v, ok := s.Banco.Veiculo(n.Placa); !ok || v.Modelo == "" {
			n.Modelo, n.Ano = l.Modelo, l.Ano
		}
	}
	obs := strings.TrimSpace(l.Documento + ": " + strings.Join(l.Produtos, "; "))
	if n.Data == "" {
		n.Data = l.Data
	}
	existente, ok := s.Banco.ServicoDoDia(n.Placa, n.Data)
	// venda que voltou para revisão (mudou no Zeus): soma no serviço que ela já gerou
	if l.ServicoID != "" {
		if sv, v, achou := s.Banco.Servico(l.ServicoID); achou && v.Placa == dados.NormalizarPlaca(n.Placa) {
			existente, ok = sv, true
		}
	}
	if ok && (n.Data != "" || l.ServicoID != "") {
		sv, err := s.Banco.MesclarServico(existente.ID, n.Itens, n.Outros, l.Ref, n.KM, obs)
		if err != nil {
			return dados.Servico{}, false, err
		}
		// a observação para o cliente revisada (ex.: marca do óleo) não se perde
		if n.ObsCliente != "" || n.ProximaKM > 0 || n.ProximaData != "" || n.ProximaCorreiaKM > 0 || n.ProximaCorreiaData != "" {
			if sv, err = s.Banco.ComplementarServico(sv.ID, n.ObsCliente, n.ProximaKM, n.ProximaData, n.ProximaCorreiaKM, n.ProximaCorreiaData); err != nil {
				return dados.Servico{}, false, err
			}
		}
		if n.Modelo != "" || n.Ano != "" {
			if v, ok := s.Banco.Veiculo(n.Placa); ok {
				if n.Modelo != "" {
					v.Modelo = n.Modelo
				}
				if n.Ano != "" {
					v.Ano = n.Ano
				}
				s.Banco.SalvarVeiculo(v)
			}
		}
		registro.Info("lançamento %s juntado ao serviço %s de %s", l.Ref, sv.ID, sv.Placa)
		s.guardarCliente(sv.Placa, l.Cliente, l.Telefone)
		return sv, existente.Impressoes > 0, s.Banco.MarcarLancamento(l.Ref, dados.Registrado, sv.ID)
	}
	if n.ObsInterna == "" {
		n.ObsInterna = obs
	}
	sv, _, err := s.Banco.RegistrarServico(n)
	if err != nil {
		return dados.Servico{}, false, err
	}
	registro.Info("lançamento %s registrado como serviço %s de %s", l.Ref, sv.ID, sv.Placa)
	s.guardarCliente(sv.Placa, l.Cliente, l.Telefone)
	return sv, false, s.Banco.MarcarLancamento(l.Ref, dados.Registrado, sv.ID)
}

// guardarCliente: o carro sem cliente cadastrado fica com o nome que veio do
// Zeus (só no programa; não vai para o site). Assim, o próximo lançamento
// desse cliente sem placa já vem com a placa sugerida.
func (s *Servidor) guardarCliente(placa, cliente, telefone string) {
	cliente, telefone = strings.TrimSpace(cliente), strings.TrimSpace(telefone)
	v, ok := s.Banco.Veiculo(placa)
	if !ok {
		return
	}
	mudou := false
	if cliente != "" && strings.TrimSpace(v.Cliente) == "" {
		v.Cliente, mudou = cliente, true
	}
	if telefone != "" && strings.TrimSpace(v.Telefone) == "" {
		v.Telefone, mudou = telefone, true
	}
	if mudou {
		if _, err := s.Banco.SalvarVeiculo(v); err != nil {
			registro.Erro("guardar cliente do carro %s: %v", placa, err)
		}
	}
}

// revisarDeNovo: uma venda já revisada mudou no Zeus (ganhou peça, mudou a
// observação). Volta para "Para revisar"; ao confirmar, junta no serviço do dia.
func (s *Servidor) revisarDeNovo(l dados.Lancamento) {
	if err := s.Banco.MarcarLancamento(l.Ref, dados.Pendente, l.ServicoID); err != nil {
		registro.Erro("voltar lançamento %s para revisão: %v", l.Ref, err)
		return
	}
	registro.Info("lançamento %s mudou no Zeus depois de revisado: voltou para revisão", l.Ref)
	s.avisarNovo(l)
}

// avisarNovo: chegou venda do Zeus para revisar — aviso no canto da tela do
// Windows (aparece mesmo com o programa minimizado).
func (s *Servidor) avisarNovo(l dados.Lancamento) {
	carro := strings.TrimSpace(strings.Join([]string{l.Modelo, dados.MascararPlaca(l.Placa)}, " "))
	if carro == "" {
		carro = "carro sem placa na observação"
	}
	msg := l.Documento + " — " + carro
	if l.Cliente != "" {
		msg += " — " + l.Cliente
	}
	// junta os avisos que chegam juntos (ex.: várias vendas numa leitura) num só
	s.muAviso.Lock()
	s.avisos = append(s.avisos, msg)
	s.refs = append(s.refs, l.Ref)
	if len(s.avisos) == 1 {
		time.AfterFunc(3*time.Second, func() {
			s.muAviso.Lock()
			lista, refs := s.avisos, s.refs
			s.avisos, s.refs = nil, nil
			s.muAviso.Unlock()
			texto := lista[0]
			if len(lista) > 1 {
				texto = fmt.Sprintf("%d serviços chegaram do Zeus. Abra o Oficina QR em \"Para revisar\".", len(lista))
			}
			sistema.Notificar("Oficina QR: serviço para revisar", texto)
			s.abrirRevisao(refs)
		})
	}
	s.muAviso.Unlock()
}

// abrirRevisao: redirecionamento automático. Com a janela do programa aberta,
// é ela mesma que vai para a revisão (ver app.js). Com a janela fechada, abre
// uma já na tela de revisar a venda que chegou.
func (s *Servidor) abrirRevisao(refs []string) {
	if s.porta == 0 || s.AbrirJanela == nil || len(refs) == 0 {
		return
	}
	// (janela minimizada pergunta menos vezes: até 1 vez por minuto)
	if time.Since(time.Unix(0, s.ultimaTela.Load())) < 90*time.Second {
		return // janela aberta: ela se redireciona sozinha
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/#/", s.porta)
	if len(refs) == 1 {
		url += "revisar/" + neturl.PathEscape(refs[0])
	}
	registro.Info("venda nova do Zeus com a janela fechada: abrindo %s", url)
	if err := s.AbrirJanela(url); err != nil {
		registro.Erro("abrir a janela para revisar: %v", err)
	}
}

/* --------------------------------------------------------------- HTTP --- */

type respErro struct {
	Erro string `json:"erro"`
}

func responder(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func falhar(w http.ResponseWriter, err error) {
	st := http.StatusBadRequest
	if errors.Is(err, dados.ErrNaoAchado) {
		st = http.StatusNotFound
	}
	responder(w, st, respErro{err.Error()})
}

func lerJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 5<<20))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("dados inválidos: %v", err)
	}
	return nil
}

// Handler monta as rotas. porta é usada para conferir o endereço (proteção).
func (s *Servidor) Handler(porta int) http.Handler {
	s.porta = porta
	m := http.NewServeMux()
	m.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("oficinaqr")) })
	m.HandleFunc("GET /api/estado", s.apiEstado)
	m.HandleFunc("GET /api/veiculos", s.apiVeiculos)
	m.HandleFunc("GET /api/veiculos/{placa}", s.apiVeiculo)
	m.HandleFunc("PUT /api/veiculos/{placa}", s.apiSalvarVeiculo)
	m.HandleFunc("GET /api/servicos/recentes", s.apiRecentes)
	m.HandleFunc("POST /api/servicos", s.apiNovoServico)
	m.HandleFunc("PUT /api/servicos/{id}", s.apiAlterarServico)
	m.HandleFunc("DELETE /api/servicos/{id}", s.apiApagarServico)
	m.HandleFunc("POST /api/apagar", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Servicos    []string `json:"servicos"`
			Carros      []string `json:"carros"`
			IgnorarZeus bool     `json:"ignorar_zeus"`
		}
		if err := lerJSON(r, &p); err != nil {
			falhar(w, err)
			return
		}
		if err := s.Banco.GuardarAntes("apagar"); err != nil {
			registro.Erro("cópia antes de apagar: %v", err)
		}
		res, err := s.Banco.ApagarVarios(p.Servicos, p.Carros, p.IgnorarZeus)
		if err != nil {
			falhar(w, err)
			return
		}
		registro.Info("apagados %d serviço(s) e %d carro(s) salvos errado: %v", res.Servicos, res.Carros, p.Carros)
		responder(w, 200, res)
	})
	m.HandleFunc("GET /api/etiqueta.png", s.apiEtiquetaPNG)
	m.HandleFunc("POST /api/imprimir", s.apiImprimir)
	m.HandleFunc("POST /api/imprimir/teste", s.apiImprimirTeste)
	m.HandleFunc("GET /api/impressoras", s.apiImpressoras)
	m.HandleFunc("GET /api/config", s.apiConfig)
	m.HandleFunc("PUT /api/config", s.apiSalvarConfig)
	m.HandleFunc("POST /api/site/testar", s.apiSiteTestar)
	m.HandleFunc("POST /api/site/publicar", s.apiSitePublicar)
	m.HandleFunc("GET /api/zeus/lancamentos", s.apiLancamentos)
	m.HandleFunc("GET /api/zeus/lancamento/{ref}", func(w http.ResponseWriter, r *http.Request) {
		l, ok := s.Banco.Lancamento(r.PathValue("ref"))
		if !ok {
			falhar(w, dados.ErrNaoAchado)
			return
		}
		// "oleo": marca/linha do óleo vendido, para a observação para o cliente
		responder(w, 200, struct {
			dados.Lancamento
			Oleo string `json:"oleo,omitempty"`
		}{l, itens.ObsOleo(l.Produtos)})
	})
	m.HandleFunc("POST /api/zeus/lancamentos/{ref}/registrar", s.apiRegistrarLancamento)
	m.HandleFunc("POST /api/zeus/lancamentos/{ref}/ignorar", s.apiIgnorarLancamento)
	m.HandleFunc("POST /api/zeus/lancamentos/{ref}/voltar", s.apiVoltarLancamento)
	m.HandleFunc("POST /api/zeus/ler", s.apiZeusLer)
	m.HandleFunc("POST /api/zeus/testar", s.apiZeusTestar)
	m.HandleFunc("GET /api/zeus/tabelas", s.apiZeusTabelas)
	m.HandleFunc("POST /api/zeus/procurar", s.apiZeusProcurar)
	m.HandleFunc("POST /api/zeus/consulta", s.apiZeusConsulta)
	m.HandleFunc("GET /api/zeus/pdfs", s.apiZeusPDFs)
	m.HandleFunc("POST /api/zeus/abrir-pasta", s.apiZeusAbrirPasta)
	m.HandleFunc("GET /api/exportar.csv", s.apiExportar)
	s.rotasExtras(m)
	m.HandleFunc("POST /api/abrir", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			URL string `json:"url"`
		}
		if err := lerJSON(r, &p); err != nil || !strings.HasPrefix(p.URL, "https://") || strings.ContainsAny(p.URL, " \"\n\r") {
			falhar(w, errors.New("link inválido"))
			return
		}
		if err := sistema.AbrirNoNavegador(p.URL); err != nil {
			falhar(w, err)
			return
		}
		responder(w, 200, map[string]bool{"ok": true})
	})
	m.HandleFunc("GET /api/registro", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write([]byte(registro.Ultimas(300)))
	})
	m.HandleFunc("POST /api/sair", func(w http.ResponseWriter, r *http.Request) {
		responder(w, 200, map[string]bool{"ok": true})
		go func() { time.Sleep(300 * time.Millisecond); s.Fechar() }()
	})
	arquivos := http.FileServer(http.FS(s.telas))
	m.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		arquivos.ServeHTTP(w, r)
	}))
	s.mux = m
	s.ajustarRede()
	return s.protecao(s.modoConsulta(m))
}

// protecao: só aceita pedidos do próprio computador, pelo endereço do programa.
func (s *Servidor) protecao(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, porta, err := net.SplitHostPort(r.Host)
		if err != nil || (host != "127.0.0.1" && host != "localhost") || (s.porta != 0 && porta != strconv.Itoa(s.porta)) {
			http.Error(w, "endereço não permitido", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-Oficina") != "1" {
			http.Error(w, "pedido não permitido", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		h.ServeHTTP(w, r)
	})
}

func (s *Servidor) apiEstado(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Chave") == "" { // só a janela deste computador (não a consulta pela rede)
		s.ultimaTela.Store(time.Now().UnixNano())
	}
	cfg := s.cfg.Ler()
	s.muImp.Lock()
	errImp := s.ultimoErr
	s.muImp.Unlock()
	responder(w, 200, map[string]any{
		"versao":       Versao,
		"site":         s.Site.Estado(),
		"zeus":         s.Monitor.Estado(),
		"impressora":   cfg.Impressora,
		"correia":      correiaPadrao(cfg.Etiqueta),
		"meses_oleo":   mesesOleo(cfg),
		"erro_impress": errImp,
		"alteracao":    s.Banco.Alteracao(),
		"pasta":        s.Banco.Pasta(),
		"itens":        dados.Itens,
		"site_url":     cfg.Etiqueta.Site,
		"registro":     registro.Caminho(),
	})
}

func (s *Servidor) apiVeiculos(w http.ResponseWriter, r *http.Request) {
	limite := 50
	if n, err := strconv.Atoi(r.URL.Query().Get("limite")); err == nil && n > 0 && n <= 5000 {
		limite = n
	}
	l := s.Banco.BuscarVeiculos(r.URL.Query().Get("q"), limite)
	if l == nil {
		l = []dados.ResumoVeiculo{}
	}
	responder(w, 200, l)
}

type veiculoResp struct {
	Veiculo  dados.Veiculo   `json:"veiculo"`
	Placa    string          `json:"placa_formatada"`
	Servicos []dados.Servico `json:"servicos"`
	Link     string          `json:"link"`
}

func (s *Servidor) apiVeiculo(w http.ResponseWriter, r *http.Request) {
	v, ok := s.Banco.Veiculo(r.PathValue("placa"))
	if !ok && r.URL.Query().Get("opcional") != "" {
		responder(w, 200, map[string]any{"veiculo": nil})
		return
	}
	if !ok {
		falhar(w, dados.ErrNaoAchado)
		return
	}
	ss := s.Banco.ServicosDoVeiculo(v.Placa)
	if ss == nil {
		ss = []dados.Servico{}
	}
	responder(w, 200, veiculoResp{v, dados.MascararPlaca(v.Placa), ss, etiqueta.Link(s.configEtiqueta().Site, v.Codigo)})
}

func (s *Servidor) apiSalvarVeiculo(w http.ResponseWriter, r *http.Request) {
	var v dados.Veiculo
	if err := lerJSON(r, &v); err != nil {
		falhar(w, err)
		return
	}
	v.Placa = r.PathValue("placa")
	salvo, err := s.Banco.SalvarVeiculo(v)
	if err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, salvo)
}

func (s *Servidor) apiRecentes(w http.ResponseWriter, r *http.Request) {
	limite := 30
	if n, err := strconv.Atoi(r.URL.Query().Get("limite")); err == nil && n > 0 && n <= 5000 {
		limite = n
	}
	l := s.Banco.UltimosServicos(limite)
	if l == nil {
		l = []dados.ServicoComVeiculo{}
	}
	responder(w, 200, l)
}

type pedidoServico struct {
	dados.NovoServico
	Imprimir *bool  `json:"imprimir"`
	Modelo   string `json:"modelo_etiqueta"`
}

type respServico struct {
	Servico   dados.Servico `json:"servico"`
	Veiculo   dados.Veiculo `json:"veiculo"`
	Link      string        `json:"link"`
	Impresso  bool          `json:"impresso"`
	Modelo    string        `json:"modelo_etiqueta,omitempty"`
	ErroImpre string        `json:"erro_impressao,omitempty"`
}

func (s *Servidor) depoisDeSalvar(w http.ResponseWriter, sv dados.Servico, imprimir *bool, modelo string) {
	_, v, _ := s.Banco.Servico(sv.ID)
	resp := respServico{Servico: sv, Veiculo: v, Link: etiqueta.Link(s.configEtiqueta().Site, v.Codigo)}
	imp := s.cfg.Ler().ImprimirAoSalvar
	if imprimir != nil {
		imp = *imprimir
	}
	if imp {
		m, err := s.imprimirServico(sv.ID, modelo, 0)
		resp.Modelo = m
		if err != nil {
			resp.ErroImpre = err.Error()
		} else {
			resp.Impresso = true
		}
	}
	responder(w, 200, resp)
}

func (s *Servidor) apiNovoServico(w http.ResponseWriter, r *http.Request) {
	var p pedidoServico
	if err := lerJSON(r, &p); err != nil {
		falhar(w, err)
		return
	}
	p.Origem, p.ZeusRef = "manual", ""
	sv, _, err := s.Banco.RegistrarServico(p.NovoServico)
	if err != nil {
		falhar(w, err)
		return
	}
	s.depoisDeSalvar(w, sv, p.Imprimir, p.Modelo)
}

func (s *Servidor) apiAlterarServico(w http.ResponseWriter, r *http.Request) {
	var p pedidoServico
	if err := lerJSON(r, &p); err != nil {
		falhar(w, err)
		return
	}
	sv, err := s.Banco.AlterarServico(r.PathValue("id"), p.NovoServico)
	if err != nil {
		falhar(w, err)
		return
	}
	nao := false
	if p.Imprimir == nil {
		p.Imprimir = &nao
	}
	s.depoisDeSalvar(w, sv, p.Imprimir, p.Modelo)
}

func (s *Servidor) apiApagarServico(w http.ResponseWriter, r *http.Request) {
	if err := s.Banco.ApagarServico(r.PathValue("id")); err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) apiEtiquetaPNG(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var d etiqueta.Dados
	if id := q.Get("servico"); id != "" {
		sv, v, ok := s.Banco.Servico(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		d = s.dadosEtiqueta(sv, v)
	} else {
		// prévia de um serviço ainda não salvo
		d = etiqueta.Dados{Codigo: "XXXXXXXX", Data: q.Get("data"), Modelo: q.Get("modelo"),
			Itens: q["item"], Outros: q["outro"]}
		d.KM, _ = strconv.Atoi(q.Get("km"))
		d.ProximaKM, _ = strconv.Atoi(q.Get("proxima"))
		d.ProximaCorreiaKM, _ = strconv.Atoi(q.Get("proxima_correia_km"))
		d.ProximaCorreiaData = q.Get("proxima_correia_data")
		d.ProximaData = dados.ProximaDataOleo(dados.Servico{Data: d.Data, Itens: d.Itens, ProximaData: q.Get("proxima_data")}, s.cfg.Ler().MesesLembrete)
		d.Correia = etiqueta.DetectarCorreia(d.Itens, d.Outros)
		if p := dados.NormalizarPlaca(q.Get("placa")); p != "" {
			d.Placa = dados.MascararPlaca(p)
			if v, ok := s.Banco.Veiculo(p); ok {
				d.Codigo = v.Codigo
			}
		}
	}
	modelo := etiqueta.ModeloPara(d)
	img, err := etiqueta.Desenhar(modelo, d, s.configEtiqueta())
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Modelo", modelo)
	w.Header().Set("X-Modelo-Nome", neturl.QueryEscape(etiqueta.NomeModelo(modelo)))
	w.Write(buf.Bytes())
}

func (s *Servidor) apiImprimir(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Servico string `json:"servico"`
		Modelo  string `json:"modelo"`
		Copias  int    `json:"copias"`
	}
	if err := lerJSON(r, &p); err != nil {
		falhar(w, err)
		return
	}
	m, err := s.imprimirServico(p.Servico, p.Modelo, p.Copias)
	if err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]any{"ok": true, "modelo": m})
}

func (s *Servidor) apiImprimirTeste(w http.ResponseWriter, r *http.Request) {
	ce := s.configEtiqueta()
	img, _ := etiqueta.Desenhar("oleo", etiqueta.Dados{Codigo: "DEMO", Data: time.Now().Format("2006-01-02"), KM: 123456,
		ProximaKM: 133456, Itens: []string{"Óleo do motor", "Filtro de óleo"}}, ce)
	s.muImp.Lock()
	t, err := s.trabalho(img, 1, "Etiqueta de teste")
	if err == nil {
		err = s.Imprimir(t)
	}
	s.muImp.Unlock()
	if err != nil {
		registro.Erro("etiqueta de teste: %v", err)
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) apiImpressoras(w http.ResponseWriter, r *http.Request) {
	l, padrao, err := impressora.Listar()
	if err != nil {
		falhar(w, err)
		return
	}
	if l == nil {
		l = []string{}
	}
	sort.Strings(l)
	responder(w, 200, map[string]any{"impressoras": l, "padrao": padrao})
}

type configResp struct {
	Config
	TemToken          bool   `json:"tem_token"`
	TemSenha          bool   `json:"tem_senha"`
	IniciarComWindows bool   `json:"iniciar_com_windows"`
	ChaveRede         string `json:"chave_rede"`
}

func (s *Servidor) apiConfig(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Ler()
	resp := configResp{Config: c, TemToken: s.cfg.Site().Token != "", TemSenha: s.cfg.Zeus().Senha != "",
		IniciarComWindows: sistema.IniciaComWindows()}
	resp.TokenCifrado, resp.SenhaCifrada, resp.ChaveRedeCifrada = "", "", ""
	resp.ChaveRede = s.cfg.Chave()
	responder(w, 200, resp)
}

func (s *Servidor) apiSalvarConfig(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Config
		Token             *string `json:"token"`
		Senha             *string `json:"senha"`
		ChaveRede         *string `json:"chave_rede"`
		IniciarComWindows *bool   `json:"iniciar_com_windows"`
	}
	p.Config = s.cfg.Ler()
	if err := lerJSON(r, &p); err != nil {
		falhar(w, err)
		return
	}
	if p.Token != nil {
		t := strings.TrimSpace(*p.Token)
		p.Token = &t
		if t == "" {
			p.Token = nil // campo vazio não apaga; para apagar use "-"
		} else if t == "-" {
			vazio := ""
			p.Token = &vazio
		}
	}
	if p.Senha != nil && *p.Senha == "" {
		p.Senha = nil
	}
	antes := s.cfg.Zeus()
	err := s.cfg.Mudar(func(c *Config) {
		tokenC, senhaC, chaveC := c.TokenCifrado, c.SenhaCifrada, c.ChaveRedeCifrada
		*c = p.Config
		c.TokenCifrado, c.SenhaCifrada, c.ChaveRedeCifrada = tokenC, senhaC, chaveC
		if c.Modo != ModoConsulta {
			c.Modo = "principal"
		}
		c.Principal = strings.TrimSpace(c.Principal)
		if c.Principal != "" && !strings.Contains(c.Principal, ":") {
			c.Principal += ":" + strconv.Itoa(PortaRede())
		}
		if c.Copias < 1 {
			c.Copias = 1
		}
		if c.Copias > 20 {
			c.Copias = 20
		}
		if c.Etiqueta.DPI <= 0 {
			c.Etiqueta.DPI = 203
		}
		if c.Etiqueta.LarguraMM < 20 || c.Etiqueta.AlturaMM < 20 {
			c.Etiqueta.LarguraMM, c.Etiqueta.AlturaMM = 60, 60
		}
		// nada do Zeus é registrado/impresso sem passar pela revisão
		c.Zeus.AutoRegist, c.Zeus.AutoImprim = false, false
		if strings.TrimSpace(c.Zeus.SQL) == "" {
			c.Zeus.SQL = zeus.SQLPadrao
		}
		if c.MesesLembrete <= 0 {
			c.MesesLembrete = 6
		}
	}, p.Token, p.Senha)
	if err != nil {
		falhar(w, err)
		return
	}
	if p.ChaveRede != nil {
		if err := s.cfg.MudarChave(strings.TrimSpace(*p.ChaveRede)); err != nil {
			falhar(w, err)
			return
		}
	}
	s.ajustarRede()
	if p.IniciarComWindows != nil {
		sistema.IniciarComWindows(*p.IniciarComWindows)
	}
	s.Banco.CopiaExtra = s.cfg.Ler().PastaCopia
	depois := s.cfg.Zeus()
	if depois.Desde != antes.Desde {
		s.Banco.SalvarZeusCursor("") // deixa ler desde a data nova (ex.: trazer vendas antigas)
	}
	if depois.Ativo && (!antes.Ativo || depois.SQL != antes.SQL || depois.Host != antes.Host ||
		depois.ModoEfetivo() != antes.ModoEfetivo() || depois.PastaPDF != antes.PastaPDF) {
		s.Monitor.LerAgora()
	}
	if p.Token != nil {
		s.Site.Avisar()
	}
	s.apiConfig(w, r)
}

func (s *Servidor) apiSiteTestar(w http.ResponseWriter, r *http.Request) {
	if err := s.Site.Testar(s.cfg.Site()); err != nil {
		falhar(w, err)
		return
	}
	// o teste de verdade é gravar: publica agora
	if err := s.Site.PublicarAgora(); err != nil {
		registro.Erro("teste do site: %v", err)
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) apiSitePublicar(w http.ResponseWriter, r *http.Request) {
	if err := s.Site.PublicarAgora(); err != nil {
		registro.Erro("publicar site: %v", err)
		falhar(w, err)
		return
	}
	responder(w, 200, s.Site.Estado())
}

func (s *Servidor) apiLancamentos(w http.ResponseWriter, r *http.Request) {
	l := s.Banco.Lancamentos(r.URL.Query().Get("status"), 100)
	if l == nil {
		l = []dados.Lancamento{}
	}
	responder(w, 200, l)
}

func (s *Servidor) apiRegistrarLancamento(w http.ResponseWriter, r *http.Request) {
	l, ok := s.Banco.Lancamento(r.PathValue("ref"))
	if !ok {
		falhar(w, dados.ErrNaoAchado)
		return
	}
	if l.Status == dados.Registrado {
		falhar(w, errors.New("esse lançamento já foi registrado"))
		return
	}
	var p pedidoServico
	if err := lerJSON(r, &p); err != nil {
		falhar(w, err)
		return
	}
	sv, _, err := s.registrarLancamento(l, p.NovoServico)
	if err != nil {
		falhar(w, err)
		return
	}
	s.depoisDeSalvar(w, sv, p.Imprimir, p.Modelo)
}

func (s *Servidor) apiIgnorarLancamento(w http.ResponseWriter, r *http.Request) {
	if err := s.Banco.MarcarLancamento(r.PathValue("ref"), dados.Ignorado, ""); err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) apiVoltarLancamento(w http.ResponseWriter, r *http.Request) {
	l, ok := s.Banco.Lancamento(r.PathValue("ref"))
	if !ok || l.Status != dados.Ignorado {
		falhar(w, errors.New("só dá para voltar um lançamento ignorado"))
		return
	}
	if err := s.Banco.MarcarLancamento(l.Ref, dados.Pendente, ""); err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) pastaPDF() string {
	p := strings.TrimSpace(s.cfg.Zeus().PastaPDF)
	if p == "" {
		p = zeus.PastaPadrao()
	}
	return p
}

// apiZeusPDFs: os PDFs mais recentes da pasta e o que foi entendido de cada um.
func (s *Servidor) apiZeusPDFs(w http.ResponseWriter, r *http.Request) {
	pasta := s.pastaPDF()
	os.MkdirAll(pasta, 0o755)
	l, err := zeus.PDFsNaPasta(pasta, 15)
	if err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]any{"pasta": pasta, "pdfs": l})
}

func (s *Servidor) apiZeusAbrirPasta(w http.ResponseWriter, r *http.Request) {
	pasta := s.pastaPDF()
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		falhar(w, err)
		return
	}
	if err := sistema.AbrirPasta(pasta); err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

func (s *Servidor) apiZeusLer(w http.ResponseWriter, r *http.Request) {
	n, err := s.Monitor.Ler()
	if err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]int{"novos": n})
}

func (s *Servidor) pg(r *http.Request) (*zeus.Postgres, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	return zeus.NovoPostgres(s.cfg.Zeus()), ctx, cancel
}

func (s *Servidor) apiZeusTestar(w http.ResponseWriter, r *http.Request) {
	p, ctx, cancel := s.pg(r)
	defer cancel()
	defer p.Fechar()
	v, err := p.Testar(ctx)
	if err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]string{"versao": v})
}

func (s *Servidor) apiZeusTabelas(w http.ResponseWriter, r *http.Request) {
	p, ctx, cancel := s.pg(r)
	defer cancel()
	defer p.Fechar()
	t, err := p.Tabelas(ctx)
	if err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, t)
}

func (s *Servidor) apiZeusProcurar(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Texto string `json:"texto"`
	}
	if err := lerJSON(r, &q); err != nil || len(strings.TrimSpace(q.Texto)) < 3 {
		falhar(w, errors.New("escreva pelo menos 3 letras para procurar"))
		return
	}
	p, ctx, cancel := s.pg(r)
	defer cancel()
	defer p.Fechar()
	a, err := p.Procurar(ctx, strings.TrimSpace(q.Texto), 30)
	if err != nil {
		falhar(w, err)
		return
	}
	if a == nil {
		a = []zeus.Achado{}
	}
	responder(w, 200, a)
}

func (s *Servidor) apiZeusConsulta(w http.ResponseWriter, r *http.Request) {
	var q struct {
		SQL   string `json:"sql"`
		Desde string `json:"desde"`
	}
	if err := lerJSON(r, &q); err != nil {
		falhar(w, err)
		return
	}
	p, ctx, cancel := s.pg(r)
	defer cancel()
	defer p.Fechar()
	var cols []string
	var linhas [][]string
	var err error
	if strings.Contains(q.SQL, "$1") {
		// testando a consulta do monitor: devolve como ela vai ser lida
		c := s.cfg.Zeus()
		c.SQL = q.SQL
		pp := zeus.NovoPostgres(c)
		defer pp.Fechar()
		desde := q.Desde
		if desde == "" {
			desde = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		}
		ls, e := pp.Buscar(ctx, desde)
		if e != nil {
			falhar(w, e)
			return
		}
		lanc := zeus.Agrupar(ls, nil)
		responder(w, 200, map[string]any{"lancamentos": lanc, "linhas": len(ls)})
		return
	}
	cols, linhas, err = p.Consulta(ctx, q.SQL)
	if err != nil {
		falhar(w, err)
		return
	}
	if linhas == nil {
		linhas = [][]string{}
	}
	responder(w, 200, map[string]any{"colunas": cols, "linhas": linhas})
}

func (s *Servidor) apiExportar(w http.ResponseWriter, r *http.Request) {
	veics, servs := s.Banco.Tudo()
	sort.Slice(servs, func(i, j int) bool { return servs[i].Data > servs[j].Data })
	var buf bytes.Buffer
	buf.WriteString("\ufeff") // Excel abre com acentos
	c := csv.NewWriter(&buf)
	c.Comma = ';'
	c.Write(append(append([]string{"Data", "Placa", "Modelo", "Ano", "Código do QR", "KM"}, dados.Itens...),
		"Outros serviços", "Próxima troca (KM)", "Obs. cliente", "Obs. interna", "Origem"))
	for _, sv := range servs {
		v := veics[sv.Placa]
		tem := map[string]bool{}
		for _, it := range sv.Itens {
			tem[it] = true
		}
		l := []string{sv.Data, dados.MascararPlaca(sv.Placa), v.Modelo, v.Ano, v.Codigo, strconv.Itoa(sv.KM)}
		for _, it := range dados.Itens {
			if tem[it] {
				l = append(l, "X")
			} else {
				l = append(l, "")
			}
		}
		prox := ""
		if sv.ProximaKM > 0 {
			prox = strconv.Itoa(sv.ProximaKM)
		}
		l = append(l, strings.Join(sv.Outros, " | "), prox, sv.ObsCliente, sv.ObsInterna, sv.Origem)
		c.Write(l)
	}
	c.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="historico-oficina-`+time.Now().Format("2006-01-02")+`.csv"`)
	w.Write(buf.Bytes())
}

// intervalo padrão da próxima troca da correia dentada (para a tela preencher)
func correiaPadrao(c etiqueta.Config) map[string]int {
	km, meses := c.CorreiaKM, c.CorreiaMeses
	if km <= 0 {
		km = 40000
	}
	if meses <= 0 {
		meses = 12
	}
	return map[string]int{"km": km, "meses": meses}
}

// opcoesPublico: mesmas regras da etiqueta para a página do QR Code.
func (s *Servidor) opcoesPublico() dados.OpcoesPublico {
	cfg := s.cfg.Ler()
	ce := s.configEtiqueta()
	termos := map[string]bool{} // placa|data com termo de peça do cliente
	for _, t := range s.Banco.Termos("", "", 0) {
		if t.Placa != "" {
			termos[t.Placa+"|"+t.Data] = true
		}
	}
	return dados.OpcoesPublico{
		MesesOleo: cfg.MesesLembrete,
		Correia: func(sv dados.Servico) (int, string, bool) {
			d := etiqueta.Dados{Data: sv.Data, KM: sv.KM, Itens: sv.Itens, Outros: sv.Outros,
				Correia: etiqueta.DetectarCorreia(sv.Itens, sv.Outros), ProximaCorreiaKM: sv.ProximaCorreiaKM, ProximaCorreiaData: sv.ProximaCorreiaData}
			if !d.Correia.Correia {
				return 0, "", false
			}
			km, data := etiqueta.ProximaCorreia(d, ce)
			return km, data, true
		},
		PecaDoCliente: func(sv dados.Servico) bool { return termos[sv.Placa+"|"+sv.Data] },
	}
}

func mesesOleo(c Config) int {
	if c.MesesLembrete <= 0 {
		return 6
	}
	return c.MesesLembrete
}

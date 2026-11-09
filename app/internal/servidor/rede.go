package servidor

// Consulta pela rede: o computador principal (o que registra, imprime e lê o
// Zeus) pode deixar outros computadores da oficina VEREM os dados, sem mudar
// nada. O computador de consulta abre o mesmo programa no modo "Somente
// consulta": as telas pedem os dados ao principal e tudo que muda é bloqueado.

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"oficinaqr/internal/registro"
)

const portaRedePadrao = 47136

// PortaRede é a porta da consulta pela rede (OFICINAQR_PORTA_REDE muda, para testes).
func PortaRede() int {
	if v, _ := strconv.Atoi(os.Getenv("OFICINAQR_PORTA_REDE")); v > 0 {
		return v
	}
	return portaRedePadrao
}

// o que o computador de consulta pode ler do principal (só leitura)
var leiturasPermitidas = []string{
	"/api/estado", "/api/hoje", "/api/veiculos", "/api/veiculos/", "/api/servicos/recentes", "/api/etiqueta.png",
	"/api/termos", "/api/termos/", "/api/termo-modelo", "/api/lembretes", "/api/relatorio",
	"/api/zeus/lancamentos", "/api/zeus/lancamento/", "/api/exportar.csv",
}

func leituraPermitida(p string) bool {
	for _, x := range leiturasPermitidas {
		if p == x || (strings.HasSuffix(x, "/") && strings.HasPrefix(p, x)) {
			return true
		}
	}
	return false
}

/* ----------------------------------------------------- lado principal --- */

// ajustarRede liga ou desliga a consulta pela rede conforme a configuração.
func (s *Servidor) ajustarRede() {
	cfg := s.cfg.Ler()
	ligar := cfg.RedeLigada && cfg.Modo != ModoConsulta && s.cfg.Chave() != "" && s.mux != nil
	s.muRede.Lock()
	defer s.muRede.Unlock()
	if !ligar {
		if s.rede != nil {
			s.rede.Close()
			s.rede = nil
			registro.Info("consulta pela rede desligada")
		}
		return
	}
	if s.rede != nil {
		return
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", PortaRede()))
	if err != nil {
		registro.Erro("consulta pela rede: não consegui abrir a porta %d: %v", PortaRede(), err)
		return
	}
	srv := &http.Server{Handler: http.HandlerFunc(s.atenderRede), ReadHeaderTimeout: 10 * time.Second, ErrorLog: registro.Logger()}
	s.rede = srv
	go srv.Serve(ln)
	registro.Info("consulta pela rede ligada na porta %d", PortaRede())
}

func nomeComputador() string {
	n, _ := os.Hostname()
	return n
}

func (s *Servidor) atenderRede(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/ping-rede" {
		w.Write([]byte("oficinaqr-principal|" + nomeComputador()))
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "somente leitura", http.StatusMethodNotAllowed)
		return
	}
	chave := s.cfg.Chave()
	if chave == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Chave")), []byte(chave)) != 1 {
		time.Sleep(600 * time.Millisecond) // atrasa quem tenta adivinhar
		responder(w, http.StatusUnauthorized, respErro{"chave da consulta errada"})
		return
	}
	if !leituraPermitida(r.URL.Path) {
		responder(w, http.StatusForbidden, respErro{"isso não pode ser consultado pela rede"})
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
}

// IPs deste computador na rede local (para mostrar na tela do principal).
func ipsLocais() []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				if ip := n.IP.To4(); ip != nil && ip.IsPrivate() {
					out = append(out, ip.String())
				}
			}
		}
	}
	return out
}

/* --------------------------------------------------- lado de consulta --- */

// rotas que o computador de consulta atende sozinho (não vão para o principal)
var rotasLocais = map[string]bool{
	"/api/ping": true, "/api/config": true, "/api/abrir": true, "/api/sair": true, "/api/registro": true,
	"/api/atualizacao": true, "/api/atualizacao/instalar": true, "/api/rede": true,
	"/api/consulta/procurar": true, "/api/consulta/testar": true,
}

func (s *Servidor) modoConsulta(m http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || rotasLocais[r.URL.Path] || !s.cfg.Consulta() {
			m.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet {
			responder(w, http.StatusForbidden, respErro{"Este computador está no modo somente consulta: para registrar ou mudar algo, use o computador principal."})
			return
		}
		if r.URL.Path == "/api/estado" {
			s.estadoConsulta(w, r)
			return
		}
		if !leituraPermitida(r.URL.Path) {
			responder(w, http.StatusForbidden, respErro{"Não disponível no modo somente consulta."})
			return
		}
		s.proxyPrincipal().ServeHTTP(w, r)
	})
}

func (s *Servidor) proxyPrincipal() *httputil.ReverseProxy {
	end := s.cfg.Ler().Principal
	s.muRede.Lock()
	defer s.muRede.Unlock()
	if s.proxy != nil && s.proxyPara == end {
		return s.proxy
	}
	alvo := &url.URL{Scheme: "http", Host: end}
	p := httputil.NewSingleHostReverseProxy(alvo)
	dir := p.Director
	p.Director = func(r *http.Request) {
		dir(r)
		r.Host = alvo.Host
		r.Header.Del("X-Oficina")
		r.Header.Del("Cookie")
		r.Header.Set("X-Chave", s.cfg.Chave())
	}
	p.Transport = &http.Transport{DialContext: (&net.Dialer{Timeout: 4 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second}
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		responder(w, http.StatusBadGateway, respErro{"Sem ligação com o computador principal (" + end + "). Ele está ligado, com o Oficina QR aberto?"})
	}
	s.proxy, s.proxyPara = p, end
	return p
}

var clienteRede = &http.Client{Timeout: 6 * time.Second}

// pedirPrincipal faz um GET no principal e devolve o JSON.
func pedirPrincipal(end, chave, caminho string, v any) error {
	if end == "" {
		return errors.New("falta o endereço do computador principal")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://"+end+caminho, nil)
	req.Header.Set("X-Chave", chave)
	r, err := clienteRede.Do(req)
	if err != nil {
		return fmt.Errorf("sem ligação com o computador principal (%s)", end)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if r.StatusCode != 200 {
		var e respErro
		if json.Unmarshal(b, &e) == nil && e.Erro != "" {
			return errors.New(e.Erro)
		}
		return fmt.Errorf("o computador principal respondeu %d", r.StatusCode)
	}
	return json.Unmarshal(b, v)
}

func (s *Servidor) estadoConsulta(w http.ResponseWriter, r *http.Request) {
	s.ultimaTela.Store(time.Now().UnixNano())
	cfg := s.cfg.Ler()
	e := map[string]any{}
	err := pedirPrincipal(cfg.Principal, s.cfg.Chave(), "/api/estado", &e)
	if err != nil {
		e = map[string]any{"erro_principal": err.Error()}
	} else {
		e["versao_principal"] = e["versao"]
	}
	e["versao"] = Versao
	e["somente_leitura"] = true
	e["principal"] = cfg.Principal
	responder(w, 200, e)
}

// rotas da tela de configuração da consulta
func (s *Servidor) rotasRede(m *http.ServeMux) {
	m.HandleFunc("GET /api/rede", func(w http.ResponseWriter, r *http.Request) {
		responder(w, 200, map[string]any{"ips": ipsLocais(), "porta": PortaRede(), "nome": nomeComputador()})
	})
	m.HandleFunc("POST /api/consulta/testar", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Principal string `json:"principal"`
			Chave     string `json:"chave"`
		}
		if err := lerJSON(r, &p); err != nil {
			falhar(w, err)
			return
		}
		end := strings.TrimSpace(p.Principal)
		if end != "" && !strings.Contains(end, ":") {
			end += ":" + strconv.Itoa(PortaRede())
		}
		e := map[string]any{}
		if err := pedirPrincipal(end, strings.TrimSpace(p.Chave), "/api/estado", &e); err != nil {
			falhar(w, err)
			return
		}
		var h map[string]int
		pedirPrincipal(end, strings.TrimSpace(p.Chave), "/api/hoje", &h)
		responder(w, 200, map[string]any{"ok": true, "versao": e["versao"], "principal": end, "hoje": h})
	})
	m.HandleFunc("POST /api/consulta/procurar", func(w http.ResponseWriter, r *http.Request) {
		responder(w, 200, procurarPrincipal(r.Context()))
	})
}

type achado struct {
	Endereco string `json:"endereco"`
	Nome     string `json:"nome"`
}

// procurarPrincipal olha a rede local (/24) atrás de um Oficina QR com a consulta ligada.
func procurarPrincipal(ctx context.Context) []achado {
	porta := PortaRede()
	meus := map[string]bool{}
	var alvos []string
	for _, ip := range ipsLocais() {
		meus[ip] = true
		p := strings.Split(ip, ".")
		base := strings.Join(p[:3], ".")
		for i := 1; i < 255; i++ {
			alvos = append(alvos, fmt.Sprintf("%s.%d", base, i))
		}
	}
	alvos = append(alvos, "127.0.0.1")
	c := &http.Client{Timeout: 700 * time.Millisecond}
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := []achado{}
	vagas := make(chan struct{}, 64)
	for _, ip := range alvos {
		wg.Add(1)
		vagas <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-vagas }()
			end := fmt.Sprintf("%s:%d", ip, porta)
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+end+"/api/ping-rede", nil)
			r, err := c.Do(req)
			if err != nil {
				return
			}
			defer r.Body.Close()
			b, _ := io.ReadAll(io.LimitReader(r.Body, 200))
			if t := string(b); strings.HasPrefix(t, "oficinaqr-principal|") {
				mu.Lock()
				out = append(out, achado{end, strings.TrimPrefix(t, "oficinaqr-principal|")})
				mu.Unlock()
			}
		}(ip)
	}
	wg.Wait()
	return out
}

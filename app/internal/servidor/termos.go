package servidor

import (
	"net/http"
	"strings"

	"oficinaqr/internal/dados"
)

// Termo de responsabilidade: o cliente comprou a peça por conta própria e a
// oficina responde só pela mão de obra.
func (s *Servidor) rotasTermos(m *http.ServeMux) {
	m.HandleFunc("GET /api/termos", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		l := s.Banco.Termos(q.Get("placa"), q.Get("q"), 200)
		if l == nil {
			l = []dados.Termo{}
		}
		responder(w, 200, l)
	})
	m.HandleFunc("GET /api/termos/{id}", func(w http.ResponseWriter, r *http.Request) {
		t, ok := s.Banco.Termo(r.PathValue("id"))
		if !ok {
			falhar(w, dados.ErrNaoAchado)
			return
		}
		responder(w, 200, t)
	})
	m.HandleFunc("POST /api/termos", func(w http.ResponseWriter, r *http.Request) { s.salvarTermo(w, r, "") })
	m.HandleFunc("PUT /api/termos/{id}", func(w http.ResponseWriter, r *http.Request) { s.salvarTermo(w, r, r.PathValue("id")) })
	m.HandleFunc("DELETE /api/termos/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Banco.ApagarTermo(r.PathValue("id")); err != nil {
			falhar(w, err)
			return
		}
		responder(w, 200, map[string]bool{"ok": true})
	})
	// texto do termo (cabeçalho e cláusulas), editável na tela do termo
	m.HandleFunc("GET /api/termo-modelo", func(w http.ResponseWriter, r *http.Request) {
		c := s.cfg.Ler()
		responder(w, 200, map[string]string{"cabecalho": c.TermoCabecalho, "clausulas": c.TermoClausulas,
			"cabecalho_padrao": TermoCabecalhoPadrao, "clausulas_padrao": TermoClausulasPadrao})
	})
	m.HandleFunc("PUT /api/termo-modelo", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Cabecalho string `json:"cabecalho"`
			Clausulas string `json:"clausulas"`
		}
		if err := lerJSON(r, &p); err != nil {
			falhar(w, err)
			return
		}
		cab, cl := strings.TrimSpace(p.Cabecalho), strings.TrimSpace(p.Clausulas)
		if cab == "" {
			cab = TermoCabecalhoPadrao
		}
		if cl == "" {
			cl = TermoClausulasPadrao
		}
		if err := s.cfg.Mudar(func(c *Config) { c.TermoCabecalho, c.TermoClausulas = cab, cl }, nil, nil); err != nil {
			falhar(w, err)
			return
		}
		responder(w, 200, map[string]string{"cabecalho": cab, "clausulas": cl})
	})
}

func (s *Servidor) salvarTermo(w http.ResponseWriter, r *http.Request, id string) {
	var t dados.Termo
	if err := lerJSON(r, &t); err != nil {
		falhar(w, err)
		return
	}
	t.ID = id
	salvo, err := s.Banco.SalvarTermo(t)
	if err != nil {
		falhar(w, err)
		return
	}
	// o carro sem cliente cadastrado fica com o nome/telefone do termo
	if salvo.Placa != "" {
		if _, ok := s.Banco.Veiculo(salvo.Placa); ok {
			s.guardarCliente(salvo.Placa, salvo.Cliente, salvo.Telefone)
		}
	}
	responder(w, 200, salvo)
}

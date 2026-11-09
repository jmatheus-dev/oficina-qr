package zeus

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"oficinaqr/internal/dados"
	"oficinaqr/internal/registro"
)

type Estado struct {
	Ativo     bool      `json:"ativo"`
	Lendo     bool      `json:"lendo"`
	Ultima    time.Time `json:"ultima,omitempty"`
	Erro      string    `json:"erro,omitempty"`
	Novos     int       `json:"novos"`     // lançamentos novos na última leitura
	Pendentes int       `json:"pendentes"` // esperando alguém escolher a placa
	Esperando int       `json:"esperando"` // vendas novas esperando terminar de ser lançadas
}

// Monitor lê o Zeus de tempos em tempos.
type Monitor struct {
	banco  *dados.Banco
	config func() Config
	fonte  func(Config) Fonte
	// chamado para cada lançamento novo com placa (registra e imprime)
	AoChegar func(l dados.Lancamento)
	// chamado quando um lançamento já registrado ganhou peças novas no Zeus
	AoAtualizar func(l dados.Lancamento)
	// chamado para cada lançamento novo que ficou esperando revisão
	AoNovo func(l dados.Lancamento)
	// para os testes trocarem o relógio
	Agora func() time.Time

	mu     sync.Mutex
	estado Estado
	agora  chan struct{}
	vistos map[string]visto // vendas novas esperando parar de mudar
}

type visto struct {
	assinatura string
	desde      time.Time
}

func NovoMonitor(b *dados.Banco, config func() Config) *Monitor {
	return &Monitor{banco: b, config: config, agora: make(chan struct{}, 1), Agora: time.Now, vistos: map[string]visto{},
		fonte: func(c Config) Fonte {
			if arq := os.Getenv("OFICINAQR_ZEUS_TESTE"); arq != "" {
				return Arquivo(arq)
			}
			if c.ModoEfetivo() == ModoPDF {
				return PastaPDF{Dir: c.PastaPDF}
			}
			return NovoPostgres(c)
		}}
}

func (m *Monitor) Estado() Estado {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.estado
	e.Ativo = m.config().Ativo
	e.Pendentes = len(m.banco.Lancamentos(dados.Pendente, 0))
	e.Esperando = len(m.vistos)
	return e
}

// LerAgora pede uma leitura já.
func (m *Monitor) LerAgora() {
	select {
	case m.agora <- struct{}{}:
	default:
	}
}

func (m *Monitor) Rodar(parar <-chan struct{}) {
	for {
		cfg := m.config()
		intervalo := time.Duration(cfg.Intervalo) * time.Second
		if intervalo < 15*time.Second {
			intervalo = 20 * time.Second
		}
		if cfg.ModoEfetivo() == ModoPDF && intervalo > 15*time.Second {
			intervalo = 15 * time.Second // olhar a pasta é rápido: a etiqueta sai logo
		}
		if cfg.Ativo {
			m.Ler()
		}
		select {
		case <-parar:
			return
		case <-m.agora:
		case <-time.After(intervalo):
		}
	}
}

// Ler faz uma leitura agora (devolve quantos lançamentos novos chegaram).
func (m *Monitor) Ler() (int, error) {
	cfg := m.config()
	m.mu.Lock()
	if m.estado.Lendo {
		m.mu.Unlock()
		return 0, nil
	}
	m.estado.Lendo = true
	m.mu.Unlock()

	n, err := m.ler(cfg)
	if err != nil {
		registro.Erro("leitura do Zeus: %v", err)
	}

	m.mu.Lock()
	m.estado.Lendo = false
	m.estado.Ultima = time.Now()
	m.estado.Novos = n
	if err != nil {
		m.estado.Erro = err.Error()
	} else {
		m.estado.Erro = ""
	}
	m.mu.Unlock()
	return n, err
}

// janela de leitura: desde a data inicial configurada, mas sem voltar mais
// que 3 dias antes da última leitura boa (a consulta não cresce para sempre).
func (m *Monitor) janela(cfg Config) string {
	desde := cfg.Desde
	if desde == "" {
		desde = m.Agora().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if c := m.banco.ZeusCursor(); len(c) == 10 {
		if t, err := time.Parse("2006-01-02", c); err == nil {
			if d := t.AddDate(0, 0, -3).Format("2006-01-02"); d > desde {
				desde = d
			}
		}
	}
	return desde
}

func assinatura(l dados.Lancamento) string {
	return strings.Join(l.Produtos, "|") + "#" + l.Texto + "#" + l.Cliente
}

func (m *Monitor) ler(cfg Config) (int, error) {
	f := m.fonte(cfg)
	if pg, ok := f.(*Postgres); ok {
		defer pg.Fechar()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	linhas, err := f.Buscar(ctx, m.janela(cfg))
	if err != nil {
		return 0, err
	}
	// clientes com um carro só: dá para sugerir a placa pelo nome
	conhecidas := map[string]string{}
	repetidos := map[string]bool{}
	for _, v := range m.banco.BuscarVeiculos("", 0) {
		c := strings.ToUpper(strings.TrimSpace(v.Cliente))
		if c == "" {
			continue
		}
		if _, ja := conhecidas[c]; ja {
			repetidos[c] = true
		}
		conhecidas[c] = v.Placa
	}
	for c := range repetidos {
		delete(conhecidas, c)
	}
	espera := time.Duration(cfg.EsperaMin) * time.Minute
	if _, ehPDF := f.(PastaPDF); ehPDF {
		espera = 0 // o PDF já é o documento pronto: registra na hora
	}
	agora := m.Agora()
	vistosAgora := map[string]bool{}
	novos := 0
	for _, l := range Agrupar(linhas, conhecidas) {
		if len(l.Itens) == 0 && len(l.Outros) == 0 && !l.SoMaoDeObra {
			continue // só combustível, taxa etc.
		}
		vistosAgora[l.Ref] = true
		if antigo, ok := m.banco.Lancamento(l.Ref); ok {
			// já chegou antes: se a venda mudou no Zeus, atualiza
			atual, mudou, err := m.banco.AtualizarLancamento(l)
			if err != nil {
				return novos, err
			}
			if !mudou {
				continue
			}
			registro.Info("lançamento %s mudou no Zeus (%d produtos)", l.Ref, len(l.Produtos))
			switch {
			case antigo.Status == dados.Registrado && m.AoAtualizar != nil:
				m.AoAtualizar(atual)
			case atual.Status == dados.Pendente && atual.Placa != "" && cfg.AutoRegist && m.AoChegar != nil:
				m.AoChegar(atual)
			}
			continue
		}
		// venda nova: espera ela parar de mudar (o balcão ainda pode estar lançando peças)
		if espera > 0 {
			a := assinatura(l)
			m.mu.Lock()
			v, ok := m.vistos[l.Ref]
			if !ok || v.assinatura != a {
				m.vistos[l.Ref] = visto{a, agora}
				m.mu.Unlock()
				continue
			}
			m.mu.Unlock()
			// folga de 2 s: a leitura nunca cai exatamente no mesmo segundo
			if agora.Sub(v.desde) < espera-2*time.Second {
				continue
			}
		}
		m.mu.Lock()
		delete(m.vistos, l.Ref)
		m.mu.Unlock()
		salvo, novo, err := m.banco.ReceberLancamento(l)
		if err != nil {
			return novos, err
		}
		if !novo {
			continue
		}
		novos++
		registro.Info("lançamento novo do Zeus: %s placa=%q itens=%v outros=%v", l.Ref, l.Placa, l.Itens, l.Outros)
		if salvo.Placa != "" && cfg.AutoRegist && m.AoChegar != nil {
			m.AoChegar(salvo)
		} else if m.AoNovo != nil {
			m.AoNovo(salvo)
		}
	}
	m.mu.Lock()
	for r := range m.vistos {
		if !vistosAgora[r] {
			delete(m.vistos, r)
		}
	}
	m.mu.Unlock()
	m.banco.SalvarZeusCursor(agora.Format("2006-01-02"))
	return novos, nil
}

// Esperando diz quantas vendas novas estão esperando parar de mudar.
func (m *Monitor) Esperando() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.vistos)
}

/* --------------------------------------------- fonte de teste (JSON) --- */

// Arquivo lê as linhas de um JSON (lista de Linha) — usado nos testes.
type Arquivo string

func (a Arquivo) Buscar(ctx context.Context, desde string) ([]Linha, error) {
	b, err := os.ReadFile(string(a))
	if err != nil {
		return nil, err
	}
	var l []Linha
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	var out []Linha
	for _, x := range l {
		if soData(x.Data) >= desde {
			out = append(out, x)
		}
	}
	return out, nil
}

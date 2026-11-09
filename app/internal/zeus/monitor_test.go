package zeus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oficinaqr/internal/dados"
)

func gravar(t *testing.T, arq string, l []Linha) {
	b, _ := json.Marshal(l)
	if err := os.WriteFile(arq, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEsperaVendaTerminarEAtualizacoes(t *testing.T) {
	pasta := t.TempDir()
	b, err := dados.Abrir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	arq := filepath.Join(pasta, "zeus.json")
	hoje := time.Now().Format("2006-01-02")
	cfg := Config{Ativo: true, AutoRegist: true, EsperaMin: 3, Desde: hoje}
	m := NovoMonitor(b, func() Config { return cfg })
	m.fonte = func(Config) Fonte { return Arquivo(arq) }
	relogio := time.Now()
	m.Agora = func() time.Time { return relogio }
	var chegou, atualizou []dados.Lancamento
	m.AoChegar = func(l dados.Lancamento) {
		chegou = append(chegou, l)
		b.MarcarLancamento(l.Ref, dados.Registrado, "S1")
	}
	m.AoAtualizar = func(l dados.Lancamento) { atualizou = append(atualizou, l) }

	venda := []Linha{{Ref: "v1", Data: hoje, Descricao: "OLEO 5W30 1L", Texto: "PLACA ABC1D23"}}
	gravar(t, arq, venda)
	if n, err := m.Ler(); err != nil || n != 0 {
		t.Fatalf("1ª leitura não pode registrar ainda: n=%d err=%v", n, err)
	}
	if m.Estado().Esperando != 1 {
		t.Fatal("venda devia estar esperando")
	}
	// o balcão lança mais uma peça 1 minuto depois: o relógio recomeça
	relogio = relogio.Add(time.Minute)
	venda = append(venda, Linha{Ref: "v1", Data: hoje, Descricao: "FILTRO DE OLEO PSL55", Texto: "PLACA ABC1D23"})
	gravar(t, arq, venda)
	m.Ler()
	relogio = relogio.Add(2 * time.Minute)
	if n, _ := m.Ler(); n != 0 {
		t.Fatal("ainda não passou 3 minutos sem mudança")
	}
	relogio = relogio.Add(90 * time.Second)
	if n, _ := m.Ler(); n != 1 || len(chegou) != 1 {
		t.Fatalf("depois de 3 min parada devia registrar (n=%d chegou=%d)", n, len(chegou))
	}
	if got := chegou[0].Itens; len(got) != 2 {
		t.Fatalf("itens: %v", got)
	}
	// depois de registrada, ganhou mais uma peça: atualiza (sem registrar de novo)
	venda = append(venda, Linha{Ref: "v1", Data: hoje, Descricao: "MANGUEIRA DE AR", Texto: "PLACA ABC1D23"})
	gravar(t, arq, venda)
	m.Ler()
	if len(chegou) != 1 || len(atualizou) != 1 || atualizou[0].Outros[0] != "Mangueira de ar" {
		t.Fatalf("atualização: chegou=%d atualizou=%v", len(chegou), atualizou)
	}
	if b.ZeusCursor() != relogio.Format("2006-01-02") {
		t.Error("cursor de leitura não foi salvo")
	}
	// lida de novo sem mudança: nada acontece
	m.Ler()
	if len(atualizou) != 1 {
		t.Error("não devia atualizar sem mudança")
	}
}

func TestJanelaDeLeitura(t *testing.T) {
	b, _ := dados.Abrir(t.TempDir())
	cfg := Config{Desde: "2026-01-01"}
	m := NovoMonitor(b, func() Config { return cfg })
	if m.janela(cfg) != "2026-01-01" {
		t.Error("sem cursor usa a data inicial")
	}
	b.SalvarZeusCursor("2026-10-22")
	if got := m.janela(cfg); got != "2026-10-19" {
		t.Errorf("com cursor volta só 3 dias: %s", got)
	}
	cfg.Desde = "2026-10-21"
	if got := m.janela(cfg); got != "2026-10-21" {
		t.Errorf("nunca antes da data inicial: %s", got)
	}
}

func TestParaUTF8(t *testing.T) {
	if paraUTF8("MANGUEIRA \xc9 \xd3LEO") != "MANGUEIRA É ÓLEO" {
		t.Errorf("WIN1252: %q", paraUTF8("MANGUEIRA \xc9 \xd3LEO"))
	}
	if paraUTF8("Óleo") != "Óleo" {
		t.Error("UTF-8 deve ficar igual")
	}
}

// Padrão da versão 1.3: a venda aparece 1 minuto depois de parar de mudar,
// lendo o banco a cada 20 segundos.
func TestEsperaPadraoUmMinuto(t *testing.T) {
	pasta := t.TempDir()
	b, _ := dados.Abrir(pasta)
	arq := filepath.Join(pasta, "zeus.json")
	hoje := time.Now().Format("2006-01-02")
	cfg := ConfigPadrao()
	cfg.Ativo, cfg.Desde = true, hoje
	if cfg.EsperaMin != 1 || cfg.Intervalo != 20 {
		t.Fatalf("padrão: espera %d min, intervalo %d s", cfg.EsperaMin, cfg.Intervalo)
	}
	m := NovoMonitor(b, func() Config { return cfg })
	m.fonte = func(Config) Fonte { return Arquivo(arq) }
	relogio := time.Now()
	m.Agora = func() time.Time { return relogio }
	gravar(t, arq, []Linha{{Ref: "v9", Data: hoje, Descricao: "OLEO LUBRAX 5W30 SINTETICO 1L", Texto: "PLACA ABC1D23"}})
	for i, quer := range []int{0, 0, 0, 1} { // leituras a cada ~20 s: a 4ª (≈ 1 min) já entra
		n, err := m.Ler()
		if err != nil || n != quer {
			t.Fatalf("leitura %d (%v): n=%d err=%v", i, i*20, n, err)
		}
		relogio = relogio.Add(20*time.Second - 300*time.Millisecond)
	}
}

package servidor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"oficinaqr/internal/dados"
)

func novoTeste(t *testing.T, configAntiga string) (*Servidor, string) {
	t.Helper()
	pasta := t.TempDir()
	if configAntiga != "" {
		os.WriteFile(filepath.Join(pasta, "config.json"), []byte(configAntiga), 0o600)
	}
	s, err := Novo(pasta, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	return s, pasta
}

// config.json da versão 1.2 (espera 3 min, leitura a cada 60 s) passa para 1 min / 20 s
func TestConfigAntigaPassaParaUmMinuto(t *testing.T) {
	s, pasta := novoTeste(t, `{"zeus":{"ativo":true,"espera_min":3,"intervalo_seg":60,"sql":"select 1"},"copias":1}`)
	z := s.cfg.Zeus()
	if z.EsperaMin != 1 || z.Intervalo != 20 {
		t.Fatalf("espera %d min, intervalo %d s", z.EsperaMin, z.Intervalo)
	}
	var gravado Config
	b, _ := os.ReadFile(filepath.Join(pasta, "config.json"))
	json.Unmarshal(b, &gravado)
	if gravado.Ajustes != 1 || gravado.Zeus.EsperaMin != 1 || gravado.TermoClausulas == "" {
		t.Fatalf("config.json não foi gravado com a mudança: %+v", gravado.Zeus)
	}
	// quem escolher 3 de novo depois não é desfeito na próxima abertura
	s.cfg.Mudar(func(c *Config) { c.Zeus.EsperaMin = 3 }, nil, nil)
	s2, err := Novo(pasta, fstest.MapFS{})
	if err != nil || s2.cfg.Zeus().EsperaMin != 3 {
		t.Fatalf("escolha da pessoa foi desfeita: %v", s2.cfg.Zeus().EsperaMin)
	}
	// programa novo (sem config.json) já nasce com 1 minuto
	s3, _ := novoTeste(t, "")
	if s3.cfg.Zeus().EsperaMin != 1 {
		t.Fatal("padrão novo devia ser 1 minuto")
	}
}

func TestAbrirJanelaNaRevisao(t *testing.T) {
	s, _ := novoTeste(t, "")
	s.porta = 47135
	var abertas []string
	s.AbrirJanela = func(u string) error { abertas = append(abertas, u); return nil }
	// janela fechada (ninguém perguntou o estado): abre direto na revisão da venda
	s.abrirRevisao([]string{"venda:12/A"})
	if len(abertas) != 1 || abertas[0] != "http://127.0.0.1:47135/#/revisar/venda:12%2FA" {
		t.Fatalf("abertas: %v", abertas)
	}
	// várias vendas: abre na lista
	s.abrirRevisao([]string{"a", "b"})
	if abertas[1] != "http://127.0.0.1:47135/#/" {
		t.Fatalf("várias: %v", abertas)
	}
	// janela aberta: ela mesma se redireciona (não abre outra)
	s.ultimaTela.Store(time.Now().UnixNano())
	s.abrirRevisao([]string{"c"})
	if len(abertas) != 2 {
		t.Fatalf("abriu janela repetida: %v", abertas)
	}
}

// a marca do óleo revisada vai para o serviço mesmo quando a venda junta no serviço do dia
func TestObsDoOleoAoJuntar(t *testing.T) {
	s, _ := novoTeste(t, "")
	hoje := time.Now().Format("2006-01-02")
	sv, _, err := s.Banco.RegistrarServico(dados.NovoServico{Placa: "BRA2E19", Data: hoje, Itens: []string{"Filtro de ar"}})
	if err != nil {
		t.Fatal(err)
	}
	l := dados.Lancamento{Ref: "v1", Documento: "Venda 1", Data: hoje, Produtos: []string{"OLEO LUBRAX 5W30 SINTETICO 1L"}, Itens: []string{"Óleo do motor"}, Placa: "BRA2E19"}
	if _, _, err := s.Banco.ReceberLancamento(l); err != nil {
		t.Fatal(err)
	}
	junto, _, err := s.registrarLancamento(l, dados.NovoServico{Placa: "BRA2E19", Data: hoje, Itens: l.Itens, ObsCliente: "Óleo Lubrax 5W30 sintético", ProximaKM: 105000})
	if err != nil || junto.ID != sv.ID {
		t.Fatalf("devia juntar: %v %v", junto.ID, err)
	}
	if junto.ObsCliente != "Óleo Lubrax 5W30 sintético" || junto.ProximaKM != 105000 || !strings.Contains(strings.Join(junto.Itens, ","), "Óleo do motor") {
		t.Fatalf("serviço juntado: %+v", junto)
	}
}

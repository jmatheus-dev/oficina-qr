package zeus

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"oficinaqr/internal/dados"
)

func copiar(t *testing.T, de, para string) {
	b, err := os.ReadFile(de)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(para, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPDFDoZeusRegistraNaHora(t *testing.T) {
	pasta := t.TempDir()
	b, err := dados.Abrir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	entrada := filepath.Join(pasta, "entrada")
	cfg := Config{Ativo: true, AutoRegist: true, EsperaMin: 3, Desde: time.Now().Format("2006-01-02"), Modo: ModoPDF, PastaPDF: entrada}
	m := NovoMonitor(b, func() Config { return cfg })
	var chegou []dados.Lancamento
	m.AoChegar = func(l dados.Lancamento) {
		chegou = append(chegou, l)
		b.MarcarLancamento(l.Ref, dados.Registrado, "S1")
	}
	// pasta ainda não existe: é criada, sem erro
	if n, err := m.Ler(); err != nil || n != 0 {
		t.Fatalf("pasta vazia: n=%d err=%v", n, err)
	}
	if _, err := os.Stat("../pdfzeus/testdata/orcamento-17-mini.pdf"); err != nil {
		t.Skip("PDF de exemplo do Zeus não incluído (dados de cliente)")
	}
	copiar(t, "../pdfzeus/testdata/orcamento-17-mini.pdf", filepath.Join(entrada, "orcamento 17.pdf"))
	n, err := m.Ler()
	if err != nil || n != 1 || len(chegou) != 1 {
		t.Fatalf("devia registrar na hora (sem esperar 3 min): n=%d err=%v chegou=%d", n, err, len(chegou))
	}
	l := chegou[0]
	if l.Ref != "pdf:orcamento-17" || l.Documento != "Orçamento 17" || l.Placa != "TST1234" || l.Cliente != "CLIENTE EXEMPLO LTDA" {
		t.Errorf("lançamento errado: %+v", l)
	}
	if len(l.Produtos) != 8 {
		t.Errorf("produtos: %v", l.Produtos)
	}
	// salvo de novo (mesmo orçamento): não duplica
	copiar(t, "../pdfzeus/testdata/orcamento-17-mini.pdf", filepath.Join(entrada, "orcamento 17 (2).pdf"))
	if n, _ := m.Ler(); n != 0 || len(chegou) != 1 {
		t.Fatalf("o mesmo orçamento salvo duas vezes não pode registrar de novo (n=%d)", n)
	}
	// PDF que não é do Zeus: ignora sem travar
	os.WriteFile(filepath.Join(entrada, "outro.pdf"), []byte("%PDF-1.4 lixo"), 0o644)
	if _, err := m.Ler(); err != nil {
		t.Fatalf("PDF estranho não pode dar erro geral: %v", err)
	}
	info, err := PDFsNaPasta(entrada, 10)
	if err != nil || len(info) != 3 {
		t.Fatalf("lista: %v %v", info, err)
	}
	// arquivos velhos vão para "lidos"
	velho := time.Now().AddDate(0, 0, -10)
	os.Chtimes(filepath.Join(entrada, "orcamento 17.pdf"), velho, velho)
	m.Ler()
	if _, err := os.Stat(filepath.Join(entrada, "lidos", "orcamento 17.pdf")); err != nil {
		t.Errorf("PDF antigo devia ir para lidos: %v", err)
	}
}

func TestModoEfetivo(t *testing.T) {
	if (Config{}).ModoEfetivo() != ModoPDF || (Config{SQL: "select 1"}).ModoEfetivo() != ModoBanco || (Config{Modo: ModoBanco}).ModoEfetivo() != ModoBanco {
		t.Fatal("modo efetivo errado")
	}
}

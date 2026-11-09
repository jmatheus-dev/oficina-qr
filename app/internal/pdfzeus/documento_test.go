package pdfzeus

import (
	"os"
	"strings"
	"testing"
)

func TestOrcamentoMiniDoZeus(t *testing.T) {
	// O PDF real do Zeus usado neste teste tem dados de cliente e não é publicado.
	b, err := os.ReadFile("testdata/orcamento-17-mini.pdf")
	if err != nil {
		t.Skip("PDF de exemplo do Zeus não incluído (dados de cliente)")
	}
	l, err := Linhas(b)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Ler(l)
	if err != nil {
		t.Fatal(err)
	}
	if d.Tipo != "Orçamento" || d.Numero != "17" || d.Data != "2025-10-07" {
		t.Errorf("cabeçalho errado: %+v", d)
	}
	if d.Cliente != "CLIENTE EXEMPLO LTDA" || d.Telefone != "(79)99999-0000" || d.Tecnico != "MECANICO EXEMPLO" {
		t.Errorf("cliente errado: %q %q %q", d.Cliente, d.Telefone, d.Tecnico)
	}
	if len(d.Produtos) != 8 || d.Produtos[0] != "LANTERN TRAS FIAT: UNO 2005/ LD DIREIT CRISTAL" || d.Produtos[7] != "COMBUSTIVEL CARRO DO CLIENTE" {
		t.Errorf("produtos errados: %q", d.Produtos)
	}
	want := "fiat uno 1.0 tst-1234 itabaiana/ se servico, troca de alternador, correia, e piscas do carro."
	if d.Observacoes != want {
		t.Errorf("observações:\n got %q\nwant %q", d.Observacoes, want)
	}
	if strings.ContainsRune(strings.Join(l, ""), '?') {
		t.Errorf("sobrou letra desconhecida")
	}
}

func TestOrcamentoA4(t *testing.T) {
	l := []string{
		"AUTO MECÂNICA ITABAIANA", "Orçamento - 17", "CPF/CNPJ: 61.145.307/0001-75", "07/10/2025",
		"Cliente: 19 - CLIENTE EXEMPLO LTDA Apelido: CLIENTE EXEMPLO",
		"Telefone: (79)99999-0000 CPF / CNPJ: 00.000.000/0001-00",
		"Impresso por icaro em 22/10/2026 às 10:27:37 Valores em R$ 1,00",
		"Código Item / Descrição Inf. Complem. Unidade QTDE Preço Desconto Valor Total",
		"302 LANTERN TRAS FIAT: UNO 2005/ LD DIREIT CRISTAL UNID 1 110,00 0,00% 110,00",
		"308 COMBUSTIVEL CARRO DO CLIENTE L 1 30,00 0,00% 30,00",
		"Número de ítens: 8 Quantidade Total: 10",
	}
	d, err := Ler(l)
	if err != nil {
		t.Fatal(err)
	}
	if d.Numero != "17" || d.Data != "2025-10-07" || d.Cliente != "CLIENTE EXEMPLO LTDA" || len(d.Produtos) != 2 || d.Produtos[1] != "COMBUSTIVEL CARRO DO CLIENTE" {
		t.Errorf("%+v", d)
	}
}

func TestNaoEhZeus(t *testing.T) {
	if _, err := Ler([]string{"qualquer coisa"}); err == nil {
		t.Error("devia dar erro")
	}
	if _, err := Linhas([]byte("não é pdf")); err == nil {
		t.Error("devia dar erro")
	}
}

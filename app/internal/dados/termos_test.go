package dados

import "testing"

func TestTermos(t *testing.T) {
	b, err := Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SalvarTermo(Termo{Cliente: "  ", Pecas: []string{"Bomba d'água"}}); err != ErrTermoCliente {
		t.Fatalf("sem cliente: %v", err)
	}
	if _, err := b.SalvarTermo(Termo{Cliente: "João", Pecas: []string{" "}}); err != ErrTermoPeca {
		t.Fatalf("sem peça: %v", err)
	}
	if _, err := b.SalvarTermo(Termo{Cliente: "João", Pecas: []string{"x"}, Placa: "123"}); err != ErrPlaca {
		t.Fatalf("placa ruim: %v", err)
	}
	t1, err := b.SalvarTermo(Termo{Cliente: "João  da Silva", Placa: "abc-1d23", Pecas: []string{"Bomba d'água Urba", ""}, Data: "2026-10-22"})
	if err != nil || t1.Numero != 1 || t1.Placa != "ABC1D23" || len(t1.Pecas) != 1 || t1.Assinado || t1.Cliente != "João da Silva" {
		t.Fatalf("1º termo: %+v %v", t1, err)
	}
	t2, _ := b.SalvarTermo(Termo{Cliente: "Maria", Pecas: []string{"Amortecedor"}, Assinatura: prefixoAssinatura + "AAAA"})
	if t2.Numero != 2 || !t2.Assinado || t2.AssinadoEm.IsZero() {
		t.Fatalf("2º termo: %+v", t2)
	}
	if _, err := b.SalvarTermo(Termo{Cliente: "Maria", Pecas: []string{"x"}, Assinatura: "javascript:x"}); err != ErrTermoAssin {
		t.Fatalf("assinatura ruim: %v", err)
	}
	// assina no papel depois
	t1.AssinadoPapel = true
	t1b, err := b.SalvarTermo(t1)
	if err != nil || !t1b.Assinado || t1b.Numero != 1 || !t1b.CriadoEm.Equal(t1.CriadoEm) {
		t.Fatalf("alterar: %+v %v", t1b, err)
	}
	l := b.Termos("", "", 0)
	if len(l) != 2 || l[0].Numero != 2 || l[0].Assinatura != "" {
		t.Fatalf("lista: %+v", l)
	}
	if got := b.Termos("ABC1D23", "", 0); len(got) != 1 {
		t.Fatalf("por placa: %d", len(got))
	}
	if got := b.Termos("", "bomba", 0); len(got) != 1 || got[0].Numero != 1 {
		t.Fatalf("busca peça: %+v", got)
	}
	if got := b.Termos("", "Nº 2", 0); len(got) != 1 || got[0].Cliente != "Maria" {
		t.Fatalf("busca número: %+v", got)
	}
	full, _ := b.Termo(t2.ID)
	if full.Assinatura == "" {
		t.Fatal("termo completo devia ter a assinatura")
	}
	// grava e lê de novo
	b2, _ := Abrir(b.Pasta())
	if len(b2.Termos("", "", 0)) != 2 {
		t.Fatal("termos não foram gravados")
	}
	if err := b.ApagarTermo(t1.ID); err != nil || len(b.Termos("", "", 0)) != 1 {
		t.Fatalf("apagar: %v", err)
	}
	t3, _ := b.SalvarTermo(Termo{Cliente: "Zé", Pecas: []string{"Correia"}})
	if t3.Numero != 3 {
		t.Fatalf("número segue: %d", t3.Numero)
	}
}

func TestComplementarServico(t *testing.T) {
	b, _ := Abrir(t.TempDir())
	sv, _, err := b.RegistrarServico(NovoServico{Placa: "ABC1D23", Data: "2026-10-22", Itens: []string{"Filtro de óleo"}})
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := b.ComplementarServico(sv.ID, "Óleo Lubrax 5W30 sintético", 60000, "", 0, "")
	if s2.ObsCliente != "Óleo Lubrax 5W30 sintético" || s2.ProximaKM != 60000 {
		t.Fatalf("%+v", s2)
	}
	s3, _ := b.ComplementarServico(sv.ID, "Óleo Lubrax 5W30 sintético", 70000, "", 0, "")
	if s3.ObsCliente != "Óleo Lubrax 5W30 sintético" || s3.ProximaKM != 60000 {
		t.Fatalf("não pode repetir nem trocar a próxima: %+v", s3)
	}
}

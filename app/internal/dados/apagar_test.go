package dados

import "testing"

func TestApagarVarios(t *testing.T) {
	b, _ := Abrir(t.TempDir())
	s1, _, _ := b.RegistrarServico(NovoServico{Placa: "ABC1D23", Data: "2026-10-20", Itens: []string{"Óleo do motor"}})
	s2, _, _ := b.RegistrarServico(NovoServico{Placa: "ABC1D23", Data: "2026-10-21", Itens: []string{"Filtro de ar"}, Origem: "zeus", ZeusRef: "v:1"})
	b.RegistrarServico(NovoServico{Placa: "XYZ9A88", Data: "2026-10-21", Itens: []string{"Filtro de ar"}, Origem: "zeus", ZeusRef: "v:2,v:3"})
	b.RegistrarServico(NovoServico{Placa: "BRA2E19", Data: "2026-10-21", Itens: []string{"Filtro de ar"}})
	for _, ref := range []string{"v:1", "v:2", "v:3"} {
		b.ReceberLancamento(Lancamento{Ref: ref, Documento: ref, Data: "2026-10-21", Produtos: []string{"x"}})
		b.MarcarLancamento(ref, Registrado, "x")
	}
	// um serviço avulso + um carro inteiro
	r, err := b.ApagarVarios([]string{s2.ID}, []string{"xyz-9a88"}, false)
	if err != nil || r.Servicos != 2 || r.Carros != 1 || len(r.Voltaram) != 3 {
		t.Fatalf("%+v %v", r, err)
	}
	if _, ok := b.Veiculo("XYZ9A88"); ok {
		t.Fatal("carro devia ter sumido")
	}
	if _, ok := b.Veiculo("ABC1D23"); !ok {
		t.Fatal("carro de quem só perdeu um serviço continua")
	}
	if l, _ := b.Lancamento("v:2"); l.Status != Pendente {
		t.Fatal("venda do Zeus devia voltar para revisar")
	}
	if len(b.ServicosDoVeiculo("ABC1D23")) != 1 || b.ServicosDoVeiculo("ABC1D23")[0].ID != s1.ID {
		t.Fatal("serviço errado apagado")
	}
	// ignorando o Zeus
	b.RegistrarServico(NovoServico{Placa: "ABC1D23", Data: "2026-10-22", Itens: []string{"Filtro de ar"}, Origem: "zeus", ZeusRef: "v:1"})
	b.MarcarLancamento("v:1", Registrado, "x")
	r, _ = b.ApagarVarios(nil, []string{"ABC1D23"}, true)
	if r.Carros != 1 || len(r.Voltaram) != 0 {
		t.Fatalf("%+v", r)
	}
	if l, _ := b.Lancamento("v:1"); l.Status != Ignorado {
		t.Fatalf("venda devia ficar ignorada: %s", l.Status)
	}
	if _, err := b.ApagarVarios([]string{"nada"}, nil, false); err != ErrNaoAchado {
		t.Fatal("nada para apagar devia dar erro")
	}
	// grava
	b2, _ := Abrir(b.Pasta())
	if len(b2.BuscarVeiculos("", 0)) != 1 {
		t.Fatal("não gravou")
	}
}

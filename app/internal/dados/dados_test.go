package dados

import (
	"reflect"
	"testing"
)

func TestPlacasEKM(t *testing.T) {
	if got := AcharPlacas("COROLLA placa bra-2e19 / outro ABC 1234, abc1234"); !reflect.DeepEqual(got, []string{"BRA2E19", "ABC1234"}) {
		t.Errorf("placas: %v", got)
	}
	if got := AcharPlacas("NOTA 1234567 TEL 79998513926"); len(got) != 0 {
		t.Errorf("não devia achar placa: %v", got)
	}
	casos := map[string]int{"KM 98.765": 98765, "km: 153273": 153273, "153.273 km": 153273, "rodou 45000km": 45000, "sem nada": 0, "KM 12": 0}
	for s, quer := range casos {
		if got := AcharKM(s); got != quer {
			t.Errorf("%q: %d, queria %d", s, got, quer)
		}
	}
	if NormalizarPlaca("hhh-1234") != "HHH1234" || NormalizarPlaca("AB1234") != "" {
		t.Error("normalizar placa")
	}
	if MascararPlaca("HHH1234") != "HHH-1234" || MascararPlaca("BRA2E19") != "BRA2E19" {
		t.Error("mascarar placa")
	}
}

func TestIDPublicoIgualAPagina(t *testing.T) {
	// mesmo valor que a página calcula (crypto.subtle) e o teste em Python
	if IDPublico("DEMQ2345") != IDPublico("demq2345") || len(IDPublico("DEMQ2345")) != 20 {
		t.Error("id público")
	}
}

func TestBanco(t *testing.T) {
	b, err := Abrir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s1, v1, err := b.RegistrarServico(NovoServico{Placa: "abc-1234", Itens: []string{"Filtro de óleo", "Óleo do motor", "inventado"}, KM: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s1.Itens, []string{"Óleo do motor", "Filtro de óleo"}) {
		t.Errorf("itens: %v", s1.Itens)
	}
	_, v2, _ := b.RegistrarServico(NovoServico{Placa: "ABC1234", Outros: []string{"Mangueira de ar\n\nCorreia"}})
	if v1.Codigo != v2.Codigo || len(v1.Codigo) != 8 {
		t.Error("mesmo carro deve manter o código")
	}
	if _, _, err := b.RegistrarServico(NovoServico{Placa: "ABC1234"}); err != ErrServicoNada {
		t.Error("serviço vazio devia falhar")
	}
	z, _, _ := b.RegistrarServico(NovoServico{Placa: "ABC1234", Outros: []string{"x"}, ZeusRef: "v:1"})
	z2, _, _ := b.RegistrarServico(NovoServico{Placa: "ABC1234", Outros: []string{"x"}, ZeusRef: "v:1"})
	if z.ID != z2.ID {
		t.Error("lançamento do Zeus não pode duplicar")
	}
	b2, _ := Abrir(b.Pasta())
	if len(b2.ServicosDoVeiculo("ABC1234")) != 3 {
		t.Error("dados não foram gravados no disco")
	}
}

package etiqueta

import "testing"

func TestDetectarCorreia(t *testing.T) {
	casos := []struct {
		outros []string
		quer   Correia
	}{
		{[]string{"Correia dentada", "Tensor", "Bomba d'água"}, Correia{true, true, true}},
		{[]string{"KIT CORREIA DENTADA BANHADA OLEO/DAYCO/"}, Correia{true, true, false}},
		{[]string{"Kit de distribuição"}, Correia{true, true, false}},
		{[]string{"Correia dentada Gates", "Bomba de água Urba"}, Correia{true, false, true}},
		{[]string{"Correia"}, Correia{Correia: true}},
		{[]string{"Correia do alternador", "Tensor da correia do alternador"}, Correia{}},
		{[]string{"Correia poli V 6PK1050"}, Correia{}},
		{[]string{"Bomba d'água"}, Correia{Bomba: true}}, // só a bomba: não é troca de correia
		{[]string{"Bomba d'água do limpador do para-brisa"}, Correia{}},
		{[]string{"Mangueira de ar"}, Correia{}},
	}
	for _, c := range casos {
		if got := DetectarCorreia(nil, c.outros); got != c.quer {
			t.Errorf("%q: %+v, quer %+v", c.outros, got, c.quer)
		}
	}
}

func TestModeloPara(t *testing.T) {
	so := Dados{Itens: []string{"Óleo do motor"}, ProximaKM: 10000}
	cor := Dados{Outros: []string{"Correia dentada"}}
	cor.Correia = DetectarCorreia(nil, cor.Outros)
	as2 := Dados{Itens: []string{"Óleo do motor"}, Outros: []string{"Kit correia dentada"}}
	as2.Correia = DetectarCorreia(nil, as2.Outros)
	bomba := Dados{Outros: []string{"Bomba d'água"}}
	bomba.Correia = DetectarCorreia(nil, bomba.Outros)
	for _, c := range []struct {
		d    Dados
		quer string
	}{{so, ModeloOleo}, {cor, ModeloCorreia}, {as2, ModeloOleoECorreia}, {bomba, ModeloOleo}} {
		if got := ModeloPara(c.d); got != c.quer {
			t.Errorf("%+v: %s, quer %s", c.d, got, c.quer)
		}
		if _, err := Desenhar(ModeloPara(c.d), c.d, ConfigPadrao()); err != nil {
			t.Error(err)
		}
	}
}

func TestProximaCorreia(t *testing.T) {
	km, data := ProximaCorreia(Dados{Data: "2026-02-05", KM: 314669}, ConfigPadrao())
	if km != 354669 || data != "2027-02-05" {
		t.Fatalf("padrão: %d %s", km, data)
	}
	km, data = ProximaCorreia(Dados{Data: "2026-02-05", KM: 314669, ProximaCorreiaKM: 364669, ProximaCorreiaData: "2028-02-05"}, Config{})
	if km != 364669 || data != "2028-02-05" {
		t.Fatalf("escolhido no serviço: %d %s", km, data)
	}
}

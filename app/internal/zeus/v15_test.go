package zeus

import "testing"

func TestAgruparMaoDeObraEPlacaDoZeus(t *testing.T) {
	ls := Agrupar([]Linha{
		{Ref: "1", Descricao: "MAO DE OBRA", Texto: "PLACA ABC1D23"},
		{Ref: "2", Descricao: "COMBUSTIVEL"},
		{Ref: "3", Descricao: "FILTRO OLEO PSL55", Texto: "obs sem placa", Placa: "bra-2e19", KM: "98.765"},
	}, nil)
	por := map[string]int{}
	for i, l := range ls {
		por[l.Ref] = i
	}
	if l := ls[por["1"]]; !l.SoMaoDeObra || l.Placa != "ABC1D23" {
		t.Errorf("só mão de obra: %+v", l)
	}
	if ls[por["2"]].SoMaoDeObra {
		t.Error("combustível não é mão de obra")
	}
	if l := ls[por["3"]]; l.Placa != "BRA2E19" || l.KM != 98765 || l.SoMaoDeObra {
		t.Errorf("placa/KM do Zeus: %+v", l)
	}
}

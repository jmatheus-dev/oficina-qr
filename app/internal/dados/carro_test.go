package dados

import "testing"

func TestAcharModelo(t *testing.T) {
	casos := []struct{ texto, modelo, ano string }{
		{"fiat uno 1.0 tst-1234  itabaiana/ se\r\nservico, troca de alternador", "Fiat Uno 1.0", ""},
		{"jetta 2.0 8v/ 2011. oep9j99: 16/10/2026...", "Jetta 2.0 8V", "2011"},
		{"golf 1.6 mtr power 2008: jmo6h00, dinheiro recebido: 17/10/2026: 4000,00", "Golf 1.6 Mtr Power", "2008"},
		{"s10 1997 2.2 gasolina: kif0f13", "S10 2.2 Gasolina", "1997"},
		{"onix preto 1.0 turbo:qmn5c65", "Onix 1.0 Turbo", ""},
		{"fiat punto essencia 1.6/16v: okn2h40", "Fiat Punto Essencia 1.6/16V", ""},
		{"pajero tr4: hzs7f72.", "Pajero TR4", ""},
		{"Gol Prata: rfh7g80 3 cilindros", "Gol", ""},
		{"corolla xei 2.0 2021/2022, PLACA BRA2E19 KM 98765", "Corolla XEI 2.0", "2021/2022"},
		{"COROLLA XEI 2.0 2021/2022 PLACA BRA2E19", "Corolla XEI 2.0", "2021/2022"},
		{"ford ka de altamir.", "", ""},
		{"PLACA ABC1D23", "", ""},
		{"corolla xei 2.0 2021/2022, de Alan Michell. rek-8g05", "Corolla XEI 2.0", "2021/2022"},
	}
	for _, c := range casos {
		m, a := AcharModelo(c.texto)
		if m != c.modelo || a != c.ano {
			t.Errorf("%q → (%q, %q), queria (%q, %q)", c.texto, m, a, c.modelo, c.ano)
		}
	}
}

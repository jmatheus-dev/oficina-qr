package itens

import "testing"

func TestDescreverOleo(t *testing.T) {
	casos := map[string]string{
		"OLEO LUBRAX 5W30 SINTETICO 1L":                       "Lubrax 5W30 sintético",
		"OLEO LUBRAX 5W30 1L":                                 "Lubrax 5W30",
		"OLEO 15W40 SYNTIUM 800AM SEMISINT. 1L/PETRONAS":      "Petronas Syntium 800 15W40 semissintético",
		"OLEO BXO/ 5W30/GA/ MOTORCRAFT FORD":                  "Motorcraft 5W30",
		"OLEO MOTOR 15W40 CASTROL DIESEL":                     "Castrol 15W40 diesel",
		"OLEO 20W50 SEMI-SINTETICO":                           "20W50 semissintético",
		"OLEO 15W40":                                          "15W40",
		"OLEO DE MOTOR: 5W30":                                 "5W30",
		"OLEO 90 (DULUB)":                                     "Dulub SAE 90",
		"OLEO DE CAIXA 90":                                    "SAE 90",
		"OLEO ULTRAGEAR MTF 75W80 (IPIRANGA)":                 "Ipiranga Ultragear 75W80",
		"OLEO TOTAL TRAX. GEAR 8 75W80/T.ENERGIES":            "TotalEnergies Trax Gear 8 75W80",
		"OLEO UNIGEROL 90/INGRAX             CAIXA DE MARCHA": "Ingrax Unigerol SAE 90",
		"ÓLEO SHELL HELIX HX7 10W40 SEMISSINTÉTICO":           "Shell Helix HX7 10W40 semissintético",
		"OLEO SELENIA 1L PETRONAS 5W30":                       "Petronas Selenia 5W30",
		"OLEO":                                                "",
	}
	for in, quer := range casos {
		if got := DescreverOleo(in); got != quer {
			t.Errorf("DescreverOleo(%q) = %q, quer %q", in, got, quer)
		}
	}
}

func TestObsOleo(t *testing.T) {
	casos := []struct {
		prods []string
		quer  string
	}{
		{[]string{"FILTRO OLEO PSL55", "OLEO LUBRAX 5W30 SINTETICO 1L", "OLEO LUBRAX 5W30 SINTETICO 1L"}, "Óleo Lubrax 5W30 sintético"},
		{[]string{"OLEO 15W40 SYNTIUM 800AM SEMISINT. 1L/PETRONAS", "OLEO ULTRAGEAR MTF 75W80 (IPIRANGA)"},
			"Óleo do motor Petronas Syntium 800 15W40 semissintético · Óleo do câmbio Ipiranga Ultragear 75W80"},
		{[]string{"OLEO DE CAIXA 90"}, "Óleo do câmbio SAE 90"},
		{[]string{"FILTRO OLEO PSL55", "PASTILHA FREIO"}, ""},
		{[]string{"BOMBA DE OLEO", "INTERRUPTOR DE OLEO"}, ""},
	}
	for _, c := range casos {
		if got := ObsOleo(c.prods); got != c.quer {
			t.Errorf("ObsOleo(%q) = %q, quer %q", c.prods, got, c.quer)
		}
	}
}

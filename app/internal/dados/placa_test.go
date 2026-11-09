package dados

import (
	"strings"
	"testing"
	"time"
)

func TestPlacaCifradaNoCSV(t *testing.T) {
	c := CifrarPlaca("BRA2E19", "DEMQ2345")
	if c == "" || strings.Contains(c, "BRA") || c != CifrarPlaca("BRA2E19", "DEMQ2345") {
		t.Fatalf("cifra: %q", c)
	}
	if CifrarPlaca("BRA2E19", "OUTRO123") == c {
		t.Fatal("código diferente devia dar cifra diferente")
	}
	v := map[string]Veiculo{"BRA2E19": {Placa: "BRA2E19", Codigo: "DEMQ2345", Modelo: "Strada"}}
	csv := string(CSVPublico(v, []Servico{{Placa: "BRA2E19", Data: "2026-10-01", Itens: []string{"Filtro de ar"}}}, time.Now()))
	if strings.Contains(csv, "BRA2E19") || !strings.Contains(csv, c) || !strings.Contains(csv, "placa_cifrada") {
		t.Fatalf("csv: %s", csv)
	}
}

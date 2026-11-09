// amostras grava PNGs das etiquetas para conferir o desenho.
package main

import (
	"image/png"
	"os"
	"path/filepath"

	"oficinaqr/internal/etiqueta"
)

func main() {
	pasta := "."
	if len(os.Args) > 1 {
		pasta = os.Args[1]
	}
	cfg := etiqueta.ConfigPadrao()
	casos := map[string]struct {
		modelo string
		d      etiqueta.Dados
	}{
		"oleo": {"oleo", etiqueta.Dados{Codigo: "DEMQ2345", Data: "2025-11-14", KM: 153273, ProximaKM: 160273, ProximaData: "2026-05-14", Modelo: "Toyota Corolla", Placa: "BRA-2E19",
			Itens: []string{"Óleo do motor", "Filtro de óleo"}}},
		"oleo-vazia": {"oleo", etiqueta.Dados{Codigo: "ABCD2345"}},
		"correia": {"correia", etiqueta.Dados{Codigo: "DEMQ2345", Data: "2026-02-05", KM: 314669,
			Correia: etiqueta.Correia{Correia: true, Tensor: true, Bomba: true}}},
		"correia-carro": {"correia", etiqueta.Dados{Codigo: "DEMQ2345", Data: "2026-02-05", KM: 314669, Modelo: "Fiat Uno 1.0", Placa: "TST-1234",
			Correia: etiqueta.Correia{Correia: true, Tensor: true}}},
		"oleo-correia": {"oleo-correia", etiqueta.Dados{Codigo: "DEMQ2345", Data: "2026-02-05", KM: 314669, ProximaKM: 324669, ProximaData: "2026-08-05", Modelo: "Fiat Uno 1.0", Placa: "TST-1234",
			Itens: []string{"Óleo do motor", "Filtro de óleo", "Filtro de ar"}, Correia: etiqueta.Correia{Correia: true, Tensor: true, Bomba: true}}},
		"servicos": {"servicos", etiqueta.Dados{Codigo: "EXPL6789", Data: "2026-10-22", KM: 98765,
			Itens: []string{"Filtro de ar"}, Outros: []string{"Mangueira de ar", "Pastilha de freio dianteira Cobreq", "Alinhamento e balanceamento"},
			Modelo: "Corolla", Placa: "BRA-2E19"}},
		"servicos-muitos": {"servicos", etiqueta.Dados{Codigo: "EXPL6789", Data: "2026-10-22", KM: 98765, ProximaKM: 105000,
			Outros: []string{"Mangueira de ar", "Correia dentada e tensor com bomba d'água original", "Velas de ignição NGK", "Cabos de vela", "Bobina", "Junta da tampa de válvulas", "Limpeza dos bicos"},
			Modelo: "Toyota Corolla XEi 2.0", Placa: "BRA2E19"}},
	}
	for nome, c := range casos {
		img, err := etiqueta.Desenhar(c.modelo, c.d, cfg)
		if err != nil {
			panic(err)
		}
		f, err := os.Create(filepath.Join(pasta, nome+".png"))
		if err != nil {
			panic(err)
		}
		png.Encode(f, img)
		f.Close()
	}
}

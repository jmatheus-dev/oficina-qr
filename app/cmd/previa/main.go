// previa: desenha uma etiqueta de exemplo em PNG (uso interno).
package main

import (
	"image/png"
	"os"

	"oficinaqr/internal/etiqueta"
)

func main() {
	d := etiqueta.Dados{Codigo: "UAMKBRRS", Data: "2026-10-22", KM: 98765, ProximaKM: 108765,
		Itens: []string{"Óleo do motor", "Filtro de óleo"}, Modelo: os.Args[2], Placa: os.Args[3]}
	img, _ := etiqueta.Desenhar("oleo", d, etiqueta.ConfigPadrao())
	f, _ := os.Create(os.Args[1])
	png.Encode(f, img)
	f.Close()
}

// Pacote impressora manda a imagem da etiqueta para a impressora do Windows
// (o driver da iDPRT iE4S, o mesmo que o BarTender usa).
package impressora

import "image"

// Trabalho de impressão.
type Trabalho struct {
	Impressora string      // nome no Windows; vazio = padrão
	Imagem     *image.Gray // já em preto e branco
	DPI        int         // da imagem (203)
	LarguraMM  float64     // tamanho da etiqueta (vai no driver)
	AlturaMM   float64
	Copias     int
	Nome       string // nome do documento na fila
}

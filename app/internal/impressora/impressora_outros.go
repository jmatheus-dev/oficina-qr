//go:build !windows

package impressora

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

// PastaTeste: fora do Windows as etiquetas viram PNG nesta pasta (para testes).
var PastaTeste = func() string {
	if p := os.Getenv("OFICINAQR_IMPRESSOES"); p != "" {
		return p
	}
	return os.TempDir()
}()

func Listar() ([]string, string, error) {
	return []string{"Arquivo PNG (teste)"}, "Arquivo PNG (teste)", nil
}

func Imprimir(t Trabalho) error {
	if t.Copias < 1 {
		t.Copias = 1
	}
	for i := 0; i < t.Copias; i++ {
		nome := filepath.Join(PastaTeste, fmt.Sprintf("etiqueta-%d-%d.png", time.Now().UnixNano(), i))
		f, err := os.Create(nome)
		if err != nil {
			return err
		}
		if err := png.Encode(f, t.Imagem); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

// Pacote sistema cuida do que depende do Windows: pasta dos dados, senhas
// guardadas cifradas, iniciar junto com o Windows e abrir a janela do programa.
package sistema

import (
	"os"
	"path/filepath"
)

// PastaDados é onde ficam dados.json, config.json e as cópias.
func PastaDados() string {
	if p := os.Getenv("OFICINAQR_PASTA"); p != "" {
		return p
	}
	return pastaPadrao()
}

func caminhoNavegador() string { return filepath.Join(PastaDados(), "janela") }

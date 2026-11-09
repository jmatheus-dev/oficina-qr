//go:build !windows

package sistema

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func pastaPadrao() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".oficinaqr")
}

func Cifrar(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	return "b64:" + base64.StdEncoding.EncodeToString([]byte(s)), nil
}

func Decifrar(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > 4 && s[:4] == "b64:" {
		b, err := base64.StdEncoding.DecodeString(s[4:])
		return string(b), err
	}
	return "", fmt.Errorf("formato desconhecido")
}

func IniciarComWindows(ligar bool) error { return nil }
func IniciaComWindows() bool             { return false }

func AbrirJanela(url string) error {
	if os.Getenv("OFICINAQR_SEM_JANELA") != "" {
		return nil
	}
	return exec.Command("xdg-open", url).Start()
}

func AbrirNoNavegador(url string) error {
	if os.Getenv("OFICINAQR_SEM_JANELA") != "" {
		return nil
	}
	return exec.Command("xdg-open", url).Start()
}

func Aviso(titulo, msg string) { fmt.Fprintln(os.Stderr, titulo+": "+msg) }

func AbrirPasta(pasta string) error { return nil }

func Notificar(titulo, msg string) error { return nil }

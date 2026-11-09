// Pacote registro grava o que acontece (e os erros) em oficinaqr.log, na pasta
// dos dados. Serve para descobrir o que deu errado num computador sem tela de
// console. O arquivo é trocado quando passa de 2 MB (fica o anterior .1).
package registro

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	mu     sync.Mutex
	saida  io.Writer = os.Stderr
	arq    *os.File
	nome   string
	limite int64 = 2 << 20
)

// Abrir passa a gravar em pasta/oficinaqr.log.
func Abrir(pasta string) error {
	mu.Lock()
	defer mu.Unlock()
	nome = filepath.Join(pasta, "oficinaqr.log")
	return abrir()
}

func abrir() error {
	if info, err := os.Stat(nome); err == nil && info.Size() > limite {
		os.Remove(nome + ".1")
		os.Rename(nome, nome+".1")
	}
	f, err := os.OpenFile(nome, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if arq != nil {
		arq.Close()
	}
	arq, saida = f, f
	return nil
}

func escrever(nivel, msg string) {
	mu.Lock()
	defer mu.Unlock()
	if arq != nil {
		if info, err := arq.Stat(); err == nil && info.Size() > limite {
			abrir()
		}
	}
	fmt.Fprintf(saida, "%s %s %s\n", time.Now().Format("2006-01-02 15:04:05"), nivel, msg)
}

func Info(formato string, args ...any) { escrever("INFO ", fmt.Sprintf(formato, args...)) }
func Erro(formato string, args ...any) { escrever("ERRO ", fmt.Sprintf(formato, args...)) }

// Logger para o servidor HTTP.
func Logger() *log.Logger { return log.New(escritor{}, "", 0) }

type escritor struct{}

func (escritor) Write(p []byte) (int, error) {
	escrever("HTTP ", string(p))
	return len(p), nil
}

// Proteger roda f numa goroutine; se der pane, registra e roda de novo
// (até parar ser fechado).
func Proteger(nomeTarefa string, parar <-chan struct{}, f func()) {
	go func() {
		for {
			terminou := func() (ok bool) {
				defer func() {
					if r := recover(); r != nil {
						Erro("pane em %s: %v\n%s", nomeTarefa, r, debug.Stack())
						ok = false
					}
				}()
				f()
				return true
			}()
			if terminou {
				return
			}
			select {
			case <-parar:
				return
			case <-time.After(10 * time.Second):
				Info("reiniciando %s", nomeTarefa)
			}
		}
	}()
}

// Caminho do arquivo de registro (para mostrar na tela).
func Caminho() string {
	mu.Lock()
	defer mu.Unlock()
	return nome
}

// Ultimas devolve as últimas n linhas do registro.
func Ultimas(n int) string {
	mu.Lock()
	c := nome
	mu.Unlock()
	if c == "" {
		return ""
	}
	b, err := os.ReadFile(c)
	if err != nil {
		return ""
	}
	cont := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			cont++
			if cont > n {
				return string(b[i+1:])
			}
		}
	}
	return string(b)
}

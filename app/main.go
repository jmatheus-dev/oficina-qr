// Oficina QR — Auto Mecânica Itabaiana
//
// Um programa só: registra os serviços de cada carro, imprime a etiqueta com
// QR Code na iDPRT iE4S, atualiza o histórico no site e lê o que foi lançado
// no Zeus. Abre numa janela própria (Edge/Chrome em modo aplicativo) e fica
// trabalhando enquanto o computador estiver ligado.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"oficinaqr/internal/atualizar"
	"oficinaqr/internal/registro"
	"oficinaqr/internal/servidor"
	"oficinaqr/internal/sistema"
	"oficinaqr/ui"
)

const portaPadrao = 47135

func main() {
	fundo := flag.Bool("fundo", false, "abre sem mostrar a janela (ao iniciar o Windows)")
	porta := flag.Int("porta", portaPadrao, "porta local")
	pasta := flag.String("pasta", "", "pasta dos dados (padrão: %APPDATA%\\OficinaQR)")
	flag.Parse()
	if *pasta != "" {
		os.Setenv("OFICINAQR_PASTA", *pasta)
	}
	endereco := fmt.Sprintf("127.0.0.1:%d", *porta)
	url := "http://" + endereco + "/"

	// depois de uma atualização o programa antigo pode estar fechando: espera a porta
	ln, err := net.Listen("tcp", endereco)
	for i := 0; err != nil && i < 20 && !jaAberto(url); i++ {
		time.Sleep(250 * time.Millisecond)
		ln, err = net.Listen("tcp", endereco)
	}
	if err != nil {
		// já está aberto? então só mostra a janela
		if jaAberto(url) {
			if !*fundo {
				sistema.AbrirJanela(url)
			}
			return
		}
		sistema.Aviso("Oficina QR", "Não consegui abrir o programa: a porta "+fmt.Sprint(*porta)+" está ocupada.\n\n"+err.Error())
		os.Exit(1)
	}

	pastaDados := sistema.PastaDados()
	os.MkdirAll(pastaDados, 0o755)
	registro.Abrir(pastaDados)
	registro.Info("Oficina QR %s abrindo (pasta %s)", servidor.Versao, pastaDados)
	if exe, err := os.Executable(); err == nil {
		atualizar.LimparAntigo(exe)
	}
	// se mudaram o programa de lugar, o "abrir com o Windows" aponta para o novo
	if sistema.IniciaComWindows() {
		sistema.IniciarComWindows(true)
	}

	s, err := servidor.Novo(pastaDados, ui.Arquivos)
	if err != nil {
		registro.Erro("abrir dados: %v", err)
		sistema.Aviso("Oficina QR", "Não consegui abrir os dados do programa:\n\n"+err.Error())
		os.Exit(1)
	}
	s.Rodar()
	srv := &http.Server{Handler: s.Handler(*porta), ReadHeaderTimeout: 10 * time.Second, ErrorLog: registro.Logger()}
	go func() {
		<-s.Parar
		srv.Close()
	}()
	if !*fundo {
		go func() {
			time.Sleep(200 * time.Millisecond)
			sistema.AbrirJanela(url)
		}()
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		registro.Erro("servidor: %v", err)
		sistema.Aviso("Oficina QR", err.Error())
	}
	registro.Info("Oficina QR fechado")
	if s.Reabrir != "" { // atualizou: abre a versão nova (a janela já aberta recarrega sozinha)
		if err := exec.Command(s.Reabrir, "--fundo").Start(); err != nil {
			registro.Erro("reabrir depois de atualizar: %v", err)
		}
	}
}

func jaAberto(url string) bool {
	c := http.Client{Timeout: 2 * time.Second}
	r, err := c.Get(url + "api/ping")
	if err != nil {
		return false
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b) == "oficinaqr"
}

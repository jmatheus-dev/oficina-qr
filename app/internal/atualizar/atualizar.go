// Pacote atualizar procura versão nova do programa nas "releases" do GitHub
// (repositório público da oficina) e troca o OficinaQR.exe sozinho.
//
// Para publicar uma versão: no GitHub, crie uma release com a tag "v1.3.0"
// (número maior que o atual) e anexe o arquivo OficinaQR.exe.
package atualizar

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// API do GitHub (os testes trocam).
var API = "https://api.github.com"

type Info struct {
	Atual  string `json:"atual"`
	Versao string `json:"versao"` // versão publicada mais nova
	Nova   bool   `json:"nova"`   // tem versão mais nova que a atual
	Notas  string `json:"notas,omitempty"`
	URL    string `json:"-"` // link do OficinaQR.exe
	Quando string `json:"quando,omitempty"`
}

var cliente = &http.Client{Timeout: 30 * time.Second}

// Verificar olha a última release de repo ("dono/nome").
func Verificar(repo, atual string) (Info, error) {
	in := Info{Atual: atual}
	req, _ := http.NewRequest("GET", strings.TrimRight(API, "/")+"/repos/"+repo+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OficinaQR/"+atual)
	r, err := cliente.Do(req)
	if err != nil {
		return in, fmt.Errorf("sem internet ou GitHub fora do ar: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode == 404 {
		return in, nil // ainda não há versão publicada
	}
	if r.StatusCode != 200 {
		return in, fmt.Errorf("GitHub respondeu %s", r.Status)
	}
	var rel struct {
		Tag    string `json:"tag_name"`
		Corpo  string `json:"body"`
		Quando string `json:"published_at"`
		Assets []struct {
			Nome string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
		return in, err
	}
	in.Versao = strings.TrimPrefix(rel.Tag, "v")
	in.Notas = rel.Corpo
	in.Quando = rel.Quando
	for _, a := range rel.Assets {
		if strings.EqualFold(a.Nome, "OficinaQR.exe") {
			in.URL = a.URL
		}
	}
	in.Nova = in.URL != "" && Maior(in.Versao, atual)
	return in, nil
}

// Maior diz se a versão a é mais nova que b ("1.10.0" > "1.9.2").
func Maior(a, b string) bool {
	pa, pb := partes(a), partes(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func partes(v string) [3]int {
	var out [3]int
	for i, p := range strings.SplitN(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".", 3) {
		n, _ := strconv.Atoi(strings.TrimFunc(p, func(r rune) bool { return r < '0' || r > '9' }))
		out[i] = n
	}
	return out
}

// Baixar a versão nova e trocar o arquivo do programa. Devolve o caminho do
// programa (para abrir de novo). O arquivo antigo fica como
// OficinaQR.antigo.exe até a próxima abertura.
func Baixar(url, exe string) error {
	if url == "" {
		return errors.New("não há arquivo para baixar")
	}
	r, err := cliente.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("download respondeu %s", r.Status)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 200<<20))
	if err != nil {
		return err
	}
	if len(b) < 1<<20 || !bytes.HasPrefix(b, []byte("MZ")) {
		return errors.New("o arquivo baixado não é um programa do Windows")
	}
	dir := filepath.Dir(exe)
	novo := filepath.Join(dir, "OficinaQR.novo.exe")
	antigo := filepath.Join(dir, "OficinaQR.antigo.exe")
	if err := os.WriteFile(novo, b, 0o755); err != nil {
		return fmt.Errorf("não consegui gravar na pasta do programa: %v", err)
	}
	os.Remove(antigo)
	// no Windows dá para renomear o programa que está aberto, mas não apagar
	if err := os.Rename(exe, antigo); err != nil {
		os.Remove(novo)
		return err
	}
	if err := os.Rename(novo, exe); err != nil {
		os.Rename(antigo, exe)
		return err
	}
	return nil
}

// LimparAntigo apaga o OficinaQR.antigo.exe que sobrou da última atualização.
func LimparAntigo(exe string) {
	os.Remove(filepath.Join(filepath.Dir(exe), "OficinaQR.antigo.exe"))
}

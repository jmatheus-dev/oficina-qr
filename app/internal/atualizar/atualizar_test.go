package atualizar

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaior(t *testing.T) {
	if !Maior("1.10.0", "1.9.2") || Maior("1.2.0", "1.2.0") || !Maior("v2", "1.9.9") || Maior("1.1.9", "1.2") {
		t.Fatal("comparação de versões errada")
	}
}

func TestVerificarEBaixar(t *testing.T) {
	exeNovo := append([]byte("MZ"), make([]byte, 2<<20)...)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name":"v9.0.0","body":"novidades","assets":[{"name":"OficinaQR.exe","browser_download_url":"%s/baixar"}]}`, srv.URL)
		case r.URL.Path == "/baixar":
			w.Write(exeNovo)
		}
	}))
	defer srv.Close()
	API = srv.URL
	in, err := Verificar("automec135/automec135.github.io", "1.2.0")
	if err != nil || !in.Nova || in.Versao != "9.0.0" {
		t.Fatalf("%+v %v", in, err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "OficinaQR.exe")
	os.WriteFile(exe, []byte("MZvelho"), 0o755)
	if err := Baixar(in.URL, exe); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(exe)
	if len(b) != len(exeNovo) {
		t.Fatal("não trocou o programa")
	}
	if _, err := os.Stat(filepath.Join(dir, "OficinaQR.antigo.exe")); err != nil {
		t.Fatal("devia guardar o antigo")
	}
	LimparAntigo(exe)
	if _, err := os.Stat(filepath.Join(dir, "OficinaQR.antigo.exe")); err == nil {
		t.Fatal("devia apagar o antigo")
	}
}

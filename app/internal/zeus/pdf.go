package zeus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"oficinaqr/internal/dados"
	"oficinaqr/internal/pdfzeus"
	"oficinaqr/internal/registro"
)

// Modos de ligação com o Zeus.
const (
	ModoBanco = "banco" // lê o PostgreSQL do Zeus (precisa de usuário de leitura)
	ModoPDF   = "pdf"   // lê os PDFs que o balcão salva pelo botão PDF do Zeus
)

// PastaPadrao é onde o balcão salva os PDFs do Zeus.
func PastaPadrao() string {
	if runtime.GOOS == "windows" {
		return `C:\OficinaQR\entrada`
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, "OficinaQR", "entrada")
	}
	return "entrada"
}

// quantos dias um PDF fica na pasta antes de ir para "lidos".
const diasNaEntrada = 7

// PastaPDF lê os PDFs (orçamento, pré-venda...) salvos numa pasta.
// Os arquivos ficam na pasta por alguns dias — assim, se o balcão salvar de
// novo o mesmo orçamento com mais peças, o histórico atualiza — e depois vão
// para a subpasta "lidos".
type PastaPDF struct {
	Dir   string
	Agora func() time.Time
}

type lido struct {
	tam  int64
	mod  time.Time
	doc  pdfzeus.Documento
	erro string
	ref  string
}

var (
	cacheMu sync.Mutex
	cache   = map[string]lido{} // caminho → último resultado (não relê o mesmo arquivo)
)

func (p PastaPDF) agora() time.Time {
	if p.Agora != nil {
		return p.Agora()
	}
	return time.Now()
}

func (p PastaPDF) Buscar(ctx context.Context, desde string) ([]Linha, error) {
	dir := strings.TrimSpace(p.Dir)
	if dir == "" {
		dir = PastaPadrao()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("não consegui criar a pasta dos PDFs (%s): %v", dir, err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("não consegui ler a pasta dos PDFs (%s): %v", dir, err)
	}
	limite := p.agora().AddDate(0, 0, -diasNaEntrada)
	// o mesmo orçamento pode ter sido salvo mais de uma vez: vale o mais novo
	type achado struct {
		l   lido
		mod time.Time
	}
	porRef := map[string]achado{}
	var ordem []string
	for _, e := range ents {
		if ctx.Err() != nil {
			break
		}
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
			continue
		}
		caminho := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(limite) {
			moverParaLidos(dir, caminho)
			continue
		}
		l := lerPDF(caminho, info)
		if l.erro != "" {
			continue
		}
		if data := l.doc.Data; data == "" {
			l.doc.Data = info.ModTime().Format("2006-01-02")
		}
		velho, ja := porRef[l.ref]
		if !ja {
			ordem = append(ordem, l.ref)
		}
		if !ja || info.ModTime().After(velho.mod) {
			porRef[l.ref] = achado{l, info.ModTime()}
		}
	}
	var out []Linha
	for _, r := range ordem {
		doc := porRef[r].l.doc
		for _, prod := range doc.Produtos {
			out = append(out, Linha{Ref: r, Documento: doc.Tipo + " " + doc.Numero, Data: doc.Data,
				Descricao: prod, Cliente: doc.Cliente, Texto: doc.Observacoes, Telefone: doc.Telefone})
		}
	}
	return out, nil
}

// RefPDF é o identificador do lançamento: o mesmo orçamento salvo de novo
// (mesmo tipo e número) atualiza o lançamento em vez de criar outro.
func RefPDF(d pdfzeus.Documento) string {
	t := strings.ToLower(strings.NewReplacer("ç", "c", "é", "e", " ", "-").Replace(d.Tipo))
	return "pdf:" + t + "-" + d.Numero
}

func lerPDF(caminho string, info os.FileInfo) lido {
	cacheMu.Lock()
	c, ok := cache[caminho]
	cacheMu.Unlock()
	if ok && c.tam == info.Size() && c.mod.Equal(info.ModTime()) {
		return c
	}
	r := lido{tam: info.Size(), mod: info.ModTime()}
	b, err := os.ReadFile(caminho)
	if err == nil {
		var linhas []string
		linhas, err = pdfzeus.Linhas(b)
		if err == nil {
			r.doc, err = pdfzeus.Ler(linhas)
		}
	}
	if err != nil {
		r.erro = err.Error()
		// arquivo ainda sendo gravado: tenta de novo na próxima leitura sem avisar
		if time.Since(info.ModTime()) > 30*time.Second {
			registro.Erro("PDF do Zeus %s: %v", filepath.Base(caminho), err)
		}
	} else {
		r.ref = RefPDF(r.doc)
		registro.Info("PDF do Zeus lido: %s → %s %s, %d produto(s)", filepath.Base(caminho), r.doc.Tipo, r.doc.Numero, len(r.doc.Produtos))
	}
	cacheMu.Lock()
	cache[caminho] = r
	cacheMu.Unlock()
	return r
}

func moverParaLidos(dir, caminho string) {
	destDir := filepath.Join(dir, "lidos")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return
	}
	base := filepath.Base(caminho)
	dest := filepath.Join(destDir, base)
	for i := 2; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(base)
		dest = filepath.Join(destDir, fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(base, ext), i, ext))
		if i > 999 {
			return
		}
	}
	if err := os.Rename(caminho, dest); err == nil {
		cacheMu.Lock()
		delete(cache, caminho)
		cacheMu.Unlock()
	}
}

// PDFsNaPasta lista os PDFs da pasta de entrada com o que foi entendido de
// cada um (para a tela de configurações mostrar se está funcionando).
type InfoPDF struct {
	Arquivo   string   `json:"arquivo"`
	Documento string   `json:"documento,omitempty"`
	Cliente   string   `json:"cliente,omitempty"`
	Placa     []string `json:"placas,omitempty"`
	Produtos  int      `json:"produtos"`
	Obs       string   `json:"obs,omitempty"`
	Erro      string   `json:"erro,omitempty"`
	Quando    string   `json:"quando"`
}

func PDFsNaPasta(dir string, max int) ([]InfoPDF, error) {
	if strings.TrimSpace(dir) == "" {
		dir = PastaPadrao()
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type arq struct {
		e    os.DirEntry
		info os.FileInfo
	}
	var lista []arq
	for _, e := range ents {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
			continue
		}
		if info, err := e.Info(); err == nil {
			lista = append(lista, arq{e, info})
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].info.ModTime().After(lista[j].info.ModTime()) })
	if max > 0 && len(lista) > max {
		lista = lista[:max]
	}
	out := []InfoPDF{}
	for _, a := range lista {
		l := lerPDF(filepath.Join(dir, a.e.Name()), a.info)
		x := InfoPDF{Arquivo: a.e.Name(), Erro: l.erro, Quando: a.info.ModTime().Format("02/01 15:04")}
		if l.erro == "" {
			x.Documento = l.doc.Tipo + " " + l.doc.Numero
			x.Cliente = l.doc.Cliente
			x.Produtos = len(l.doc.Produtos)
			x.Obs = l.doc.Observacoes
			x.Placa = dados.AcharPlacas(l.doc.Observacoes)
		}
		out = append(out, x)
	}
	return out, nil
}

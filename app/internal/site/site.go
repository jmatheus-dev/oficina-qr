// Pacote site publica o histórico (só dados públicos) no GitHub, de onde a
// página do QR Code (automec135.github.io) lê.
//
// O arquivo historico.csv vai num ramo separado ("dados") do repositório do
// site, para não refazer o site a cada serviço. Opcionalmente, uma cópia de
// segurança completa (dados.json) vai para um repositório PRIVADO.
package site

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"oficinaqr/internal/dados"
)

type Config struct {
	Repo       string `json:"repo"`        // dono/nome do site
	Ramo       string `json:"ramo"`        // ramo dos dados
	Arquivo    string `json:"arquivo"`     // historico.csv
	Token      string `json:"-"`           // token do GitHub (fica cifrado no disco)
	RepoBackup string `json:"repo_backup"` // dono/nome privado (opcional)
}

func ConfigPadrao() Config {
	return Config{Repo: "automec135/automec135.github.io", Ramo: "dados", Arquivo: "historico.csv"}
}

type Estado struct {
	Configurado bool      `json:"configurado"`
	Pendente    bool      `json:"pendente"`
	Publicando  bool      `json:"publicando"`
	Ultimo      time.Time `json:"ultimo,omitempty"`
	Erro        string    `json:"erro,omitempty"`
	Backup      time.Time `json:"backup,omitempty"`
	ErroBackup  string    `json:"erro_backup,omitempty"`
}

type Publicador struct {
	// regras para a página (próxima troca, correia, garantia); definido pelo servidor
	Opcoes  func() dados.OpcoesPublico
	API     string // https://api.github.com (trocado nos testes)
	Espera  time.Duration
	banco   *dados.Banco
	config  func() Config
	cliente *http.Client

	mu       sync.Mutex
	estado   Estado
	hashSite string
	hashBkp  string
	gatilho  chan struct{}
}

func Novo(b *dados.Banco, config func() Config) *Publicador {
	p := &Publicador{API: "https://api.github.com", Espera: 20 * time.Second, banco: b, config: config,
		cliente: &http.Client{Timeout: 40 * time.Second}, gatilho: make(chan struct{}, 1)}
	b.AoMudar(p.Avisar)
	return p
}

// Avisar que os dados mudaram (publica daqui a pouco).
func (p *Publicador) Avisar() {
	p.mu.Lock()
	p.estado.Pendente = true
	p.mu.Unlock()
	select {
	case p.gatilho <- struct{}{}:
	default:
	}
}

func (p *Publicador) Estado() Estado {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.estado
	e.Configurado = p.config().Token != ""
	return e
}

// Rodar fica publicando quando há mudança (chamar numa goroutine).
func (p *Publicador) Rodar(parar <-chan struct{}) {
	p.Avisar() // confere ao abrir o programa
	tentarDeNovo := time.NewTicker(5 * time.Minute)
	defer tentarDeNovo.Stop()
	for {
		select {
		case <-parar:
			return
		case <-p.gatilho:
			// junta várias mudanças seguidas numa publicação só
			espera := time.NewTimer(p.Espera)
			for fim := false; !fim; {
				select {
				case <-p.gatilho:
				case <-espera.C:
					fim = true
				case <-parar:
					espera.Stop()
					return
				}
			}
			p.PublicarAgora()
		case <-tentarDeNovo.C:
			if e := p.Estado(); e.Pendente || e.Erro != "" || e.ErroBackup != "" {
				p.PublicarAgora()
			}
		}
	}
}

// PublicarAgora manda o histórico (e a cópia de segurança, se configurada).
func (p *Publicador) PublicarAgora() error {
	cfg := p.config()
	if cfg.Token == "" {
		return errors.New("falta o token do GitHub (Configurações > Site)")
	}
	p.mu.Lock()
	if p.estado.Publicando {
		p.mu.Unlock()
		return nil
	}
	p.estado.Publicando = true
	p.estado.Pendente = false
	p.mu.Unlock()

	veics, servs := p.banco.Tudo()
	var op dados.OpcoesPublico
	if p.Opcoes != nil {
		op = p.Opcoes()
	}
	csv := dados.CSVPublico(veics, servs, time.Now(), op)
	err := p.publicarSite(cfg, csv)

	var errB error
	if cfg.RepoBackup != "" {
		j, e := p.banco.JSON()
		if e == nil {
			errB = p.publicarBackup(cfg, j)
		} else {
			errB = e
		}
	}

	p.mu.Lock()
	p.estado.Publicando = false
	if err != nil {
		p.estado.Erro = err.Error()
		p.estado.Pendente = true
	} else {
		p.estado.Erro = ""
		p.estado.Ultimo = time.Now()
	}
	if cfg.RepoBackup != "" {
		if errB != nil {
			p.estado.ErroBackup = errB.Error()
		} else {
			p.estado.ErroBackup = ""
			p.estado.Backup = time.Now()
		}
	} else {
		p.estado.ErroBackup = ""
	}
	p.mu.Unlock()
	if err != nil {
		return err
	}
	return errB
}

// hash sem a linha da hora (para não publicar se nada mudou)
func hashSemHora(csv []byte) string {
	linhas := strings.SplitN(string(csv), "\n", 3)
	if len(linhas) == 3 && strings.HasPrefix(linhas[1], "_,") {
		csv = []byte(linhas[0] + "\n" + linhas[2])
	}
	h := sha256.Sum256(csv)
	return fmt.Sprintf("%x", h[:])
}

func (p *Publicador) publicarSite(cfg Config, csv []byte) error {
	h := hashSemHora(csv)
	p.mu.Lock()
	igual := p.hashSite == h
	p.mu.Unlock()
	if igual {
		return nil
	}
	if err := p.garantirRamo(cfg, cfg.Repo, cfg.Ramo); err != nil {
		return err
	}
	atual, sha, err := p.lerArquivo(cfg, cfg.Repo, cfg.Arquivo, cfg.Ramo)
	if err != nil {
		return err
	}
	if atual != nil && hashSemHora(atual) == h {
		p.mu.Lock()
		p.hashSite = h
		p.mu.Unlock()
		return nil
	}
	msg := "Atualiza histórico (" + time.Now().Format("02/01/2006 15:04") + ")"
	if err := p.gravarArquivo(cfg, cfg.Repo, cfg.Arquivo, cfg.Ramo, sha, csv, msg); err != nil {
		return err
	}
	p.mu.Lock()
	p.hashSite = h
	p.mu.Unlock()
	return nil
}

func (p *Publicador) publicarBackup(cfg Config, j []byte) error {
	sum := sha256.Sum256(j)
	h := fmt.Sprintf("%x", sum[:])
	p.mu.Lock()
	igual := p.hashBkp == h
	p.mu.Unlock()
	if igual {
		return nil
	}
	_, sha, err := p.lerArquivo(cfg, cfg.RepoBackup, "dados.json", "")
	if err != nil {
		return err
	}
	if err := p.gravarArquivo(cfg, cfg.RepoBackup, "dados.json", "", sha, j, "Cópia de segurança "+time.Now().Format("02/01/2006 15:04")); err != nil {
		return err
	}
	p.mu.Lock()
	p.hashBkp = h
	p.mu.Unlock()
	return nil
}

/* ------------------------------------------------------------- API --- */

type ErroGitHub struct {
	Status int
	Msg    string
}

func (e *ErroGitHub) Error() string {
	switch e.Status {
	case 401:
		return "o GitHub recusou o token (vencido ou errado). Gere outro e cole em Configurações > Site"
	case 403:
		return "o token não tem permissão de gravar no repositório (precisa de Contents: Read and write)"
	case 404:
		return "repositório não encontrado ou o token não tem acesso a ele"
	}
	return fmt.Sprintf("GitHub respondeu %d: %s", e.Status, e.Msg)
}

func (p *Publicador) chamar(cfg Config, metodo, caminho string, corpo any, resp any) error {
	var r io.Reader
	if corpo != nil {
		j, err := json.Marshal(corpo)
		if err != nil {
			return err
		}
		r = bytes.NewReader(j)
	}
	req, err := http.NewRequest(metodo, strings.TrimRight(p.API, "/")+caminho, r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "OficinaQR")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.Token))
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := p.cliente.Do(req)
	if err != nil {
		return fmt.Errorf("sem conexão com o GitHub: %w", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 50<<20))
	if res.StatusCode >= 300 {
		var m struct {
			Message string `json:"message"`
		}
		json.Unmarshal(b, &m)
		return &ErroGitHub{Status: res.StatusCode, Msg: m.Message}
	}
	if resp != nil {
		return json.Unmarshal(b, resp)
	}
	return nil
}

func status(err error) int {
	var e *ErroGitHub
	if errors.As(err, &e) {
		return e.Status
	}
	return 0
}

func caminhoArq(repo, arq string) string {
	return "/repos/" + repo + "/contents/" + url.PathEscape(arq)
}

func (p *Publicador) lerArquivo(cfg Config, repo, arq, ramo string) ([]byte, string, error) {
	c := caminhoArq(repo, arq)
	if ramo != "" {
		c += "?ref=" + url.QueryEscape(ramo)
	}
	var r struct {
		SHA      string `json:"sha"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	err := p.chamar(cfg, "GET", c, nil, &r)
	if status(err) == 404 {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	var conteudo []byte
	if r.Encoding == "base64" {
		conteudo, _ = base64.StdEncoding.DecodeString(strings.ReplaceAll(r.Content, "\n", ""))
	}
	return conteudo, r.SHA, nil
}

func (p *Publicador) gravarArquivo(cfg Config, repo, arq, ramo, sha string, conteudo []byte, msg string) error {
	corpo := map[string]string{"message": msg, "content": base64.StdEncoding.EncodeToString(conteudo)}
	if ramo != "" {
		corpo["branch"] = ramo
	}
	if sha != "" {
		corpo["sha"] = sha
	}
	return p.chamar(cfg, "PUT", caminhoArq(repo, arq), corpo, nil)
}

func (p *Publicador) garantirRamo(cfg Config, repo, ramo string) error {
	err := p.chamar(cfg, "GET", "/repos/"+repo+"/git/ref/heads/"+url.PathEscape(ramo), nil, nil)
	if err == nil {
		return nil
	}
	if status(err) != 404 {
		return err
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := p.chamar(cfg, "GET", "/repos/"+repo, nil, &info); err != nil {
		return err
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := p.chamar(cfg, "GET", "/repos/"+repo+"/git/ref/heads/"+url.PathEscape(info.DefaultBranch), nil, &ref); err != nil {
		return err
	}
	return p.chamar(cfg, "POST", "/repos/"+repo+"/git/refs",
		map[string]string{"ref": "refs/heads/" + ramo, "sha": ref.Object.SHA}, nil)
}

// Testar confere se o token consegue gravar no site (e no backup).
func (p *Publicador) Testar(cfg Config) error {
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("cole o token do GitHub primeiro")
	}
	for _, repo := range []string{cfg.Repo, cfg.RepoBackup} {
		if repo == "" {
			continue
		}
		var info struct {
			Permissions struct {
				Push bool `json:"push"`
			} `json:"permissions"`
		}
		if err := p.chamar(cfg, "GET", "/repos/"+repo, nil, &info); err != nil {
			return fmt.Errorf("%s: %w", repo, err)
		}
		if !info.Permissions.Push {
			return fmt.Errorf("%s: o token só consegue ler; marque Contents: Read and write", repo)
		}
	}
	return nil
}

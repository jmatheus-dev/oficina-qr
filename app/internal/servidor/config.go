package servidor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"oficinaqr/internal/etiqueta"
	"oficinaqr/internal/sistema"
	"oficinaqr/internal/site"
	"oficinaqr/internal/zeus"
)

// Config é o que fica em config.json (senhas e token vão cifrados).
type Config struct {
	Etiqueta         etiqueta.Config `json:"etiqueta"`
	Impressora       string          `json:"impressora"`
	Copias           int             `json:"copias"`
	ImprimirAoSalvar bool            `json:"imprimir_ao_salvar"`
	TamanhoDoDriver  bool            `json:"tamanho_do_driver"` // não manda o tamanho da etiqueta para o driver
	WhatsApp         string          `json:"whatsapp"`
	Site             site.Config     `json:"site"`
	TokenCifrado     string          `json:"token_cifrado,omitempty"`
	Zeus             zeus.Config     `json:"zeus"`
	SenhaCifrada     string          `json:"senha_zeus_cifrada,omitempty"`
	// pasta extra para a cópia diária dos dados (ex.: OneDrive); vazio = só a do programa
	PastaCopia string `json:"pasta_copia"`
	// meses sem troca de óleo para entrar na lista de lembretes
	MesesLembrete int `json:"meses_lembrete"`
	// termo de responsabilidade da peça trazida pelo cliente
	TermoCabecalho string `json:"termo_cabecalho"`
	TermoClausulas string `json:"termo_clausulas"`
	// versão das mudanças automáticas já feitas no config.json
	Ajustes int `json:"ajustes"`
	// modo deste computador: "" ou "principal" (registra, imprime, lê o Zeus)
	// ou "consulta" (só lê os dados do computador principal pela rede)
	Modo      string `json:"modo_computador"`
	Principal string `json:"principal"` // consulta: endereço do principal (ex.: 192.168.1.20:47136)
	// principal: deixa outros computadores consultarem (somente leitura)
	RedeLigada bool `json:"rede_ligada"`
	// chave da consulta pela rede (no principal e nos de consulta), cifrada
	ChaveRedeCifrada string `json:"chave_rede_cifrada,omitempty"`
}

const ajustesAtuais = 1

const TermoCabecalhoPadrao = "Auto Mecânica Itabaiana — Itabaiana/SE — WhatsApp (79) 9851-3926"

// uma cláusula por linha (a tela numera)
const TermoClausulasPadrao = `Declaro que a(s) peça(s) descrita(s) acima foi(foram) comprada(s) por mim, por minha conta e escolha, e entregue(s) à oficina somente para ser(em) instalada(s) no veículo acima.
A oficina não vendeu nem indicou a(s) peça(s) e não se responsabiliza por sua procedência, originalidade, qualidade, compatibilidade com o veículo, defeito de fabricação ou durabilidade.
A oficina é responsável apenas pela mão de obra executada, com garantia de 90 (noventa) dias a contar da entrega do veículo, cobrindo somente falhas no serviço de instalação.
Se a peça apresentar defeito, a garantia deverá ser pedida por mim diretamente à loja onde comprei ou ao fabricante. A mão de obra para retirar a peça com defeito e instalar outra será cobrada à parte.
A oficina não responde por danos ao veículo causados por defeito, má qualidade ou incompatibilidade da peça fornecida por mim.
Se, durante o serviço, a peça se mostrar incompatível ou com defeito aparente, a oficina me avisará antes de continuar, e a mão de obra já realizada poderá ser cobrada.`

func configPadrao() Config {
	return Config{Etiqueta: etiqueta.ConfigPadrao(), Copias: 1, ImprimirAoSalvar: true, WhatsApp: "557998513926",
		Site: site.ConfigPadrao(), Zeus: zeus.ConfigPadrao(), PastaCopia: pastaCopiaPadrao(), MesesLembrete: 6,
		TermoCabecalho: TermoCabecalhoPadrao, TermoClausulas: TermoClausulasPadrao, Ajustes: ajustesAtuais}
}

type configs struct {
	mu  sync.RWMutex
	arq string
	c   Config
	// já decifrados (em memória)
	token, senha, chave string
}

func abrirConfig(pasta string) (*configs, error) {
	cs := &configs{arq: filepath.Join(pasta, "config.json"), c: configPadrao()}
	b, err := os.ReadFile(cs.arq)
	if errors.Is(err, os.ErrNotExist) {
		return cs, cs.gravar()
	}
	if err != nil {
		return nil, err
	}
	cs.c.Ajustes = 0 // arquivo antigo sem "ajustes": ainda não passou pelas mudanças
	if err := json.Unmarshal(b, &cs.c); err != nil {
		return nil, err
	}
	if cs.c.Etiqueta.DPI <= 0 {
		cs.c.Etiqueta = etiqueta.ConfigPadrao()
	}
	if cs.c.Site.Repo == "" {
		cs.c.Site = site.ConfigPadrao()
	}
	// versão 1.2: consulta do Zeus pronta e nada registrado/impresso sem revisão
	if strings.TrimSpace(cs.c.Zeus.SQL) == "" {
		cs.c.Zeus.SQL = zeus.SQLPadrao
	}
	cs.c.Zeus.AutoRegist, cs.c.Zeus.AutoImprim = false, false
	// versão 1.3: venda do Zeus aparece 1 minuto depois de terminada (era 3) e o
	// banco é lido a cada 20 segundos (era 60), para o minuto ser de verdade
	if cs.c.Ajustes < 1 {
		if cs.c.Zeus.EsperaMin == 3 {
			cs.c.Zeus.EsperaMin = 1
		}
		if cs.c.Zeus.Intervalo == 60 || cs.c.Zeus.Intervalo == 0 {
			cs.c.Zeus.Intervalo = 20
		}
		cs.c.Ajustes = 1
		defer cs.gravar()
	}
	if strings.TrimSpace(cs.c.TermoCabecalho) == "" {
		cs.c.TermoCabecalho = TermoCabecalhoPadrao
	}
	if strings.TrimSpace(cs.c.TermoClausulas) == "" {
		cs.c.TermoClausulas = TermoClausulasPadrao
	}
	cs.token, _ = sistema.Decifrar(cs.c.TokenCifrado)
	cs.senha, _ = sistema.Decifrar(cs.c.SenhaCifrada)
	cs.chave, _ = sistema.Decifrar(cs.c.ChaveRedeCifrada)
	return cs, nil
}

// pastaCopiaPadrao: se o computador tem OneDrive, a cópia vai também para lá
// (fica guardada na nuvem se o HD der defeito).
func pastaCopiaPadrao() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if st, err := os.Stat(filepath.Join(h, "OneDrive")); err == nil && st.IsDir() {
		return filepath.Join(h, "OneDrive", "OficinaQR-copias")
	}
	return ""
}

func (cs *configs) gravar() error {
	b, err := json.MarshalIndent(cs.c, "", " ")
	if err != nil {
		return err
	}
	tmp := cs.arq + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, cs.arq)
}

func (cs *configs) Ler() Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.c
}

func (cs *configs) Site() site.Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	c := cs.c.Site
	c.Token = cs.token
	if cs.c.Modo == ModoConsulta {
		c.Token = "" // computador de consulta nunca publica no site
	}
	return c
}

const ModoConsulta = "consulta"

// Consulta diz se este computador só consulta o principal.
func (cs *configs) Consulta() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.c.Modo == ModoConsulta
}

func (cs *configs) Chave() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.chave
}

// MudarChave troca a chave da consulta pela rede.
func (cs *configs) MudarChave(chave string) error {
	c, err := sistema.Cifrar(chave)
	if err != nil {
		return err
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.c.ChaveRedeCifrada, cs.chave = c, chave
	return cs.gravar()
}

func (cs *configs) Zeus() zeus.Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	c := cs.c.Zeus
	c.Senha = cs.senha
	if cs.c.Modo == ModoConsulta {
		c.Ativo = false // quem lê o Zeus é o principal
	}
	return c
}

// Mudar aplica f e grava. token/senha: nil = não mexe; "" = apaga.
func (cs *configs) Mudar(f func(*Config), token, senha *string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	novo := cs.c
	f(&novo)
	if token != nil {
		c, err := sistema.Cifrar(*token)
		if err != nil {
			return err
		}
		novo.TokenCifrado = c
		cs.token = *token
	}
	if senha != nil {
		c, err := sistema.Cifrar(*senha)
		if err != nil {
			return err
		}
		novo.SenhaCifrada = c
		cs.senha = *senha
	}
	cs.c = novo
	return cs.gravar()
}

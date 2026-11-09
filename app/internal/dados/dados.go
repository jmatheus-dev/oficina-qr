// Pacote dados guarda os veículos, os serviços e o que chegou do Zeus.
//
// Tudo fica num arquivo JSON (dados.json) na pasta do programa. Cada mudança
// grava o arquivo inteiro de forma segura (escreve um temporário e troca), e
// uma cópia por dia vai para a pasta "copias".
package dados

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Itens padrão da troca de óleo (mesma ordem da etiqueta e da página).
var Itens = []string{
	"Óleo do motor",
	"Filtro de óleo",
	"Filtro de ar",
	"Filtro de combustível",
	"Filtro de cabine",
	"Óleo de freio",
	"Óleo de câmbio",
}

// Sem 0/O e 1/I para ninguém confundir ao digitar.
const alfabeto = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
const tamanhoCodigo = 8

type Veiculo struct {
	Placa    string    `json:"placa"`
	Codigo   string    `json:"codigo"`
	Extras   []string  `json:"codigos_extras,omitempty"` // códigos antigos que também abrem o histórico
	Modelo   string    `json:"modelo"`
	Ano      string    `json:"ano"`
	Cliente  string    `json:"cliente,omitempty"`
	Telefone string    `json:"telefone,omitempty"`
	CriadoEm time.Time `json:"criado_em"`
	// último lembrete de revisão mandado ao cliente (AAAA-MM-DD)
	AvisadoEm string `json:"avisado_em,omitempty"`
}

type Servico struct {
	ID        string   `json:"id"`
	Placa     string   `json:"placa"`
	Data      string   `json:"data"` // AAAA-MM-DD
	KM        int      `json:"km,omitempty"`
	Itens     []string `json:"itens"`
	Outros    []string `json:"outros"`
	ProximaKM int      `json:"proxima_troca_km,omitempty"`
	// data da próxima troca de óleo (vazio = data do serviço + meses do padrão)
	ProximaData string `json:"proxima_troca_data,omitempty"`
	// correia dentada: próxima troca (vazio = padrão da etiqueta)
	ProximaCorreiaKM   int       `json:"proxima_correia_km,omitempty"`
	ProximaCorreiaData string    `json:"proxima_correia_data,omitempty"`
	ObsCliente         string    `json:"obs,omitempty"`
	ObsInterna         string    `json:"obs_interna,omitempty"`
	Origem             string    `json:"origem"`             // "manual" ou "zeus"
	ZeusRef            string    `json:"zeus_ref,omitempty"` // um ou mais lançamentos do Zeus, separados por vírgula
	Impressoes         int       `json:"impressoes,omitempty"`
	CriadoEm           time.Time `json:"criado_em"`
	AlteradoEm         time.Time `json:"alterado_em"`
}

// Lancamento é uma venda/OS lida do Zeus, esperando virar serviço.
type Lancamento struct {
	Ref       string   `json:"ref"`       // identificador único no Zeus
	Documento string   `json:"documento"` // ex.: "Venda 1234"
	Data      string   `json:"data"`
	Cliente   string   `json:"cliente,omitempty"`
	Texto     string   `json:"texto,omitempty"` // observação / campos livres
	Produtos  []string `json:"produtos"`        // descrições como estão no Zeus
	Itens     []string `json:"itens"`           // itens padrão reconhecidos
	Outros    []string `json:"outros"`          // demais peças/serviços
	Placa     string   `json:"placa,omitempty"` // placa encontrada (ou escolhida)
	KM        int      `json:"km,omitempty"`
	Modelo    string   `json:"modelo,omitempty"`   // tirado da observação (ex.: "Fiat Uno 1.0")
	Ano       string   `json:"ano,omitempty"`      // idem (ex.: "2011")
	Telefone  string   `json:"telefone,omitempty"` // do cadastro do cliente no Zeus
	// a venda só tem mão de obra (o cliente pode ter trazido a peça)
	SoMaoDeObra bool      `json:"so_mao_de_obra,omitempty"`
	Status      string    `json:"status"` // pendente, registrado, ignorado
	ServicoID   string    `json:"servico_id,omitempty"`
	RecebidoEm  time.Time `json:"recebido_em"`
}

const (
	Pendente   = "pendente"
	Registrado = "registrado"
	Ignorado   = "ignorado"
)

type conteudo struct {
	Versao      int                 `json:"versao"`
	Veiculos    map[string]*Veiculo `json:"veiculos"`
	Servicos    []*Servico          `json:"servicos"`
	Lancamentos []*Lancamento       `json:"lancamentos"`
	ZeusCursor  string              `json:"zeus_cursor,omitempty"`
	Termos      []*Termo            `json:"termos,omitempty"`
	Alteracao   time.Time           `json:"alteracao"`
}

// Banco é o acesso aos dados, seguro para várias goroutines.
type Banco struct {
	mu       sync.RWMutex
	pasta    string
	c        conteudo
	ouvintes []func()
	// pasta extra para a cópia do dia (ex.: OneDrive); vazio = nenhuma
	CopiaExtra string
}

var (
	ErrPlaca       = errors.New("placa inválida: use o formato ABC1D23 ou ABC1234")
	ErrNaoAchado   = errors.New("não encontrado")
	ErrServicoNada = errors.New("marque pelo menos um item ou escreva o que foi feito")
)

// Abrir lê (ou cria) os dados na pasta.
func Abrir(pasta string) (*Banco, error) {
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		return nil, err
	}
	b := &Banco{pasta: pasta}
	b.c = conteudo{Versao: 1, Veiculos: map[string]*Veiculo{}}
	arq := b.arquivo()
	dadosBrutos, err := os.ReadFile(arq)
	if errors.Is(err, os.ErrNotExist) {
		return b, b.gravar()
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dadosBrutos, &b.c); err != nil {
		return nil, fmt.Errorf("o arquivo %s está danificado (%v); use uma cópia da pasta copias", arq, err)
	}
	if b.c.Veiculos == nil {
		b.c.Veiculos = map[string]*Veiculo{}
	}
	return b, nil
}

func (b *Banco) Pasta() string   { return b.pasta }
func (b *Banco) arquivo() string { return filepath.Join(b.pasta, "dados.json") }

// AoMudar registra uma função chamada depois de cada gravação (fora da trava).
func (b *Banco) AoMudar(f func()) {
	b.mu.Lock()
	b.ouvintes = append(b.ouvintes, f)
	b.mu.Unlock()
}

func (b *Banco) gravar() error {
	b.c.Alteracao = time.Now()
	j, err := json.MarshalIndent(b.c, "", " ")
	if err != nil {
		return err
	}
	if err := gravarSeguro(b.arquivo(), j); err != nil {
		return err
	}
	// cópia do dia (fica a última de cada dia, por 60 dias)
	pastaCopias := filepath.Join(b.pasta, "copias")
	if os.MkdirAll(pastaCopias, 0o755) == nil {
		_ = gravarSeguro(filepath.Join(pastaCopias, "dados-"+time.Now().Format("2006-01-02")+".json"), j)
		limparCopias(pastaCopias, 60)
	}
	// cópia também numa pasta de fora (ex.: OneDrive), se configurada
	if extra := strings.TrimSpace(b.CopiaExtra); extra != "" {
		if os.MkdirAll(extra, 0o755) == nil {
			_ = gravarSeguro(filepath.Join(extra, "dados-"+time.Now().Format("2006-01-02")+".json"), j)
			limparCopias(extra, 60)
		}
	}
	return nil
}

func gravarSeguro(arq string, j []byte) error {
	tmp := arq + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := f.Write(j); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, arq)
}

func limparCopias(pasta string, manter int) {
	m, _ := filepath.Glob(filepath.Join(pasta, "dados-*.json"))
	if len(m) <= manter {
		return
	}
	sort.Strings(m)
	for _, f := range m[:len(m)-manter] {
		os.Remove(f)
	}
}

// mudou grava e avisa os ouvintes. Chamar com a trava de escrita e soltar depois.
func (b *Banco) mudou() error {
	return b.gravar()
}

func (b *Banco) avisar() {
	b.mu.RLock()
	fs := append([]func(){}, b.ouvintes...)
	b.mu.RUnlock()
	for _, f := range fs {
		go f()
	}
}

/* ------------------------------------------------------------ placas --- */

var rePlaca = regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`)

// NormalizarPlaca tira espaço/hífen e põe em maiúsculas. Devolve "" se não for placa.
func NormalizarPlaca(s string) string {
	var r strings.Builder
	for _, ch := range strings.ToUpper(s) {
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			r.WriteRune(ch)
		}
	}
	p := r.String()
	if !rePlaca.MatchString(p) {
		return ""
	}
	return p
}

// MascararPlaca mostra ABC-1234 (modelo antigo) ou ABC1D23 (Mercosul).
func MascararPlaca(p string) string {
	if len(p) == 7 && p[4] >= '0' && p[4] <= '9' {
		return p[:3] + "-" + p[3:]
	}
	return p
}

// Achar placas num texto livre (observação da venda, nome do cliente...).
var rePlacaTexto = regexp.MustCompile(`(?i)\b([A-Z]{3})[\s-]?([0-9][A-Z0-9][0-9]{2})\b`)

func AcharPlacas(texto string) []string {
	visto := map[string]bool{}
	var out []string
	for _, m := range rePlacaTexto.FindAllStringSubmatch(semAcento(texto), -1) {
		p := NormalizarPlaca(m[1] + m[2])
		if p != "" && !visto[p] {
			visto[p] = true
			out = append(out, p)
		}
	}
	return out
}

// Achar KM num texto livre: "KM 153.273", "153273 km", "km: 153273".
var reKM = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bkm\b[\s:.=-]*([0-9]{1,3}(?:[.\s][0-9]{3})+|[0-9]{3,7})`),
	regexp.MustCompile(`(?i)\b([0-9]{1,3}(?:[.\s][0-9]{3})+|[0-9]{3,7})\s*km\b`),
}

func AcharKM(texto string) int {
	for _, re := range reKM {
		if m := re.FindStringSubmatch(texto); m != nil {
			n := 0
			for _, ch := range m[1] {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				}
			}
			if n >= 100 && n < 3000000 {
				return n
			}
		}
	}
	return 0
}

func semAcento(s string) string {
	var r strings.Builder
	for _, ch := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, ch) {
			continue
		}
		r.WriteRune(ch)
	}
	return r.String()
}

/* ----------------------------------------------------------- códigos --- */

func aleatorio(n int, alfa string) string {
	var s strings.Builder
	max := big.NewInt(int64(len(alfa)))
	for i := 0; i < n; i++ {
		k, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		s.WriteByte(alfa[k.Int64()])
	}
	return s.String()
}

func (b *Banco) codigoLivre() string {
	usados := map[string]bool{}
	for _, v := range b.c.Veiculos {
		usados[v.Codigo] = true
		for _, e := range v.Extras {
			usados[e] = true
		}
	}
	for {
		c := aleatorio(tamanhoCodigo, alfabeto)
		if !usados[c] {
			return c
		}
	}
}

/* ---------------------------------------------------------- veículos --- */

// Veiculo devolve uma cópia do veículo da placa.
func (b *Banco) Veiculo(placa string) (Veiculo, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	v, ok := b.c.Veiculos[NormalizarPlaca(placa)]
	if !ok {
		return Veiculo{}, false
	}
	return *v, true
}

// VeiculoPorCodigo acha o veículo pelo código do QR.
func (b *Banco) VeiculoPorCodigo(codigo string) (Veiculo, bool) {
	codigo = strings.ToUpper(strings.TrimSpace(codigo))
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, v := range b.c.Veiculos {
		if v.Codigo == codigo {
			return *v, true
		}
		for _, e := range v.Extras {
			if e == codigo {
				return *v, true
			}
		}
	}
	return Veiculo{}, false
}

// garantirVeiculo cria o veículo se não existe (com trava de escrita).
func (b *Banco) garantirVeiculo(placa, modelo, ano string) *Veiculo {
	v, ok := b.c.Veiculos[placa]
	if !ok {
		v = &Veiculo{Placa: placa, Codigo: b.codigoLivre(), CriadoEm: time.Now()}
		b.c.Veiculos[placa] = v
	}
	if strings.TrimSpace(modelo) != "" {
		v.Modelo = strings.TrimSpace(modelo)
	}
	if strings.TrimSpace(ano) != "" {
		v.Ano = strings.TrimSpace(ano)
	}
	return v
}

// SalvarVeiculo cria ou altera os dados do veículo (não muda o código).
func (b *Banco) SalvarVeiculo(v Veiculo) (Veiculo, error) {
	placa := NormalizarPlaca(v.Placa)
	if placa == "" {
		return Veiculo{}, ErrPlaca
	}
	b.mu.Lock()
	atual := b.garantirVeiculo(placa, "", "")
	atual.Modelo = strings.TrimSpace(v.Modelo)
	atual.Ano = strings.TrimSpace(v.Ano)
	atual.Cliente = strings.TrimSpace(v.Cliente)
	atual.Telefone = strings.TrimSpace(v.Telefone)
	copia := *atual
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return copia, err
}

// ResumoVeiculo é uma linha da lista de veículos.
type ResumoVeiculo struct {
	Veiculo
	Servicos int    `json:"servicos"`
	Ultimo   string `json:"ultimo,omitempty"` // data do último serviço
}

// BuscarVeiculos procura por placa, modelo ou cliente (vazio = todos).
func (b *Banco) BuscarVeiculos(q string, limite int) []ResumoVeiculo {
	qn := strings.ToUpper(semAcento(strings.TrimSpace(q)))
	qp := strings.NewReplacer("-", "", " ", "").Replace(qn)
	b.mu.RLock()
	defer b.mu.RUnlock()
	cont := map[string]int{}
	ult := map[string]string{}
	for _, s := range b.c.Servicos {
		cont[s.Placa]++
		if s.Data > ult[s.Placa] {
			ult[s.Placa] = s.Data
		}
	}
	var out []ResumoVeiculo
	for _, v := range b.c.Veiculos {
		if qn != "" {
			alvo := strings.ToUpper(semAcento(v.Modelo + " " + v.Cliente + " " + v.Codigo))
			if !strings.Contains(v.Placa, qp) && !strings.Contains(alvo, qn) {
				continue
			}
		}
		out = append(out, ResumoVeiculo{Veiculo: *v, Servicos: cont[v.Placa], Ultimo: ult[v.Placa]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ultimo != out[j].Ultimo {
			return out[i].Ultimo > out[j].Ultimo
		}
		return out[i].Placa < out[j].Placa
	})
	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

/* ---------------------------------------------------------- serviços --- */

// NovoServico é o que vem da tela (ou do Zeus) para registrar.
type NovoServico struct {
	Placa              string   `json:"placa"`
	Modelo             string   `json:"modelo"`
	Ano                string   `json:"ano"`
	Data               string   `json:"data"`
	KM                 int      `json:"km"`
	Itens              []string `json:"itens"`
	Outros             []string `json:"outros"`
	ProximaKM          int      `json:"proxima_troca_km"`
	ProximaData        string   `json:"proxima_troca_data"`
	ProximaCorreiaKM   int      `json:"proxima_correia_km"`
	ProximaCorreiaData string   `json:"proxima_correia_data"`
	ObsCliente         string   `json:"obs"`
	ObsInterna         string   `json:"obs_interna"`
	Origem             string   `json:"origem"`
	ZeusRef            string   `json:"zeus_ref"`
}

var reData = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func limparLista(l []string, validos []string) []string {
	ok := map[string]bool{}
	for _, v := range validos {
		ok[v] = true
	}
	var out []string
	visto := map[string]bool{}
	for _, s := range l {
		s = strings.TrimSpace(s)
		if s == "" || visto[s] || (validos != nil && !ok[s]) {
			continue
		}
		visto[s] = true
		out = append(out, s)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// ordenar os itens na ordem padrão
func ordenarItens(l []string) []string {
	pos := map[string]int{}
	for i, it := range Itens {
		pos[it] = i
	}
	sort.SliceStable(l, func(i, j int) bool { return pos[l[i]] < pos[l[j]] })
	return l
}

func (n *NovoServico) validar() (string, error) {
	placa := NormalizarPlaca(n.Placa)
	if placa == "" {
		return "", ErrPlaca
	}
	if n.Data == "" {
		n.Data = time.Now().Format("2006-01-02")
	}
	if !reData.MatchString(n.Data) {
		return "", errors.New("data inválida")
	}
	if n.KM < 0 || n.KM > 3000000 || n.ProximaKM < 0 || n.ProximaKM > 3000000 || n.ProximaCorreiaKM < 0 || n.ProximaCorreiaKM > 3000000 {
		return "", errors.New("KM inválido")
	}
	if n.ProximaData != "" && !reData.MatchString(n.ProximaData) {
		return "", errors.New("data da próxima troca de óleo inválida")
	}
	if n.ProximaCorreiaData != "" && !reData.MatchString(n.ProximaCorreiaData) {
		return "", errors.New("data da próxima troca da correia inválida")
	}
	n.Itens = ordenarItens(limparLista(n.Itens, Itens))
	var outros []string
	for _, o := range n.Outros {
		for _, linha := range strings.Split(o, "\n") {
			outros = append(outros, linha)
		}
	}
	n.Outros = limparLista(outros, nil)
	if len(n.Itens) == 0 && len(n.Outros) == 0 && n.ProximaKM == 0 {
		return "", ErrServicoNada
	}
	n.ObsCliente = strings.TrimSpace(n.ObsCliente)
	n.ObsInterna = strings.TrimSpace(n.ObsInterna)
	if n.Origem == "" {
		n.Origem = "manual"
	}
	return placa, nil
}

// RegistrarServico grava um serviço novo e devolve ele com o veículo.
func (b *Banco) RegistrarServico(n NovoServico) (Servico, Veiculo, error) {
	placa, err := n.validar()
	if err != nil {
		return Servico{}, Veiculo{}, err
	}
	b.mu.Lock()
	if n.ZeusRef != "" {
		for _, s := range b.c.Servicos {
			if TemRef(s.ZeusRef, n.ZeusRef) {
				v := *b.c.Veiculos[s.Placa]
				cp := *s
				b.mu.Unlock()
				return cp, v, nil // já registrado
			}
		}
	}
	v := b.garantirVeiculo(placa, n.Modelo, n.Ano)
	agora := time.Now()
	s := &Servico{
		ID: aleatorio(10, alfabeto), Placa: placa, Data: n.Data, KM: n.KM,
		Itens: n.Itens, Outros: n.Outros, ProximaKM: n.ProximaKM,
		ProximaCorreiaKM: n.ProximaCorreiaKM, ProximaCorreiaData: n.ProximaCorreiaData, ProximaData: n.ProximaData,
		ObsCliente: n.ObsCliente, ObsInterna: n.ObsInterna,
		Origem: n.Origem, ZeusRef: n.ZeusRef, CriadoEm: agora, AlteradoEm: agora,
	}
	b.c.Servicos = append(b.c.Servicos, s)
	cs, cv := *s, *v
	err = b.mudou()
	b.mu.Unlock()
	if err != nil {
		return Servico{}, Veiculo{}, err
	}
	b.avisar()
	return cs, cv, nil
}

// AlterarServico troca os dados de um serviço (placa inclusive).
func (b *Banco) AlterarServico(id string, n NovoServico) (Servico, error) {
	placa, err := n.validar()
	if err != nil {
		return Servico{}, err
	}
	b.mu.Lock()
	s := b.servico(id)
	if s == nil {
		b.mu.Unlock()
		return Servico{}, ErrNaoAchado
	}
	b.garantirVeiculo(placa, n.Modelo, n.Ano)
	s.Placa, s.Data, s.KM = placa, n.Data, n.KM
	s.Itens, s.Outros, s.ProximaKM = n.Itens, n.Outros, n.ProximaKM
	s.ProximaCorreiaKM, s.ProximaCorreiaData = n.ProximaCorreiaKM, n.ProximaCorreiaData
	s.ProximaData = n.ProximaData
	s.ObsCliente, s.ObsInterna = n.ObsCliente, n.ObsInterna
	s.AlteradoEm = time.Now()
	cp := *s
	err = b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return cp, err
}

func (b *Banco) ApagarServico(id string) error {
	b.mu.Lock()
	idx := -1
	for i, s := range b.c.Servicos {
		if s.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		b.mu.Unlock()
		return ErrNaoAchado
	}
	refs := b.c.Servicos[idx].ZeusRef
	b.c.Servicos = append(b.c.Servicos[:idx], b.c.Servicos[idx+1:]...)
	// os lançamentos do Zeus desse serviço voltam a ficar pendentes
	for _, l := range b.c.Lancamentos {
		if refs != "" && TemRef(refs, l.Ref) {
			l.Status, l.ServicoID = Pendente, ""
		}
	}
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return err
}

func (b *Banco) servico(id string) *Servico {
	for _, s := range b.c.Servicos {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Servico devolve uma cópia do serviço e do veículo dele.
func (b *Banco) Servico(id string) (Servico, Veiculo, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s := b.servico(id)
	if s == nil {
		return Servico{}, Veiculo{}, false
	}
	v := b.c.Veiculos[s.Placa]
	if v == nil {
		return *s, Veiculo{Placa: s.Placa}, true
	}
	return *s, *v, true
}

// TemRef diz se a lista "a,b,c" tem a referência.
func TemRef(lista, ref string) bool {
	if ref == "" {
		return false
	}
	for _, r := range strings.Split(lista, ",") {
		if strings.TrimSpace(r) == ref {
			return true
		}
	}
	return false
}

// ServicoDoDia acha o serviço mais recente da placa naquela data.
func (b *Banco) ServicoDoDia(placa, data string) (Servico, bool) {
	placa = NormalizarPlaca(placa)
	b.mu.RLock()
	defer b.mu.RUnlock()
	var achado *Servico
	for _, s := range b.c.Servicos {
		if s.Placa == placa && s.Data == data && (achado == nil || s.CriadoEm.After(achado.CriadoEm)) {
			achado = s
		}
	}
	if achado == nil {
		return Servico{}, false
	}
	return *achado, true
}

// MesclarServico junta itens/outros (e a referência do Zeus) num serviço que já existe.
func (b *Banco) MesclarServico(id string, itens, outros []string, ref string, km int, obsInterna string) (Servico, error) {
	b.mu.Lock()
	s := b.servico(id)
	if s == nil {
		b.mu.Unlock()
		return Servico{}, ErrNaoAchado
	}
	s.Itens = ordenarItens(limparLista(append(append([]string{}, s.Itens...), itens...), Itens))
	s.Outros = limparLista(append(append([]string{}, s.Outros...), outros...), nil)
	if ref != "" && !TemRef(s.ZeusRef, ref) {
		if s.ZeusRef == "" {
			s.ZeusRef = ref
		} else {
			s.ZeusRef += "," + ref
		}
	}
	if s.KM == 0 && km > 0 {
		s.KM = km
	}
	if o := strings.TrimSpace(obsInterna); o != "" && !strings.Contains(s.ObsInterna, o) {
		s.ObsInterna = strings.TrimSpace(s.ObsInterna + " | " + o)
		s.ObsInterna = strings.TrimPrefix(s.ObsInterna, "| ")
	}
	s.AlteradoEm = time.Now()
	cp := *s
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return cp, err
}

// ComplementarServico junta a observação para o cliente (ex.: marca do óleo)
// e a próxima troca num serviço que já existe (venda juntada no serviço do dia).
func (b *Banco) ComplementarServico(id, obsCliente string, proximaKM int, proximaData string, correiaKM int, correiaData string) (Servico, error) {
	b.mu.Lock()
	s := b.servico(id)
	if s == nil {
		b.mu.Unlock()
		return Servico{}, ErrNaoAchado
	}
	mudou := false
	if o := strings.TrimSpace(obsCliente); o != "" && !strings.Contains(s.ObsCliente, o) {
		if s.ObsCliente == "" {
			s.ObsCliente = o
		} else {
			s.ObsCliente += " · " + o
		}
		mudou = true
	}
	if proximaKM > 0 && s.ProximaKM == 0 {
		s.ProximaKM, mudou = proximaKM, true
	}
	if proximaData != "" && s.ProximaData == "" && reData.MatchString(proximaData) {
		s.ProximaData, mudou = proximaData, true
	}
	if correiaKM > 0 && s.ProximaCorreiaKM == 0 {
		s.ProximaCorreiaKM, mudou = correiaKM, true
	}
	if correiaData != "" && s.ProximaCorreiaData == "" && reData.MatchString(correiaData) {
		s.ProximaCorreiaData, mudou = correiaData, true
	}
	if !mudou {
		cp := *s
		b.mu.Unlock()
		return cp, nil
	}
	s.AlteradoEm = time.Now()
	cp := *s
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return cp, err
}

// ContarImpressao soma uma impressão no serviço.
func (b *Banco) ContarImpressao(id string) {
	b.mu.Lock()
	if s := b.servico(id); s != nil {
		s.Impressoes++
		_ = b.mudou()
	}
	b.mu.Unlock()
}

func ordenarServicos(l []Servico) {
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].Data != l[j].Data {
			return l[i].Data > l[j].Data
		}
		if l[i].KM != l[j].KM {
			return l[i].KM > l[j].KM
		}
		return l[i].CriadoEm.After(l[j].CriadoEm)
	})
}

// ServicosDoVeiculo do mais novo para o mais antigo.
func (b *Banco) ServicosDoVeiculo(placa string) []Servico {
	placa = NormalizarPlaca(placa)
	b.mu.RLock()
	var out []Servico
	for _, s := range b.c.Servicos {
		if s.Placa == placa {
			out = append(out, *s)
		}
	}
	b.mu.RUnlock()
	ordenarServicos(out)
	return out
}

// ServicoComVeiculo é uma linha da lista de últimos serviços.
type ServicoComVeiculo struct {
	Servico
	Modelo string `json:"modelo"`
	Codigo string `json:"codigo"`
}

// UltimosServicos pela data de registro (mais novo primeiro).
func (b *Banco) UltimosServicos(limite int) []ServicoComVeiculo {
	b.mu.RLock()
	var out []ServicoComVeiculo
	for _, s := range b.c.Servicos {
		x := ServicoComVeiculo{Servico: *s}
		if v := b.c.Veiculos[s.Placa]; v != nil {
			x.Modelo, x.Codigo = v.Modelo, v.Codigo
		}
		out = append(out, x)
	}
	b.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].CriadoEm.After(out[j].CriadoEm) })
	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

// Tudo devolve cópias de tudo (para publicar e para cópia de segurança).
func (b *Banco) Tudo() (map[string]Veiculo, []Servico) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	vs := make(map[string]Veiculo, len(b.c.Veiculos))
	for k, v := range b.c.Veiculos {
		vs[k] = *v
	}
	ss := make([]Servico, 0, len(b.c.Servicos))
	for _, s := range b.c.Servicos {
		ss = append(ss, *s)
	}
	return vs, ss
}

// JSON devolve o arquivo de dados como está no disco (cópia de segurança).
func (b *Banco) JSON() ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return json.MarshalIndent(b.c, "", " ")
}

// Alteracao é a hora da última mudança.
func (b *Banco) Alteracao() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.c.Alteracao
}

/* ------------------------------------------------- lançamentos do Zeus --- */

// ReceberLancamento guarda um lançamento novo. Devolve false se já existia.
func (b *Banco) ReceberLancamento(l Lancamento) (Lancamento, bool, error) {
	b.mu.Lock()
	for _, x := range b.c.Lancamentos {
		if x.Ref == l.Ref {
			cp := *x
			b.mu.Unlock()
			return cp, false, nil
		}
	}
	if l.Status == "" {
		l.Status = Pendente
	}
	if l.RecebidoEm.IsZero() {
		l.RecebidoEm = time.Now()
	}
	if l.Itens == nil {
		l.Itens = []string{}
	}
	if l.Outros == nil {
		l.Outros = []string{}
	}
	novo := l
	b.c.Lancamentos = append(b.c.Lancamentos, &novo)
	// guarda só os últimos 2000
	if n := len(b.c.Lancamentos); n > 2000 {
		b.c.Lancamentos = b.c.Lancamentos[n-2000:]
	}
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return novo, true, err
}

// AtualizarLancamento troca produtos/itens/texto de um lançamento que mudou no
// Zeus (ex.: puseram mais uma peça na venda depois). Mantém status e placa
// escolhida; se ainda não tinha placa e agora tem, usa a nova.
func (b *Banco) AtualizarLancamento(novo Lancamento) (Lancamento, bool, error) {
	b.mu.Lock()
	var l *Lancamento
	for _, x := range b.c.Lancamentos {
		if x.Ref == novo.Ref {
			l = x
		}
	}
	if l == nil {
		b.mu.Unlock()
		return Lancamento{}, false, ErrNaoAchado
	}
	mudou := strings.Join(l.Produtos, "|") != strings.Join(novo.Produtos, "|") || l.Texto != novo.Texto || l.Cliente != novo.Cliente
	if !mudou {
		cp := *l
		b.mu.Unlock()
		return cp, false, nil
	}
	l.Produtos, l.Itens, l.Outros, l.Texto, l.Cliente = novo.Produtos, novo.Itens, novo.Outros, novo.Texto, novo.Cliente
	l.SoMaoDeObra = novo.SoMaoDeObra
	if l.Placa == "" {
		l.Placa = novo.Placa
	}
	if l.KM == 0 {
		l.KM = novo.KM
	}
	if l.Modelo == "" {
		l.Modelo, l.Ano = novo.Modelo, novo.Ano
	}
	if novo.Telefone != "" {
		l.Telefone = novo.Telefone
	}
	cp := *l
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return cp, true, err
}

// Lancamentos devolve os lançamentos (pendentes primeiro, mais novos primeiro).
func (b *Banco) Lancamentos(status string, limite int) []Lancamento {
	b.mu.RLock()
	var out []Lancamento
	for i := len(b.c.Lancamentos) - 1; i >= 0; i-- {
		l := b.c.Lancamentos[i]
		if status == "" || l.Status == status {
			out = append(out, *l)
		}
	}
	b.mu.RUnlock()
	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

func (b *Banco) Lancamento(ref string) (Lancamento, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, l := range b.c.Lancamentos {
		if l.Ref == ref {
			return *l, true
		}
	}
	return Lancamento{}, false
}

// MarcarLancamento muda o status (registrado/ignorado/pendente).
func (b *Banco) MarcarLancamento(ref, status, servicoID string) error {
	b.mu.Lock()
	var achou bool
	for _, l := range b.c.Lancamentos {
		if l.Ref == ref {
			l.Status, l.ServicoID, achou = status, servicoID, true
		}
	}
	if !achou {
		b.mu.Unlock()
		return ErrNaoAchado
	}
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return err
}

func (b *Banco) ZeusCursor() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.c.ZeusCursor
}

func (b *Banco) SalvarZeusCursor(c string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.c.ZeusCursor == c {
		return nil
	}
	b.c.ZeusCursor = c
	return b.mudou()
}

// MarcarAvisado guarda que o cliente do carro recebeu lembrete de revisão.
func (b *Banco) MarcarAvisado(placa, data string) error {
	b.mu.Lock()
	v, ok := b.c.Veiculos[NormalizarPlaca(placa)]
	if !ok {
		b.mu.Unlock()
		return ErrNaoAchado
	}
	v.AvisadoEm = data
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return err
}

// RevisarNomes passa de novo os nomes dos serviços que vieram do Zeus pelo
// "tradutor" atual (uma vez por versão de regras): tira material de consumo e
// mão de obra genérica, acerta os nomes e move óleo/filtros para os itens.
// revisar recebe itens e outros e devolve os novos. Devolve quantos mudaram.
func (b *Banco) RevisarNomes(versao int, revisar func(itens, outros []string) ([]string, []string)) (int, error) {
	b.mu.Lock()
	if b.c.Versao >= versao {
		b.mu.Unlock()
		return 0, nil
	}
	n := 0
	for _, s := range b.c.Servicos {
		if s.Origem != "zeus" {
			continue
		}
		it, ou := revisar(s.Itens, s.Outros)
		if strings.Join(it, "|") != strings.Join(s.Itens, "|") || strings.Join(ou, "|") != strings.Join(s.Outros, "|") {
			s.Itens, s.Outros = it, ou
			s.AlteradoEm = time.Now()
			n++
		}
	}
	b.c.Versao = versao
	err := b.mudou()
	b.mu.Unlock()
	if err == nil && n > 0 {
		b.avisar()
	}
	return n, err
}

// RecalcularPendentes passa de novo os lançamentos ainda não revisados pela
// leitura atual de modelo/ano e de cliente (uma vez por versão de regras).
func (b *Banco) RecalcularPendentes(versao int, cliente func(string) string) error {
	b.mu.Lock()
	if b.c.Versao >= versao {
		b.mu.Unlock()
		return nil
	}
	for _, l := range b.c.Lancamentos {
		if l.Status != Pendente {
			continue
		}
		if m, a := AcharModelo(l.Texto); m != "" || a != "" {
			l.Modelo, l.Ano = m, a
		}
		l.Cliente = cliente(l.Cliente)
	}
	// a versão é marcada pelo RevisarNomes(versao) que roda logo depois
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return err
}

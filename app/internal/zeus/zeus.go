// Pacote zeus lê (só lê, nunca altera) o banco PostgreSQL do Zeus Gestão e
// transforma as vendas/OS em lançamentos para o histórico do carro.
package zeus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "github.com/lib/pq"
	"golang.org/x/text/encoding/charmap"

	"oficinaqr/internal/dados"
	"oficinaqr/internal/itens"
)

type Config struct {
	Ativo      bool   `json:"ativo"`
	Host       string `json:"host"`
	Porta      int    `json:"porta"`
	Banco      string `json:"banco"`
	Usuario    string `json:"usuario"`
	Senha      string `json:"-"` // fica cifrada no disco
	SQL        string `json:"sql"`
	Intervalo  int    `json:"intervalo_seg"`
	Desde      string `json:"desde"` // AAAA-MM-DD: só lê lançamentos a partir daqui
	AutoRegist bool   `json:"auto_registrar"`
	AutoImprim bool   `json:"auto_imprimir"`
	EsperaMin  int    `json:"espera_min"` // minutos sem mudança antes de registrar uma venda nova
	Modo       string `json:"modo"`       // "pdf" (padrão) ou "banco"
	PastaPDF   string `json:"pasta_pdf"`  // pasta onde o balcão salva os PDFs do Zeus
}

// ModoEfetivo: sem modo escolhido, usa o banco só se já houver consulta.
func (c Config) ModoEfetivo() string {
	switch c.Modo {
	case ModoBanco, ModoPDF:
		return c.Modo
	}
	if strings.TrimSpace(c.SQL) != "" {
		return ModoBanco
	}
	return ModoPDF
}

func ConfigPadrao() Config {
	return Config{Host: "192.168.1.150", Porta: 5432, Intervalo: 20, AutoRegist: true, AutoImprim: true, EsperaMin: 1,
		Desde: time.Now().Format("2006-01-02"), Modo: ModoPDF, PastaPDF: PastaPadrao()}
}

// Linha é uma linha devolvida pela consulta do Zeus (um produto de uma venda).
//
// A consulta (Config.SQL) recebe $1 = data inicial e precisa devolver estas
// colunas (nomes exatos; só "ref" e "descricao" são obrigatórias):
//
//	ref        identificador único da venda/OS
//	documento  como mostrar (ex.: 'Venda 123')
//	data       data da venda
//	descricao  descrição do produto/serviço
//	cliente    nome do cliente
//	texto      observação / campos livres onde pode estar a placa e o KM
//	telefone   telefone do cliente (opcional; vai para os lembretes de revisão)
//	placa      placa do veículo (opcional; se o Zeus tiver o carro cadastrado
//	           na venda, a placa vem daqui e não precisa estar na observação)
//	km         KM do veículo (opcional)
type Linha struct {
	Ref, Documento, Data, Descricao, Cliente, Texto, Telefone string
	Placa, KM                                                 string `json:",omitempty"`
}

// SQLPadrao lê as pré-vendas PAGAS do Zeus da Auto Mecânica Itabaiana (não
// lê as canceladas nem as peças devolvidas). $1 = data inicial. Material de
// consumo, uso e consumo e ativo imobilizado ficam de fora já aqui; o resto
// (silicone, desengraxante, mão de obra genérica) o programa separa pelo nome.
const SQLPadrao = `select 'prevenda:' || pv.codigo as ref,
  'Pré-venda ' || pv.codigo as documento,
  to_char(coalesce(f.datahora, pv.datavenda + coalesce(pv.horavenda, time '00:00')), 'YYYY-MM-DD HH24:MI:SS') as data,
  trim(pp.nome) as descricao,
  case when pv.codcliente = 1 then '' else trim(coalesce(pv.nomecliente, '')) end as cliente,
  coalesce(pv.observacao, '') as texto,
  trim(coalesce(pv.fonecliente, '')) as telefone
from prevendas pv
join prevendas_faturamento f on f.codprevenda = pv.codigo
join prevendasprod pp on pp.codprevenda = pv.codigo
left join produtos pr on pr.codigo = pp.codproduto
where f.datahora >= $1::date
  and not exists (select 1 from prevendas_cancelamento c where c.prevendas_codigo = pv.codigo)
  and not exists (select 1 from devolucoesprod d where d.codprevendasprod = pp.codigo)
  and coalesce(pr.codgrupo, 0) not in (18, 44, 45)
order by pv.codigo, pp.ordem`

// Fonte de onde vêm as linhas (o banco do Zeus ou um arquivo de teste).
type Fonte interface {
	Buscar(ctx context.Context, desde string) ([]Linha, error)
}

/* ------------------------------------------------------------ banco --- */

func dsn(c Config) string {
	q := url.Values{}
	q.Set("sslmode", "disable")
	q.Set("connect_timeout", "8")
	q.Set("default_transaction_read_only", "on") // garante: só leitura
	porta := c.Porta
	if porta == 0 {
		porta = 5432
	}
	u := url.URL{Scheme: "postgres", Host: fmt.Sprintf("%s:%d", c.Host, porta), Path: "/" + c.Banco,
		User: url.UserPassword(c.Usuario, c.Senha), RawQuery: q.Encode()}
	return u.String()
}

type Postgres struct {
	cfg Config
	mu  sync.Mutex
	db  *sql.DB
}

func NovoPostgres(c Config) *Postgres { return &Postgres{cfg: c} }

func (p *Postgres) conectar() (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db != nil {
		return p.db, nil
	}
	if p.cfg.Host == "" || p.cfg.Banco == "" || p.cfg.Usuario == "" {
		return nil, errors.New("preencha servidor, banco e usuário do Zeus em Configurações")
	}
	db, err := sql.Open("postgres", dsn(p.cfg))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetConnMaxIdleTime(2 * time.Minute)
	p.db = db
	return db, nil
}

func (p *Postgres) Fechar() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db != nil {
		p.db.Close()
		p.db = nil
	}
}

// consultar roda um SELECT numa transação só de leitura.
func (p *Postgres) consultar(ctx context.Context, q string, args ...any) ([]string, [][]string, error) {
	db, err := p.conectar()
	if err != nil {
		return nil, nil, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, traduzir(err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, traduzir(err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out [][]string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		linha := make([]string, len(cols))
		for i, v := range vals {
			linha[i] = paraUTF8(v.String)
		}
		out = append(out, linha)
		if len(out) >= 5000 {
			break
		}
	}
	return cols, out, traduzir(rows.Err())
}

// paraUTF8: bancos antigos (SQL_ASCII/LATIN1) mandam acentos do Windows.
func paraUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	if t, err := charmap.Windows1252.NewDecoder().String(s); err == nil {
		return t
	}
	return strings.ToValidUTF8(s, "?")
}

func traduzir(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "password authentication failed"):
		return errors.New("usuário ou senha do banco do Zeus errados")
	case strings.Contains(s, "connection refused"), strings.Contains(s, "i/o timeout"), strings.Contains(s, "no such host"):
		return errors.New("não consegui falar com o servidor do Zeus (" + s + "). O computador do servidor está ligado?")
	case strings.Contains(s, "does not exist") && strings.Contains(s, "database"):
		return errors.New("o banco de dados com esse nome não existe no servidor do Zeus")
	}
	return err
}

func (p *Postgres) Buscar(ctx context.Context, desde string) ([]Linha, error) {
	if strings.TrimSpace(p.cfg.SQL) == "" {
		return nil, errors.New("falta a consulta do Zeus (Configurações > Zeus > Avançado)")
	}
	cols, linhas, err := p.consultar(ctx, p.cfg.SQL, desde)
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i, c := range cols {
		idx[strings.ToLower(c)] = i
	}
	pega := func(l []string, nome string) string {
		if i, ok := idx[nome]; ok {
			return strings.TrimSpace(l[i])
		}
		return ""
	}
	if _, ok := idx["ref"]; !ok {
		return nil, errors.New("a consulta do Zeus precisa ter uma coluna chamada ref")
	}
	if _, ok := idx["descricao"]; !ok {
		return nil, errors.New("a consulta do Zeus precisa ter uma coluna chamada descricao")
	}
	out := make([]Linha, 0, len(linhas))
	for _, l := range linhas {
		out = append(out, Linha{Ref: pega(l, "ref"), Documento: pega(l, "documento"), Data: pega(l, "data"),
			Descricao: pega(l, "descricao"), Cliente: pega(l, "cliente"), Texto: pega(l, "texto"), Telefone: pega(l, "telefone"),
			Placa: pega(l, "placa"), KM: pega(l, "km")})
	}
	return out, nil
}

// Testar só conecta e pergunta a versão.
func (p *Postgres) Testar(ctx context.Context) (string, error) {
	_, l, err := p.consultar(ctx, "select version()")
	if err != nil {
		return "", err
	}
	if len(l) == 0 {
		return "", errors.New("sem resposta")
	}
	return l[0][0], nil
}

/* --------------------------------------------------------- explorar --- */

type Tabela struct {
	Esquema string `json:"esquema"`
	Nome    string `json:"nome"`
	Linhas  int64  `json:"linhas"`
}

// Tabelas lista as tabelas com a quantidade aproximada de linhas.
func (p *Postgres) Tabelas(ctx context.Context) ([]Tabela, error) {
	_, l, err := p.consultar(ctx, `select n.nspname, c.relname, greatest(c.reltuples,0)::bigint
		from pg_class c join pg_namespace n on n.oid = c.relnamespace
		where c.relkind in ('r','v','m','p') and n.nspname not in ('pg_catalog','information_schema') and n.nspname not like 'pg_toast%'
		order by 3 desc, 2`)
	if err != nil {
		return nil, err
	}
	out := make([]Tabela, 0, len(l))
	for _, x := range l {
		var n int64
		fmt.Sscan(x[2], &n)
		out = append(out, Tabela{x[0], x[1], n})
	}
	return out, nil
}

// Consulta devolve colunas e linhas de um SELECT (só leitura, até 200 linhas).
func (p *Postgres) Consulta(ctx context.Context, q string) ([]string, [][]string, error) {
	t := strings.ToLower(strings.TrimSpace(q))
	if !strings.HasPrefix(t, "select") && !strings.HasPrefix(t, "with") {
		return nil, nil, errors.New("só é permitido SELECT")
	}
	cols, l, err := p.consultar(ctx, q)
	if len(l) > 200 {
		l = l[:200]
	}
	return cols, l, err
}

// Achado é um lugar onde um texto apareceu.
type Achado struct {
	Tabela  string   `json:"tabela"`
	Coluna  string   `json:"coluna"`
	Exemplo []string `json:"exemplo"`
}

// Procurar um texto (ex.: "MANGUEIRA") em todas as colunas de texto.
func (p *Postgres) Procurar(ctx context.Context, texto string, maxTabelas int) ([]Achado, error) {
	_, cols, err := p.consultar(ctx, `select c.table_schema, c.table_name, c.column_name
		from information_schema.columns c join information_schema.tables t
		  on t.table_schema = c.table_schema and t.table_name = c.table_name
		where c.table_schema not in ('pg_catalog','information_schema') and t.table_type = 'BASE TABLE'
		  and c.data_type in ('character varying','text','character')
		order by 1,2,3`)
	if err != nil {
		return nil, err
	}
	var out []Achado
	tabelas := map[string]bool{}
	for _, c := range cols {
		if ctx.Err() != nil {
			break
		}
		tab := fmt.Sprintf(`"%s"."%s"`, strings.ReplaceAll(c[0], `"`, `""`), strings.ReplaceAll(c[1], `"`, `""`))
		col := fmt.Sprintf(`"%s"`, strings.ReplaceAll(c[2], `"`, `""`))
		cx, cancel := context.WithTimeout(ctx, 4*time.Second)
		_, l, err := p.consultar(cx, "select "+col+"::text from "+tab+" where "+col+" ilike $1 limit 3", "%"+texto+"%")
		cancel()
		if err != nil || len(l) == 0 {
			continue
		}
		var ex []string
		for _, x := range l {
			ex = append(ex, x[0])
		}
		out = append(out, Achado{Tabela: c[0] + "." + c[1], Coluna: c[2], Exemplo: ex})
		tabelas[tab] = true
		if maxTabelas > 0 && len(tabelas) >= maxTabelas {
			break
		}
	}
	return out, nil
}

/* ----------------------------------------------------- lançamentos --- */

// Agrupar junta as linhas por venda/OS e já separa itens e placa.
func Agrupar(linhas []Linha, placasConhecidas map[string]string) []dados.Lancamento {
	por := map[string]*dados.Lancamento{}
	placaDoZeus, kmDoZeus := map[string]string{}, map[string]int{}
	var ordem []string
	for _, l := range linhas {
		if l.Ref == "" {
			continue
		}
		x, ok := por[l.Ref]
		if !ok {
			x = &dados.Lancamento{Ref: l.Ref, Documento: l.Documento, Data: soData(l.Data), Cliente: limparNome(l.Cliente), Texto: l.Texto, Telefone: telefoneBonito(l.Telefone)}
			if x.Documento == "" {
				x.Documento = l.Ref
			}
			por[l.Ref] = x
			ordem = append(ordem, l.Ref)
		}
		if p := dados.NormalizarPlaca(l.Placa); p != "" && placaDoZeus[l.Ref] == "" {
			placaDoZeus[l.Ref] = p
		}
		if k := dados.AcharKM(l.KM + " km"); k > 0 && kmDoZeus[l.Ref] == 0 {
			kmDoZeus[l.Ref] = k
		}
		if l.Texto != "" && !strings.Contains(x.Texto, l.Texto) {
			x.Texto = strings.TrimSpace(x.Texto + " " + l.Texto)
		}
		if strings.TrimSpace(l.Descricao) != "" {
			x.Produtos = append(x.Produtos, strings.TrimSpace(l.Descricao))
		}
	}
	out := make([]dados.Lancamento, 0, len(ordem))
	for _, r := range ordem {
		x := por[r]
		x.Itens, x.Outros = itens.Separar(x.Produtos)
		// a placa vem da observação; nomes de peças como "UNO 2005" parecem
		// placa, então só procura no resto se a observação não tiver nenhuma
		achadas := dados.AcharPlacas(x.Texto)
		if len(achadas) == 0 {
			achadas = dados.AcharPlacas(x.Cliente + " " + strings.Join(x.Produtos, " "))
		}
		if len(achadas) == 1 {
			x.Placa = achadas[0]
			x.Modelo, x.Ano = dados.AcharModelo(x.Texto)
		} else if len(achadas) == 0 && x.Cliente != "" && placasConhecidas != nil {
			// o cliente tem um carro só cadastrado: sugere ele
			if p, ok := placasConhecidas[strings.ToUpper(strings.TrimSpace(x.Cliente))]; ok {
				x.Placa = p
			}
		}
		x.KM = dados.AcharKM(x.Texto)
		// placa/KM cadastrados no Zeus (colunas placa e km da consulta) valem mais
		if p := placaDoZeus[r]; p != "" {
			x.Placa = p
			if x.Modelo == "" {
				x.Modelo, x.Ano = dados.AcharModelo(x.Texto)
			}
		}
		if k := kmDoZeus[r]; k > 0 {
			x.KM = k
		}
		// só mão de obra: pode ser peça trazida pelo cliente (vai para revisão com aviso)
		if len(x.Itens) == 0 && len(x.Outros) == 0 {
			for _, p := range x.Produtos {
				if reMaoDeObra.MatchString(p) {
					x.SoMaoDeObra = true
					break
				}
			}
		}
		out = append(out, *x)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Data < out[j].Data })
	return out
}

var reMaoDeObra = regexp.MustCompile(`(?i)M[ÃA]O\s*DE\s*OBRA|\bM\.?\s*O\.?\s*$|\bSERVI[ÇC]OS?\b`)

// limparNome: "DANILO ESQUADRIA." → "DANILO ESQUADRIA"; "ZOMINHO  DO  POVOADO" → "ZOMINHO DO POVOADO".
// LimparNome é o limparNome para fora do pacote.
func LimparNome(s string) string { return limparNome(s) }

func limparNome(s string) string {
	s = strings.Trim(strings.Join(strings.Fields(s), " "), " .,;-")
	switch strings.ToUpper(s) {
	case "VENDA A CONSUMIDOR", "CONSUMIDOR", "CONSUMIDOR FINAL", "CLIENTE": // cliente genérico do Zeus
		return ""
	}
	return s
}

// telefoneBonito: "998256066" → "(79) 99825-6066" (DDD de Sergipe quando falta).
func telefoneBonito(s string) string {
	var d []rune
	for _, r := range s {
		if r >= '0' && r <= '9' {
			d = append(d, r)
		}
	}
	n := string(d)
	n = strings.TrimPrefix(n, "55")
	if len(n) == 8 || len(n) == 9 {
		n = "79" + n
	}
	switch len(n) {
	case 11:
		return "(" + n[:2] + ") " + n[2:7] + "-" + n[7:]
	case 10:
		return "(" + n[:2] + ") " + n[2:6] + "-" + n[6:]
	}
	return strings.TrimSpace(s)
}

func soData(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	// 21/10/2026
	if len(s) >= 10 && s[2] == '/' && s[5] == '/' {
		return s[6:10] + "-" + s[3:5] + "-" + s[0:2]
	}
	return time.Now().Format("2006-01-02")
}

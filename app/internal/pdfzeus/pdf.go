// Pacote pdfzeus lê o texto dos PDFs que o Zeus exporta (FastReport).
//
// Esses PDFs trazem a tabela ToUnicode incompleta: várias letras somem quando
// o texto é copiado ("fiat uno" vira "at o"). Por isso o texto é lido direto
// das fontes embutidas no PDF: cada código de glifo é traduzido pela tabela
// cmap da própria fonte, e só se não der usamos a ToUnicode.
package pdfzeus

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

/* ------------------------------------------------------- objetos PDF --- */

type nome string
type ref struct{ num, gen int }
type dict map[string]any
type hexstr []byte

type stream struct {
	d    dict
	data []byte // ainda comprimido
}

type documento struct {
	objs map[int]any
}

var reObj = regexp.MustCompile(`(\d+)\s+(\d+)\s+obj\b`)

func abrir(b []byte) (*documento, error) {
	if !bytes.HasPrefix(bytes.TrimLeft(b, "\x00\r\n\t "), []byte("%PDF")) {
		return nil, errors.New("o arquivo não é um PDF")
	}
	d := &documento{objs: map[int]any{}}
	for _, m := range reObj.FindAllSubmatchIndex(b, -1) {
		num, _ := strconv.Atoi(string(b[m[2]:m[3]]))
		p := &parser{b: b, i: m[1]}
		v, err := p.valor()
		if err != nil {
			continue
		}
		p.espacos()
		if dd, ok := v.(dict); ok && p.tem("stream") {
			p.i += len("stream")
			if p.i < len(b) && b[p.i] == '\r' {
				p.i++
			}
			if p.i < len(b) && b[p.i] == '\n' {
				p.i++
			}
			ini := p.i
			fim := -1
			if n, ok := dd["Length"].(float64); ok && ini+int(n) <= len(b) {
				resto := bytes.TrimLeft(b[ini+int(n):min(len(b), ini+int(n)+20)], "\r\n ")
				if bytes.HasPrefix(resto, []byte("endstream")) {
					fim = ini + int(n)
				}
			}
			if fim < 0 { // Length indireto ou errado: procura o fim
				k := bytes.Index(b[ini:], []byte("endstream"))
				if k < 0 {
					continue
				}
				fim = ini + k
			}
			v = &stream{d: dd, data: b[ini:fim]}
		}
		d.objs[num] = v // o último vence (atualizações incrementais)
	}
	if len(d.objs) == 0 {
		return nil, errors.New("PDF vazio ou danificado")
	}
	return d, nil
}

// resolve segue referências indiretas.
func (d *documento) resolve(v any) any {
	for i := 0; i < 20; i++ {
		r, ok := v.(ref)
		if !ok {
			return v
		}
		v = d.objs[r.num]
	}
	return v
}

func (d *documento) dict(v any) dict {
	switch x := d.resolve(v).(type) {
	case dict:
		return x
	case *stream:
		return x.d
	}
	return nil
}

func (d *documento) dados(v any) ([]byte, error) {
	s, ok := d.resolve(v).(*stream)
	if !ok {
		return nil, errors.New("não é stream")
	}
	filtros := []any{}
	switch f := d.resolve(s.d["Filter"]).(type) {
	case nome:
		filtros = append(filtros, f)
	case []any:
		filtros = f
	}
	out := s.data
	for _, f := range filtros {
		switch d.resolve(f) {
		case nome("FlateDecode"):
			r, err := zlib.NewReader(bytes.NewReader(out))
			if err != nil {
				return nil, err
			}
			out, err = io.ReadAll(r)
			if err != nil && len(out) == 0 {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("filtro %v não suportado", f)
		}
	}
	return out, nil
}

/* ------------------------------------------------------------ parser --- */

type parser struct {
	b []byte
	i int
}

func espaco(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func delim(c byte) bool {
	return espaco(c) || strings.IndexByte("()<>[]{}/%", c) >= 0
}

func (p *parser) espacos() {
	for p.i < len(p.b) {
		c := p.b[p.i]
		if espaco(c) {
			p.i++
		} else if c == '%' {
			for p.i < len(p.b) && p.b[p.i] != '\n' && p.b[p.i] != '\r' {
				p.i++
			}
		} else {
			return
		}
	}
}

func (p *parser) tem(s string) bool { return bytes.HasPrefix(p.b[p.i:], []byte(s)) }

var errFim = errors.New("fim")

// token devolve o próximo valor; palavras soltas (operadores) vêm como string.
func (p *parser) valor() (any, error) {
	p.espacos()
	if p.i >= len(p.b) {
		return nil, errFim
	}
	c := p.b[p.i]
	switch {
	case c == '/':
		p.i++
		ini := p.i
		for p.i < len(p.b) && !delim(p.b[p.i]) {
			p.i++
		}
		return nome(decodNome(string(p.b[ini:p.i]))), nil
	case c == '<' && p.i+1 < len(p.b) && p.b[p.i+1] == '<':
		p.i += 2
		d := dict{}
		for {
			p.espacos()
			if p.tem(">>") {
				p.i += 2
				return d, nil
			}
			k, err := p.valor()
			if err != nil {
				return nil, err
			}
			kn, ok := k.(nome)
			if !ok {
				return nil, errors.New("chave inválida")
			}
			v, err := p.valor()
			if err != nil {
				return nil, err
			}
			d[string(kn)] = v
		}
	case c == '<':
		p.i++
		ini := p.i
		for p.i < len(p.b) && p.b[p.i] != '>' {
			p.i++
		}
		h := make([]byte, 0, p.i-ini)
		for _, x := range p.b[ini:p.i] {
			if !espaco(x) {
				h = append(h, x)
			}
		}
		p.i++
		if len(h)%2 == 1 {
			h = append(h, '0')
		}
		out := make([]byte, len(h)/2)
		for k := range out {
			v, _ := strconv.ParseUint(string(h[2*k:2*k+2]), 16, 8)
			out[k] = byte(v)
		}
		return hexstr(out), nil
	case c == '(':
		return p.literal(), nil
	case c == '[':
		p.i++
		var a []any
		for {
			p.espacos()
			if p.i >= len(p.b) {
				return nil, errFim
			}
			if p.b[p.i] == ']' {
				p.i++
				return a, nil
			}
			v, err := p.valor()
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
	case c == ']' || c == '>' || c == ')' || c == '{' || c == '}':
		p.i++
		return string(c), nil
	default:
		ini := p.i
		for p.i < len(p.b) && !delim(p.b[p.i]) {
			p.i++
		}
		if p.i == ini {
			p.i++
		}
		w := string(p.b[ini:p.i])
		if f, err := strconv.ParseFloat(w, 64); err == nil {
			// "12 0 R" é referência
			salva := p.i
			p.espacos()
			j := p.i
			for j < len(p.b) && p.b[j] >= '0' && p.b[j] <= '9' {
				j++
			}
			if j > p.i && !strings.Contains(w, ".") {
				gen, _ := strconv.Atoi(string(p.b[p.i:j]))
				k := j
				for k < len(p.b) && espaco(p.b[k]) {
					k++
				}
				if k < len(p.b) && p.b[k] == 'R' && (k+1 >= len(p.b) || delim(p.b[k+1])) {
					p.i = k + 1
					return ref{int(f), gen}, nil
				}
			}
			p.i = salva
			return f, nil
		}
		switch w {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		}
		return w, nil
	}
}

func (p *parser) literal() []byte {
	p.i++ // (
	nivel := 1
	var out []byte
	for p.i < len(p.b) {
		c := p.b[p.i]
		p.i++
		switch c {
		case '\\':
			if p.i >= len(p.b) {
				return out
			}
			e := p.b[p.i]
			p.i++
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '\r':
				if p.i < len(p.b) && p.b[p.i] == '\n' {
					p.i++
				}
			case '\n':
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for k := 0; k < 2 && p.i < len(p.b) && p.b[p.i] >= '0' && p.b[p.i] <= '7'; k++ {
						v = v*8 + int(p.b[p.i]-'0')
						p.i++
					}
					out = append(out, byte(v))
				} else {
					out = append(out, e)
				}
			}
		case '(':
			nivel++
			out = append(out, c)
		case ')':
			nivel--
			if nivel == 0 {
				return out
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

func decodNome(s string) string {
	if !strings.Contains(s, "#") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

/* ------------------------------------------------------------ fontes --- */

type fonte struct {
	doisBytes bool
	glifo     map[uint16]string // código → texto, pela cmap/GSUB da fonte embutida
	toUni     map[uint16]string // código → texto, pela ToUnicode do PDF
}

// letras "coladas" (ﬁ, ﬂ...) viram letras normais.
var semLigadura = strings.NewReplacer("\uFB00", "ff", "\uFB01", "fi", "\uFB02", "fl", "\uFB03", "ffi", "\uFB04", "ffl", "\uFB05", "st", "\uFB06", "st")

// valido: a ToUnicode do Zeus às vezes manda lixo (\x01) no lugar da letra.
func valido(t string) bool {
	if t == "" {
		return false
	}
	for _, r := range t {
		if r < 0x20 || r == 0xFFFD {
			return false
		}
	}
	return true
}

func (f *fonte) texto(s []byte) string {
	var b strings.Builder
	if !f.doisBytes {
		for _, c := range s {
			if t, ok := f.toUni[uint16(c)]; ok && valido(t) {
				b.WriteString(t)
			} else if t, ok := f.glifo[uint16(c)]; ok {
				b.WriteString(t)
			} else {
				b.WriteRune(charmap.Windows1252.DecodeByte(c))
			}
		}
		return b.String()
	}
	for i := 0; i+1 < len(s); i += 2 {
		c := uint16(s[i])<<8 | uint16(s[i+1])
		if t, ok := f.glifo[c]; ok {
			b.WriteString(t)
		} else if t, ok := f.toUni[c]; ok && valido(t) {
			b.WriteString(semLigadura.Replace(t))
		} else {
			b.WriteRune('?')
		}
	}
	return b.String()
}

func (d *documento) carregarFonte(v any) *fonte {
	fd := d.dict(v)
	f := &fonte{glifo: map[uint16]string{}, toUni: map[uint16]string{}}
	if fd == nil {
		return f
	}
	if tu := fd["ToUnicode"]; tu != nil {
		if b, err := d.dados(tu); err == nil {
			lerToUnicode(b, f.toUni)
		}
	}
	if fd["Subtype"] != nome("Type0") {
		return f
	}
	f.doisBytes = true
	desc, _ := d.resolve(fd["DescendantFonts"]).([]any)
	if len(desc) == 0 {
		return f
	}
	cid := d.dict(desc[0])
	if cid == nil {
		return f
	}
	if m, ok := d.resolve(cid["CIDToGIDMap"]).(nome); cid["CIDToGIDMap"] != nil && (!ok || m != "Identity") {
		return f // mapa próprio: fica só com a ToUnicode
	}
	desc2 := d.dict(cid["FontDescriptor"])
	if desc2 == nil || desc2["FontFile2"] == nil {
		return f
	}
	b, err := d.dados(desc2["FontFile2"])
	if err != nil {
		return f
	}
	t := lerTTF(b)
	if t == nil {
		return f
	}
	for g, r := range t.glifos() {
		f.glifo[g] = semLigadura.Replace(string(r))
	}
	for lig, comp := range t.ligaduras() {
		if _, ja := f.glifo[lig]; ja {
			continue
		}
		var s strings.Builder
		ok := true
		for _, c := range comp {
			x, tem := f.glifo[c]
			if !tem {
				ok = false
				break
			}
			s.WriteString(x)
		}
		if ok {
			f.glifo[lig] = s.String()
		}
	}
	return f
}

var reBF = regexp.MustCompile(`(?s)beginbf(char|range)(.*?)endbf(char|range)`)

func lerToUnicode(b []byte, m map[uint16]string) {
	for _, bl := range reBF.FindAllSubmatch(b, -1) {
		p := &parser{b: bl[2]}
		var vals []any
		for {
			v, err := p.valor()
			if err != nil {
				break
			}
			vals = append(vals, v)
		}
		num := func(v any) (int, bool) {
			h, ok := v.(hexstr)
			if !ok {
				return 0, false
			}
			n := 0
			for _, x := range h {
				n = n<<8 | int(x)
			}
			return n, true
		}
		if string(bl[1]) == "char" {
			for i := 0; i+1 < len(vals); i += 2 {
				c, ok1 := num(vals[i])
				h, ok2 := vals[i+1].(hexstr)
				if ok1 && ok2 {
					m[uint16(c)] = utf16be(h)
				}
			}
			continue
		}
		for i := 0; i+2 < len(vals); i += 3 {
			a, ok1 := num(vals[i])
			z, ok2 := num(vals[i+1])
			if !ok1 || !ok2 || z < a || z-a > 0xFFFF {
				continue
			}
			switch dst := vals[i+2].(type) {
			case hexstr:
				base := []rune(utf16be(dst))
				if len(base) == 0 {
					continue
				}
				for c := a; c <= z; c++ {
					r := append([]rune{}, base...)
					r[len(r)-1] += rune(c - a)
					m[uint16(c)] = string(r)
				}
			case []any:
				for k, x := range dst {
					if h, ok := x.(hexstr); ok && a+k <= z {
						m[uint16(a+k)] = utf16be(h)
					}
				}
			}
		}
	}
}

func utf16be(h []byte) string {
	var r []rune
	for i := 0; i+1 < len(h); i += 2 {
		u := rune(h[i])<<8 | rune(h[i+1])
		if u >= 0xD800 && u < 0xDC00 && i+3 < len(h) {
			l := rune(h[i+2])<<8 | rune(h[i+3])
			r = append(r, 0x10000+(u-0xD800)<<10+(l-0xDC00))
			i += 2
			continue
		}
		r = append(r, u)
	}
	return string(r)
}

/* ------------------------------------------------------------- texto --- */

type pedaco struct {
	x, y, tam float64
	texto     string
}

// Linhas devolve o texto do PDF em linhas, de cima para baixo e da esquerda
// para a direita, como aparece impresso.
func Linhas(arquivo []byte) ([]string, error) {
	d, err := abrir(arquivo)
	if err != nil {
		return nil, err
	}
	// páginas na ordem da árvore /Pages
	var paginas []dict
	var visitar func(v any, prof int)
	visitar = func(v any, prof int) {
		n := d.dict(v)
		if n == nil || prof > 30 {
			return
		}
		if n["Type"] == nome("Page") {
			paginas = append(paginas, n)
			return
		}
		kids, _ := d.resolve(n["Kids"]).([]any)
		for _, k := range kids {
			visitar(k, prof+1)
		}
	}
	for _, o := range d.objs {
		if dd := d.dict(o); dd != nil && dd["Type"] == nome("Catalog") {
			visitar(dd["Pages"], 0)
			break
		}
	}
	if len(paginas) == 0 {
		nums := make([]int, 0, len(d.objs))
		for k := range d.objs {
			nums = append(nums, k)
		}
		sort.Ints(nums)
		for _, k := range nums {
			if dd := d.dict(d.objs[k]); dd != nil && dd["Type"] == nome("Page") {
				paginas = append(paginas, dd)
			}
		}
	}
	var out []string
	for _, pg := range paginas {
		out = append(out, d.linhasPagina(pg)...)
	}
	return out, nil
}

func (d *documento) linhasPagina(pg dict) []string {
	res := d.dict(pg["Resources"])
	fontes := map[string]*fonte{}
	if res != nil {
		if fd := d.dict(res["Font"]); fd != nil {
			for k, v := range fd {
				fontes[k] = d.carregarFonte(v)
			}
		}
	}
	var conteudo []byte
	switch c := d.resolve(pg["Contents"]).(type) {
	case []any:
		for _, x := range c {
			if b, err := d.dados(x); err == nil {
				conteudo = append(conteudo, b...)
				conteudo = append(conteudo, '\n')
			}
		}
	case *stream:
		conteudo, _ = d.dados(c)
	}
	pedacos := lerConteudo(conteudo, fontes)
	return montarLinhas(pedacos)
}

func lerConteudo(b []byte, fontes map[string]*fonte) []pedaco {
	p := &parser{b: b}
	var pilha []any
	var out []pedaco
	var f *fonte
	tam := 10.0
	lider := 0.0
	// matriz de texto simplificada: posição da linha e escala
	var lx, ly, esc float64 = 0, 0, 1
	num := func(i int) float64 {
		if i < 0 || i >= len(pilha) {
			return 0
		}
		v, _ := pilha[i].(float64)
		return v
	}
	n := 0 // tamanho da pilha antes do operador
	adiciona := func(t string) {
		if f == nil || t == "" {
			return
		}
		out = append(out, pedaco{x: lx, y: ly, tam: tam * esc, texto: t})
	}
	for {
		v, err := p.valor()
		if err != nil {
			break
		}
		op, ehOp := v.(string)
		if !ehOp || op == "]" || op == ">" || op == ")" {
			pilha = append(pilha, v)
			continue
		}
		n = len(pilha)
		switch op {
		case "BT":
			lx, ly, esc = 0, 0, 1
		case "Tf":
			if n >= 2 {
				if nm, ok := pilha[n-2].(nome); ok {
					f = fontes[string(nm)]
				}
				tam = num(n - 1)
			}
		case "Td":
			lx += num(n-2) * esc
			ly += num(n-1) * esc
		case "TD":
			lider = -num(n - 1)
			lx += num(n-2) * esc
			ly += num(n-1) * esc
		case "TL":
			lider = num(n - 1)
		case "Tm":
			if n >= 6 {
				esc = num(n - 3)
				if esc == 0 {
					esc = 1
				}
				lx, ly = num(n-2), num(n-1)
			}
		case "T*":
			ly -= lider * esc
		case "Tj", "'", "\"":
			if op != "Tj" {
				ly -= lider * esc
			}
			if n >= 1 {
				adiciona(textoDe(f, pilha[n-1]))
			}
		case "TJ":
			if n >= 1 {
				if a, ok := pilha[n-1].([]any); ok {
					var s strings.Builder
					for _, x := range a {
						if k, ok := x.(float64); ok {
							if k < -250 { // espaço grande entre palavras
								s.WriteByte(' ')
							}
							continue
						}
						s.WriteString(textoDe(f, x))
					}
					adiciona(s.String())
				}
			}
		}
		pilha = pilha[:0]
	}
	return out
}

func textoDe(f *fonte, v any) string {
	if f == nil {
		return ""
	}
	switch s := v.(type) {
	case hexstr:
		return f.texto(s)
	case []byte:
		return f.texto(s)
	}
	return ""
}

func montarLinhas(ps []pedaco) []string {
	sort.SliceStable(ps, func(i, j int) bool {
		if d := ps[i].y - ps[j].y; d > 1.5 || d < -1.5 {
			return ps[i].y > ps[j].y
		}
		return ps[i].x < ps[j].x
	})
	var out []string
	var atual []pedaco
	fecha := func() {
		if len(atual) == 0 {
			return
		}
		sort.SliceStable(atual, func(i, j int) bool { return atual[i].x < atual[j].x })
		var b strings.Builder
		for i, p := range atual {
			t := p.texto
			if i > 0 && !strings.HasSuffix(b.String(), " ") && !strings.HasPrefix(t, " ") {
				b.WriteByte(' ')
			}
			b.WriteString(t)
		}
		if s := strings.Join(strings.Fields(b.String()), " "); s != "" {
			out = append(out, s)
		}
		atual = atual[:0]
	}
	for _, p := range ps {
		if len(atual) > 0 {
			ref := atual[0].y
			if d := ref - p.y; d > 1.5 || d < -1.5 {
				fecha()
			}
		}
		atual = append(atual, p)
	}
	fecha()
	return out
}

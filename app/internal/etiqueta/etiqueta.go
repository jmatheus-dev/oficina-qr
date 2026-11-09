// Pacote etiqueta desenha as etiquetas (60 x 60 mm, 203 dpi) com o QR Code.
//
// Modelos (escolhidos sozinhos por ModeloPara):
//   - "oleo": igual à etiqueta de troca de óleo do BarTender (Data, KM,
//     PRÓXIMA MANUTENÇÃO com os 7 itens e o QR Code);
//   - "correia": correia dentada trocada (data, KM, próxima troca, correia,
//     tensor e bomba d'água);
//   - "oleo-correia": as duas numa etiqueta só;
//   - "servicos": lista o que foi feito (não usado pela oficina hoje).
//
// As medidas são em milímetros e viram pontos da impressora no fim.
package etiqueta

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"sync"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fontes/LiberationSans-Regular.ttf
var ttfNormal []byte

//go:embed fontes/LiberationSans-Bold.ttf
var ttfNegrito []byte

// Itens na ordem da etiqueta, com o rótulo impresso.
var Itens = []string{"Óleo do motor", "Filtro de óleo", "Filtro de ar", "Filtro de combustível", "Filtro de cabine", "Óleo de freio", "Óleo de câmbio"}
var rotulos = []string{"Óleo do Motor:", "Filtro de Óleo:", "Filtro de Ar:", "Filtro de Comb.:", "Filtro de Cabine:", "Óleo de Freio:", "Óleo de Câmbio:"}

// Config da etiqueta (vem das Configurações do programa).
type Config struct {
	LarguraMM float64 `json:"largura_mm"`
	AlturaMM  float64 `json:"altura_mm"`
	DPI       int     `json:"dpi"`
	AjusteXMM float64 `json:"ajuste_x_mm"` // desloca o desenho se sair torto
	AjusteYMM float64 `json:"ajuste_y_mm"`
	Titulo    string  `json:"titulo"`
	Instagram string  `json:"instagram"`
	Telefone  string  `json:"telefone"`
	Site      string  `json:"site"`
	// correia dentada: próxima troca em KM + CorreiaKM e data + CorreiaMeses
	CorreiaKM    int `json:"correia_km"`
	CorreiaMeses int `json:"correia_meses"`
	// etiqueta de óleo no modelo antigo (igual ao BarTender, sem a data da próxima troca)
	OleoModeloAntigo bool `json:"oleo_modelo_antigo"`
}

func ConfigPadrao() Config {
	return Config{LarguraMM: 60, AlturaMM: 60, DPI: 203, Titulo: "AUTO MECÂNICA ITABAIANA",
		Instagram: "automec135", Telefone: "(79) 9851-3926", Site: "https://automec135.github.io/",
		CorreiaKM: 40000, CorreiaMeses: 12}
}

// Dados que vão na etiqueta.
type Dados struct {
	Codigo    string
	Data      string // AAAA-MM-DD
	KM        int
	ProximaKM int
	Itens     []string
	Outros    []string
	Modelo    string
	Placa     string // já formatada
	// data da próxima troca de óleo (AAAA-MM-DD; vazio = não mostra)
	ProximaData string
	// correia dentada (detectada pelo nome das peças)
	Correia            Correia
	ProximaCorreiaKM   int
	ProximaCorreiaData string
}

// Link que o QR Code abre.
func Link(site, codigo string) string {
	if site == "" {
		site = "https://automec135.github.io/"
	}
	return strings.TrimRight(site, "/") + "/?c=" + codigo
}

// Milhar formata 160273 como 160.273.
func Milhar(n int) string {
	s := fmt.Sprint(n)
	var out []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, s[i])
	}
	return string(out)
}

func dataBR(iso string) string {
	if len(iso) == 10 && iso[4] == '-' && iso[7] == '-' {
		return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
	}
	return iso
}

/* ------------------------------------------------------------- fontes --- */

var (
	fontesUmaVez sync.Once
	fNormal      *opentype.Font
	fNegrito     *opentype.Font
	faces        sync.Map // chave -> font.Face
)

func carregarFontes() {
	var err error
	if fNormal, err = opentype.Parse(ttfNormal); err != nil {
		panic(err)
	}
	if fNegrito, err = opentype.Parse(ttfNegrito); err != nil {
		panic(err)
	}
}

/* -------------------------------------------------------------- tela --- */

type tela struct {
	cfg    Config
	k      float64 // pontos por mm
	img    *image.Gray
	dx, dy float64
}

func novaTela(cfg Config) *tela {
	fontesUmaVez.Do(carregarFontes)
	k := float64(cfg.DPI) / 25.4
	w := int(math.Round(cfg.LarguraMM * k))
	h := int(math.Round(cfg.AlturaMM * k))
	img := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)
	return &tela{cfg: cfg, k: k, img: img, dx: cfg.AjusteXMM, dy: cfg.AjusteYMM}
}

func (t *tela) px(mm float64) int { return int(math.Round(mm * t.k)) }
func (t *tela) x(mm float64) int  { return int(math.Round((mm + t.dx) * t.k)) }
func (t *tela) y(mm float64) int  { return int(math.Round((mm + t.dy) * t.k)) }

// letra com altura das maiúsculas em mm
type letra struct {
	face  font.Face
	capMM float64
}

func (t *tela) fonte(capMM float64, negrito bool) letra {
	// Liberation Sans/Arial: maiúscula = 0,716 do tamanho
	tamPx := capMM * t.k / 0.716
	tamPx = math.Round(tamPx*4) / 4
	chave := fmt.Sprint(negrito, tamPx)
	if f, ok := faces.Load(chave); ok {
		return letra{f.(font.Face), capMM}
	}
	ft := fNormal
	if negrito {
		ft = fNegrito
	}
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: tamPx, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	faces.Store(chave, face)
	return letra{face, capMM}
}

func (t *tela) largura(texto string, l letra) float64 {
	return float64(font.MeasureString(l.face, texto)) / 64 / t.k
}

// caber diminui a letra até o texto caber na largura.
func (t *tela) caber(texto string, capMM, largMax float64, negrito bool) letra {
	c := capMM
	for c > 1.2 && t.largura(texto, t.fonte(c, negrito)) > largMax {
		c -= 0.05
	}
	return t.fonte(c, negrito)
}

const (
	esq = iota
	centro
	dir
)

// texto escreve com o topo das maiúsculas em topoMM. Devolve a largura em mm.
func (t *tela) texto(xMM, topoMM float64, s string, l letra, alinhar int, branco bool) float64 {
	larg := t.largura(s, l)
	switch alinhar {
	case centro:
		xMM -= larg / 2
	case dir:
		xMM -= larg
	}
	var cor image.Image = image.Black
	if branco {
		cor = image.White
	}
	d := &font.Drawer{Dst: t.img, Src: cor, Face: l.face,
		Dot: fixed.Point26_6{X: fixed.I(t.x(xMM)), Y: fixed.I(t.y(topoMM + l.capMM))}}
	d.DrawString(s)
	return larg
}

func (t *tela) retangulo(x1, y1, x2, y2 float64, cor color.Gray) {
	r := image.Rect(t.x(x1), t.y(y1), t.x(x2), t.y(y2))
	draw.Draw(t.img, r, &image.Uniform{cor}, image.Point{}, draw.Src)
}

func (t *tela) linhaH(x1, x2, y, esp float64) {
	e := t.px(esp)
	if e < 1 {
		e = 1
	}
	y0 := t.y(y) - e/2
	r := image.Rect(t.x(x1), y0, t.x(x2), y0+e)
	draw.Draw(t.img, r, image.Black, image.Point{}, draw.Src)
}

func (t *tela) moldura(x1, y1, x2, y2, esp float64) {
	t.linhaH(x1, x2, y1, esp)
	t.linhaH(x1, x2, y2, esp)
	e := t.px(esp)
	if e < 1 {
		e = 1
	}
	for _, xx := range []float64{x1, x2} {
		x0 := t.x(xx) - e/2
		draw.Draw(t.img, image.Rect(x0, t.y(y1), x0+e, t.y(y2)), image.Black, image.Point{}, draw.Src)
	}
}

func (t *tela) circulo(cx, cy, r, esp float64, cheio bool) {
	pcx, pcy := (cx+t.dx)*t.k, (cy+t.dy)*t.k
	pr, pe := r*t.k, esp*t.k
	b := image.Rect(int(pcx-pr-2), int(pcy-pr-2), int(pcx+pr+3), int(pcy+pr+3))
	for yy := b.Min.Y; yy < b.Max.Y; yy++ {
		for xx := b.Min.X; xx < b.Max.X; xx++ {
			d := math.Hypot(float64(xx)+0.5-pcx, float64(yy)+0.5-pcy)
			if d <= pr && (cheio || d >= pr-pe) {
				t.img.SetGray(xx, yy, color.Gray{0})
			}
		}
	}
}

// retângulo de cantos arredondados (contorno)
func (t *tela) arredondado(x1, y1, x2, y2, raio, esp float64) {
	X1, Y1, X2, Y2 := (x1+t.dx)*t.k, (y1+t.dy)*t.k, (x2+t.dx)*t.k, (y2+t.dy)*t.k
	R, E := raio*t.k, esp*t.k
	dentro := func(px, py, enc float64) bool {
		ax, ay := X1+enc, Y1+enc
		bx, by := X2-enc, Y2-enc
		r := math.Max(R-enc, 0)
		if px < ax || px > bx || py < ay || py > by {
			return false
		}
		cx := math.Min(math.Max(px, ax+r), bx-r)
		cy := math.Min(math.Max(py, ay+r), by-r)
		return math.Hypot(px-cx, py-cy) <= r+0.0001
	}
	for yy := int(Y1) - 1; yy <= int(Y2)+1; yy++ {
		for xx := int(X1) - 1; xx <= int(X2)+1; xx++ {
			px, py := float64(xx)+0.5, float64(yy)+0.5
			if dentro(px, py, 0) && !dentro(px, py, E) {
				t.img.SetGray(xx, yy, color.Gray{0})
			}
		}
	}
}

// QR com módulos de tamanho inteiro (pontos da impressora), sem borrar.
func (t *tela) qr(texto string, xMM, yMM, ladoMax float64) float64 {
	q, err := qrcode.New(texto, qrcode.Medium)
	if err != nil {
		panic(err)
	}
	q.DisableBorder = true
	m := q.Bitmap()
	n := len(m)
	mod := int(ladoMax*t.k) / n
	if mod < 1 {
		mod = 1
	}
	x0, y0 := t.x(xMM), t.y(yMM)
	for yy := 0; yy < n; yy++ {
		for xx := 0; xx < n; xx++ {
			if m[yy][xx] {
				r := image.Rect(x0+xx*mod, y0+yy*mod, x0+(xx+1)*mod, y0+(yy+1)*mod)
				draw.Draw(t.img, r, image.Black, image.Point{}, draw.Src)
			}
		}
	}
	return float64(mod*n) / t.k
}

/* ------------------------------------------------------------- ícones --- */

func (t *tela) iconeInstagram(x, y, lado float64) {
	esp := lado * 0.13
	t.arredondado(x, y, x+lado, y+lado, lado*0.3, esp)
	t.circulo(x+lado/2, y+lado/2, lado*0.24, esp, false)
	t.circulo(x+lado*0.76, y+lado*0.24, lado*0.07, 0, true)
}

func (t *tela) iconeWhatsapp(x, y, lado float64) {
	r := lado / 2
	cx, cy := x+r, y+r
	esp := lado * 0.12
	t.circulo(cx, cy, r, esp, false)
	// "rabinho" do balão, embaixo à esquerda
	for i := 0.0; i <= 1; i += 0.02 {
		px := cx - r*0.62 - i*r*0.32
		py := cy + r*0.62 + i*r*0.32
		t.circulo(px, py, esp*0.65, 0, true)
	}
	// telefone no meio (fone simplificado)
	t.circulo(cx-r*0.22, cy-r*0.2, r*0.16, 0, true)
	t.circulo(cx+r*0.2, cy+r*0.24, r*0.16, 0, true)
	for i := 0.0; i <= 1; i += 0.05 {
		t.circulo(cx-r*0.22+i*r*0.42, cy-r*0.2+i*r*0.44, r*0.08, 0, true)
	}
}

/* ------------------------------------------------------------ partes --- */

func (t *tela) cabecalho(d Dados) float64 {
	W := t.cfg.LarguraMM
	titulo := t.cfg.Titulo
	f := t.caber(titulo, 2.75, W-6, true)
	larg := t.texto(W/2, 1.9, titulo, f, centro, false)
	t.linhaH(W/2-larg/2, W/2+larg/2, 1.9+f.capMM+0.65, 0.3)

	// modelo e placa do carro (ex.: "COROLLA XEI 2.0 · REK-8G05")
	if carro := linhaCarro(d); carro != "" {
		t.texto(W/2, 6.15, carro, t.caber(carro, 2.25, W-5, true), centro, false)
	}

	rot := t.fonte(2.4, false)
	y := 9.9
	for _, par := range [][2]string{{"Data:", dataBR(d.Data)}, {"KM:", kmOuVazio(d.KM)}} {
		t.texto(17.5, y, par[0], rot, dir, false)
		t.linhaH(19, 50, y+2.9, 0.25)
		if par[1] != "" {
			t.texto(34.5, y-0.05, par[1], t.caber(par[1], 2.5, 30, true), centro, false)
		}
		y += 3.85
	}
	return y
}

// linhaCarro junta modelo e placa numa linha só.
func linhaCarro(d Dados) string {
	var p []string
	if m := strings.TrimSpace(d.Modelo); m != "" {
		p = append(p, strings.ToUpper(m))
	}
	if pl := strings.TrimSpace(d.Placa); pl != "" {
		p = append(p, pl)
	}
	return strings.Join(p, " · ")
}

func kmOuVazio(km int) string {
	if km <= 0 {
		return ""
	}
	return Milhar(km)
}

func (t *tela) faixa(y1, y2 float64, s string) {
	W := t.cfg.LarguraMM
	t.retangulo(1.8, y1, W-1.8, y2, color.Gray{0})
	f := t.caber(s, (y2-y1)*0.56, W-8, false)
	t.texto(W/2, y1+((y2-y1)-f.capMM)/2, s, f, centro, true)
}

func (t *tela) rodape(xMax float64) {
	H := t.cfg.AlturaMM
	lado := 2.9
	f := t.caber(t.cfg.Instagram, 2.25, xMax-2.4-lado-1, false)
	t.iconeInstagram(2.4, H-8.15, lado)
	t.texto(2.4+lado+0.9, H-8.15+(lado-f.capMM)/2, t.cfg.Instagram, f, esq, false)
	f2 := t.caber(t.cfg.Telefone, 2.25, xMax-2.4-lado-1, false)
	t.iconeWhatsapp(2.4, H-4.45, lado)
	t.texto(2.4+lado+0.9, H-4.45+(lado-f2.capMM)/2, t.cfg.Telefone, f2, esq, false)
}

func (t *tela) chamada(cx, topo, largMax float64, linhas []string) {
	y := topo
	for _, s := range linhas {
		t.texto(cx, y, s, t.caber(s, 2.05, largMax, true), centro, false)
		y += 3.0
	}
}

/* ----------------------------------------------------------- modelos --- */

// Oleo desenha a etiqueta de troca de óleo (igual à do BarTender).
func Oleo(d Dados, cfg Config) *image.Gray {
	t := novaTela(cfg)
	W, H := cfg.LarguraMM, cfg.AlturaMM
	t.cabecalho(d)
	t.faixa(17.45, 21.3, "PRÓXIMA MANUTENÇÃO")

	trocados := map[string]bool{}
	for _, it := range d.Itens {
		trocados[it] = true
	}
	rot := t.fonte(2.15, false)
	val := t.fonte(2.4, true)
	y0, passo := 22.8, 3.8
	xQR := W - 1.8 - 15.0
	for i, it := range Itens {
		y := y0 + float64(i)*passo
		v := "NÃO"
		if trocados[it] {
			v = "SIM"
		}
		if i == 0 && d.ProximaKM > 0 {
			v = Milhar(d.ProximaKM)
		}
		fimLinha := W - 2.2
		if y+3 > 33.6 { // linhas ao lado do QR são mais curtas
			fimLinha = xQR - 1.6
		}
		t.texto(2.4, y, rotulos[i], rot, esq, false)
		t.linhaH(24.6, fimLinha, y+2.85, 0.25)
		t.texto((24.6+math.Min(fimLinha, 41))/2, y-0.05, v, val, centro, false)
	}
	lado := t.qr(Link(cfg.Site, d.Codigo), xQR, 34.2, 15.0)
	_ = lado
	t.chamada(xQR+7.5-1.6, H-7.6, W-xQR+1.4, []string{"Escaneie e veja", "os serviços"})
	t.rodape(xQR - 4)
	return t.img
}

// quebrar em linhas que caibam na largura
func (t *tela) quebrar(s string, l letra, largMM float64) []string {
	var linhas []string
	atual := ""
	for _, p := range strings.Fields(s) {
		teste := strings.TrimSpace(atual + " " + p)
		if atual == "" || t.largura(teste, l) <= largMM {
			atual = teste
		} else {
			linhas = append(linhas, atual)
			atual = p
		}
	}
	if atual != "" {
		linhas = append(linhas, atual)
	}
	return linhas
}

// Servicos desenha a etiqueta com a lista do que foi feito.
func Servicos(d Dados, cfg Config) *image.Gray {
	t := novaTela(cfg)
	W, H := cfg.LarguraMM, cfg.AlturaMM
	y := t.cabecalho(d)
	sub := strings.TrimSpace(strings.Join(naoVazios(strings.ToUpper(d.Modelo), d.Placa), " · "))
	if sub != "" {
		t.texto(W/2, y-0.4, sub, t.caber(sub, 1.95, W-6, false), centro, false)
		y += 2.8
	}
	topoFaixa := y - 0.1
	t.faixa(topoFaixa, topoFaixa+4.2, "SERVIÇOS REALIZADOS")

	xQR := W - 1.8 - 15.0
	topoQR := topoFaixa + 5.6
	t.qr(Link(cfg.Site, d.Codigo), xQR, topoQR, 15.0)
	t.chamada(xQR+7.5-1.6, H-7.6, W-xQR+1.4, []string{"Escaneie e veja", "o histórico"})

	largLista := xQR - 2.4 - 1.4
	l := t.fonte(2.25, true)
	var entradas []string
	entradas = append(entradas, d.Itens...)
	entradas = append(entradas, d.Outros...)
	if len(entradas) == 0 && d.ProximaKM > 0 {
		entradas = []string{"Revisão"}
	}
	limite := H - 9.2 // acima do rodapé
	proxLinha := ""
	if d.ProximaKM > 0 {
		proxLinha = "Próx. troca: " + Milhar(d.ProximaKM) + " km"
		limite -= 3.3
	}
	type lin struct {
		marca bool
		txt   string
	}
	var ls []lin
	for _, e := range entradas {
		for k, parte := range t.quebrar(e, l, largLista-2.2) {
			ls = append(ls, lin{k == 0, parte})
		}
	}
	yy, passo := topoFaixa+5.8, 3.25
	cabe := int((limite-yy)/passo) + 1
	if len(ls) > cabe {
		resto := len(ls) - (cabe - 1)
		ls = append(ls[:cabe-1], lin{false, fmt.Sprintf("+ %d no QR Code", resto)})
	}
	for _, x := range ls {
		if x.marca {
			t.circulo(3.0, yy+l.capMM/2, 0.45, 0, true)
		}
		lt := l
		if strings.HasPrefix(x.txt, "+ ") && !x.marca {
			lt = t.fonte(1.9, false)
		}
		t.texto(4.4, yy, x.txt, lt, esq, false)
		yy += passo
	}
	if proxLinha != "" {
		t.texto(2.4, limite+0.5, proxLinha, t.caber(proxLinha, 2.2, largLista, true), esq, false)
	}
	t.rodape(xQR - 4)
	return t.img
}

func naoVazios(s ...string) []string {
	var out []string
	for _, x := range s {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}

// ModeloFixo era a etiqueta única de troca de óleo (até a versão 1.3.1).
const ModeloFixo = ModeloOleo

// Desenhar o modelo pedido ("oleo" ou "servicos").
func Desenhar(modelo string, d Dados, cfg Config) (*image.Gray, error) {
	if cfg.DPI <= 0 {
		cfg = ConfigPadrao()
	}
	switch modelo {
	case "oleo":
		if cfg.OleoModeloAntigo {
			return PretoEBranco(Oleo(d, cfg)), nil
		}
		return PretoEBranco(OleoNovo(d, cfg)), nil
	case "servicos":
		return PretoEBranco(Servicos(d, cfg)), nil
	case ModeloCorreia:
		return PretoEBranco(CorreiaDentada(d, cfg)), nil
	case ModeloOleoECorreia:
		return PretoEBranco(OleoECorreia(d, cfg)), nil
	}
	return nil, fmt.Errorf("modelo de etiqueta desconhecido: %q", modelo)
}

// PretoEBranco: impressora térmica não tem cinza.
func PretoEBranco(img *image.Gray) *image.Gray {
	for i, v := range img.Pix {
		if v < 140 {
			img.Pix[i] = 0
		} else {
			img.Pix[i] = 255
		}
	}
	return img
}

package etiqueta

import (
	"image"
	"image/color"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Correia diz o que foi trocado na correia dentada (detectado sozinho pelo
// nome das peças/serviços).
type Correia struct {
	Correia bool `json:"correia"`
	Tensor  bool `json:"tensor"`
	Bomba   bool `json:"bomba"`
}

var (
	reCorreiaDentada = regexp.MustCompile(`CORREIA\s+(DENTADA|DENT\b|SINCRON|DO\s+COMANDO|DE\s+COMANDO|COMANDO|BANHADA|DISTRIBUI)|KIT\s+(DE\s+)?(CORREIA|DISTRIBUI)|KIT\s+DENTADA|^\s*CORREIA\s*$|^\s*TROCA\s+(DA\s+|DE\s+)?CORREIA\s*$`)
	reKit            = regexp.MustCompile(`\bKIT\b`)
	reTensor         = regexp.MustCompile(`TENSOR|TENSIONADOR`)
	reBomba          = regexp.MustCompile(`BOMBA\s+(D\s*'?\s*AGUA|DE\s+AGUA|DAGUA)`)
	// correias e tensores que NÃO são da correia dentada
	reOutraCorreia = regexp.MustCompile(`ALTERNADOR|POLI|MICRO\s*-?\s*V|ACESSORIO|AUXILIAR|AR\s+CONDICIONADO|\bA/?C\b|DIRECAO|COMPRESSOR|\bV\s*\d|\b\d+PK\d|\bPK\b`)
	reOutraBomba   = regexp.MustCompile(`LIMPADOR|PARA[\s-]?BRISA|LAVADOR|ESGUICHO|RESERVATORIO`)
)

func normal(s string) string {
	var r strings.Builder
	for _, ch := range norm.NFD.String(strings.ToUpper(s)) {
		if unicode.Is(unicode.Mn, ch) {
			continue
		}
		r.WriteRune(ch)
	}
	return strings.NewReplacer("’", "'", "`", "'", ".", " ").Replace(r.String())
}

// DetectarCorreia procura correia dentada, tensor e bomba d'água no que foi feito.
func DetectarCorreia(itens, outros []string) Correia {
	var c Correia
	for _, s := range append(append([]string{}, itens...), outros...) {
		d := normal(s)
		if reCorreiaDentada.MatchString(d) && !reOutraCorreia.MatchString(d) {
			c.Correia = true
			if reKit.MatchString(d) { // kit de correia dentada já vem com o tensor
				c.Tensor = true
			}
		}
		if reTensor.MatchString(d) && !reOutraCorreia.MatchString(d) {
			c.Tensor = true
		}
		if reBomba.MatchString(d) && !reOutraBomba.MatchString(d) {
			c.Bomba = true
		}
	}
	return c
}

// Modelos de etiqueta.
const (
	ModeloOleo         = "oleo"
	ModeloCorreia      = "correia"
	ModeloOleoECorreia = "oleo-correia"
)

// ModeloPara escolhe a etiqueta sozinho:
//   - só troca de óleo (ou outros itens) → etiqueta de troca de óleo;
//   - só correia dentada → etiqueta da correia dentada;
//   - as duas coisas → uma etiqueta só com as duas.
func ModeloPara(d Dados) string {
	c := d.Correia
	if !c.Correia {
		return ModeloOleo
	}
	if len(d.Itens) > 0 || d.ProximaKM > 0 {
		return ModeloOleoECorreia
	}
	return ModeloCorreia
}

// NomeModelo para mostrar na tela.
func NomeModelo(m string) string {
	switch m {
	case ModeloCorreia:
		return "Correia dentada"
	case ModeloOleoECorreia:
		return "Troca de óleo + correia dentada"
	}
	return "Troca de óleo"
}

// ProximaCorreia: o que veio no serviço ou o padrão (KM + intervalo, data + meses).
func ProximaCorreia(d Dados, cfg Config) (km int, data string) {
	km, data = d.ProximaCorreiaKM, d.ProximaCorreiaData
	intKM, meses := cfg.CorreiaKM, cfg.CorreiaMeses
	if intKM <= 0 {
		intKM = 40000
	}
	if meses <= 0 {
		meses = 12
	}
	if km <= 0 && d.KM > 0 {
		km = d.KM + intKM
	}
	if data == "" {
		if t, err := time.Parse("2006-01-02", d.Data); err == nil {
			data = t.AddDate(0, meses, 0).Format("2006-01-02")
		}
	}
	return
}

/* --------------------------------------------------------- desenho --- */

// faixa preta com texto branco em negrito, de x1 a x2
func (t *tela) faixaB(x1, x2, y1, y2 float64, s string, capMax float64) {
	t.retangulo(x1, y1, x2, y2, color.Gray{0})
	f := t.caber(s, math.Min((y2-y1)*0.62, capMax), x2-x1-2.4, true)
	t.texto((x1+x2)/2, y1+((y2-y1)-f.capMM)/2, s, f, centro, true)
}

// traço grosso (para o "✓")
func (t *tela) traco(x1, y1, x2, y2, esp float64, cor color.Gray) {
	n := int(math.Hypot(x2-x1, y2-y1)*t.k) + 1
	r := esp * t.k / 2
	for i := 0; i <= n; i++ {
		f := float64(i) / float64(n)
		cx, cy := (x1+(x2-x1)*f+t.dx)*t.k, (y1+(y2-y1)*f+t.dy)*t.k
		for yy := int(cy - r - 1); yy <= int(cy+r+1); yy++ {
			for xx := int(cx - r - 1); xx <= int(cx+r+1); xx++ {
				if math.Hypot(float64(xx)+0.5-cx, float64(yy)+0.5-cy) <= r {
					if image.Pt(xx, yy).In(t.img.Bounds()) {
						t.img.SetGray(xx, yy, cor)
					}
				}
			}
		}
	}
}

// caixinha: marcada = quadrado preto com "✓" branco; vazia = só o contorno
func (t *tela) caixinha(x, y, lado float64, marcada bool) {
	if !marcada {
		t.moldura(x, y, x+lado, y+lado, 0.3)
		return
	}
	t.retangulo(x, y, x+lado, y+lado, color.Gray{0})
	e := lado * 0.13
	t.traco(x+lado*0.24, y+lado*0.52, x+lado*0.42, y+lado*0.72, e, color.Gray{255})
	t.traco(x+lado*0.42, y+lado*0.72, x+lado*0.78, y+lado*0.26, e, color.Gray{255})
}

// linha "DATA ______ 05/02/2026"
func (t *tela) campo(x, y, xFim float64, rotulo, valor string, capRot, capVal float64) {
	f := t.fonte(capRot, false)
	larg := t.texto(x, y, rotulo, f, esq, false)
	ini := x + larg + 0.5
	t.linhaH(ini, xFim, y+capRot+0.35, 0.25)
	if valor != "" {
		t.texto((ini+xFim)/2+1, y-(capVal-capRot)-0.15, valor, t.caber(valor, capVal, xFim-ini-1, true), centro, false)
	}
}

// CorreiaDentada desenha a etiqueta da correia dentada (igual à da foto).
func CorreiaDentada(d Dados, cfg Config) *image.Gray {
	t := novaTela(cfg)
	W, H := cfg.LarguraMM, cfg.AlturaMM
	t.faixaB(1.8, W-1.8, 1.5, 6.6, "CORREIA DENTADA TROCADA", 3.4)
	y := 9.4
	if carro := linhaCarro(d); carro != "" {
		t.texto(W/2, 7.5, carro, t.caber(carro, 1.9, W-5, true), centro, false)
		y = 11.2
	}
	t.campo(2.6, y, W-3, "DATA", dataBR(d.Data), 2.3, 2.7)
	t.campo(2.6, y+4.0, W-3, "KM DA TROCA", kmOuVazio(d.KM), 2.3, 2.7)

	t.faixaB(3.5, W-3.5, y+8.0, y+12.0, "PRÓXIMA TROCA", 2.8)
	km, data := ProximaCorreia(d, cfg)
	t.campo(9, y+13.4, W-6, "DATA", dataBR(data), 2.2, 2.6)
	t.campo(9, y+17.2, W-6, "KM DA TROCA", kmOuVazio(km), 2.2, 2.6)

	xQR := W - 1.8 - 15.0
	topo := y + 21.3
	lado, passo := 3.4, 4.35
	rot := t.fonte(2.2, false)
	for i, it := range []struct {
		nome string
		ok   bool
	}{{"CORREIA", d.Correia.Correia}, {"TENSOR", d.Correia.Tensor}, {"BOMBA D'ÁGUA", d.Correia.Bomba}} {
		yy := topo + float64(i)*passo
		t.caixinha(5, yy, lado, it.ok)
		t.texto(10.2, yy+(lado-rot.capMM)/2, it.nome, rot, esq, false)
	}
	qrTopo := math.Min(topo-0.4, H-9.3-15)
	t.qr(Link(cfg.Site, d.Codigo), xQR, qrTopo, 15.0)
	t.chamada(xQR+7.5-1.6, H-7.6, W-xQR+1.4, []string{"Escaneie e veja", "os serviços"})
	t.rodape(xQR - 4)
	return t.img
}

// textoProxima: "160.273 km ou 14/05/2027" (o que tiver).
func textoProxima(km int, data string) string {
	switch {
	case km > 0 && data != "":
		return Milhar(km) + " km  ou  " + dataBR(data)
	case km > 0:
		return Milhar(km) + " km"
	case data != "":
		return "até " + dataBR(data)
	}
	return "—"
}

var rotulosCurtos = []string{"Óleo do motor", "Filtro de óleo", "Filtro de ar", "Filtro de comb.", "Filtro de cabine", "Óleo de freio", "Óleo de câmbio"}

// OleoNovo: etiqueta de troca de óleo com a próxima troca por KM e por data
// e, separado, o que foi trocado hoje (✓).
func OleoNovo(d Dados, cfg Config) *image.Gray {
	t := novaTela(cfg)
	W, H := cfg.LarguraMM, cfg.AlturaMM
	t.cabecalho(d)
	t.faixaB(1.8, W-1.8, 17.4, 21.2, "PRÓXIMA TROCA DE ÓLEO", 2.4)
	prox := textoProxima(d.ProximaKM, d.ProximaData)
	t.texto(W/2, 22.3, prox, t.caber(prox, 2.8, W-5, true), centro, false)

	xQR := W - 1.8 - 15.0
	t.texto(2.4, 26.2, "TROCADO HOJE:", t.fonte(1.7, true), esq, false)
	trocados := map[string]bool{}
	for _, it := range d.Itens {
		trocados[it] = true
	}
	rot := t.fonte(1.95, false)
	lado := 2.4
	for i, it := range Itens {
		yy := 28.6 + float64(i)*3.12
		t.caixinha(2.4, yy, lado, trocados[it])
		t.texto(2.4+lado+1.0, yy+(lado-rot.capMM)/2, rotulosCurtos[i], rot, esq, false)
	}
	t.qr(Link(cfg.Site, d.Codigo), xQR, 26.0, 15.0)
	t.chamada(xQR+7.5-1.6, H-7.6, W-xQR+1.4, []string{"Escaneie e veja", "os serviços"})
	t.rodape(xQR - 4)
	return t.img
}

// OleoECorreia: troca de óleo e correia dentada numa etiqueta só.
func OleoECorreia(d Dados, cfg Config) *image.Gray {
	t := novaTela(cfg)
	W, H := cfg.LarguraMM, cfg.AlturaMM
	titulo := t.cfg.Titulo
	f := t.caber(titulo, 2.4, W-6, true)
	larg := t.texto(W/2, 1.3, titulo, f, centro, false)
	t.linhaH(W/2-larg/2, W/2+larg/2, 1.3+f.capMM+0.55, 0.3)
	y := 5.0
	if carro := linhaCarro(d); carro != "" {
		t.texto(W/2, 4.9, carro, t.caber(carro, 1.9, W-5, true), centro, false)
		y = 8.0
	}
	duasColunas := func(y float64, data string, km int) {
		t.campo(2.4, y, W/2-1, "Data:", dataBR(data), 2.0, 2.2)
		t.campo(W/2+1, y, W-2.4, "KM:", kmOuVazio(km), 2.0, 2.2)
	}
	duasColunas(y, d.Data, d.KM)

	// troca de óleo: próxima troca + o que foi trocado hoje
	y += 3.4
	t.faixaB(1.8, W-1.8, y, y+3.3, "PRÓXIMA TROCA DE ÓLEO", 2.1)
	xQR := W - 1.8 - 14.6
	topoQR := y + 4.3
	prox := textoProxima(d.ProximaKM, d.ProximaData)
	t.texto((2.4+xQR-1.2)/2, y+4.3, prox, t.caber(prox, 2.3, xQR-1.2-2.4, true), centro, false)
	t.texto(2.4, y+8.0, "Trocado hoje:", t.fonte(1.5, true), esq, false)
	trocados := map[string]bool{}
	for _, it := range d.Itens {
		trocados[it] = true
	}
	rot := t.fonte(1.6, false)
	lado := 2.1
	for i, it := range Itens {
		col, lin := i/4, i%4
		x := 2.4 + float64(col)*20.0
		yy := y + 10.2 + float64(lin)*2.75
		t.caixinha(x, yy, lado, trocados[it])
		t.texto(x+lado+0.7, yy+(lado-rot.capMM)/2, rotulosCurtos[i], rot, esq, false)
	}
	t.qr(Link(cfg.Site, d.Codigo), xQR, topoQR, 15.0)

	// correia dentada
	y = y + 10.2 + 4*2.75 + 0.6
	t.faixaB(1.8, W-1.8, y, y+3.3, "CORREIA DENTADA · PRÓXIMA TROCA", 2.1)
	km, data := ProximaCorreia(d, cfg)
	duasColunas(y+4.3, data, km)
	yy := y + 8.2
	lado = 2.6
	rot = t.fonte(1.8, false)
	x := 2.4
	for _, it := range []struct {
		nome string
		ok   bool
	}{{"Correia", d.Correia.Correia}, {"Tensor", d.Correia.Tensor}, {"Bomba d'água", d.Correia.Bomba}} {
		t.caixinha(x, yy, lado, it.ok)
		x += lado + 0.8 + t.texto(x+lado+0.8, yy+(lado-rot.capMM)/2, it.nome, rot, esq, false) + 3.2
	}
	t.chamada(W-14.5, H-7.6, 26, []string{"Escaneie e veja", "os serviços"})
	t.rodape(W - 30)
	return t.img
}

// Pacote itens reconhece, pela descrição do produto no Zeus, se é um dos itens
// padrão da troca de óleo (óleo do motor, filtros...) ou outra peça/serviço.
package itens

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type regra struct {
	item  string
	todas []*regexp.Regexp // todas precisam bater
	nenhu []*regexp.Regexp // nenhuma pode bater
}

func re(s string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + s) }

// A ordem importa: a primeira regra que bate decide.
var regras = []regra{
	{"Filtro de cabine", []*regexp.Regexp{reFiltro, re(`CABINE|CABINA|AR[\s.]*CONDICIONADO|\bA/?C\b|\bACD\b|\bAKX\s*\d|ANTI[\s-]?POLEN|POLEN|\bACP\s*\d|CARVAO`)}, []*regexp.Regexp{re(`SECADOR|COMPRESS|VALVULA|EXPANS`)}},
	{"Filtro de óleo", []*regexp.Regexp{reFiltro, re(`\bOLEO\b|\bOL\b|\bOLE\b|LUBRIF|\bLUB\b|\bLB\s*\d|\bLBM\s*\d|\bLE\s*\d{2,}|\bPSL\s*\d|\bPEL\s*\d|\bPH\s*\d|\bW\s*\d{3}|\bOC\s*\d|\bWOE?\s*\d|\bW0\d|\bLF\s*\d`)}, []*regexp.Regexp{re(`CAMBIO|TRANSMISS|HIDRAUL|DIRECAO|\bCOMB\b|COMBUST|\bGI\s*\d`)}},
	{"Filtro de combustível", []*regexp.Regexp{reFiltro, re(`COMBUST|COMBUSIT|COMUST|COMBST|\bCOMB\b|GASOLINA|DIESEL|ETANOL|ALCOOL|\bGI\s*\d|\bPEC\s*\d|\bPSC\s*\d|\bWK\s*\d|\bFCI\s*\d|\bG\s*\d{2,}`)}, nil},
	{"Filtro de ar", []*regexp.Regexp{reFiltro, re(`\bAR\b|\bARL\s*\d|\bARS\s*\d|\bART\s*\d|\bWR\s*\d|\bHLT\s*\d|\bAP\s*\d|\bCA\s*\d|MOTOR`)}, []*regexp.Regexp{re(`COMPRESS|FREIO|SECADOR|CAMBIO`)}},
	{"Óleo de freio", []*regexp.Regexp{re(`FLUIDO|\bOLEO\b|\bDOT\s*[345]`), re(`FREIO|\bFR\b.*\bDOT|\bDOT\s*[345]`)}, []*regexp.Regexp{re(`BOMBA|CILINDRO|PASTILHA|DISCO|SENSOR|MANG|\bLIMPA\b|SAPATA|LONA|TAMBOR|CABO|RESERVATORIO|TAMPA`)}},
	{"Óleo de câmbio", []*regexp.Regexp{reOleoNoInicio, re(`CAMBIO|TRANSMISS|CAIXA|GEROL|ULTRAGEAR|\bGEAR\b|\bMTF\b|\bATF\b|\bCVT\b|\bDCT\b|\bDSG\b|DIFERENCIAL|^\s*OLEO\s+(SAE\s*)?(80|85|90|140)\b|\bSAE\s*(75|80|85|90|140)\b|\b(75|80|85)W[\s-]?(80|85|90|140)\b|\bGL[\s-]?[45]\b|DEXRON`)}, []*regexp.Regexp{re(`BOMBA|RETENTOR|FILTRO|JUNTA|COXIM|SENSOR|HIDRAUL|DIRECAO`)}},
	{"Óleo do motor", []*regexp.Regexp{reOleoNoInicio}, []*regexp.Regexp{re(`FILTRO|FREIO|CAMBIO|TRANSMISS|HIDRAUL|DIRECAO|2T\b|\bDOIS TEMPOS|\bMOTO\b|DESENGRIP|\bWD[\s-]?40|SPRAY|LIMPA|ADITIVO|VEDA|RETENTOR|JUNTA|BOMBA|SENSOR|MANGUEIRA|TAMPA|BUJAO|CARTER|PRESSAO|VARETA|RADIADOR|ARREF|\bPVC\b`)}},
}

// o nome tem que começar com o item: "FILTRO OLEO ..." é filtro; "MANGUEIRA FILTRO AR",
// "ANEL FILTRO BICO" e "PRE-FILTRO" não são.
var reFiltro = re(`^\s*(ELEMENTO\s+)?(FILTRO|FILTR|FILT|FLT|FILREO)\b`)

// óleo/fluido no começo do nome: "OLEO 15W40..." é óleo; "DEFLETOR DE OLEO",
// "INTERRUPTOR DE OLEO" e "BOMBA OLEO DO CAMBIO" não são.
var reOleoNoInicio = re(`^\s*(OLEO|FLUIDO|LUBRIFICANTE|LUBRIF|TROCA\s+(DE\s+|DO\s+)?OLEO)\b|^\s*\d?\s*(0|5|10|15|20|25)W[\s-]?(16|20|30|40|50|60)\b`)

// Ignorar linhas que não dizem o que foi feito no carro.
var ignorar = []*regexp.Regexp{
	re(`^\s*(MAO DE OBRA|M\.?\s*O\.?|SERVICOS?|SERV\.?|DIVERSOS|DESCONTO|ACRESCIMO|FRETE|TAXA)\s*\.?\s*$`),
	// genéricos do cadastro do Zeus da oficina
	re(`^\s*(SERVICOS?|SERV\.?|MAO DE OBRA)\s+(MECANICOS?|ELETRICOS?|GERAIS|DIVERSOS)\s*\.?\s*$`),
	re(`^\s*COMBUSTIVEL\b`),
	// mão de obra genérica, do jeito que está no cadastro da oficina
	re(`^\s*SERVICOS?(\s+(TECNICOS?|DOS?|DAS?|DE|MECANICOS?|GERAIS|DIVERSOS|TERCERIZADOR|TERCEIRIZADOS?))*\s*\.?\s*$`),
	re(`^\s*(TROCA|REPARO|SERVICO|SERVICOS)\s*\.?\s*$`),
	re(`\bTAXI\b|\bGUINCHO\b|LAVAGEM\s+DE\s+CARRO`),
	// material de consumo usado no serviço (não é peça trocada no carro)
	re(`DESENGRAX|DESENGRIP|SILICONE|DESCARBONIZANTE|DESCARBONIZADOR|\bCAR\s*-?\s*80\b|\bLIMPA\b|\bLIMPAR\b|GRAXA|SELANTE|\bCOLA\b|FITA\s+ISOLANTE|\bWD[\s-]?40\b|ESTOPA|\bLIXA\b|ABRILHANTADOR|SHAMPOO|\bCERA\b|ETIQUETA\s+ADESIVA|AGUA\s+DESMINERALIZADA`),
}

// parafuso/porca/arruela avulsos ("PARAFUSO 13 GRANDE", "PORCA COMUM 10")
var reFerragem = re(`^\s*(PARAFUSOS?|PORCAS?|ARRUELAS?)\b`)

// abraçadeira e ferragem de catálogo (com medida ou norma): "ABRAÇ. SEM FIM ZNB 10X16X9",
// "ARRUELA LISA PO DIN125A M8X16X1,5MM"
var reFerragemMedida = re(`^\s*(PARAFUSOS?|PORCAS?|ARRUELAS?|ABRAC\w*|PRESILHAS?)\b.*(\d\s*X\s*\d|\bDIN\s*\d|\bM\d+\b)|^\s*ABRAC(ADEIRA)?\.?\s+(NYLON|PRESSAO|PRESILHA|MICRO|UNIVERSAL|TIPO|SEM\s+FIM|S\s+FIM)\b`)
var palavrasFerragem = map[string]bool{"COMUM": true, "GRANDE": true, "PEQUENO": true, "PEQUENA": true, "MEDIO": true, "MEDIA": true,
	"DE": true, "PRESSAO": true, "TRAVANTE": true, "LISA": true, "SIMPLES": true, "ACO": true, "INOX": true, "ZINCADO": true, "ZINCADA": true}

func ferragemAvulsa(d string) bool {
	if reFerragemMedida.MatchString(d) {
		return true
	}
	if !reFerragem.MatchString(d) {
		return false
	}
	for i, w := range strings.Fields(strings.NewReplacer(".", " ", ",", " ", "/", " ").Replace(d)) {
		if i == 0 {
			continue
		}
		if palavrasFerragem[w] || strings.Trim(w, "0123456789MX") == "" {
			continue
		}
		return false // tem nome de peça ("PARAFUSO CABECOTE")
	}
	return true
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

// Classificar diz se a descrição é um item padrão. Devolve "" se não é.
func Classificar(descricao string) string {
	d := strings.ToUpper(semAcento(descricao))
	d = strings.NewReplacer(".", " ", "_", " ").Replace(d)
	for _, r := range regras {
		ok := true
		for _, x := range r.todas {
			if !x.MatchString(d) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for _, x := range r.nenhu {
			if x.MatchString(d) {
				ok = false
				break
			}
		}
		if ok {
			return r.item
		}
	}
	return ""
}

// Ignorar diz se a linha deve ficar de fora do histórico (mão de obra genérica etc.).
func Ignorar(descricao string) bool {
	d := strings.ToUpper(semAcento(strings.TrimSpace(descricao)))
	if d == "" {
		return true
	}
	for _, x := range ignorar {
		if x.MatchString(d) {
			return true
		}
	}
	return ferragemAvulsa(d)
}

// Separar divide as descrições em itens padrão e outros (já com texto bonito).
func Separar(descricoes []string) (padrao []string, outros []string) {
	vistoP := map[string]bool{}
	vistoO := map[string]bool{}
	for _, d := range descricoes {
		if Ignorar(d) {
			continue
		}
		if it := Classificar(d); it != "" {
			if !vistoP[it] {
				vistoP[it] = true
				padrao = append(padrao, it)
			}
			continue
		}
		t := Bonito(Limpar(d))
		if t != "" && !vistoO[strings.ToLower(t)] {
			vistoO[strings.ToLower(t)] = true
			outros = append(outros, t)
		}
	}
	if padrao == nil {
		padrao = []string{}
	}
	if outros == nil {
		outros = []string{}
	}
	return
}

// palavras que ficam em minúsculas no meio da frase
var minusculas = map[string]bool{"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true,
	"com": true, "p/": true, "para": true, "a": true, "o": true, "em": true, "na": true, "no": true}

var siglas = map[string]bool{"VW": true, "GM": true, "AC": true, "A/C": true, "ABS": true, "ATF": true, "CVT": true,
	"DOT": true, "NGK": true, "SKF": true, "TRW": true, "LED": true, "EGR": true, "PCV": true, "GNV": true, "MTE": true,
	"BMW": true, "KIA": true, "JAC": true, "CAOA": false, "4X4": true, "TB": true, "TBI": true, "SOHC": true, "DOHC": true}

// Bonito transforma "MANGUEIRA DE AR  SUP. 1.6" em "Mangueira de ar sup. 1.6".
func Bonito(s string) string {
	campos := strings.Fields(s)
	if len(campos) == 0 {
		return ""
	}
	tudoMaiusculo := strings.ToUpper(s) == s
	if !tudoMaiusculo {
		r := []rune(strings.Join(campos, " "))
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	}
	for i, c := range campos {
		baixo := strings.ToLower(c)
		temDigito := strings.IndexFunc(c, unicode.IsDigit) >= 0
		switch {
		case i == 0:
			r := []rune(baixo)
			r[0] = unicode.ToUpper(r[0])
			campos[i] = string(r)
		case temDigito || siglas[c]:
			campos[i] = c // códigos e siglas (1.6, 5W30, VW, ABS) ficam como estão
		default:
			campos[i] = baixo
		}
	}
	return strings.Join(campos, " ")
}

// Revisar passa uma lista já gravada (itens + outros) pelas regras atuais.
// Os nomes antigos podem estar em minúsculas ("Lantern tras fiat: uno 2005/...").
func Revisar(itensAntigos, outrosAntigos []string) ([]string, []string) {
	var brutos []string
	for _, o := range outrosAntigos {
		brutos = append(brutos, strings.ToUpper(o))
	}
	padrao, outros := Separar(brutos)
	visto := map[string]bool{}
	var it []string
	for _, x := range append(append([]string{}, itensAntigos...), padrao...) {
		if !visto[x] {
			visto[x] = true
			it = append(it, x)
		}
	}
	// mantém a ordem da etiqueta
	ordem := map[string]int{"Óleo do motor": 0, "Filtro de óleo": 1, "Filtro de ar": 2, "Filtro de combustível": 3, "Filtro de cabine": 4, "Óleo de freio": 5, "Óleo de câmbio": 6}
	sort.SliceStable(it, func(i, j int) bool { return ordem[it[i]] < ordem[it[j]] })
	if it == nil {
		it = []string{}
	}
	return it, outros
}

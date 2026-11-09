package itens

import (
	"regexp"
	"strings"
)

// Limpar deixa o nome da peça como o cliente entende, tirando do nome do
// cadastro do Zeus a marca do fabricante, os carros em que serve e as
// abreviações: "LANTERN TRAS FIAT: UNO 2005/ LD DIREIT CRISTAL" → "LANTERNA
// TRASEIRA"; "BOMBA AGUA/  SCHADEK/   GOL/1.0/G4" → "BOMBA ÁGUA".
// Depois o Bonito() acerta as maiúsculas.
func Limpar(s string) string {
	orig := strings.TrimSpace(s)
	if orig == "" {
		return ""
	}
	t := reParenteses.ReplaceAllString(orig, " ")
	t = reParaBarra.ReplaceAllString(t, "PARA ")    // "P/ COIFA" → "PARA COIFA" (não é aplicação)
	t = rePontoColado.ReplaceAllString(t, "$1. $2") // "RAD.ORG.ROSA" → "RAD. ORG. ROSA"
	// "SERVICO TECNICO: ABRIR ROSCA..." → o que interessa vem depois dos dois pontos
	if m := reServicoDoisPontos.FindStringSubmatch(t); m != nil && strings.TrimSpace(m[1]) != "" {
		t = m[1]
	}
	// colunas separadas por muitos espaços: o resto é aplicação ("GOL/POLO/FOX")
	// (só quando o resto tem barra; senão são só espaços a mais no nome)
	if i := reMuitosEspacos.FindStringIndex(t); i != nil && i[0] > 0 && strings.Contains(t[i[1]:], "/") {
		t = t[:i[0]]
	}
	// marca de carro no meio: dali em diante é aplicação ("ALTERNADOR FIAT: UNO 2005")
	if i := reMarcaCarro.FindStringIndex(" " + t); i != nil && i[0] > 0 {
		t = t[:i[0]]
	}
	// "COXIM MOTOR: JETTA" → antes dos dois pontos
	if i := strings.Index(t, ":"); i > 0 {
		t = t[:i]
	}
	// "BOMBA AGUA/ SCHADEK/" → antes da barra (se sobrar nome de verdade)
	if i := strings.Index(t, "/"); i > 0 {
		antes := t[:i]
		if reTemPalavra.MatchString(antes) {
			t = antes
		}
	}
	var out []string
	for _, w := range strings.Fields(t) {
		chave := strings.ToUpper(semAcento(strings.TrimRight(w, ".,;-")))
		if v, ok := abreviacoes[chave]; ok {
			if v != "" {
				out = append(out, v)
			}
			continue
		}
		out = append(out, strings.TrimRight(w, ";-"))
	}
	r := strings.Join(out, " ")
	// "SERVIÇO TÉCNICO DA RETÍFICA" → "SERVIÇO DA RETÍFICA"
	r = reServicoTecnico.ReplaceAllString(r, "SERVIÇO ")
	r = strings.Trim(strings.Join(strings.Fields(r), " "), " .,/-:")
	if len([]rune(r)) < 3 {
		return orig
	}
	return r
}

var (
	reParenteses        = regexp.MustCompile(`\([^)]*\)?`)
	reParaBarra         = regexp.MustCompile(`(?i)\bP/\s*`)
	rePontoColado       = regexp.MustCompile(`([A-Za-zÀ-ú]{2,})\.([A-Za-zÀ-ú]{2,})`)
	reServicoDoisPontos = regexp.MustCompile(`(?i)^\s*servi[cç]os?(?:\s+t[eé]cnicos?)?\s*:\s*(.+)$`)
	reMuitosEspacos     = regexp.MustCompile(`\s{3,}`)
	reMarcaCarro        = regexp.MustCompile(`(?i)\s(FIAT|VW|VOLKSWAGEN|GM|CHEVROLET|FORD|RENAULT|TOYOTA|HONDA|HYUNDAI|NISSAN|PEUGEOT|CITROEN|JEEP|MITSUBISHI|KIA|AUDI|BMW|MERCEDES)\b\s*:?`)
	reTemPalavra        = regexp.MustCompile(`[A-Za-zÀ-ú]{3,}`)
	reServicoTecnico    = regexp.MustCompile(`(?i)^SERVIÇOS?\s+TÉCNICOS?\s+`)
)

// abreviações do cadastro (sem acento, maiúsculas, sem ponto final) → como fica.
// Vazio = some ("PC", "UNID").
var abreviacoes = map[string]string{
	"TRAS": "TRASEIRA", "TRAZ": "TRASEIRA", "TRASEIR": "TRASEIRA",
	"DIANT": "DIANTEIRA", "DIANTEIR": "DIANTEIRA", "DAINTEIRA": "DIANTEIRA", "DINATEIRO": "DIANTEIRO", "DIANTEIRO": "DIANTEIRO",
	"LD": "LADO", "DIREIT": "DIREITO", "DIR": "DIREITO", "ESQ": "ESQUERDO", "ESQUERD": "ESQUERDO",
	"JG": "JOGO", "ALTERNAD": "ALTERNADOR", "COMB": "COMBUSTÍVEL", "RAD": "RADIADOR", "ORG": "ORGÂNICO",
	"CONC": "CONCENTRADO", "MTR": "MOTOR", "VALV": "VÁLVULA", "VALVULA": "VÁLVULA", "VALVULAS": "VÁLVULAS",
	"VIRAB": "VIRABREQUIM", "AMORT": "AMORTECEDOR", "SUSP": "SUSPENSÃO", "SUSPENSAO": "SUSPENSÃO",
	"EMBREAG": "EMBREAGEM", "ROLAM": "ROLAMENTO", "CABECOTE": "CABEÇOTE", "CABEC": "CABEÇOTE",
	"AGUA": "ÁGUA", "OLEO": "ÓLEO", "BRONZ": "BRONZINA", "LANTERN": "LANTERNA", "TERMOST": "TERMOSTÁTICA",
	"CAMBIO": "CÂMBIO", "DIRECAO": "DIREÇÃO", "IGNICAO": "IGNIÇÃO", "INJECAO": "INJEÇÃO", "PRESSAO": "PRESSÃO",
	"SERVICO": "SERVIÇO", "SERVICOS": "SERVIÇOS", "TECNICO": "TÉCNICO", "TECNICA": "TÉCNICA",
	"ELETRICO": "ELÉTRICO", "ELETRICA": "ELÉTRICA", "ELETRICAS": "ELÉTRICAS", "MECANICO": "MECÂNICO",
	"ARREIFECIMENTO": "ARREFECIMENTO", "RETIFICA": "RETÍFICA", "LAMPADA": "LÂMPADA", "HIDRAULICO": "HIDRÁULICO",
	"SINTETICO": "SINTÉTICO", "RESERVATORIO": "RESERVATÓRIO", "ELETRONICA": "ELETRÔNICA", "PISTAO": "PISTÃO",
	"ANEIS": "ANÉIS", "LUB": "LUBRIFICANTE", "ELICOIDE": "HELICOIDE", "OURINGUE": "ANEL O-RING",
	"SATND": "STANDARD", "STAND": "STANDARD", "FLT": "FILTRO", "FILT": "FILTRO", "HOMOCINETICA": "HOMOCINÉTICA",
	"DIAGNOSTICO": "DIAGNÓSTICO", "PROGRAMACAO": "PROGRAMAÇÃO", "CALIBRACAO": "CALIBRAÇÃO",
	"DESCARBONIZACAO": "DESCARBONIZAÇÃO", "MODULO": "MÓDULO", "CARTER": "CÁRTER", "CARTE": "CÁRTER", "BUJAO": "BUJÃO",
	"PASTILHAS": "PASTILHAS", "BALANCA": "BALANÇA", "CAMAR": "CARCAÇA", "ENGRENAGEM": "ENGRENAGEM",
	"ABRAC": "ABRAÇADEIRA", "HOMOC": "HOMOCINÉTICA", "MANG": "MANGUEIRA", "RESP": "RESPIRO", "INJ": "INJETOR",
	"RETENCAO": "RETENÇÃO", "COMBUSTIVEL": "COMBUSTÍVEL", "AUTOMATICO": "AUTOMÁTICO", "AUTOMATIZADO": "AUTOMATIZADO",
	"PC": "", "PÇ": "", "PCS": "", "UNID": "", "UND": "", "UN": "", "KIT.": "KIT",
}

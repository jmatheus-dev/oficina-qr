package itens

import (
	"regexp"
	"strings"
)

// Marca e linha do óleo, para a "Observação para o cliente".
// "OLEO LUBRAX 5W30 SINTETICO 1L"                  → "Lubrax 5W30 sintético"
// "OLEO 15W40 SYNTIUM 800AM SEMISINT. 1L/PETRONAS" → "Petronas Syntium 800 15W40 semissintético"

type marca struct {
	re   *regexp.Regexp
	nome string
}

// linhas conhecidas (já dizem a marca)
var linhasOleo = []struct {
	re           *regexp.Regexp
	marca, linha string
}{
	{re(`\bSYNTIUM\b`), "Petronas", "Syntium"},
	{re(`\bSELENIA\b`), "Petronas", "Selenia"},
	{re(`\bHELIX\b`), "Shell", "Helix"},
	{re(`\bMAGNATEC\b`), "Castrol", "Magnatec"},
	{re(`\bGTX\b`), "Castrol", "GTX"},
	{re(`\bEDGE\b`), "Castrol", "Edge"},
	{re(`\bQUARTZ\b`), "TotalEnergies", "Quartz"},
	{re(`\bTRAX\s*GEAR\b`), "TotalEnergies", "Trax Gear"},
	{re(`\bHAVOLINE\b`), "Texaco", "Havoline"},
	{re(`\bULTRAGEAR\b`), "Ipiranga", "Ultragear"},
	{re(`\bBRUTUS\b`), "Ipiranga", "Brutus"},
	{re(`\bF1\s*MASTER\b`), "Ipiranga", "F1 Master"},
	{re(`\bUNIGEROL\b`), "Ingrax", "Unigerol"},
	{re(`\bRUBIA\b`), "TotalEnergies", "Rubia"},
	{re(`\bDELVAC\b`), "Mobil", "Delvac"},
	{re(`\bRIMULA\b`), "Shell", "Rimula"},
	{re(`\bURSA\b`), "Texaco", "Ursa"},
}

var marcasOleo = []marca{
	{re(`\bLUBRAX\b|\bPETROBRAS\b|\bBR\s+DISTRIBUIDORA\b`), "Lubrax"},
	{re(`\bPETRONAS\b`), "Petronas"},
	{re(`\bCASTROL\b`), "Castrol"},
	{re(`\bMOBIL\b`), "Mobil"},
	{re(`\bSHELL\b`), "Shell"},
	{re(`\bIPIRANGA\b`), "Ipiranga"},
	{re(`\bTEXACO\b`), "Texaco"},
	{re(`\bMOTUL\b`), "Motul"},
	{re(`\bELF\b`), "Elf"},
	{re(`\bT\s*ENERGIES\b|\bTOTALENERGIES\b|\bTOTAL\b`), "TotalEnergies"},
	{re(`\bVALVOLINE\b`), "Valvoline"},
	{re(`\bMOTORCRAFT\b`), "Motorcraft"},
	{re(`\bAC\s*DELCO\b`), "ACDelco"},
	{re(`\bMOPAR\b`), "Mopar"},
	{re(`\bBARDAHL\b`), "Bardahl"},
	{re(`\bDULUB\b`), "Dulub"},
	{re(`\bINGRAX\b`), "Ingrax"},
	{re(`\bREPSOL\b`), "Repsol"},
	{re(`\bLIQUI\s*MOLY\b`), "Liqui Moly"},
	{re(`\bPENNZOIL\b`), "Pennzoil"},
	{re(`\bYPF\b`), "YPF"},
	{re(`\bGULF\b`), "Gulf"},
	{re(`\bKIXX\b`), "Kixx"},
	{re(`\bZIC\b`), "ZIC"},
	{re(`\bRADNAQ\b`), "Radnaq"},
	{re(`\bTUTELA\b`), "Tutela"},
	{re(`\bAGIP\b|\bENI\b`), "Eni"},
	{re(`\bFUCHS\b`), "Fuchs"},
	{re(`\bVOLKSWAGEN\b|\bVW\b`), "Volkswagen (original)"},
	{re(`\bTOYOTA\b`), "Toyota (original)"},
	{re(`\bHONDA\b`), "Honda (original)"},
	{re(`\bHYUNDAI\b`), "Hyundai (original)"},
}

var (
	reViscosidade  = re(`\b(0|5|10|15|20|25|75|80|85)\s*W\s*-?\s*(\d{2,3})\b`)
	reSAEsozinho   = re(`^\s*OLEO\s+(DE\s+)?(CAIXA\s+|CAMBIO\s+)?(SAE\s*)?(80|85|90|140)\b|\bSAE\s*(80|85|90|140)\b|\bOLEO\s+(80|85|90|140)\b`)
	reSemi         = re(`\bSEMI\s*-?\s*S?SINT|\bSEMI\b`)
	reSintetico    = re(`\bSINTETICO\b|\bSINT\b|\bSYNTH|\bSINTET`)
	reMineral      = re(`\bMINERAL\b`)
	reDiesel       = re(`\bDIESEL\b`)
	reATF          = re(`\bATF\b|\bDEXRON\b`)
	reNumeroDaLinh = regexp.MustCompile(`^(\d{1,5})[A-Z]{0,2}$|^([A-Z]{1,2}\d{1,2})$`)
	reVolume       = regexp.MustCompile(`^\d+(L|LT|ML)$`)
	grausSAE       = map[string]bool{"80": true, "85": true, "90": true, "140": true}
)

// DescreverOleo devolve "Marca Linha 5W30 sintético" (o que achar) para uma
// descrição de óleo do Zeus. Vazio se não achar nada útil.
func DescreverOleo(descricao string) string {
	d := strings.ToUpper(semAcento(descricao))
	d = strings.NewReplacer(".", " ", "/", " ", "(", " ", ")", " ", ",", " ", ":", " ", ";", " ", "_", " ").Replace(d)
	d = strings.Join(strings.Fields(d), " ")
	var partes []string
	marcaAchada, saeDaLinha := "", ""
	for _, l := range linhasOleo {
		loc := l.re.FindStringIndex(d)
		if loc == nil {
			continue
		}
		nome := l.marca + " " + l.linha
		// número/modelo logo depois da linha: "SYNTIUM 800AM" → 800, "HELIX HX7" → HX7
		if resto := strings.Fields(d[loc[1]:]); len(resto) > 0 && !reViscosidade.MatchString(resto[0]) {
			if m := reNumeroDaLinh.FindStringSubmatch(resto[0]); m != nil && !reVolume.MatchString(resto[0]) {
				if grausSAE[m[1]] && m[1] == resto[0] {
					saeDaLinha = m[1] // "UNIGEROL 90": o 90 é o grau SAE
				} else {
					nome += " " + m[1] + m[2]
				}
			}
		}
		partes, marcaAchada = append(partes, nome), l.marca
		break
	}
	if marcaAchada == "" {
		for _, m := range marcasOleo {
			if m.re.MatchString(d) {
				partes, marcaAchada = append(partes, m.nome), m.nome
				break
			}
		}
	}
	if m := reViscosidade.FindStringSubmatch(d); m != nil {
		partes = append(partes, m[1]+"W"+m[2])
	} else if m := reSAEsozinho.FindStringSubmatch(d); m != nil {
		for _, g := range m[4:] {
			if g != "" {
				partes = append(partes, "SAE "+g)
				break
			}
		}
	} else if saeDaLinha != "" {
		partes = append(partes, "SAE "+saeDaLinha)
	} else if reATF.MatchString(d) {
		partes = append(partes, "ATF")
	}
	switch {
	case reSemi.MatchString(d):
		partes = append(partes, "semissintético")
	case reSintetico.MatchString(d):
		partes = append(partes, "sintético")
	case reMineral.MatchString(d):
		partes = append(partes, "mineral")
	}
	if reDiesel.MatchString(d) {
		partes = append(partes, "diesel")
	}
	return strings.Join(partes, " ")
}

// ObsOleo monta a observação para o cliente com o óleo usado na venda.
// Só óleo do motor: "Óleo Lubrax 5W30 sintético". Com câmbio também:
// "Óleo do motor Lubrax 5W30 sintético · Óleo do câmbio Ipiranga Ultragear 75W80".
func ObsOleo(produtos []string) string {
	var motor, cambio []string
	juntar := func(l []string, s string) []string {
		if s == "" {
			return l
		}
		for _, x := range l {
			if strings.EqualFold(x, s) {
				return l
			}
		}
		return append(l, s)
	}
	for _, p := range produtos {
		switch Classificar(p) {
		case "Óleo do motor":
			motor = juntar(motor, DescreverOleo(p))
		case "Óleo de câmbio":
			cambio = juntar(cambio, DescreverOleo(p))
		}
	}
	m, c := strings.Join(motor, " + "), strings.Join(cambio, " + ")
	switch {
	case m != "" && c != "":
		return "Óleo do motor " + m + " · Óleo do câmbio " + c
	case m != "":
		return "Óleo " + m
	case c != "":
		return "Óleo do câmbio " + c
	}
	return ""
}

package dados

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	reAno        = regexp.MustCompile(`\b((?:19[5-9]\d|20[0-4]\d)(?:\s*/\s*(?:(?:19|20)\d{2}|\d{2}))?)\b`)
	reSeparador  = regexp.MustCompile(`[,;:()\[\]]|\.\s|\.$|\s-\s`)
	reCores      = regexp.MustCompile(`(?i)\b(preto|preta|branco|branca|prata|vermelho|vermelha|azul|cinza|verde|amarelo|amarela|bege|marrom|dourado|dourada|vinho|grafite|laranja|champanhe)\b`)
	reDono       = regexp.MustCompile(`(?i)^\s*(de|do|da|dono|dona|cliente|proprietario|proprietaria|propriet[aá]rio)\s+\S`)
	reLixoModelo = regexp.MustCompile(`(?i)\b(placa|carro|veiculo|veículo|do|da|de|cliente|km)\s*$`)
)

// AcharModelo tira modelo e ano do texto livre da venda, olhando o que vem
// antes da placa: "jetta 2.0 8v/ 2011. oep9j99" → ("Jetta 2.0 8V", "2011");
// "fiat uno 1.0 tst-1234" → ("Fiat Uno 1.0", ""). Sem placa no texto, não
// arrisca e devolve vazio.
func AcharModelo(texto string) (modelo, ano string) {
	t := semAcento(texto)
	m := rePlacaTexto.FindStringIndex(t)
	if m == nil || NormalizarPlaca(t[m[0]:m[1]]) == "" {
		return "", ""
	}
	// semAcento pode mudar o tamanho; usa o texto sem acento dos dois lados
	antes := t[:m[0]]
	// fica com o último trecho antes da placa (depois de vírgula, ponto final...)
	// que tenha alguma coisa além de "placa"/"carro"
	partes := reSeparador.Split(antes, -1)
	for i := len(partes) - 1; i >= 0 && i >= len(partes)-3; i-- {
		trecho := partes[i]
		// "de Alan Michell", "do Seu Zé": é o dono, não o carro
		if reDono.MatchString(trecho) {
			continue
		}
		// o ano às vezes vem sozinho logo depois do modelo: "jetta 2.0 8v/ 2011."
		if i > 0 && len(strings.Fields(trecho)) == 1 && reAno.MatchString(strings.TrimSpace(trecho)) {
			trecho = partes[i-1] + " " + trecho
			i--
		}
		if mo, an := limparModelo(trecho); mo != "" || an != "" {
			if mo == "" && i > 0 { // só o ano: o modelo pode estar no trecho anterior
				if mo2, _ := limparModelo(partes[i-1]); mo2 != "" {
					return mo2, an
				}
			}
			return mo, an
		}
	}
	return "", ""
}

func limparModelo(trecho string) (modelo, ano string) {
	if a := reAno.FindString(trecho); a != "" {
		ano = strings.ReplaceAll(a, " ", "")
		trecho = strings.Replace(trecho, a, " ", 1)
	}
	trecho = reCores.ReplaceAllString(trecho, " ")
	trecho = strings.Trim(trecho, " /-.:,")
	for {
		novo := strings.TrimSpace(reLixoModelo.ReplaceAllString(trecho, ""))
		novo = strings.Trim(novo, " /-.:,")
		if novo == trecho {
			break
		}
		trecho = novo
	}
	campos := strings.Fields(trecho)
	if len(campos) == 0 {
		return "", ano
	}
	if len(campos) > 6 { // texto comprido: o modelo costuma estar no fim
		campos = campos[len(campos)-6:]
	}
	for i, c := range campos {
		campos[i] = palavraModelo(c)
	}
	modelo = strings.Join(campos, " ")
	if len([]rune(modelo)) > 40 {
		modelo = string([]rune(modelo)[:40])
	}
	return strings.TrimSpace(modelo), ano
}

// siglas de modelo que ficam em maiúsculas
var siglasModelo = map[string]bool{"VW": true, "GM": true, "BMW": true, "KIA": true, "JAC": true, "HB20": true, "HB20S": true,
	"CR-V": true, "HR-V": true, "WR-V": true, "ZR-V": true, "SW4": true, "RAV4": true, "XEI": true, "GLI": true, "XLI": true,
	"LTZ": true, "LT": true, "LS": true, "GTI": true, "TSI": true, "MPI": true, "TDI": true, "GT": true, "EX": true, "LX": true}

func palavraModelo(c string) string {
	up := strings.ToUpper(c)
	if siglasModelo[up] || strings.IndexFunc(c, unicode.IsDigit) >= 0 {
		return up // 1.0, 8V, S10, TR4, 1.6/16V
	}
	r := []rune(strings.ToLower(c))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

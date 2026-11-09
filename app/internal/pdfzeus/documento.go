package pdfzeus

import (
	"errors"
	"regexp"
	"strings"
)

// Documento é o que interessa de um orçamento / pré-venda / venda do Zeus.
type Documento struct {
	Tipo        string   // "Orçamento", "Pré-Venda"...
	Numero      string   // "17"
	Data        string   // AAAA-MM-DD (data do documento)
	Cliente     string   // "CLIENTE EXEMPLO LTDA"
	Fantasia    string   // apelido / nome fantasia
	Telefone    string   // telefone do cliente
	Tecnico     string   // mecânico
	Produtos    []string // descrição de cada produto/serviço
	Observacoes string   // campo Observações (onde vai o carro, a placa e o KM)
}

var (
	reCabecalho = regexp.MustCompile(`(?i)^(or[çc]amento|pr[ée][\s-]?venda|pedido(?: de venda)?|venda|ordem de servi[çc]o|o\.?s\.?)\s*(?:n[º°o.]*\s*)?[-:]?\s*(\d{1,10})\b`)
	reData      = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)
	reRotulo    = regexp.MustCompile(`(?i)^(cliente|fantasia|apelido|t[ée]cnico|fone|telefone|observa[çc][õo]es|obs)\s*\.?\s*:\s*(.*)$`)
	// modelo Mini: "302 - LANTERN TRAS FIAT..."
	reItemMini = regexp.MustCompile(`^(\d{1,9})\s+-\s+(.+)$`)
	// modelo A4: "302 LANTERN TRAS FIAT... UNID 1 110,00 0,00% 110,00"
	reItemA4 = regexp.MustCompile(`(?i)^(\d{1,9})\s+(.+?)\s+(UNID|UN|UND|PC|P[ÇC]|PCT|L|LT|KG|G|M|MT|CX|JG|KIT|PAR|CJ|FR|GL)\s+[\d.,]+\s+[\d.,]+`)
	// linha de quantidade do modelo Mini: "1 UNID 110,00 0,00 110,00"
	reQtdUnid = regexp.MustCompile(`(?i)^[\d.,]+\s*(UNID|UN|UND|PC|P[ÇC]|PCT|L|LT|KG|G|M|MT|CX|JG|KIT|PAR|CJ|FR|GL)\b`)
	// fim do bloco de observações
	reFimObs = regexp.MustCompile(`(?i)^(assinatura|n[ãa]o [ée] documento fiscal|reimpresso por|impresso por|vendedor\s*:|t[ée]cnico\s*:|_{4,})`)
	// linhas que nunca são produto
	reNaoItem = regexp.MustCompile(`(?i)^(cpf|cnpj|cep|ie|inscri|fone|n[úu]mero de|quantidade|total|valor|previs|vendedor|t[ée]cnico|observa|cliente|endere|bairro|cidade|estado)`)
)

var tiposBonitos = map[string]string{"orcamento": "Orçamento", "pre-venda": "Pré-Venda", "prevenda": "Pré-Venda", "pre venda": "Pré-Venda",
	"venda": "Venda", "pedido": "Pedido", "pedido de venda": "Pedido", "ordem de servico": "OS", "os": "OS", "o.s.": "OS", "o.s": "OS"}

func tipoBonito(s string) string {
	k := strings.ToLower(s)
	k = strings.NewReplacer("ç", "c", "é", "e", "ê", "e").Replace(k)
	if t, ok := tiposBonitos[k]; ok {
		return t
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// Ler entende as linhas de um PDF exportado pelo Zeus.
func Ler(linhas []string) (Documento, error) {
	var d Documento
	nosItens := false
	obs := []string{}
	nasObs := false
	for i, l := range linhas {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if d.Numero == "" {
			if m := reCabecalho.FindStringSubmatch(l); m != nil {
				d.Tipo, d.Numero = tipoBonito(m[1]), m[2]
				if dt := reData.FindStringSubmatch(l); dt != nil {
					d.Data = dt[3] + "-" + dt[2] + "-" + dt[1]
				} else if i+1 < len(linhas) {
					if dt := reData.FindStringSubmatch(linhas[i+1]); dt != nil && !strings.Contains(strings.ToLower(linhas[i+1]), "impresso") {
						d.Data = dt[3] + "-" + dt[2] + "-" + dt[1]
					}
				}
				continue
			}
		}
		if nasObs {
			if reFimObs.MatchString(l) {
				nasObs = false
			} else {
				obs = append(obs, l)
				continue
			}
		}
		if m := reRotulo.FindStringSubmatch(l); m != nil {
			rot := strings.ToLower(m[1])
			v := strings.TrimSpace(m[2])
			switch {
			case rot == "cliente":
				d.Cliente = limparCliente(v)
			case rot == "fantasia" || rot == "apelido":
				if d.Fantasia == "" {
					d.Fantasia = v
				}
			case strings.HasPrefix(rot, "t"):
				if strings.HasPrefix(rot, "tel") {
					if d.Telefone == "" {
						d.Telefone = v
					}
				} else if d.Tecnico == "" {
					d.Tecnico = v
				}
			case rot == "fone":
				if d.Telefone == "" && d.Cliente != "" {
					d.Telefone = v
				}
			default: // observações
				nasObs = true
				nosItens = false
				if v != "" {
					obs = append(obs, v)
				}
			}
			continue
		}
		low := strings.ToLower(l)
		if strings.HasPrefix(low, "produto") || strings.HasPrefix(low, "código") || strings.HasPrefix(low, "codigo") {
			nosItens = true
			continue
		}
		if strings.HasPrefix(low, "número de itens") || strings.HasPrefix(low, "numero de itens") || strings.HasPrefix(low, "total") {
			nosItens = false
			continue
		}
		if !nosItens || reNaoItem.MatchString(l) {
			continue
		}
		if m := reItemMini.FindStringSubmatch(l); m != nil {
			d.Produtos = append(d.Produtos, strings.TrimSpace(m[2]))
		} else if reQtdUnid.MatchString(l) {
			continue
		} else if m := reItemA4.FindStringSubmatch(l); m != nil {
			d.Produtos = append(d.Produtos, strings.TrimSpace(m[2]))
		}
	}
	d.Observacoes = strings.Join(obs, " ")
	if d.Data == "" { // A4: a data fica sozinha numa linha do cabeçalho
		for _, l := range linhas {
			low := strings.ToLower(l)
			if strings.Contains(low, "impresso") || strings.Contains(low, "válido") || strings.Contains(low, "valido") {
				continue
			}
			if dt := reData.FindStringSubmatch(l); dt != nil {
				d.Data = dt[3] + "-" + dt[2] + "-" + dt[1]
				break
			}
		}
	}
	if d.Numero == "" {
		return d, errors.New("não achei o número do orçamento/pré-venda neste PDF")
	}
	if len(d.Produtos) == 0 {
		return d, errors.New("não achei produtos neste PDF")
	}
	return d, nil
}

// "CLIENTE EXEMPLO LTDA (19)" → "CLIENTE EXEMPLO LTDA"; "19 - CLIENTE EXEMPLO LTDA" também.
var reCodCliente = regexp.MustCompile(`^\d+\s*-\s*|\s*\(\d+\)\s*$`)

func limparCliente(s string) string {
	s = reCodCliente.ReplaceAllString(strings.TrimSpace(s), "")
	// A4 junta "Apelido: ..." na mesma linha
	if i := strings.Index(strings.ToLower(s), " apelido:"); i > 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

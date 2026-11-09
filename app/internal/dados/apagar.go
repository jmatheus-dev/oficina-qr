package dados

import "sort"

// Resultado de apagar vários registros de uma vez.
type Apagados struct {
	Servicos int      `json:"servicos"`
	Carros   int      `json:"carros"`
	Voltaram []string `json:"voltaram"` // vendas do Zeus que voltaram para "Para revisar"
}

// ApagarVarios apaga os serviços (por id) e os carros (por placa, com todos os
// serviços deles) salvos errado. As vendas do Zeus desses serviços voltam para
// "Para revisar" — ou ficam como ignoradas, se ignorarZeus.
func (b *Banco) ApagarVarios(ids, placas []string, ignorarZeus bool) (Apagados, error) {
	var r Apagados
	tirarID := map[string]bool{}
	for _, id := range ids {
		tirarID[id] = true
	}
	tirarPlaca := map[string]bool{}
	for _, p := range placas {
		if p = NormalizarPlaca(p); p != "" {
			tirarPlaca[p] = true
		}
	}
	b.mu.Lock()
	refs := map[string]bool{}
	var ficam []*Servico
	for _, s := range b.c.Servicos {
		if tirarID[s.ID] || tirarPlaca[s.Placa] {
			r.Servicos++
			for _, ref := range splitRefs(s.ZeusRef) {
				refs[ref] = true
			}
			continue
		}
		ficam = append(ficam, s)
	}
	b.c.Servicos = ficam
	for p := range tirarPlaca {
		if _, ok := b.c.Veiculos[p]; ok {
			delete(b.c.Veiculos, p)
			r.Carros++
		}
	}
	for _, l := range b.c.Lancamentos {
		if refs[l.Ref] || (tirarPlaca[l.Placa] && l.Status == Registrado) {
			l.ServicoID = ""
			if ignorarZeus {
				l.Status = Ignorado
			} else {
				l.Status = Pendente
				r.Voltaram = append(r.Voltaram, l.Ref)
			}
		}
	}
	if r.Servicos == 0 && r.Carros == 0 {
		b.mu.Unlock()
		return r, ErrNaoAchado
	}
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	sort.Strings(r.Voltaram)
	if r.Voltaram == nil {
		r.Voltaram = []string{}
	}
	return r, err
}

func splitRefs(s string) []string {
	var out []string
	ini := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > ini {
				out = append(out, s[ini:i])
			}
			ini = i + 1
		}
	}
	return out
}

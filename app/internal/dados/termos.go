package dados

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// Termo é o termo de responsabilidade assinado pelo cliente quando ele mesmo
// compra a peça: a oficina responde só pela mão de obra.
type Termo struct {
	ID            string    `json:"id"`
	Numero        int       `json:"numero"`
	Data          string    `json:"data"` // AAAA-MM-DD
	Cliente       string    `json:"cliente"`
	Documento     string    `json:"documento,omitempty"` // CPF ou RG
	Telefone      string    `json:"telefone,omitempty"`
	Placa         string    `json:"placa,omitempty"`
	Modelo        string    `json:"modelo,omitempty"`
	Ano           string    `json:"ano,omitempty"`
	KM            int       `json:"km,omitempty"`
	Pecas         []string  `json:"pecas"`             // peças trazidas pelo cliente
	Servico       string    `json:"servico,omitempty"` // mão de obra a fazer
	Obs           string    `json:"obs,omitempty"`
	Assinatura    string    `json:"assinatura,omitempty"` // imagem PNG (data:...) da assinatura na tela
	AssinadoPapel bool      `json:"assinado_papel,omitempty"`
	AssinadoEm    time.Time `json:"assinado_em,omitempty"`
	Assinado      bool      `json:"assinado"`
	ZeusRef       string    `json:"zeus_ref,omitempty"`
	CriadoEm      time.Time `json:"criado_em"`
	AlteradoEm    time.Time `json:"alterado_em"`
}

var (
	ErrTermoCliente = errors.New("escreva o nome do cliente")
	ErrTermoPeca    = errors.New("escreva a peça (ou peças) que o cliente trouxe")
	ErrTermoAssin   = errors.New("assinatura inválida: limpe e assine de novo")
)

const prefixoAssinatura = "data:image/png;base64,"

func (t *Termo) limpar() error {
	t.Cliente = strings.Join(strings.Fields(t.Cliente), " ")
	t.Documento = strings.TrimSpace(t.Documento)
	t.Telefone = strings.TrimSpace(t.Telefone)
	t.Modelo = strings.TrimSpace(t.Modelo)
	t.Ano = strings.TrimSpace(t.Ano)
	t.Servico = strings.TrimSpace(t.Servico)
	t.Obs = strings.TrimSpace(t.Obs)
	if t.Cliente == "" {
		return ErrTermoCliente
	}
	var pecas []string
	for _, p := range t.Pecas {
		if p = strings.TrimSpace(p); p != "" {
			pecas = append(pecas, p)
		}
	}
	if len(pecas) == 0 {
		return ErrTermoPeca
	}
	t.Pecas = pecas
	if strings.TrimSpace(t.Placa) != "" {
		p := NormalizarPlaca(t.Placa)
		if p == "" {
			return ErrPlaca
		}
		t.Placa = p
	}
	if !reData.MatchString(t.Data) {
		t.Data = time.Now().Format("2006-01-02")
	}
	if t.KM < 0 {
		t.KM = 0
	}
	if t.Assinatura != "" && (!strings.HasPrefix(t.Assinatura, prefixoAssinatura) || len(t.Assinatura) > 400_000) {
		return ErrTermoAssin
	}
	return nil
}

// SalvarTermo cria (sem ID) ou altera um termo. O número é sequencial.
func (b *Banco) SalvarTermo(t Termo) (Termo, error) {
	if err := t.limpar(); err != nil {
		return Termo{}, err
	}
	agora := time.Now()
	b.mu.Lock()
	var atual *Termo
	if t.ID != "" {
		for _, x := range b.c.Termos {
			if x.ID == t.ID {
				atual = x
				break
			}
		}
		if atual == nil {
			b.mu.Unlock()
			return Termo{}, ErrNaoAchado
		}
		t.Numero, t.CriadoEm = atual.Numero, atual.CriadoEm
		if t.AssinadoEm.IsZero() {
			t.AssinadoEm = atual.AssinadoEm
		}
	} else {
		max := 0
		for _, x := range b.c.Termos {
			if x.Numero > max {
				max = x.Numero
			}
		}
		t.ID = aleatorio(10, alfabeto)
		t.Numero, t.CriadoEm = max+1, agora
		atual = &Termo{}
		b.c.Termos = append(b.c.Termos, atual)
	}
	t.Assinado = t.Assinatura != "" || t.AssinadoPapel
	switch {
	case !t.Assinado:
		t.AssinadoEm = time.Time{}
	case t.AssinadoEm.IsZero():
		t.AssinadoEm = agora
	}
	t.AlteradoEm = agora
	*atual = t
	err := b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return t, err
}

// Termo devolve o termo completo (com a assinatura).
func (b *Banco) Termo(id string) (Termo, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, x := range b.c.Termos {
		if x.ID == id {
			return *x, true
		}
	}
	return Termo{}, false
}

// Termos lista (mais novo primeiro), sem a imagem da assinatura. placa e q
// filtram (q procura em cliente, placa, modelo, peças e número).
func (b *Banco) Termos(placa, q string, limite int) []Termo {
	placa = NormalizarPlaca(placa)
	qn := strings.ToUpper(semAcento(strings.TrimSpace(q)))
	qp := strings.NewReplacer("-", "", " ", "").Replace(qn)
	b.mu.RLock()
	var out []Termo
	for _, x := range b.c.Termos {
		if placa != "" && x.Placa != placa {
			continue
		}
		if qn != "" {
			texto := strings.ToUpper(semAcento(strings.Join(append([]string{x.Cliente, x.Placa, x.Modelo, x.Documento, x.Servico}, x.Pecas...), " ")))
			num := strings.TrimLeft(qn, "Nº0 ")
			if !strings.Contains(texto, qn) && !(qp != "" && strings.Contains(x.Placa, qp)) && !(num != "" && itoa(x.Numero) == num) {
				continue
			}
		}
		c := *x
		c.Assinatura = ""
		out = append(out, c)
	}
	b.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Numero > out[j].Numero })
	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

// ApagarTermo tira o termo (o número não é reaproveitado se não for o último).
func (b *Banco) ApagarTermo(id string) error {
	b.mu.Lock()
	for i, x := range b.c.Termos {
		if x.ID == id {
			b.c.Termos = append(b.c.Termos[:i], b.c.Termos[i+1:]...)
			err := b.mudou()
			b.mu.Unlock()
			if err == nil {
				b.avisar()
			}
			return err
		}
	}
	b.mu.Unlock()
	return ErrNaoAchado
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

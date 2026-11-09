package dados

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Sal igual ao "sal" do index.html (e ao SAL_PUBLICO do Apps Script).
const Sal = "automec135|"

// IDPublico transforma o código do QR num identificador que não dá para reverter.
func IDPublico(codigo string) string {
	h := sha256.Sum256([]byte(Sal + strings.ToUpper(codigo)))
	return hex.EncodeToString(h[:10])
}

// CabecalhoPublico tem as mesmas colunas da aba Público da planilha.
// (as 4 últimas vieram na versão 1.5; a página antiga ignora colunas a mais)
var CabecalhoPublico = []string{"id", "modelo", "ano", "data", "km", "itens", "outros", "proxima_troca_km", "obs",
	"proxima_troca_data", "proxima_correia_km", "proxima_correia_data", "garantia", "placa_cifrada"}

// CifrarPlaca tranca a placa com o código do QR (AES-GCM, chave = SHA-256("placa|"+código)).
// No arquivo público ela fica embaralhada; só a página aberta pelo QR daquele carro
// (que conhece o código) consegue mostrar. O nonce vem da própria placa, para o
// arquivo não mudar à toa a cada publicação.
func CifrarPlaca(placa, codigo string) string {
	if placa == "" || codigo == "" {
		return ""
	}
	chave := sha256.Sum256([]byte("placa|" + strings.ToUpper(codigo)))
	bloco, err := aes.NewCipher(chave[:])
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(bloco)
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, chave[:])
	mac.Write([]byte(placa))
	nonce := mac.Sum(nil)[:gcm.NonceSize()]
	return base64.StdEncoding.EncodeToString(gcm.Seal(append([]byte{}, nonce...), nonce, []byte(placa), nil))
}

// OpcoesPublico: o que o programa calcula para a página (mesmas regras da etiqueta).
type OpcoesPublico struct {
	MesesOleo int // próxima troca de óleo por tempo
	// próxima troca da correia dentada, se o serviço teve correia
	Correia func(s Servico) (km int, data string, tem bool)
	// a peça foi do cliente (tem termo): garantia só da mão de obra
	PecaDoCliente func(s Servico) bool
}

// ProximaDataOleo: a data marcada no serviço ou data + meses (só com troca de óleo).
func ProximaDataOleo(s Servico, meses int) string {
	if s.ProximaData != "" {
		return s.ProximaData
	}
	tem := false
	for _, it := range s.Itens {
		if it == "Óleo do motor" {
			tem = true
		}
	}
	if !tem {
		return ""
	}
	if meses <= 0 {
		meses = 6
	}
	d, err := time.Parse("2006-01-02", s.Data)
	if err != nil {
		return ""
	}
	return d.AddDate(0, meses, 0).Format("2006-01-02")
}

// CSVPublico monta o arquivo que a página do QR lê. Só tem dados que o cliente
// pode ver: nada de placa, código, nome, telefone ou observação interna.
func CSVPublico(veiculos map[string]Veiculo, servicos []Servico, agora time.Time, opcoes ...OpcoesPublico) []byte {
	var op OpcoesPublico
	if len(opcoes) > 0 {
		op = opcoes[0]
	}
	porPlaca := map[string][]Servico{}
	for _, s := range servicos {
		porPlaca[s.Placa] = append(porPlaca[s.Placa], s)
	}
	placas := make([]string, 0, len(veiculos))
	for p := range veiculos {
		placas = append(placas, p)
	}
	sort.Strings(placas)

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write(CabecalhoPublico)
	vazio := func(n int) []string { return make([]string, n) }
	w.Write(append([]string{"_", agora.Format("2006-01-02T15:04")}, vazio(len(CabecalhoPublico)-2)...))
	for _, p := range placas {
		v := veiculos[p]
		codigos := append([]string{v.Codigo}, v.Extras...)
		ss := porPlaca[p]
		ordenarServicos(ss)
		for _, cod := range codigos {
			id, pc := IDPublico(cod), CifrarPlaca(v.Placa, cod)
			if len(ss) == 0 {
				w.Write(append(append([]string{id, v.Modelo, v.Ano}, vazio(len(CabecalhoPublico)-4)...), pc))
				continue
			}
			for _, s := range ss {
				km, prox := "", ""
				if s.KM > 0 {
					km = strconv.Itoa(s.KM)
				}
				if s.ProximaKM > 0 {
					prox = strconv.Itoa(s.ProximaKM)
				}
				ckm, cdata, gar := "", "", ""
				if op.Correia != nil {
					if k, d, tem := op.Correia(s); tem {
						if k > 0 {
							ckm = strconv.Itoa(k)
						}
						cdata = d
					}
				}
				if op.PecaDoCliente != nil && op.PecaDoCliente(s) {
					gar = "mao_de_obra"
				}
				w.Write([]string{id, v.Modelo, v.Ano, s.Data, km,
					strings.Join(s.Itens, "; "), strings.Join(s.Outros, "\n"), prox, s.ObsCliente,
					ProximaDataOleo(s, op.MesesOleo), ckm, cdata, gar, pc})
			}
		}
	}
	w.Flush()
	return buf.Bytes()
}

package servidor

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"oficinaqr/internal/atualizar"
	"oficinaqr/internal/dados"
	"oficinaqr/internal/etiqueta"
	"oficinaqr/internal/registro"
)

func (s *Servidor) rotasExtras(m *http.ServeMux) {
	m.HandleFunc("GET /api/lembretes", s.apiLembretes)
	m.HandleFunc("POST /api/lembretes/{placa}/avisado", s.apiLembreteAvisado)
	m.HandleFunc("GET /api/relatorio", s.apiRelatorio)
	m.HandleFunc("GET /api/atualizacao", s.apiAtualizacao)
	m.HandleFunc("POST /api/atualizacao/instalar", s.apiInstalarAtualizacao)
	s.rotasTermos(m)
	s.rotasRede(m)
	m.HandleFunc("GET /api/hoje", s.apiHoje)
	m.HandleFunc("GET /api/copias", func(w http.ResponseWriter, r *http.Request) {
		l := s.Banco.Copias()
		if l == nil {
			l = []dados.Copia{}
		}
		responder(w, 200, l)
	})
	m.HandleFunc("POST /api/copias/restaurar", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Nome string `json:"nome"`
			Onde string `json:"onde"`
		}
		if err := lerJSON(r, &p); err != nil {
			falhar(w, err)
			return
		}
		if err := s.Banco.Restaurar(p.Nome, p.Onde); err != nil {
			falhar(w, err)
			return
		}
		registro.Info("dados restaurados da cópia %s (%s)", p.Nome, p.Onde)
		responder(w, 200, map[string]bool{"ok": true})
	})
}

// apiHoje: resumo do dia para a tela inicial.
func (s *Servidor) apiHoje(w http.ResponseWriter, r *http.Request) {
	hoje := time.Now().Format("2006-01-02")
	veiculos, servicos := s.Banco.Tudo()
	n, impressos := 0, 0
	for _, sv := range servicos {
		if sv.CriadoEm.Format("2006-01-02") == hoje || sv.Data == hoje {
			n++
			if sv.Impressoes > 0 {
				impressos++
			}
		}
	}
	novos := 0
	for _, v := range veiculos {
		if v.CriadoEm.Format("2006-01-02") == hoje {
			novos++
		}
	}
	falta := 0
	for _, t := range s.Banco.Termos("", "", 0) {
		if !t.Assinado {
			falta++
		}
	}
	responder(w, 200, map[string]int{"servicos": n, "impressos": impressos, "carros_novos": novos,
		"pendentes": len(s.Banco.Lancamentos(dados.Pendente, 0)), "termos_sem_assinatura": falta})
}

/* ----------------------------------------------------------- lembretes --- */

type lembrete struct {
	Placa       string `json:"placa"`
	Modelo      string `json:"modelo"`
	Ano         string `json:"ano"`
	Cliente     string `json:"cliente"`
	Telefone    string `json:"telefone"`
	Ultima      string `json:"ultima"` // data da última troca de óleo
	KM          int    `json:"km,omitempty"`
	ProximaKM   int    `json:"proxima_km,omitempty"`
	Meses       int    `json:"meses"`    // meses desde a última troca
	Situacao    string `json:"situacao"` // atrasado, chegando
	AvisadoEm   string `json:"avisado_em,omitempty"`
	Avisado     bool   `json:"avisado"` // já avisado depois da última troca
	Tipo        string `json:"tipo"`    // "oleo" ou "correia"
	ProximaData string `json:"proxima_data,omitempty"`
	WhatsApp    string `json:"whatsapp,omitempty"`
	Mensagem    string `json:"mensagem"`
}

func mesesEntre(de, ate time.Time) int {
	m := (ate.Year()-de.Year())*12 + int(ate.Month()-de.Month())
	if ate.Day() < de.Day() {
		m--
	}
	return m
}

func (s *Servidor) apiLembretes(w http.ResponseWriter, r *http.Request) {
	limite := s.cfg.Ler().MesesLembrete
	if limite <= 0 {
		limite = 6
	}
	veiculos, servicos := s.Banco.Tudo()
	ultimaTroca := map[string]dados.Servico{}
	for _, sv := range servicos {
		temOleo := false
		for _, it := range sv.Itens {
			if it == "Óleo do motor" {
				temOleo = true
			}
		}
		if !temOleo {
			continue
		}
		if u, ok := ultimaTroca[sv.Placa]; !ok || sv.Data > u.Data {
			ultimaTroca[sv.Placa] = sv
		}
	}
	hoje := time.Now()
	site := s.configEtiqueta().Site
	out := []lembrete{}
	for placa, sv := range ultimaTroca {
		d, err := time.Parse("2006-01-02", sv.Data)
		if err != nil {
			continue
		}
		meses := mesesEntre(d, hoje)
		if meses < limite-1 {
			continue
		}
		v := veiculos[placa]
		l := lembrete{Placa: placa, Modelo: v.Modelo, Ano: v.Ano, Cliente: v.Cliente, Telefone: v.Telefone, Ultima: sv.Data,
			KM: sv.KM, ProximaKM: sv.ProximaKM, Meses: meses, Situacao: "chegando", AvisadoEm: v.AvisadoEm, Tipo: "oleo"}
		if meses >= limite {
			l.Situacao = "atrasado"
		}
		l.Avisado = v.AvisadoEm != "" && v.AvisadoEm >= sv.Data
		l.Mensagem = mensagemLembrete(v, sv, etiqueta.Link(site, v.Codigo))
		if tel := soDigitos(v.Telefone); len(tel) >= 10 {
			if !strings.HasPrefix(tel, "55") {
				tel = "55" + tel
			}
			l.WhatsApp = "https://wa.me/" + tel + "?text=" + url.QueryEscape(l.Mensagem)
		}
		out = append(out, l)
	}
	out = append(out, s.lembretesCorreia(veiculos, servicos, hoje, site)...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Avisado != out[j].Avisado {
			return !out[i].Avisado
		}
		return out[i].Ultima < out[j].Ultima
	})
	responder(w, 200, map[string]any{"meses": limite, "lista": out})
}

func soDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dataBR(iso string) string {
	if len(iso) == 10 {
		return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
	}
	return iso
}

func mensagemLembrete(v dados.Veiculo, sv dados.Servico, link string) string {
	nome := strings.TrimSpace(v.Cliente)
	if i := strings.IndexByte(nome, ' '); i > 0 {
		nome = nome[:i]
	}
	if nome != "" {
		nome = strings.ToUpper(nome[:1]) + strings.ToLower(nome[1:])
	}
	carro := strings.TrimSpace(v.Modelo)
	if carro == "" {
		carro = "carro"
	}
	m := "Olá"
	if nome != "" {
		m += ", " + nome
	}
	m += "! Aqui é da Auto Mecânica Itabaiana. A última troca de óleo do seu " + carro + " (" + dados.MascararPlaca(v.Placa) + ") foi em " + dataBR(sv.Data)
	if sv.KM > 0 {
		m += ", com " + etiqueta.Milhar(sv.KM) + " km"
	}
	m += "."
	if sv.ProximaKM > 0 {
		m += " A próxima está marcada para " + etiqueta.Milhar(sv.ProximaKM) + " km."
	}
	m += " Já está na hora de revisar. Quer agendar um horário? Histórico do carro: " + link
	return m
}

func (s *Servidor) apiLembreteAvisado(w http.ResponseWriter, r *http.Request) {
	if err := s.Banco.MarcarAvisado(r.PathValue("placa"), time.Now().Format("2006-01-02")); err != nil {
		falhar(w, err)
		return
	}
	responder(w, 200, map[string]bool{"ok": true})
}

/* ----------------------------------------------------------- relatório --- */

type contagem struct {
	Nome string `json:"nome"`
	N    int    `json:"n"`
}

func (s *Servidor) apiRelatorio(w http.ResponseWriter, r *http.Request) {
	mes := r.URL.Query().Get("mes") // AAAA-MM
	if len(mes) != 7 {
		mes = time.Now().Format("2006-01")
	}
	veiculos, servicos := s.Banco.Tudo()
	primeiro := map[string]string{} // primeira visita de cada carro
	ultimo := map[string]string{}
	for _, sv := range servicos {
		if p, ok := primeiro[sv.Placa]; !ok || sv.Data < p {
			primeiro[sv.Placa] = sv.Data
		}
		if sv.Data > ultimo[sv.Placa] {
			ultimo[sv.Placa] = sv.Data
		}
	}
	carros := map[string]bool{}
	itens := map[string]int{}
	outros := map[string]int{}
	porDia := map[string]int{}
	nServ, nZeus, novos := 0, 0, 0
	for _, sv := range servicos {
		if !strings.HasPrefix(sv.Data, mes) {
			continue
		}
		nServ++
		if sv.Origem == "zeus" {
			nZeus++
		}
		carros[sv.Placa] = true
		porDia[sv.Data]++
		for _, it := range sv.Itens {
			itens[it]++
		}
		for _, o := range sv.Outros {
			outros[o]++
		}
	}
	for placa := range carros {
		if strings.HasPrefix(primeiro[placa], mes) {
			novos++
		}
	}
	// clientes que não voltam há mais de 6 meses
	limite := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	type sumido struct {
		Placa   string `json:"placa"`
		Modelo  string `json:"modelo"`
		Cliente string `json:"cliente"`
		Ultimo  string `json:"ultimo"`
	}
	sumidos := []sumido{}
	for placa, u := range ultimo {
		if u < limite {
			v := veiculos[placa]
			sumidos = append(sumidos, sumido{placa, v.Modelo, v.Cliente, u})
		}
	}
	sort.Slice(sumidos, func(i, j int) bool { return sumidos[i].Ultimo > sumidos[j].Ultimo })
	ordenar := func(m map[string]int, max int) []contagem {
		l := []contagem{}
		for k, n := range m {
			l = append(l, contagem{k, n})
		}
		sort.Slice(l, func(i, j int) bool {
			if l[i].N != l[j].N {
				return l[i].N > l[j].N
			}
			return l[i].Nome < l[j].Nome
		})
		if len(l) > max {
			l = l[:max]
		}
		return l
	}
	dias := []contagem{}
	for d, n := range porDia {
		dias = append(dias, contagem{d, n})
	}
	sort.Slice(dias, func(i, j int) bool { return dias[i].Nome < dias[j].Nome })
	responder(w, 200, map[string]any{
		"mes": mes, "servicos": nServ, "do_zeus": nZeus, "carros": len(carros), "carros_novos": novos,
		"itens": ordenar(itens, 10), "outros": ordenar(outros, 15), "por_dia": dias,
		"sem_voltar": sumidos, "total_carros": len(veiculos),
	})
}

/* --------------------------------------------------------- atualização --- */

var (
	muAtual    sync.Mutex
	ultimaInfo atualizar.Info
	ultimaVez  time.Time
)

func (s *Servidor) verificarAtualizacao(forcar bool) (atualizar.Info, error) {
	muAtual.Lock()
	defer muAtual.Unlock()
	if !forcar && time.Since(ultimaVez) < 6*time.Hour && ultimaInfo.Atual != "" {
		return ultimaInfo, nil
	}
	in, err := atualizar.Verificar(s.cfg.Site().Repo, Versao)
	if err != nil {
		return in, err
	}
	ultimaInfo, ultimaVez = in, time.Now()
	return in, nil
}

func (s *Servidor) apiAtualizacao(w http.ResponseWriter, r *http.Request) {
	in, err := s.verificarAtualizacao(r.URL.Query().Get("agora") != "")
	if err != nil {
		responder(w, 200, map[string]any{"atual": Versao, "nova": false, "erro": err.Error()})
		return
	}
	responder(w, 200, in)
}

func (s *Servidor) apiInstalarAtualizacao(w http.ResponseWriter, r *http.Request) {
	in, err := s.verificarAtualizacao(true)
	if err != nil {
		falhar(w, err)
		return
	}
	if !in.Nova {
		falhar(w, fmt.Errorf("o programa já está na versão mais nova (%s)", Versao))
		return
	}
	exe, err := os.Executable()
	if err != nil {
		falhar(w, err)
		return
	}
	if err := atualizar.Baixar(in.URL, exe); err != nil {
		falhar(w, err)
		return
	}
	registro.Info("atualizado de %s para %s; reabrindo", Versao, in.Versao)
	s.Reabrir = exe
	responder(w, 200, map[string]any{"ok": true, "versao": in.Versao})
	go func() { time.Sleep(500 * time.Millisecond); s.Fechar() }()
}

// lembretesCorreia: carros com a próxima troca da correia dentada vencida ou
// chegando (em até 30 dias).
func (s *Servidor) lembretesCorreia(veiculos map[string]dados.Veiculo, servicos []dados.Servico, hoje time.Time, site string) []lembrete {
	ce := s.configEtiqueta()
	ultima := map[string]dados.Servico{}
	for _, sv := range servicos {
		if !etiqueta.DetectarCorreia(sv.Itens, sv.Outros).Correia {
			continue
		}
		if u, ok := ultima[sv.Placa]; !ok || sv.Data > u.Data {
			ultima[sv.Placa] = sv
		}
	}
	out := []lembrete{}
	for placa, sv := range ultima {
		km, data := etiqueta.ProximaCorreia(etiqueta.Dados{Data: sv.Data, KM: sv.KM, ProximaCorreiaKM: sv.ProximaCorreiaKM, ProximaCorreiaData: sv.ProximaCorreiaData}, ce)
		prazo, err := time.Parse("2006-01-02", data)
		if err != nil || prazo.After(hoje.AddDate(0, 0, 30)) {
			continue
		}
		d, _ := time.Parse("2006-01-02", sv.Data)
		v := veiculos[placa]
		l := lembrete{Placa: placa, Modelo: v.Modelo, Ano: v.Ano, Cliente: v.Cliente, Telefone: v.Telefone, Ultima: sv.Data,
			KM: sv.KM, ProximaKM: km, ProximaData: data, Meses: mesesEntre(d, hoje), Situacao: "chegando", AvisadoEm: v.AvisadoEm, Tipo: "correia"}
		if !prazo.After(hoje) {
			l.Situacao = "atrasado"
		}
		// avisado nos últimos 45 dias (depois da troca)
		l.Avisado = v.AvisadoEm != "" && v.AvisadoEm >= sv.Data && v.AvisadoEm >= hoje.AddDate(0, 0, -45).Format("2006-01-02")
		carro := strings.TrimSpace(v.Modelo)
		if carro == "" {
			carro = "carro"
		}
		m := "Olá"
		if nome := primeiroNome(v.Cliente); nome != "" {
			m += ", " + nome
		}
		m += "! Aqui é da Auto Mecânica Itabaiana. A correia dentada do seu " + carro + " (" + dados.MascararPlaca(placa) + ") foi trocada em " + dataBR(sv.Data) +
			" e a próxima troca está marcada para " + dataBR(data)
		if km > 0 {
			m += " ou " + etiqueta.Milhar(km) + " km, o que vier primeiro"
		}
		m += ". Correia vencida pode arrebentar e danificar o motor. Quer agendar? Histórico do carro: " + etiqueta.Link(site, v.Codigo)
		l.Mensagem = m
		if tel := soDigitos(v.Telefone); len(tel) >= 10 {
			if !strings.HasPrefix(tel, "55") {
				tel = "55" + tel
			}
			l.WhatsApp = "https://wa.me/" + tel + "?text=" + url.QueryEscape(m)
		}
		out = append(out, l)
	}
	return out
}

func primeiroNome(nome string) string {
	nome = strings.TrimSpace(nome)
	if i := strings.IndexByte(nome, ' '); i > 0 {
		nome = nome[:i]
	}
	if nome == "" {
		return ""
	}
	r := []rune(nome)
	return strings.ToUpper(string(r[:1])) + strings.ToLower(string(r[1:]))
}

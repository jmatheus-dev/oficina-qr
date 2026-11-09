package itens

import (
	"strings"
	"testing"
)

func TestClassificar(t *testing.T) {
	casos := map[string]string{
		"OLEO LUBRAX 5W30 SINTETICO 1L":      "Óleo do motor",
		"Óleo Mobil Super 15W40 1L":          "Óleo do motor",
		"OLEO 20W50 SEMI-SINTETICO":          "Óleo do motor",
		"LUBRIFICANTE SHELL HELIX HX7 10W40": "Óleo do motor",
		"FILTRO DE OLEO TECFIL PSL55":        "Filtro de óleo",
		"FILTRO OLEO FRAM PH5949":            "Filtro de óleo",
		"FILT. OLEO MANN W712/95":            "Filtro de óleo",
		"FILTRO DE AR TECFIL ARL6079":        "Filtro de ar",
		"FILTRO AR MOTOR COROLLA":            "Filtro de ar",
		"FILTRO DE COMBUSTIVEL GI04/7":       "Filtro de combustível",
		"FILTRO COMB. FRAM G5767":            "Filtro de combustível",
		"FILTRO DE CABINE ACP450":            "Filtro de cabine",
		"FILTRO AR CONDICIONADO WEGA AKX":    "Filtro de cabine",
		"FILTRO ANTI POLEN":                  "Filtro de cabine",
		"FLUIDO DE FREIO DOT 4 500ML":        "Óleo de freio",
		"OLEO DE FREIO DOT3":                 "Óleo de freio",
		"OLEO CAMBIO 75W90 GL4":              "Óleo de câmbio",
		"OLEO TRANSMISSAO AUTOMATICA ATF":    "Óleo de câmbio",
		"FLUIDO ATF DEXRON III":              "Óleo de câmbio",
		"MANGUEIRA DE AR":                    "",
		"MANGUEIRA DO RADIADOR SUPERIOR":     "",
		"PASTILHA DE FREIO DIANTEIRA COBREQ": "",
		"JUNTA DA TAMPA DE VALVULAS":         "",
		"BUJAO DO CARTER OLEO":               "",
		"RETENTOR VIRABREQUIM":               "",
		"OLEO 2T MOTO":                       "",
		"FILTRO SECADOR AR CONDICIONADO":     "",
		"SENSOR DE PRESSAO DE OLEO":          "",
		"CORREIA DENTADA GATES":              "",
		"ADITIVO RADIADOR PARAFLU":           "",
		"DESENGRIPANTE WD-40":                "",
		"VELA DE IGNICAO NGK":                "",
		"AMORTECEDOR DIANTEIRO COFAP":        "",
		"BOMBA DE OLEO":                      "",
	}
	for d, quer := range casos {
		if got := Classificar(d); got != quer {
			t.Errorf("%q: veio %q, queria %q", d, got, quer)
		}
	}
}

func TestSepararEBonito(t *testing.T) {
	p, o := Separar([]string{"OLEO LUBRAX 5W30 1L", "OLEO LUBRAX 5W30 1L", "FILTRO OLEO PSL55", "MANGUEIRA DE AR", "MAO DE OBRA", "PASTILHA DE FREIO DIANT. VW GOL 1.6"})
	if len(p) != 2 || p[0] != "Óleo do motor" || p[1] != "Filtro de óleo" {
		t.Errorf("padrão: %v", p)
	}
	if len(o) != 2 || o[0] != "Mangueira de ar" || o[1] != "Pastilha de freio dianteira" {
		t.Errorf("outros: %q", o)
	}
	if Bonito("Troca da correia") != "Troca da correia" {
		t.Error("texto já com minúsculas deve ficar igual")
	}
}

func TestIgnorarGenericosDoZeus(t *testing.T) {
	for _, d := range []string{"SERVICOS MECANICO", "Serviços mecânicos", "COMBUSTIVEL CARRO DO CLIENTE"} {
		if !Ignorar(d) {
			t.Errorf("%q devia ficar de fora", d)
		}
	}
	for _, d := range []string{"ALTERNADOR FIAT: UNO 2005 MOTOR FIRE:", "FILTRO DE COMBUSTIVEL"} {
		if Ignorar(d) {
			t.Errorf("%q não pode ficar de fora", d)
		}
	}
}

func TestNomesDoZeus(t *testing.T) {
	casos := map[string]string{
		"LANTERN TRAS FIAT: UNO 2005/ LD DIREIT  CRISTAL":              "Lanterna traseira",
		"BOMBA AGUA/  SCHADEK/            GOL/1.0/G4/AUDI A3/POLO1.6/": "Bomba água",
		"ALTERNADOR FIAT: UNO 2005 MOTOR FIRE:":                        "Alternador",
		"CORREIA DE ALTERNAD FIAT FIRE: 1.0/ 1.3/ 1.4":                 "Correia de alternador",
		"RETENTOR VIRAB./TRAS/VOL.              GOL/POLO/FOX/AUDI A3":  "Retentor virabrequim",
		"JUNTA CABEÇOTE VW                SAVEIRO/GOL/G5/G6/POLO/G2":   "Junta cabeçote",
		"COXIM MTR LADO DIREITO VW: JETTA, GOLF, TIGUAN":               "Coxim motor lado direito",
		"SERVICO  TECNICO  DA   RETIFICA":                              "Serviço da retífica",
		"SERVICO  DO  ALINHAMENTO E CAMBAGEM":                          "Serviço do alinhamento e cambagem",
		"SERVICO  TECNICO: ABRIR ROSCA, COLOCAR BUCHA OU ELICOIDE":     "Abrir rosca, colocar bucha ou helicoide",
		"PISTAO COM ANEIS ( STAND ) VW, MTR POWER":                     "Pistão com anéis",
		"ADITIVO DE RADIADOR ORG ROSA 1L (WURTH)":                      "Aditivo de radiador orgânico rosa 1L",
		"PASTILHA FREIO DIANTEIRA GM":                                  "Pastilha freio dianteira",
		"BOMBA OLEO DO CAMBIO VW: JETTA, GOLF 2.0 8V 2011...":          "Bomba óleo do câmbio",
		"LIMPEZA  TECNICA   DE  ARREIFECIMENTO":                        "Limpeza técnica de arrefecimento",
		"MANGUEIRA DE AR":                                              "Mangueira de ar",
		"CORREIA DENTADA GATES":                                        "Correia dentada gates",
	}
	for d, quer := range casos {
		if got := Bonito(Limpar(d)); got != quer {
			t.Errorf("%q → %q, queria %q", d, got, quer)
		}
	}
}

func TestIgnorarConsumoEFerragem(t *testing.T) {
	for _, d := range []string{"SERVICO TECNICO MECANICO", "SERVICO  TECNICO  DO  MECANICO", "SERVICOS", "TROCA",
		"DESENGRAXANTE EXPRESS 500ML/310G (WURTH)", "SILICONE NEUTRO ALTA TEMP HT BLACK 60G BLI ( WURTH)",
		"CAR 80 DESCARBONIZANTE 300 ML/SNAP-ON", "LIMPA RADIADOR 200ML", "LIMPAR FREIO WURTH", "SACHE GRAXA GRAFITADA 80G (WURTH)",
		"PARAFUSO 10", "PORCA COMUM 13", "ARRUELA  10  DE  PRESSAO", "PARAFUSO  17  GRANDE", "COLA  VEDA  ESCAPE", "FITA  ISOLANTE",
		"COMBUSTIVEL CARRO DE CLIENTA", "SERVICO DE TAXI, TRAZER MERCADORIAS PRA ITABAIANA."} {
		if !Ignorar(d) {
			t.Errorf("%q devia ficar de fora", d)
		}
	}
	for _, d := range []string{"PARAFUSO CABEÇOTE JG/SABO", "SERVICO  TECNICO  DA   RETIFICA", "DESCARBONIZACAO DO MOTOR",
		"LIMPEZA  TECNICA   DE  ARREIFECIMENTO", "ADITIVO DE RADIADOR ORG ROSA 1L (WURTH)", "PARAFUSO DE RODA  UNO( CROMADO)"} {
		if Ignorar(d) {
			t.Errorf("%q não pode ficar de fora", d)
		}
	}
	casos := map[string]string{
		"FILTRO LUB./ LB 560/1 (VOX)      GOL/POLO/FOX":                     "Filtro de óleo",
		"FILTRO LUB. LE 119/ VOX/         TORO/RENEGADE/SIENA/PALIO/STRADA": "Filtro de óleo",
		"OLEO UNIGEROL 90/INGRAX             CAIXA DE MARCHA":               "Óleo de câmbio",
		"OLEO ULTRAGEAR MTF 75W80 (IPIRANGA)":                               "Óleo de câmbio",
		"OLEO 15W40 SYNTIUM 800AM SEMISINT. 1L/PETRONAS":                    "Óleo do motor",
		"TROCA DE OLEO DO CAMBIO":                                           "Óleo de câmbio",
		"FLUIDO PARA FREIO DOT 4 (RADNAQ)":                                  "Óleo de freio",
	}
	for d, quer := range casos {
		if got := Classificar(d); got != quer {
			t.Errorf("%q: veio %q, queria %q", d, got, quer)
		}
	}
}

// nomes reais do cadastro do Zeus da oficina (auditoria de 22/10/2026)
func TestNomesReaisDoZeus(t *testing.T) {
	casos := map[string]string{
		// não são itens da etiqueta
		"BOMBA OLEO DO CAMBIO VW: JETTA, GOLF 2.0 8V 2011...": "",
		"ANEL 28.17 X 3.53 TPA OLEO FIRE/ OURINGUE":           "",
		"DEFLETOR DE OLEO":                             "",
		"INTERRUPTOR DE OLEO":                          "",
		"INTERRUPTOR PRESSAO DE OLEO":                  "",
		"KIT CORREIA DENTADA BANHADA OLEO/DAYCO/":      "",
		"MANG RESP OLEO GOL 02/ F":                     "",
		"MANGOTE RESPIRO OLEO ANTI CHAMA":              "",
		"VALVULA RETENCAO OLEO CRUZE":                  "",
		"MANGUEIRA FILTRO AR":                          "",
		"MANG FILTRO AR CELTA/COR":                     "",
		"ANEL FILTRO BICO INJ":                         "",
		"PRE-FILTRO":                                   "",
		"FILTRO CAMBIO AUTOMATICO/WEGA/WEOC002/ KICKS": "",
		"OLEO HIDRAULICO (DULUB)":                      "",
		"BOMBA DE FREIO":                               "",
		"MANGUEIRA DE FREIO DIANTEIRA":                 "",
		"PASTILHA FREIO DIANTEIRA GM":                  "",
		// são
		"FILREO DE OLEO":                                        "Filtro de óleo",
		"FILTRO COMB/OLEO/GI04/7/TECFIL/":                       "Filtro de combustível",
		"FILTRO ACD VW: FOX 2003/ GOL G5/G6":                    "Filtro de cabine",
		"ELEMENTO FILTRO AKX1397 WEGA/ LOGAN":                   "Filtro de cabine",
		"ELEMENTO FILTRO WR245 WEGA/ LOGAN":                     "Filtro de ar",
		"FILTR COMB":                                            "Filtro de combustível",
		"FILTRO COMBUSITVEL":                                    "Filtro de combustível",
		"FILTRO LUB. LBM 1/ VOX/ UNO/FIORINO":                   "Filtro de óleo",
		"FILTRO LUB. WOE 703/ WEGA/ HYUNDAI":                    "Filtro de óleo",
		"OLEO 90 (DULUB)":                                       "Óleo de câmbio",
		"OLEO DE CAIXA 90":                                      "Óleo de câmbio",
		"OLEO ATF TIPO A UNI FTA":                               "Óleo de câmbio",
		"OLEO TOTAL TRAX. GEAR 8 75W80/T.ENERGIES":              "Óleo de câmbio",
		"OLEO ROBO CAMBIO MULTI SYNTH(DCT/DSG)":                 "Óleo de câmbio",
		"OLEO BXO/ 5W30/GA/ MOTORCRAFT FORD":                    "Óleo do motor",
		"OLEO DE MOTOR: 5W30":                                   "Óleo do motor",
		"OLEO MOTOR 15W40 CASTROL DIESEL":                       "Óleo do motor",
		"FLUIDO FR BOSCH DOT 4 500ML":                           "Óleo de freio",
		"LIMPEZA DO SISTEMA DE FREIO, E TROCA DE OLEO DO FREIO": "Óleo de freio",
	}
	for d, quer := range casos {
		if got := Classificar(d); got != quer {
			t.Errorf("%q: veio %q, queria %q", d, got, quer)
		}
	}
	for _, d := range []string{"ARRUELA LISA PO DIN125A M8X16X1,5MM/WURTH 13 ESCURA", "ABRAÇ. SEM FIM ZNB 10X16X9/WURTH",
		"ABRACADEIRA NYLON", "ABRAÇADEIRA PRESSÃO INOX 12.3", "M28B ETIQUETA ADESIVA OLEO - ITABAIANA 60,00X60,00X1"} {
		if !Ignorar(d) {
			t.Errorf("%q devia ficar de fora", d)
		}
	}
	for _, d := range []string{"ARRUELA BUJAO CARTER", "ARRUELA SINCRONIZADA: 1 E 2", "ABRAC P/COIFA HOMOC/DESL TEMPRA"} {
		if Ignorar(d) {
			t.Errorf("%q não pode ficar de fora (é peça)", d)
		}
	}
	if got := Bonito(Limpar("ABRAC P/COIFA HOMOC/DESL TEMPRA")); got != "Abraçadeira para coifa homocinética" {
		t.Errorf("nome: %q", got)
	}
	if got := Bonito(Limpar("MANG RESP OLEO GOL 02/ F")); got != "Mangueira respiro óleo gol 02" {
		t.Errorf("nome: %q", got)
	}
}

func TestRevisarServicosAntigos(t *testing.T) {
	it, ou := Revisar([]string{"Óleo do motor", "Filtro de óleo", "Filtro de combustível"}, []string{
		"Ultrasson de bico alta pressao", "Descarbonizacao do motor", "Car 80 descarbonizante 300 ml/snap-on",
		"Silicone neutro alta temp ht black 60G bli ( wurth)", "Aditivo conc. rad.org. rosa 1L/WURTH", "Agua desmineralizada 1L.",
		"Limpa radiador biod. 250ML/WURTH", "Desengraxante express 500ML/310G /wurth", "Limpeza tecnica de arreifecimento",
		"Servico tecnico mecanico", "Oring vedação DS;12101"})
	if strings.Join(it, ",") != "Óleo do motor,Filtro de óleo,Filtro de combustível" {
		t.Errorf("itens: %v", it)
	}
	quer := "Ultrasson de bico alta pressão|Descarbonização do motor|Aditivo concentrado radiador orgânico rosa 1L|Limpeza técnica de arrefecimento|Oring vedação DS;12101"
	if strings.Join(ou, "|") != quer {
		t.Errorf("outros:\n%q\n%q", strings.Join(ou, "|"), quer)
	}
	it, ou = Revisar(nil, []string{"Lantern tras fiat: uno 2005/ ld direit cristal", "Alternador fiat: uno 2005 motor fire:", "Lampada de 2 polo", "Filtro lub. le 119/ vox/ toro"})
	if strings.Join(it, ",") != "Filtro de óleo" || strings.Join(ou, "|") != "Lanterna traseira|Alternador|Lâmpada de 2 polo" {
		t.Errorf("%v %q", it, ou)
	}
}

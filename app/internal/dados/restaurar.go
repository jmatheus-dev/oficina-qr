package dados

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Copia é uma cópia de segurança (uma por dia) que dá para restaurar.
type Copia struct {
	Nome     string    `json:"nome"`
	Onde     string    `json:"onde"` // "programa" ou "extra" (ex.: OneDrive)
	Data     string    `json:"data"`
	Hora     time.Time `json:"hora"`
	Servicos int       `json:"servicos"`
	Carros   int       `json:"carros"`
	Termos   int       `json:"termos"`
}

func (b *Banco) pastaCopias(onde string) string {
	if onde == "extra" {
		return strings.TrimSpace(b.CopiaExtra)
	}
	return filepath.Join(b.pasta, "copias")
}

func lerConteudo(arq string) (conteudo, error) {
	var c conteudo
	j, err := os.ReadFile(arq)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(j, &c); err != nil {
		return c, errors.New("cópia danificada")
	}
	if c.Veiculos == nil {
		c.Veiculos = map[string]*Veiculo{}
	}
	return c, nil
}

// Copias lista as cópias (mais nova primeiro).
func (b *Banco) Copias() []Copia {
	var out []Copia
	for _, onde := range []string{"programa", "extra"} {
		dir := b.pastaCopias(onde)
		if dir == "" {
			continue
		}
		arqs, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		for _, a := range arqs {
			base := filepath.Base(a)
			if !strings.HasPrefix(base, "dados-") && !strings.HasPrefix(base, "antes-de-") {
				continue
			}
			info, err := os.Stat(a)
			if err != nil {
				continue
			}
			c, err := lerConteudo(a)
			if err != nil {
				continue
			}
			out = append(out, Copia{Nome: base, Onde: onde, Data: info.ModTime().Format("2006-01-02"), Hora: info.ModTime(),
				Servicos: len(c.Servicos), Carros: len(c.Veiculos), Termos: len(c.Termos)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hora.After(out[j].Hora) })
	return out
}

// Restaurar volta os dados para uma cópia. Antes guarda como está agora
// ("antes-de-restaurar-...json"), para dar para desfazer.
func (b *Banco) Restaurar(nome, onde string) error {
	if nome != filepath.Base(nome) || !strings.HasSuffix(nome, ".json") {
		return errors.New("cópia inválida")
	}
	dir := b.pastaCopias(onde)
	if dir == "" {
		return ErrNaoAchado
	}
	c, err := lerConteudo(filepath.Join(dir, nome))
	if err != nil {
		return err
	}
	b.mu.Lock()
	if err := b.guardarAntes("restaurar"); err != nil {
		b.mu.Unlock()
		return err
	}
	b.c = c
	err = b.mudou()
	b.mu.Unlock()
	if err == nil {
		b.avisar()
	}
	return err
}

// guardarAntes grava como os dados estão agora (com a trava já pega).
func (b *Banco) guardarAntes(motivo string) error {
	atual, err := json.MarshalIndent(b.c, "", " ")
	if err != nil {
		return err
	}
	os.MkdirAll(b.pastaCopias("programa"), 0o755)
	return gravarSeguro(filepath.Join(b.pastaCopias("programa"), "antes-de-"+motivo+"-"+time.Now().Format("2006-01-02-150405")+".json"), atual)
}

// GuardarAntes grava uma cópia antes de uma mudança grande (ex.: apagar vários).
func (b *Banco) GuardarAntes(motivo string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.guardarAntes(motivo)
}

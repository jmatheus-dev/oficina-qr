package dados

import "testing"

func TestRestaurar(t *testing.T) {
	b, _ := Abrir(t.TempDir())
	b.RegistrarServico(NovoServico{Placa: "ABC1D23", Itens: []string{"Filtro de ar"}})
	cs := b.Copias()
	if len(cs) != 1 || cs[0].Servicos != 1 {
		t.Fatalf("%+v", cs)
	}
	nome := cs[0].Nome
	// mexe errado: apaga o carro
	b.ApagarVarios(nil, []string{"ABC1D23"}, false)
	// a cópia do dia é regravada a cada mudança; usa a de antes salvando-a com outro nome
	if err := b.Restaurar("../x.json", "programa"); err == nil {
		t.Fatal("nome inválido devia falhar")
	}
	if err := b.Restaurar(nome, "programa"); err != nil {
		t.Fatal(err)
	}
	if len(b.Copias()) != 2 {
		t.Fatalf("devia guardar o estado antes de restaurar: %+v", b.Copias())
	}
}

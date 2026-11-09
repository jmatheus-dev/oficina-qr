// auditar: mostra como cada nome do Zeus é classificado (uso interno, não vai no programa).
package main

import (
	"bufio"
	"fmt"
	"os"

	"oficinaqr/internal/itens"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		n := sc.Text()
		r := "OUTRO: " + itens.Bonito(itens.Limpar(n))
		if itens.Ignorar(n) {
			r = "--ignorado--"
		} else if c := itens.Classificar(n); c != "" {
			r = "ITEM: " + c
		}
		fmt.Printf("%-70.70s => %s\n", n, r)
	}
}

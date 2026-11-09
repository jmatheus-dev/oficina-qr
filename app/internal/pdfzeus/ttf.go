package pdfzeus

import "encoding/binary"

// Leitor mínimo de TrueType: só o necessário para saber que letra cada glifo
// desenha. Os PDFs do Zeus embutem fontes com outras tabelas danificadas
// (ex.: "post"), então não dá para usar um leitor completo.

type ttf struct {
	b      []byte
	tabela map[string][]byte
}

func lerTTF(b []byte) *ttf {
	if len(b) < 12 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(b[4:]))
	t := &ttf{b: b, tabela: map[string][]byte{}}
	for i := 0; i < n; i++ {
		r := 12 + 16*i
		if r+16 > len(b) {
			break
		}
		tag := string(b[r : r+4])
		off := int(binary.BigEndian.Uint32(b[r+8:]))
		tam := int(binary.BigEndian.Uint32(b[r+12:]))
		if off >= 0 && tam >= 0 && off+tam <= len(b) {
			t.tabela[tag] = b[off : off+tam]
		}
	}
	return t
}

func u16(b []byte, i int) int {
	if i < 0 || i+2 > len(b) {
		return 0
	}
	return int(binary.BigEndian.Uint16(b[i:]))
}

func u32(b []byte, i int) int {
	if i < 0 || i+4 > len(b) {
		return 0
	}
	return int(binary.BigEndian.Uint32(b[i:]))
}

// glifos devolve glifo → letra pela tabela cmap (Unicode).
func (t *ttf) glifos() map[uint16]rune {
	out := map[uint16]rune{}
	c := t.tabela["cmap"]
	if len(c) < 4 {
		return out
	}
	n := u16(c, 2)
	melhor, nota := -1, -1
	for i := 0; i < n; i++ {
		r := 4 + 8*i
		pid, eid, off := u16(c, r), u16(c, r+2), u32(c, r+4)
		f := u16(c, off)
		v := -1
		switch {
		case pid == 3 && eid == 10 && f == 12:
			v = 5
		case pid == 0 && f == 12:
			v = 4
		case pid == 3 && eid == 1 && f == 4:
			v = 3
		case pid == 0 && f == 4:
			v = 2
		}
		if v > nota {
			melhor, nota = off, v
		}
	}
	if melhor < 0 {
		return out
	}
	guarda := func(r rune, g int) {
		if g <= 0 || g > 0xFFFF || r < 0x20 {
			return
		}
		if velho, ja := out[uint16(g)]; ja && velho <= r {
			return // prefere a letra "mais simples" (menor código)
		}
		out[uint16(g)] = r
	}
	s := c[melhor:]
	switch u16(s, 0) {
	case 4:
		seg := u16(s, 6) / 2
		fimI, iniI := 14, 16+2*seg
		deltaI, rangeI := iniI+2*seg, iniI+4*seg
		for k := 0; k < seg; k++ {
			fim, ini := u16(s, fimI+2*k), u16(s, iniI+2*k)
			delta := int16(u16(s, deltaI+2*k))
			ro := u16(s, rangeI+2*k)
			if fim < ini || fim == 0xFFFF && ini == 0xFFFF {
				continue
			}
			for ch := ini; ch <= fim && ch < 0xFFFF; ch++ {
				var g int
				if ro == 0 {
					g = (ch + int(delta)) & 0xFFFF
				} else {
					p := rangeI + 2*k + ro + 2*(ch-ini)
					g = u16(s, p)
					if g != 0 {
						g = (g + int(delta)) & 0xFFFF
					}
				}
				guarda(rune(ch), g)
			}
		}
	case 12:
		n := u32(s, 12)
		for k := 0; k < n && k < 100000; k++ {
			r := 16 + 12*k
			ini, fim, g := u32(s, r), u32(s, r+4), u32(s, r+8)
			for ch := ini; ch <= fim && ch-ini < 70000; ch++ {
				guarda(rune(ch), g+(ch-ini))
			}
		}
	}
	return out
}

// ligaduras devolve glifo de ligadura → glifos que a formam (fi → f, i),
// pela tabela GSUB (lookups do tipo 4, também dentro de extensões tipo 7).
func (t *ttf) ligaduras() map[uint16][]uint16 {
	out := map[uint16][]uint16{}
	g := t.tabela["GSUB"]
	if len(g) < 10 {
		return out
	}
	ll := u16(g, 8)
	nl := u16(g, ll)
	for i := 0; i < nl && i < 2000; i++ {
		lk := ll + u16(g, ll+2+2*i)
		tipo := u16(g, lk)
		ns := u16(g, lk+4)
		for j := 0; j < ns && j < 500; j++ {
			st := lk + u16(g, lk+6+2*j)
			tp := tipo
			if tp == 7 && u16(g, st) == 1 {
				tp = u16(g, st+2)
				st += u32(g, st+4)
			}
			if tp != 4 || u16(g, st) != 1 {
				continue
			}
			cobertos := cobertura(g, st+u16(g, st+2))
			nset := u16(g, st+4)
			for k := 0; k < nset && k < len(cobertos); k++ {
				ls := st + u16(g, st+6+2*k)
				nlig := u16(g, ls)
				for m := 0; m < nlig && m < 500; m++ {
					lg := ls + u16(g, ls+2+2*m)
					lig := uint16(u16(g, lg))
					nc := u16(g, lg+2)
					if nc < 2 || nc > 8 {
						continue
					}
					comp := []uint16{cobertos[k]}
					for c := 1; c < nc; c++ {
						comp = append(comp, uint16(u16(g, lg+4+2*(c-1))))
					}
					if _, ja := out[lig]; !ja {
						out[lig] = comp
					}
				}
			}
		}
	}
	return out
}

func cobertura(g []byte, c int) []uint16 {
	var out []uint16
	switch u16(g, c) {
	case 1:
		n := u16(g, c+2)
		for i := 0; i < n && i < 20000; i++ {
			out = append(out, uint16(u16(g, c+4+2*i)))
		}
	case 2:
		n := u16(g, c+2)
		for i := 0; i < n && i < 5000; i++ {
			r := c + 4 + 6*i
			ini, fim := u16(g, r), u16(g, r+2)
			for x := ini; x <= fim && len(out) < 20000; x++ {
				out = append(out, uint16(x))
			}
		}
	}
	return out
}

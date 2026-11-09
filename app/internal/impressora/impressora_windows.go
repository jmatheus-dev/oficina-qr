//go:build windows

package impressora

import (
	"errors"
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"
)

// Conferência dos tamanhos das estruturas do Windows (x64). Se algum estiver
// errado, o programa nem compila.
var (
	_ = [1]struct{}{}[unsafe.Sizeof(printerInfo4{})-24]
	_ = [1]struct{}{}[unsafe.Sizeof(docInfo{})-40]
	_ = [1]struct{}{}[unsafe.Sizeof(bitmapInfo{})-48]
	_ = [1]struct{}{}[unsafe.Offsetof(bitmapInfo{}.paleta)-40]
	_ = [1]struct{}{}[unsafe.Offsetof(docInfo{}.fwType)-32]
)

var (
	winspool               = syscall.NewLazyDLL("winspool.drv")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procEnumPrinters       = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinter  = winspool.NewProc("GetDefaultPrinterW")
	procOpenPrinter        = winspool.NewProc("OpenPrinterW")
	procClosePrinter       = winspool.NewProc("ClosePrinter")
	procDocumentProperties = winspool.NewProc("DocumentPropertiesW")
	procCreateDC           = gdi32.NewProc("CreateDCW")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procStartDoc           = gdi32.NewProc("StartDocW")
	procEndDoc             = gdi32.NewProc("EndDoc")
	procAbortDoc           = gdi32.NewProc("AbortDoc")
	procStartPage          = gdi32.NewProc("StartPage")
	procEndPage            = gdi32.NewProc("EndPage")
	procGetDeviceCaps      = gdi32.NewProc("GetDeviceCaps")
	procStretchDIBits      = gdi32.NewProc("StretchDIBits")
)

const (
	printerEnumLocal       = 2
	printerEnumConnections = 4
	dmOutBuffer            = 2
	dmInBuffer             = 8
	dmOrientation          = 0x1
	dmPaperSize            = 0x2
	dmPaperLength          = 0x4
	dmPaperWidth           = 0x8
	logPixelsX             = 88
	logPixelsY             = 90
	physicalOffsetX        = 112
	physicalOffsetY        = 113
	dibRGBColors           = 0
	srcCopy                = 0x00CC0020
)

func utf16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func texto(p *uint16) string {
	if p == nil {
		return ""
	}
	var r []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		r = append(r, c)
	}
	return syscall.UTF16ToString(r)
}

type printerInfo4 struct {
	nome      *uint16
	servidor  *uint16
	atributos uint32
}

// Listar devolve as impressoras instaladas e a padrão.
func Listar() ([]string, string, error) {
	var precisa, qtd uint32
	flags := uintptr(printerEnumLocal | printerEnumConnections)
	procEnumPrinters.Call(flags, 0, 4, 0, 0, uintptr(unsafe.Pointer(&precisa)), uintptr(unsafe.Pointer(&qtd)))
	var nomes []string
	if precisa > 0 {
		buf := make([]byte, precisa)
		r, _, e := procEnumPrinters.Call(flags, 0, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(precisa),
			uintptr(unsafe.Pointer(&precisa)), uintptr(unsafe.Pointer(&qtd)))
		if r == 0 {
			return nil, "", fmt.Errorf("não deu para listar as impressoras: %v", e)
		}
		tam := unsafe.Sizeof(printerInfo4{})
		for i := uint32(0); i < qtd; i++ {
			pi := (*printerInfo4)(unsafe.Pointer(&buf[uintptr(i)*tam]))
			nomes = append(nomes, texto(pi.nome))
		}
	}
	var n uint32 = 512
	b := make([]uint16, n)
	padrao := ""
	if r, _, _ := procGetDefaultPrinter.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&n))); r != 0 {
		padrao = syscall.UTF16ToString(b)
	}
	return nomes, padrao, nil
}

// devmode pega as configurações do driver e põe o tamanho da etiqueta.
func devmode(nome string, larguraMM, alturaMM float64) []byte {
	var h syscall.Handle
	if r, _, _ := procOpenPrinter.Call(uintptr(unsafe.Pointer(utf16(nome))), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return nil
	}
	defer procClosePrinter.Call(uintptr(h))
	tam, _, _ := procDocumentProperties.Call(0, uintptr(h), uintptr(unsafe.Pointer(utf16(nome))), 0, 0, 0)
	if int32(tam) <= 0 || tam < 92 {
		return nil
	}
	dm := make([]byte, tam)
	r, _, _ := procDocumentProperties.Call(0, uintptr(h), uintptr(unsafe.Pointer(utf16(nome))),
		uintptr(unsafe.Pointer(&dm[0])), 0, dmOutBuffer)
	if int32(r) < 0 || len(dm) < 92 {
		return nil
	}
	if larguraMM > 0 && alturaMM > 0 {
		campos := (*uint32)(unsafe.Pointer(&dm[72]))
		*campos |= dmOrientation | dmPaperSize | dmPaperLength | dmPaperWidth
		*(*int16)(unsafe.Pointer(&dm[76])) = 1 // retrato
		*(*int16)(unsafe.Pointer(&dm[78])) = 0 // tamanho personalizado
		*(*int16)(unsafe.Pointer(&dm[80])) = int16(math.Round(alturaMM * 10))
		*(*int16)(unsafe.Pointer(&dm[82])) = int16(math.Round(larguraMM * 10))
		procDocumentProperties.Call(0, uintptr(h), uintptr(unsafe.Pointer(utf16(nome))),
			uintptr(unsafe.Pointer(&dm[0])), uintptr(unsafe.Pointer(&dm[0])), dmInBuffer|dmOutBuffer)
	}
	return dm
}

type docInfo struct {
	cb     int32
	nome   *uint16
	saida  *uint16
	tipo   *uint16
	fwType uint32
}

type bitmapInfo struct {
	tamanho     uint32
	largura     int32
	altura      int32
	planos      uint16
	bits        uint16
	compressao  uint32
	tamImagem   uint32
	xPPM, yPPM  int32
	cores       uint32
	importantes uint32
	paleta      [2][4]byte
}

// Imprimir manda a etiqueta pelo driver da impressora.
func Imprimir(t Trabalho) error {
	if t.Imagem == nil {
		return errors.New("etiqueta vazia")
	}
	nome := t.Impressora
	if nome == "" {
		_, padrao, _ := Listar()
		nome = padrao
	}
	if nome == "" {
		return errors.New("nenhuma impressora escolhida (veja em Configurações)")
	}
	if t.Copias < 1 {
		t.Copias = 1
	}
	if t.DPI <= 0 {
		t.DPI = 203
	}
	dm := devmode(nome, t.LarguraMM, t.AlturaMM)
	var pdm uintptr
	if dm != nil {
		pdm = uintptr(unsafe.Pointer(&dm[0]))
	}
	hdc, _, e := procCreateDC.Call(uintptr(unsafe.Pointer(utf16("WINSPOOL"))), uintptr(unsafe.Pointer(utf16(nome))), 0, pdm)
	if hdc == 0 {
		return fmt.Errorf("não achei a impressora %q: %v", nome, e)
	}
	defer procDeleteDC.Call(hdc)

	dpiX, _, _ := procGetDeviceCaps.Call(hdc, logPixelsX)
	dpiY, _, _ := procGetDeviceCaps.Call(hdc, logPixelsY)
	offX, _, _ := procGetDeviceCaps.Call(hdc, physicalOffsetX)
	offY, _, _ := procGetDeviceCaps.Call(hdc, physicalOffsetY)
	if dpiX == 0 || dpiY == 0 {
		dpiX, dpiY = uintptr(t.DPI), uintptr(t.DPI)
	}

	// imagem 1 bit por ponto, de baixo para cima, linhas múltiplas de 4 bytes
	b := t.Imagem.Bounds()
	w, h := b.Dx(), b.Dy()
	passo := ((w + 31) / 32) * 4
	bits := make([]byte, passo*h)
	for y := 0; y < h; y++ {
		linha := bits[(h-1-y)*passo:]
		for x := 0; x < w; x++ {
			if t.Imagem.GrayAt(b.Min.X+x, b.Min.Y+y).Y >= 128 { // branco = 1
				linha[x/8] |= 0x80 >> (x % 8)
			}
		}
	}
	bi := bitmapInfo{tamanho: 40, largura: int32(w), altura: int32(h), planos: 1, bits: 1, cores: 2, importantes: 2}
	bi.paleta[1] = [4]byte{255, 255, 255, 0}

	destW := int(math.Round(float64(w) * float64(dpiX) / float64(t.DPI)))
	destH := int(math.Round(float64(h) * float64(dpiY) / float64(t.DPI)))

	nomeDoc := t.Nome
	if nomeDoc == "" {
		nomeDoc = "Etiqueta"
	}
	di := docInfo{nome: utf16(nomeDoc)}
	if arq := os.Getenv("OFICINAQR_SAIDA_ARQUIVO"); arq != "" { // testes: "Microsoft Print to PDF" grava aqui
		di.saida = utf16(arq)
	}
	di.cb = int32(unsafe.Sizeof(di))
	if r, _, e := procStartDoc.Call(hdc, uintptr(unsafe.Pointer(&di))); int32(r) <= 0 {
		return fmt.Errorf("a impressora não aceitou o trabalho: %v", e)
	}
	for i := 0; i < t.Copias; i++ {
		if r, _, e := procStartPage.Call(hdc); int32(r) <= 0 {
			procAbortDoc.Call(hdc)
			return fmt.Errorf("erro ao começar a etiqueta: %v", e)
		}
		r, _, e := procStretchDIBits.Call(hdc, uintptr(-int32(offX)), uintptr(-int32(offY)), uintptr(destW), uintptr(destH),
			0, 0, uintptr(w), uintptr(h), uintptr(unsafe.Pointer(&bits[0])), uintptr(unsafe.Pointer(&bi)), dibRGBColors, srcCopy)
		if int32(r) == 0 || int32(r) == -1 {
			procAbortDoc.Call(hdc)
			return fmt.Errorf("erro ao desenhar a etiqueta: %v", e)
		}
		if r, _, e := procEndPage.Call(hdc); int32(r) <= 0 {
			procAbortDoc.Call(hdc)
			return fmt.Errorf("erro ao terminar a etiqueta: %v", e)
		}
	}
	if r, _, e := procEndDoc.Call(hdc); int32(r) <= 0 {
		return fmt.Errorf("erro ao fechar o trabalho: %v", e)
	}
	return nil
}

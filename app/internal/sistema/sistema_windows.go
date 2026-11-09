//go:build windows

package sistema

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	advapi32               = syscall.NewLazyDLL("advapi32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
	procRegCreateKeyEx     = advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueEx      = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValue     = advapi32.NewProc("RegDeleteValueW")
	procRegQueryValueEx    = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey        = advapi32.NewProc("RegCloseKey")
	procMessageBox         = user32.NewProc("MessageBoxW")
)

func pastaPadrao() string {
	if a := os.Getenv("APPDATA"); a != "" {
		return filepath.Join(a, "OficinaQR")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "OficinaQR")
}

type blob struct {
	n uint32
	p *byte
}

// DATA_BLOB no Windows x64 tem 16 bytes (confere ao compilar).
var _ = [1]struct{}{}[unsafe.Sizeof(blob{})-16]

func novoBlob(b []byte) *blob {
	if len(b) == 0 {
		return &blob{}
	}
	return &blob{n: uint32(len(b)), p: &b[0]}
}

func (b *blob) bytes() []byte {
	out := make([]byte, b.n)
	copy(out, unsafe.Slice(b.p, b.n))
	return out
}

// Cifrar guarda o segredo com a proteção do Windows (só este usuário lê).
func Cifrar(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	var saida blob
	r, _, e := procCryptProtectData.Call(uintptr(unsafe.Pointer(novoBlob([]byte(s)))), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&saida)))
	if r == 0 {
		return "", e
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(saida.p)))
	return "dpapi:" + base64.StdEncoding.EncodeToString(saida.bytes()), nil
}

func Decifrar(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > 4 && s[:4] == "b64:" {
		b, err := base64.StdEncoding.DecodeString(s[4:])
		return string(b), err
	}
	if len(s) < 6 || s[:6] != "dpapi:" {
		return "", errors.New("formato desconhecido")
	}
	b, err := base64.StdEncoding.DecodeString(s[6:])
	if err != nil {
		return "", err
	}
	var saida blob
	r, _, e := procCryptUnprotectData.Call(uintptr(unsafe.Pointer(novoBlob(b))), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&saida)))
	if r == 0 {
		return "", e
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(saida.p)))
	return string(saida.bytes()), nil
}

const (
	hkeyCurrentUser = 0x80000001
	keyAllAccess    = 0xF003F
	regSZ           = 1
	chaveRun        = `Software\Microsoft\Windows\CurrentVersion\Run`
	nomeRun         = "OficinaQR"
)

func abrirRun() (syscall.Handle, error) {
	var h syscall.Handle
	p, _ := syscall.UTF16PtrFromString(chaveRun)
	r, _, _ := procRegCreateKeyEx.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(p)), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&h)), 0)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return h, nil
}

// IniciarComWindows liga/desliga abrir o programa (em segundo plano) ao entrar no Windows.
func IniciarComWindows(ligar bool) error {
	h, err := abrirRun()
	if err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(h))
	nome, _ := syscall.UTF16PtrFromString(nomeRun)
	if !ligar {
		procRegDeleteValue.Call(uintptr(h), uintptr(unsafe.Pointer(nome)))
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	v, _ := syscall.UTF16FromString(`"` + exe + `" --fundo`)
	r, _, _ := procRegSetValueEx.Call(uintptr(h), uintptr(unsafe.Pointer(nome)), 0, regSZ,
		uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func IniciaComWindows() bool {
	h, err := abrirRun()
	if err != nil {
		return false
	}
	defer procRegCloseKey.Call(uintptr(h))
	nome, _ := syscall.UTF16PtrFromString(nomeRun)
	var tam uint32
	r, _, _ := procRegQueryValueEx.Call(uintptr(h), uintptr(unsafe.Pointer(nome)), 0, 0, 0, uintptr(unsafe.Pointer(&tam)))
	return r == 0
}

func achar(candidatos ...string) string {
	for _, c := range candidatos {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// AbrirJanela abre o programa numa janela própria (Edge ou Chrome em modo app).
func AbrirJanela(url string) error {
	pf86, pf, local := os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")
	nav := achar(
		filepath.Join(pf86, `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(pf, `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(pf, `Google\Chrome\Application\chrome.exe`),
		filepath.Join(pf86, `Google\Chrome\Application\chrome.exe`),
		filepath.Join(local, `Google\Chrome\Application\chrome.exe`),
	)
	if nav != "" {
		cmd := exec.Command(nav, "--app="+url, "--user-data-dir="+caminhoNavegador(),
			"--no-first-run", "--no-default-browser-check", "--disable-sync", "--window-size=1280,880")
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return nil
		}
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// AbrirNoNavegador abre um link no navegador padrão (onde a pessoa já tem login).
func AbrirNoNavegador(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// Notificar mostra um aviso no canto da tela do Windows (central de
// notificações), mesmo com o programa minimizado.
func Notificar(titulo, msg string) error {
	script := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$x = $t.GetElementsByTagName('text')
$x.Item(0).AppendChild($t.CreateTextNode($env:OQR_TITULO)) > $null
$x.Item(1).AppendChild($t.CreateTextNode($env:OQR_MSG)) > $null
$n = [Windows.UI.Notifications.ToastNotification]::new($t)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($n)`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(), "OQR_TITULO="+titulo, "OQR_MSG="+msg)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd.Run()
}

// AbrirPasta mostra a pasta no Explorador de Arquivos.
func AbrirPasta(pasta string) error {
	return exec.Command("explorer.exe", pasta).Start()
}

// Aviso mostra uma caixa de mensagem do Windows.
func Aviso(titulo, msg string) {
	t, _ := syscall.UTF16PtrFromString(titulo)
	m, _ := syscall.UTF16PtrFromString(msg)
	procMessageBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x30)
}

# Oficina QR

Sistema de **histórico veicular com QR Code** feito para a Auto Mecânica Itabaiana (Itabaiana-SE).

Desenvolvido por **José Matheus** ([@jmatheus-dev](https://github.com/jmatheus-dev)), estudante de Sistemas da Informação na UFS.

🔗 **Site no ar:** [automec135.github.io](https://automec135.github.io/?c=DEMO) (o link abre um exemplo com dados fictícios)

---

## O problema

A oficina usa um ERP de terceiros (Zeus) para estoque e finanças, mas o cliente não tinha como consultar o que já foi feito no carro, e a etiqueta de troca de óleo era escrita à mão.

## A solução

Quando um serviço é lançado no Zeus, ele vira uma etiqueta com QR Code colada no carro. Ao escanear o código, o cliente vê o histórico completo do veículo.

```
 Zeus (PostgreSQL, só leitura) ──► Oficina QR (app desktop) ──► etiqueta térmica com QR Code
                                          │
                                          └─► historico.csv (GitHub) ──► site estático ◄── cliente escaneia
```

### Aplicativo desktop: `app/` (Go, versão 1.6.1)
- Lê as vendas do ERP Zeus direto no banco PostgreSQL, com acesso somente leitura.
- Espera a venda ficar estável, junta vendas do mesmo carro no mesmo dia e manda tudo para revisão antes de imprimir. Nada sai impresso sem conferência.
- Escolhe sozinho a etiqueta certa (troca de óleo, correia dentada ou as duas juntas) e imprime direto na impressora térmica iDPRT/Teclabel iE4S.
- Publica o histórico no GitHub pela API, com token guardado cifrado.
- Também lê os PDFs de venda/orçamento do Zeus, para quando o banco não está disponível.
- Interface web local (HTML/CSS/JS embutidos no executável), com computador principal e computadores só de consulta na rede da oficina.
- Termo de responsabilidade assinado na tela para peças trazidas pelo cliente.
- Executável único, sem instalação.

### Site público: `site/`
- Página estática hospedada no GitHub Pages.
- O link do QR Code traz um código do carro, que vira um identificador **SHA-256** no navegador. O site **não guarda placa, nome nem telefone**, só modelo, ano e serviços.
- Mostra selo de revisão, garantia, plano de manutenção, lembrete na agenda e gera o histórico em PDF.

## Tecnologias
Go · PostgreSQL · HTML/CSS/JavaScript · GitHub Pages · API do GitHub · impressão térmica (GDI) · Google Apps Script · testes com Go `testing` e Python + Playwright

## Estrutura
```
app/          aplicativo desktop Oficina QR (Go)
  internal/zeus       leitura do ERP Zeus (banco e PDFs)
  internal/pdfzeus    leitor de PDF feito do zero
  internal/etiqueta   desenho das etiquetas com QR Code
  internal/impressora impressão direta na iE4S (Windows)
  internal/dados      veículos, serviços, histórico público e cifra das placas
  internal/servidor   API e interface local
  ui/                 telas do aplicativo
site/         página pública do histórico (GitHub Pages)
apps-script/  versão anterior com Google Planilhas + Apps Script
test/         testes de ponta a ponta (Python + Playwright)
```

## Como compilar
```bash
cd app
go test ./...
GOOS=windows GOARCH=amd64 go build -o OficinaQR.exe .
```
As dependências já estão em `app/vendor/`, então a compilação funciona sem internet.

> Os dados reais dos clientes **não** estão neste repositório. Placas, códigos e nomes nos testes são fictícios.

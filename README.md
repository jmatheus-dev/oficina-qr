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

### Aplicativo desktop: `programa/`
- Lê as vendas do ERP Zeus direto no banco PostgreSQL, com acesso somente leitura.
- Espera a venda ficar estável, junta vendas do mesmo carro no mesmo dia e manda tudo para revisão antes de imprimir. Nada sai impresso sem conferência.
- Escolhe sozinho a etiqueta certa (troca de óleo, correia dentada ou as duas juntas) e imprime direto na impressora térmica iDPRT/Teclabel iE4S.
- Publica o histórico no GitHub pela API, com token guardado cifrado.
- Executável único, sem instalação.

### Site público: `site/`
- Página estática hospedada no GitHub Pages.
- O link do QR Code traz um código do carro, que vira um identificador **SHA-256** no navegador. O site **não guarda placa, nome nem telefone**, só modelo, ano e serviços.
- Mostra selo de revisão, garantia, plano de manutenção, lembrete na agenda e gera o histórico em PDF.

## Tecnologias
Go · PostgreSQL · HTML/CSS/JavaScript · GitHub Pages · API do GitHub · impressão térmica (GDI)

## Estrutura
```
site/       página pública do histórico (GitHub Pages)
programa/   código-fonte do aplicativo desktop Oficina QR (em breve)
```

> Os dados reais dos clientes **não** estão neste repositório.

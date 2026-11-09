# Histórico do veículo — Auto Mecânica Itabaiana

Página aberta pelo QR Code das etiquetas.

- `index.html` lê o código do carro no link (`?c=CODIGO`), transforma o código num
  identificador (SHA-256) e procura esse identificador em duas fontes:
  1. `historico.csv` no ramo **dados** deste repositório, publicado pelo programa
     **Oficina QR** do computador da oficina;
  2. a aba **Público** da planilha antiga, publicada na Web como CSV.
- As duas fontes só têm: identificador, modelo, ano e os serviços. Sem placa,
  código, nome ou telefone.
- `?c=DEMO` mostra um exemplo com dados fictícios.

Não renomeie este repositório, a conta do GitHub nem apague o ramo `dados`:
os QR Codes já impressos apontam para `https://automec135.github.io/`.

Versão da página: 1.2 (selo de revisão, garantia, lembrete na agenda, plano de manutenção, avaliação no Google e histórico em PDF).

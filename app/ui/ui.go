// Pacote ui tem as telas do programa (HTML, CSS e JS), embutidas no .exe.
package ui

import "embed"

//go:embed index.html app.js estilo.css logo.png logo-escura.png icone.png site-index.html
var Arquivos embed.FS

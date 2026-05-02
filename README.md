# Crom MediaStream 📺

<div align="center">
  <p><strong>Uma "Estação de TV na caixa" via Terminal.</strong></p>
  <p>Crom MediaStream é uma aplicação CLI/TUI (Terminal User Interface) construída em Go que permite gerenciar e transmitir uma pasta de arquivos MP4 para plataformas como Twitch e YouTube (via RTMP) utilizando o FFmpeg de maneira inteligente, com transições em tempo real e estabilidade extrema.</p>
</div>

---

## 🚀 Visão Geral

Desenvolvido para operar em modo 24/7 de maneira *headless* (ou monitorado via Terminal), o `crom-mediastream` escaneia um diretório, mantém uma fila de vídeos e interage com o `FFmpeg` como um subprocesso para realizar ingestão direta de RTMP. Tudo isso consumindo o mínimo de CPU e garantindo que o seu streaming não sofra quedas entre as trocas de arquivo.

## 🛠 Tecnologias Principais

*   **Linguagem:** Go (Golang)
*   **Video Engine:** FFmpeg (Manipulação via `os/exec` e complex filters como `xfade`)
*   **TUI Framework:** Charm.sh Bubbletea & Lipgloss
*   **Configuração:** Viper (`config.yaml`)

## 🏗 Arquitetura do Sistema

O projeto está dividido internamente em 5 componentes principais (pacotes em `internal/`):
1.  **`engine/`**: Gerencia o subprocesso do FFmpeg, controlando os pipes de dados e as transições (`xfade`) sem derrubar a conexão RTMP.
2.  **`queue/`**: Gerencia a *playlist*, escaneia os diretórios `.mp4` dinamicamente e suporta "Manual Override" (pulos).
3.  **`ui/`**: O Dashboard TUI baseado em Bubbletea. Mostra status em tempo real, bitrate, lista de reprodução e logs de sistema.
4.  **`api/`**: Camada para futuras integrações de rede/status e atualizações dinâmicas (ex: mudar título na Twitch API).
5.  **`config/`**: Carregamento seguro de chaves de stream via arquivos YAML.

---

## ⚙️ Pré-requisitos

Para rodar ou compilar o projeto localmente, você precisa de:

*   **Go** `>= 1.20`
*   **FFmpeg** instalado no sistema e acessível via `$PATH`. *Nota: Seu binário do FFmpeg precisa ter suporte a filtros (ex: `xfade` e `testsrc` em caso de mock/testes).*

## 🚀 Como Usar e Configuração de Produção

### 1. Instalação e Preparação

Clone o repositório, crie as pastas necessárias e baixe as fontes do letreiro:

```bash
git clone https://github.com/mrjcrom/crom-mediastream.git
cd crom-mediastream
mkdir -p videos assets
wget -qO assets/Roboto.ttf https://github.com/googlefonts/roboto/raw/main/src/hinted/Roboto-Regular.ttf
```

### 2. Configuração Protegida (`config_prod.yaml`)

O arquivo `config.yaml` original na raiz do projeto é apenas um "template limpo" livre de chaves. Para rodar a stream de verdade, clone-o para criar a versão de produção (que já está protegida pelo `.gitignore`):

```bash
cp config.yaml config_prod.yaml
```

Abra o seu novo `config_prod.yaml` e configure a sua `stream_key` da Twitch e altere as opções padrão (ex: `auto_dj: true`).

### 3. Compilação e Build

Faça o download das bibliotecas do Go e gere o binário principal:

```bash
go mod tidy
go build -o crom-mediastream ./cmd/crom
```

### 4. Executando a Estação (Daemon/UI)

Graças à arquitetura avançada desacoplada, o núcleo (Daemon) e a tela (UI) funcionam em harmonia. Basta iniciar a aplicação apontando para as suas configurações através das flags `--config` e `--videos`:

```bash
./crom-mediastream --config config_prod.yaml --videos ./videos
```

> **Aviso de Tempo Real:** Se o motor estiver desligado, a tela levantará a base no plano de fundo automaticamente. Se você apertar `Q`, a interface gráfica vai fechar, **MAS A LIVE CONTINUARÁ RODANDO!** Para se reconectar à live, basta rodar o mesmo comando novamente.

---

### 5. Controles do TUI (Modo Monitor)

Quando a interface for inicializada no seu terminal, utilize as seguintes teclas:
*   `Enter`: Forçar uma transição imediata para o vídeo selecionado na lista (Aba Monitor) ou alterar a opção (Aba Settings).
*   `Tab`: Alternar entre a aba de Monitor de Live e aba de Configurações (Settings).
*   `S`: Parar/Encerrar o streaming de forma segura.
*   `R`: Forçar o scan do diretório (refresh da fila).
*   `q` ou `Ctrl+C`: Sair.

### 4. Modo Auto-DJ & Settings (Loop Ininterrupto)

A interface de terminal possui abas interativas. Ao pressionar a tecla `Tab` e acessar **SETTINGS**, você pode ativar os controladores dinâmicos:

*   **Enable Auto-DJ**: Transforma a aplicação em uma estação 24/7. O sistema lerá os arquivos via `ffprobe` e executará um *crossfade* automático entre o final do vídeo atual e o começo do próximo (em ordem alfabética).
*   **Enable Loop Mode**: Trabalha em conjunto com o Auto-DJ. Se a reprodução chegar ao último arquivo da pasta `videos/`, o loop redirecionará o fluxo para o primeiro vídeo sem derrubar a live na Twitch.

---

## 🧪 Bateria de Testes / SRE

O projeto possui uma suíte completa de *Black-Box Testing* baseada em *shell scripts* automatizados para garantir que todo o ambiente e infraestrutura estejam operando perfeitamente.

Para executar todos os testes automatizados, utilize o orquestrador:

```bash
cd tests
./run_all.sh
```

**O que o script de testes valida:**
*   **01:** Dependências do sistema (Go, FFmpeg).
*   **02:** Integridade do processo de *Build* da linguagem Go.
*   **03:** Consistência de chaves (`stream_url`, `stream_key`) no `config.yaml`.
*   **04:** Existência ou criação do diretório definido de mídia (`video_dir`).
*   **05:** Geração de um vídeo `.mp4` de Mock/Teste usando `lavfi`.
*   **06:** Validação se o FFmpeg local suporta o filtro complexo de fade (`xfade`).
*   **07:** *Smoke Test* de inicialização da interface de linha de comando.
*   **08:** *Linting* e validação estrutural (`gofmt`).
*   **09:** *Code Analysis* com `go vet` para varredura lógica e sintática.
*   **10:** Limpeza idempotente dos artefatos efêmeros do teste (*Teardown*).

Você também pode verificar o log completo das execuções em `tests/test_run.log`.

---

## 📄 Licença
Distribuído sob os termos da licença MIT. Veja o arquivo `LICENSE` para mais detalhes.
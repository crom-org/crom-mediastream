# Crom MediaStream - Guia de Produção 📺

Este guia contém as rotinas básicas para acessar, monitorar e atualizar a sua transmissão 24/7 na VPS (`root@crom.run`).

---

## 🔑 1. Como Acessar a VPS via SSH
Abra o terminal do seu computador e execute o comando:
```bash
ssh root@crom.run
```
*(Será solicitada a senha do seu usuário root da VPS).*

---

## ⚙️ 2. Como Atualizar a Stream Key (ou Configurações)
O arquivo de configuração da live fica localizado em `/root/crom-mediastream/config_prod.yaml`.

1. Acesse a VPS via SSH.
2. Abra o arquivo com o editor de texto `nano`:
   ```bash
   nano /root/crom-mediastream/config_prod.yaml
   ```
3. Navegue com as setas do teclado até a linha `stream_key:` e cole a sua chave de transmissão entre as aspas:
   ```yaml
   stream_key: "sua_nova_chave_aqui"
   ```
4. Pressione `Ctrl + O` (Enter) para salvar e `Ctrl + X` para fechar o editor.
5. Para que a alteração tenha efeito, você precisará reiniciar a live (veja a seção 4).

---

## 📁 3. Como Adicionar ou Atualizar Vídeos
A pasta onde a live busca os vídeos fica em `/root/crom-mediastream/videos/`.

*   Para enviar novos vídeos do seu computador local para a VPS, você pode usar o comando `scp` do seu terminal local:
    ```bash
    # Execute este comando no terminal local (substituindo o nome do vídeo)
    scp "/caminho/do/video.mp4" root@crom.run:/root/crom-mediastream/videos/
    ```
*   O motor do `crom-mediastream` escaneia a pasta automaticamente. No próximo ciclo de vídeos, os novos arquivos adicionados serão puxados para a grade automaticamente sem precisar reiniciar a live.

---

## 🔄 4. Como Reiniciar / Parar a Live
Se você alterou a Stream Key ou deseja reiniciar a transmissão por qualquer motivo:

1. Acesse a VPS via SSH.
2. Execute o comando para parar o processo atual:
   ```bash
   pkill -f crom-mediastream
   ```
3. Para iniciar a transmissão novamente em segundo plano (modo daemon):
   ```bash
   cd /root/crom-mediastream
   nohup ./crom-mediastream daemon > daemon.log 2>&1 &
   ```

---

## 📊 5. Como Monitorar a Transmissão (Interface de Terminal TUI)
O projeto possui uma interface interativa que você pode abrir a qualquer momento para ver o status da live, a fila de vídeos ou alterar opções (como ativar/desativar chat ou letreiro).

### Se a live já estiver ligada e rodando na VPS:
Você pode abrir a interface de monitoramento com segurança. O sistema detectará que o processo (daemon) já está ativo e **apenas se conectará a ele**, sem reiniciar a transmissão ou interromper a live.

1. Acesse a VPS via SSH.
2. Execute o comando de monitoramento:
   ```bash
   cd /root/crom-mediastream
   ./crom-mediastream
   ```
3. A tela TUI abrirá exibindo o status em tempo real da transmissão atual.
4. **Para sair do monitor sem desligar a live**: Pressione `q` ou `Ctrl + C`. A interface se fechará, mas a live continuará transmitindo normalmente em segundo plano na VPS.

### Controles Úteis na Interface:
*   `Tab`: Alterna entre a aba **MONITOR** (fila de vídeos) e **SETTINGS** (letreiro, resolução, etc).
*   `S`: Encerra o streaming e a live completamente (para se quiser desligar tudo).
*   `Enter`: Executa ações nos itens selecionados.



---

## 🪵 6. Verificando Logs em caso de Problemas
Se a live não estiver subindo na Twitch/YouTube, você pode ver o log de execução do Daemon ou do FFmpeg:

*   **Log do Daemon** (Status da fila e inicialização):
    ```bash
    cat /root/crom-mediastream/daemon.log
    ```
*   **Log do FFmpeg** (Erros de codecs ou conexão com a Twitch):
    ```bash
    cat /root/crom-mediastream/ffmpeg_master.log
    cat /root/crom-mediastream/ffmpeg_encoder.log
    ```

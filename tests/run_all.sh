#!/bin/bash
# run_all.sh - Orquestrador de testes
# Executa todos os scripts na pasta tests e gera um relatório

LOG_FILE="test_run.log"
echo "=== INICIANDO BATERIA DE TESTES CROM-MEDIASTREAM ===" > "$LOG_FILE"
date >> "$LOG_FILE"
echo "----------------------------------------------------" >> "$LOG_FILE"

PASS_COUNT=0
FAIL_COUNT=0

# Pega todos os scripts de teste (01 a 10)
TEST_SCRIPTS=$(ls [0-9][0-9]_*.sh | sort)

for script in $TEST_SCRIPTS; do
    echo ">> Executando $script..." | tee -a "$LOG_FILE"
    
    # Executa o script e redireciona stdout e stderr
    # O script roda num subshell, então os `cd ..` dentro dele não afetam este orquestrador
    bash "$script" 2>&1 | tee -a "$LOG_FILE"
    RET=${PIPESTATUS[0]}
    
    if [ $RET -eq 0 ]; then
        echo "[ OK ] $script passou." | tee -a "$LOG_FILE"
        ((PASS_COUNT++))
    else
        echo "[ FALHA ] $script retornou erro ($RET)." | tee -a "$LOG_FILE"
        ((FAIL_COUNT++))
    fi
    echo "----------------------------------------------------" >> "$LOG_FILE"
done

echo "" | tee -a "$LOG_FILE"
echo "=== RESUMO DA EXECUÇÃO ===" | tee -a "$LOG_FILE"
echo "Testes com sucesso: $PASS_COUNT" | tee -a "$LOG_FILE"
echo "Testes com falha  : $FAIL_COUNT" | tee -a "$LOG_FILE"

if [ $FAIL_COUNT -gt 0 ]; then
    echo "RESULTADO: FALHA. Verifique $LOG_FILE para mais detalhes." | tee -a "$LOG_FILE"
    exit 1
else
    echo "RESULTADO: SUCESSO ABSOLUTO!" | tee -a "$LOG_FILE"
    exit 0
fi

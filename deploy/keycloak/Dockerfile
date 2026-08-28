# ==========================================================
# 🔑 KEYCLOAK do StudFy — enxuto o bastante para 512 MB
# ==========================================================
# Duas coisas derrubavam este contêiner no plano free:
#
# 1) MEMÓRIA (saía com status 137 = morto por falta de RAM).
#    O Keycloak se auto-compila ao ligar ("installing your custom providers"),
#    e isso não cabe em 512 MB. Aqui essa compilação é feita no BUILD, onde há
#    folga, e o contêiner sobe já pronto com `--optimized`.
#
# 2) PORTA ("No open ports detected"). O Keycloak abre a 8080, mas a nuvem
#    manda a porta na variável PORT e fica esperando ali. Agora ele escuta
#    exatamente onde mandarem.

# ---------- etapa 1: compila (com folga de recursos) ----------
FROM quay.io/keycloak/keycloak:26.0 AS builder

ENV KC_DB=dev-file
ENV KC_HEALTH_ENABLED=true
# Sem cluster: é uma instância só. O modo cluster tenta parear com os
# contêineres vizinhos da nuvem e trava o boot com erros de JGroups/NioServer.
ENV KC_CACHE=local

RUN /opt/keycloak/bin/kc.sh build

# ---------- etapa 2: a imagem que roda ----------
FROM quay.io/keycloak/keycloak:26.0

COPY --from=builder /opt/keycloak/ /opt/keycloak/
COPY realm-studfy.json /opt/keycloak/data/import/realm-studfy.json

ENV KC_DB=dev-file
ENV KC_CACHE=local
ENV KC_HEALTH_ENABLED=true
ENV KC_HTTP_ENABLED=true
ENV KC_HOSTNAME_STRICT=false
# Atrás do proxy da nuvem o TLS termina antes de chegar aqui; sem isto o
# Keycloak monta as URLs com http e o login entra em loop de redirecionamento.
ENV KC_PROXY_HEADERS=xforwarded

# Teto de memória da JVM. Sem isto o Java tenta usar a máquina inteira,
# estoura os 512 MB do plano free e o contêiner é morto (status 137).
ENV JAVA_OPTS_APPEND="-Xms64m -Xmx320m -XX:MaxMetaspaceSize=128m"

# Forma shell (e não exec) de propósito: precisamos expandir ${PORT}, que a
# nuvem define em tempo de execução. Local, sem PORT, cai para 8080.
ENTRYPOINT ["/bin/sh", "-c", "exec /opt/keycloak/bin/kc.sh start --optimized --import-realm --http-port=${PORT:-8080}"]

# ==========================================================
# 🔑 KEYCLOAK do StudFy — já sobe com o realm configurado
# ==========================================================
# O realm-studfy.json é importado no primeiro boot: cria o realm, o cliente
# do front (studfy-app), os cargos (user/teacher/admin) e os usuários de teste.
# Assim, no minuto em que o serviço subir, já dá para logar e testar.
FROM quay.io/keycloak/keycloak:26.0

COPY realm-studfy.json /opt/keycloak/data/import/realm-studfy.json

# Em nuvem o proxy termina o TLS; sem isto o Keycloak monta as URLs erradas
# (http em vez de https) e o login entra em loop de redirecionamento.
ENV KC_PROXY_HEADERS=xforwarded
ENV KC_HTTP_ENABLED=true
ENV KC_HEALTH_ENABLED=true

ENTRYPOINT ["/opt/keycloak/bin/kc.sh", "start", "--import-realm", "--optimized=false"]

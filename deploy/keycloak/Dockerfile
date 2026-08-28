# ==========================================================
# 🔑 KEYCLOAK do StudFy — já sobe com o realm configurado
# ==========================================================
# O realm-studfy.json é importado no primeiro boot: cria o realm, o cliente
# do front (studfy-app), os cargos (user/teacher/admin) e os usuários de teste.
# Assim, no minuto em que o serviço subir, já dá para logar e testar.
FROM quay.io/keycloak/keycloak:26.0

COPY realm-studfy.json /opt/keycloak/data/import/realm-studfy.json

# Atrás do proxy da nuvem o TLS termina antes de chegar aqui. Sem isto o
# Keycloak monta as URLs com http e o login entra em loop de redirecionamento.
ENV KC_PROXY_HEADERS=xforwarded
ENV KC_HTTP_ENABLED=true
ENV KC_HEALTH_ENABLED=true
ENV KC_HOSTNAME_STRICT=false

# ⚠️ `start-dev` e NÃO `start`.
# O modo `start` (produção) liga o cluster do Infinispan/JGroups: numa rede
# compartilhada de nuvem ele tenta parear com containers vizinhos e enche o log
# de "NioServer ... cookie does not match own cookie", sem nunca subir.
# `start-dev` usa cache local, aceita o banco embutido e é o suficiente para
# uma instância única — que é o caso aqui (plano free, um contêiner só).
ENTRYPOINT ["/opt/keycloak/bin/kc.sh", "start-dev", "--import-realm"]

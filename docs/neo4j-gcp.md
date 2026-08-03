# Runbook Neo4j en GCP (AVLP Ola 7)

Fuente de verdad del currículum: **Git** (`data/knowledge/curriculum.json`),
sincronizado a Neo4j con `cmd/graph-sync`. El router **solo lee**
(`pkg/knowledge/neo4jgraph`); nunca escribe.

## LO EDITADO A MANO EN NEO4J BROWSER LO PISA EL PRÓXIMO SYNC — LA FUENTE DE VERDAD ES GIT

Cualquier `MERGE`/`DELETE` hecho a mano en Browser o cypher-shell desaparece (o
queda inconsistente) en la próxima corrida de `graph-sync`. Curá el JSON en el
repo, abrí PR, y sincronizá.

---

## 1. VM as-built (São Paulo)

Estado real del despliegue lab (sync inaugural: 20 conceptos, 20 aristas):

| Ítem | Valor |
|------|--------|
| Nombre | `avlp-neo4j` |
| Zona | `southamerica-east1-a` |
| Machine | `e2-small` |
| OS | Debian 13 (trixie) |
| Neo4j | Docker `neo4j:2026.06.0` (pinneado) |
| Contenedor | `neo4j-vectorial`, volumen nombrado `neo4j-data`, `--restart unless-stopped` |
| Bolt | `127.0.0.1:7687` en el host |
| Red / tag | `avlp-neo4j` |
| IP externa | **ninguna** (VM sellada) |

### Sellado sin IP externa

```bash
gcloud compute instances delete-access-config avlp-neo4j \
  --zone=southamerica-east1-a \
  --access-config-name="External NAT"
```

El acceso **SSH-over-IAP no depende** de esa IP: IAP llega por el plano de control
de GCP. Tras el sellado seguís entrando con `--tunnel-through-iap` (sección 3).

Para ahorrar: **instance schedule** (apagado fuera de horario lab).

---

## 2. Instalación Neo4j (Docker as-built)

Imagen pinneada, Bolt solo en loopback del host, volumen persistente,
reinicio automático:

```bash
sudo docker run -d --name neo4j-vectorial \
  --restart unless-stopped \
  -p 127.0.0.1:7687:7687 \
  -e NEO4J_AUTH=neo4j/"$NEO4J_PASSWORD_PRIMER_ARRANQUE" \
  -v neo4j-data:/data \
  neo4j:2026.06.0
```

`-p 127.0.0.1:7687:7687` evita exposición pública. Verificá:
`ss -lntp | grep 7687` → `127.0.0.1:7687`.

`NEO4J_AUTH` vale solo para el **primer arranque** del volumen: rotá de inmediato
con la doctrina de la sección 4 (no dejes esa clave como permanente).

### APT (alternativa, no es el as-built)

Si en otra VM preferís paquete: `server.bolt.listen_address=127.0.0.1:7687` y
`server.http.enabled=false`. Misma regla de loopback.

---

## 3. Acceso: túnel SSH sobre IAP (no IAP TCP al 7687)

### Por qué no `start-iap-tunnel` directo al Bolt

`gcloud compute start-iap-tunnel … 7687` habla con el puerto **desde el
agente IAP hacia la VM**. Si Neo4j está publicado solo como `127.0.0.1:7687`,
el forwarding TCP de IAP **no alcanza** ese listener y falla con error **4003**
(verificado en vivo). No abras Bolt a `0.0.0.0` solo para hacer feliz al túnel TCP.

### Vía correcta: SSH over IAP + LocalForward

```bash
gcloud compute ssh avlp-neo4j \
  --zone=southamerica-east1-a \
  --tunnel-through-iap \
  -- -L 7687:localhost:7687 -N
```

Desde la laptop: `bolt://127.0.0.1:7687`.

Firewall (SSH vía IAP, no Bolt público), target tag `avlp-neo4j`:

```bash
gcloud compute firewall-rules create allow-iap-ssh \
  --direction=INGRESS \
  --action=ALLOW \
  --rules=tcp:22 \
  --source-ranges=35.235.240.0/20 \
  --target-tags=avlp-neo4j
```

**No** abras `0.0.0.0/0` a 7687/7474.

### Alternativa: Tailscale

Misma regla: **sin** puerto público en GCP. Si Neo4j solo escucha en loopback,
necesitás forwarder en la VM o publicar Bolt en la IP Tailscale de forma controlada.

---

## 4. Credenciales — SIEMPRE fuera del repo

Nunca commitees passwords ni pongas secretos Neo4j en el árbol Git.

En la laptop (p. ej. `~/.config/avlp/env.sh`, gitignoreado / fuera del repo):

```bash
export AVLP_NEO4J_URI=bolt://127.0.0.1:7687
export AVLP_NEO4J_USER=neo4j
export AVLP_NEO4J_PASSWORD='…placeholder — nunca un valor real en docs…'
```

### Rotación de claves sin manos

Reglas de oro (en este orden):

1. La clave **NUNCA** viaja en argumentos de comandos remotos: `sudo` loguea la
   línea completa en el journal de la VM. Tampoco se tipéa ni se pega a mano en
   la sesión SSH.
2. Se genera directo a variable y viaja por **stdin**; `env.sh` se actualiza
   **solo si** el servidor aceptó el cambio (`&&`).

Bloque canónico (macOS; `sed -i ''` es sintaxis BSD). `LA_CLAVE_ACTUAL` es
**placeholder** — jamás valores reales en el repo, ni como ejemplo:

```bash
NUEVA=$(openssl rand -base64 24)
echo "ALTER CURRENT USER SET PASSWORD FROM 'LA_CLAVE_ACTUAL' TO '$NUEVA';" \
  | gcloud compute ssh avlp-neo4j \
      --zone=southamerica-east1-a \
      --tunnel-through-iap \
      --command="sudo docker exec -i neo4j-vectorial cypher-shell -u neo4j -p 'LA_CLAVE_ACTUAL'" \
  && sed -i '' "s|^export AVLP_NEO4J_PASSWORD=.*|export AVLP_NEO4J_PASSWORD='$NUEVA'|" ~/.config/avlp/env.sh \
  && echo "ROTACION OK"
```

Tras `ROTACION OK`, recargá el env (`source ~/.config/avlp/env.sh`) y verificá
con `graph-sync` (sección 8).

---

## 5. Sincronización (`cmd/graph-sync`)

Valida el archivo con el **mismo** `knowledge.LoadFile` del router **antes**
de tocar la base (ciclo / referencia rota → aborta).

`graph-sync` ejecuta el **constraint en su propia query** (auto-commit) y
después cada lote de datos (`MERGE` conceptos, y un `ExecuteWrite` por tipo de
relación, más prune) en transacciones separadas. Mezclar schema + datos en la
misma tx falla en Neo4j reciente con
`Neo.ClientError.Transaction.ForbiddenDueToTransactionType`.

```bash
# Plan sin escribir
go run ./cmd/graph-sync -dry-run -validate-seeds

# Sync aditivo (default)
go run ./cmd/graph-sync

# Reconciliar borrados respecto del archivo
go run ./cmd/graph-sync -prune
```

Flags:

| Flag | Efecto |
|------|--------|
| `-dry-run` | Imprime plan; **cero** escrituras |
| `-prune` | Borra conceptos/aristas ausentes del JSON (default: aditivo) |
| `-validate-seeds` | Reporta `concepts` en seeds que no existen en el currículum |
| `-curriculum` | Ruta al JSON (default `data/knowledge/curriculum.json`) |

---

## 6. Dump nocturno opcional → GCS

```bash
# Adaptá paths al volumen Docker `neo4j-data`.
neo4j-admin database dump neo4j --to-path=/var/backups/neo4j
gsutil cp /var/backups/neo4j/*.dump gs://$GCS_BUCKET/neo4j/$(date -u +%Y%m%d)/
```

El dump **no** reemplaza a Git como fuente de verdad del currículum.

---

## 7. Costos (orden de magnitud)

| Palanca | Notas |
|---------|--------|
| `e2-small` (as-built) | Suficiente para lab; subí si el sync/lecturas lo piden |
| **Instance schedule** | Apagar noches/fines de semana → mayor ahorro |
| Sin IP pública + Bolt en loopback | Reduce superficie; sin balanceadores |

---

## 8. Verificación rápida

```bash
# Router sin Neo4j (CI / default)
unset AVLP_NEO4J_URI
go run ./cmd/router   # MemoryGraph archivo

# Router / sync con read-through (túnel SSH/IAP arriba)
source ~/.config/avlp/env.sh
go run ./cmd/graph-sync
```

**Idempotencia (prueba operativa estándar):** correr `graph-sync` **dos veces
seguidas** debe dar salida idéntica del estilo
`sync ok: conceptos=N aristas=M` (p. ej. N=20, M=20 en el inaugural). El
`MERGE` no debe crear duplicados ni cambiar el conteo.

### Troubleshooting de auth

| Síntoma | Causa | Remedio |
|---------|--------|---------|
| `graph-sync` → `Neo.ClientError.Security.Unauthorized` | Clave en `env.sh` desincronizada del servidor | Rotación sin manos (sección 4); no reintentar a ciegas |
| `cypher-shell` → `42NFF` permission/access denied | Candado anti fuerza bruta de Neo4j tras fallidos seguidos | Esperar unos segundos; reintentar con la clave **correcta**. **No** confundir con `Unauthorized` |
| Contenedor recreado con volumen **nuevo** | Clave vuelve a lo que diga `NEO4J_AUTH` | Tratar `NEO4J_AUTH` como valor de **primer arranque** solamente; rotar de inmediato (sección 4) |

### Tests de integración (`RUN_NEO4J_INTEGRATION`) — ESCRIBEN Y PUEDEN PODAR

`TestParityMemoryGraphVsNeo4j` sincroniza el fixture con **prune** y por tanto
puede borrar nodos/aristas que no estén en `curriculum.json`.

**Jamás** apuntes `AVLP_NEO4J_URI` a la base real del currículum. Usá un
contenedor local efímero:

```bash
docker run --rm -d --name avlp-neo4j-it \
  -p 127.0.0.1:17687:7687 \
  -e NEO4J_AUTH=neo4j/test-it-only \
  neo4j:2026.06.0

export AVLP_NEO4J_URI=bolt://127.0.0.1:17687
export AVLP_NEO4J_USER=neo4j
export AVLP_NEO4J_PASSWORD=test-it-only
export RUN_NEO4J_INTEGRATION=1
export AVLP_NEO4J_ALLOW_INTEGRATION_WRITE=1   # confirmación explícita

go test ./cmd/graph-sync/ -run Parity -count=1 -v
docker stop avlp-neo4j-it
```

Sin `AVLP_NEO4J_ALLOW_INTEGRATION_WRITE=1` el test **falla a propósito** para
no escribir por accidente.

Si Bolt cae en runtime, el Advisor/orientación degrada al último currículum en
Git vía `MemoryGraph`; **el ruteo k-NN no usa Neo4j**.

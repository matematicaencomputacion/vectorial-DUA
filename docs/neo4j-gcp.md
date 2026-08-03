# Runbook Neo4j en GCP (AVLP Ola 7)

Fuente de verdad del currículum: **Git** (`data/knowledge/curriculum.json`),
sincronizado a Neo4j con `cmd/graph-sync`. El router **solo lee**
(`pkg/knowledge/neo4jgraph`); nunca escribe.

## LO EDITADO A MANO EN NEO4J BROWSER LO PISA EL PRÓXIMO SYNC — LA FUENTE DE VERDAD ES GIT

Cualquier `MERGE`/`DELETE` hecho a mano en Browser o cypher-shell desaparece (o
queda inconsistente) en la próxima corrida de `graph-sync`. Curá el JSON en el
repo, abrí PR, y sincronizá.

---

## 1. VM recomendada

| Ítem | Valor |
|------|--------|
| Región | `southamerica-east1` (São Paulo) |
| Machine | `e2-medium` (lab / carga baja: `e2-small`) |
| Disco | 20–40 GB SSD |
| OS | Debian 12 / Ubuntu 22.04 LTS |
| Neo4j | **Community** en Docker (producción lab AVLP) o APT |

Creación típica:

```bash
gcloud compute instances create avlp-neo4j \
  --project="$GCP_PROJECT" \
  --zone=southamerica-east1-a \
  --machine-type=e2-medium \
  --image-family=debian-12 \
  --image-project=debian-cloud \
  --boot-disk-size=30GB \
  --scopes=cloud-platform \
  --tags=neo4j-iap
```

Para ahorrar: **instance schedule** (apagado fuera de horario lab) — es la
palanca de costo más efectiva en Community single-node.

---

## 2. Instalación Neo4j Community

### 2.a Docker (vía en producción lab)

Imagen pinneada, Bolt **solo en loopback del host**, volumen persistente,
reinicio automático:

```bash
sudo mkdir -p /var/lib/neo4j/data
# Pinneá un digest o tag concreto (ej. neo4j:2025.x.x / community).
# Sustituí $NEO4J_IMAGE por la imagen verificada del despliegue.
sudo docker run -d --name avlp-neo4j \
  --restart unless-stopped \
  -p 127.0.0.1:7687:7687 \
  -e NEO4J_AUTH=neo4j/"$NEO4J_PASSWORD" \
  -e NEO4J_server_default__listen__address=0.0.0.0 \
  -v /var/lib/neo4j/data:/data \
  "$NEO4J_IMAGE"
```

`-p 127.0.0.1:7687:7687` evita exposición pública aunque el contenedor escuche
en `0.0.0.0` dentro de la red Docker. Verificá:
`ss -lntp | grep 7687` → debe listar `127.0.0.1:7687`.

### 2.b APT (alternativa)

Seguí la documentación oficial de Neo4j para añadir el repo APT. En
`/etc/neo4j/neo4j.conf`:

```properties
server.default_listen_address=127.0.0.1
server.bolt.listen_address=127.0.0.1:7687
server.http.enabled=false
```

Reiniciá (`sudo systemctl restart neo4j`). Bolt queda **solo en localhost**.

---

## 3. Acceso: túnel SSH sobre IAP (no IAP TCP al 7687)

### Por qué no `start-iap-tunnel` directo al Bolt

`gcloud compute start-iap-tunnel … 7687` habla con el puerto **desde el
agente IAP hacia la VM**. Si Neo4j está bindeado a `127.0.0.1` en el host
(APT) o publicado solo como `127.0.0.1:7687` en Docker, el forwarding TCP de
IAP **no alcanza** ese listener y falla con error **4003** (verificado en vivo
en São Paulo). No abras Bolt a `0.0.0.0` solo para hacer feliz al túnel TCP.

### Vía correcta: SSH over IAP + LocalForward

```bash
gcloud compute ssh avlp-neo4j \
  --project="$GCP_PROJECT" \
  --zone=southamerica-east1-a \
  --tunnel-through-iap \
  -- -L 7687:localhost:7687 -N
```

Desde la laptop: `bolt://127.0.0.1:7687` (mismo `AVLP_NEO4J_URI` que abajo).

Firewall (SSH vía IAP, no Bolt público):

```bash
gcloud compute firewall-rules create allow-iap-ssh \
  --project="$GCP_PROJECT" \
  --direction=INGRESS \
  --action=ALLOW \
  --rules=tcp:22 \
  --source-ranges=35.235.240.0/20 \
  --target-tags=neo4j-iap
```

**No** abras `0.0.0.0/0` a 7687/7474.

### Alternativa: Tailscale

Si el equipo ya usa Tailscale, uní la VM al tailnet y hablá a
`bolt://100.x.y.z:7687` sin IAP. Misma regla: **sin** puerto público en GCP.
Si Neo4j solo escucha en loopback, necesitás un forwarder en la VM o publicar
Bolt en la IP Tailscale de forma controlada.

---

## 4. Credenciales — SIEMPRE fuera del repo

Nunca commitees passwords ni `data/avlp.json` con secretos Neo4j.

```bash
export AVLP_NEO4J_URI=bolt://127.0.0.1:7687
export AVLP_NEO4J_USER=neo4j
export AVLP_NEO4J_PASSWORD='…desde secret manager…'
```

Cambio de password inicial: solo por consola en la VM (`cypher-shell` por
SSH/IAP o `docker exec`), nunca en el árbol Git.

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
# Ejemplo con Neo4j en el host; adaptá paths si usás Docker volumes.
neo4j-admin database dump neo4j --to-path=/var/backups/neo4j
gsutil cp /var/backups/neo4j/*.dump gs://$GCS_BUCKET/neo4j/$(date -u +%Y%m%d)/
```

Retención: lifecycle rule en el bucket (p. ej. 14–30 días). El dump **no**
reemplaza a Git como fuente de verdad del currículum.

---

## 7. Costos (orden de magnitud)

| Palanca | Notas |
|---------|--------|
| `e2-small` vs `e2-medium` | Suficiente para lab; subí si el sync/lecturas lo piden |
| **Instance schedule** | Apagar noches/fines de semana en lab → suele ser el mayor ahorro |
| Disco SSD tamaño mínimo | 20–30 GB suele alcanzar Community + dumps locales rotados |
| Sin IP pública + sin HTTP | Reduce superficie; no suma costo de balanceadores |

---

## 8. Verificación rápida

```bash
# Router sin Neo4j (CI / default)
unset AVLP_NEO4J_URI
go run ./cmd/router   # MemoryGraph archivo

# Router con read-through (túnel SSH/IAP arriba)
export AVLP_NEO4J_URI=bolt://127.0.0.1:7687
# USER/PASSWORD…
go run ./cmd/router
```

### Tests de integración (`RUN_NEO4J_INTEGRATION`) — ESCRIBEN Y PUEDEN PODAR

`TestParityMemoryGraphVsNeo4j` sincroniza el fixture con **prune** y por tanto
puede borrar nodos/aristas que no estén en `curriculum.json`.

**Jamás** apuntes `AVLP_NEO4J_URI` a la base real del currículum. Usá un
contenedor local efímero:

```bash
docker run --rm -d --name avlp-neo4j-it \
  -p 127.0.0.1:17687:7687 \
  -e NEO4J_AUTH=neo4j/test-it-only \
  neo4j:5  # o la misma imagen pinneada del lab

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

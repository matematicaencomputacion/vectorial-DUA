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
| Neo4j | **Community** vía APT oficial |

Creación típica:

```bash
gcloud compute instances create avlp-neo4j \
  --project="$GCP_PROJECT" \
  --zone=southamerica-east1-a \
  --machine-type=e2-medium \
  --image-family=debian-12 \
  --image-project=debian-cloud \
  --boot-disk-size=30GB \
  --scopes=cloud-platform
```

Para ahorrar: **instance schedule** (apagado fuera de horario lab) — es la
palanca de costo más efectiva en Community single-node.

---

## 2. Instalación Neo4j Community (APT)

Seguí la documentación oficial de Neo4j para añadir el repo APT. Ejemplo
orientativo (verificá la URL/firma vigente en docs.neo4j.com):

```bash
sudo apt-get update && sudo apt-get install -y wget gnupg
# … añadir firma + list file del repo Neo4j …
sudo apt-get update
sudo apt-get install -y neo4j
```

### Escucha solo loopback + HTTP apagado

En `/etc/neo4j/neo4j.conf` (o drop-in):

```properties
server.default_listen_address=127.0.0.1
server.bolt.listen_address=127.0.0.1:7687
server.http.enabled=false
# no habilitar HTTPS público
```

Reiniciá el servicio (`sudo systemctl restart neo4j`). Bolt queda **solo en
localhost** de la VM.

---

## 3. Acceso: IAP TCP forwarding (cero puertos públicos)

### Firewall: solo rango IAP

```bash
gcloud compute firewall-rules create allow-iap-neo4j-bolt \
  --project="$GCP_PROJECT" \
  --direction=INGRESS \
  --action=ALLOW \
  --rules=tcp:7687 \
  --source-ranges=35.235.240.0/20 \
  --target-tags=neo4j-iap
```

Etiquetá la VM con `neo4j-iap`. **No** abras `0.0.0.0/0` a 7687/7474.

### Túnel desde la laptop

```bash
gcloud compute start-iap-tunnel avlp-neo4j 7687 \
  --local-host-port=localhost:7687 \
  --zone=southamerica-east1-a \
  --project="$GCP_PROJECT"
```

### Alternativa: Tailscale

Si el equipo ya usa Tailscale, uní la VM al tailnet y hablá a
`bolt://100.x.y.z:7687` sin IAP. Misma regla: **sin** puerto público en GCP.

---

## 4. Credenciales — SIEMPRE fuera del repo

Nunca commitees passwords ni `data/avlp.json` con secretos Neo4j.

En la laptop / CI de sync (Secret Manager, `.env` local gitignoreado, o
`direnv`):

```bash
export AVLP_NEO4J_URI=bolt://127.0.0.1:7687
export AVLP_NEO4J_USER=neo4j
export AVLP_NEO4J_PASSWORD='…desde secret manager…'
```

Cambio de password inicial de Neo4j: solo por consola en la VM
(`cypher-shell` por SSH/IAP), nunca en el árbol Git.

---

## 5. Sincronización (`cmd/graph-sync`)

Valida el archivo con el **mismo** `knowledge.LoadFile` del router **antes**
de tocar la base (ciclo / referencia rota → aborta).

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

Constraints idempotentes (`concept_id_unique`), `MERGE` por lotes **un
statement por tipo de relación** (Cypher no permite tipo dinámico sin APOC),
marca `synced_at` por corrida.

---

## 6. Dump nocturno opcional → GCS

```bash
# En la VM, cron/systemd timer (ejemplo)
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

Revisá la calculadora de GCP de tu proyecto; los números cambian por
descuento/comprometido.

---

## 8. Verificación rápida

```bash
# Router sin Neo4j (CI / default)
unset AVLP_NEO4J_URI
go run ./cmd/router   # MemoryGraph archivo

# Router con read-through (túnel IAP arriba)
export AVLP_NEO4J_URI=bolt://127.0.0.1:7687
# USER/PASSWORD…
go run ./cmd/router

# Paridad (lab)
RUN_NEO4J_INTEGRATION=1 go test ./cmd/graph-sync/ -run Parity -count=1 -v
```

Si Bolt cae, el Advisor/orientación degrada al último currículum en Git vía
`MemoryGraph`; **el ruteo k-NN no usa Neo4j**.

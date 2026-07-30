## 1. Configuración calibrada

- [x] 1.1 Implementar config versionada con escritura atómica y precedencia env > archivo > default
- [x] 1.2 Agregar `calibrate --apply`, captura del umbral y log de valor/origen en router
- [x] 1.3 Cubrir lectura, escritura, fallback y precedencia con tests

## 2. Normalización simétrica

- [x] 2.1 Implementar normalización Unicode en el límite de ambos embedders
- [x] 2.2 Aplicarla a queries, descriptors y chunks mediante el choke point común
- [x] 2.3 Verificar `variables y escopes` contra el nodo Variables y Scope y la fuente `.env`

## 3. Simmatrix RAG

- [x] 3.1 Exponer snapshots seguros de chunks y construir matriz query×chunk
- [x] 3.2 Calcular piso sugerido, margen y warning con casos on/off-topic
- [x] 3.3 Emitir `simmatrix_rag.json` y resumen humano junto a la matriz de nodos

## 4. Documentación y cierre

- [x] 4.1 Documentar flags, precedencia, normalización y lectura del solapamiento hash
- [x] 4.2 Ejecutar gofmt, build, vet, OpenSpec estricto y `scripts/test-clean.sh`
- [ ] 4.3 Publicar PR 8.6 y confirmar CI verde
